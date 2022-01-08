package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	dinner "github.com/damoon/d8s/pkg"
	"github.com/urfave/cli/v2"
)

var (
	gitHash string
	gitRef  string
)

func main() {
	app := &cli.App{
		Name:   "Dinner",
		Usage:  "Proxy to minimize data transfer volume of docker build contexts.",
		Action: run,
		Commands: []*cli.Command{
			{
				Name:  "server",
				Usage: "Start the server.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "addr", Value: ":2375", Usage: "Address to run service on."},
					&cli.StringFlag{Name: "s3-endpoint", Required: true, Usage: "s3 endpoint."},
					&cli.StringFlag{Name: "s3-access-key-file", Required: true, Usage: "Path to s3 access key."},
					&cli.StringFlag{Name: "s3-secret-key-file", Required: true, Usage: "Path to s3 secret access key."},
					&cli.BoolFlag{Name: "s3-ssl", Value: true, Usage: "s3 uses SSL."},
					&cli.StringFlag{Name: "s3-location", Value: "us-east-1", Usage: "s3 bucket location."},
					&cli.StringFlag{Name: "s3-bucket", Required: true, Usage: "s3 bucket name."},
					&cli.StringFlag{Name: "upstream", Required: true, Usage: "Upstream to forward requests to."},
				},
				Action: run,
			},
			{
				Name:  "version",
				Usage: "Show the version",
				Action: func(c *cli.Context) error {
					_, err := os.Stdout.WriteString(fmt.Sprintf("version: %s\ngit commit: %s", gitRef, gitHash))
					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(c *cli.Context) error {
	log.Printf("version: %v", gitRef)
	log.Printf("git commit: %v", gitHash)

	log.Println("set up storage")

	storage, err := setupObjectStore(
		c.String("s3-endpoint"),
		c.String("s3-access-key-file"),
		c.String("s3-secret-key-file"),
		c.Bool("s3-ssl"),
		c.String("s3-location"),
		c.String("s3-bucket"))
	if err != nil {
		return fmt.Errorf("setup minio s3 client: %v", err)
	}

	log.Println("set up upstream proxy")
	upstream, err := setupUpstreamProxy(c.String("upstream"))
	if err != nil {
		return fmt.Errorf("setup upstream proxy: %v", err)
	}

	log.Println("set up service")

	svc := dinner.NewService(gitHash, gitRef, storage, upstream)

	svcServer := httpServer(svc, c.String("addr"))

	log.Println("starting server")

	go mustListenAndServe(svcServer)

	log.Println("running")

	awaitShutdown()

	ctx, cancel := context.WithTimeout(context.Background(), dinner.MaxExecutionTime)
	defer cancel()

	err = shutdown(ctx, svcServer)
	if err != nil {
		return fmt.Errorf("shutdown service server: %v", err)
	}

	log.Println("shutdown complete")

	return nil
}

func setupObjectStore(
	endpoint, accessKeyPath, secretKeyPath string,
	useSSL bool,
	region, bucket string,
) (*dinner.ObjectStore, error) {
	accessKeyBytes, err := ioutil.ReadFile(accessKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading secret access key from %s: %v", accessKeyPath, err)
	}

	secretKeyBytes, err := ioutil.ReadFile(secretKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading secret access key from %s: %v", secretKeyPath, err)
	}

	accessKey := strings.TrimSpace(string(accessKeyBytes))
	secretKey := strings.TrimSpace(string(secretKeyBytes))

	endpointProtocol := "http"
	if useSSL {
		endpointProtocol = "https"
	}

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(fmt.Sprintf("%s://%s", endpointProtocol, endpoint)),
		Region:           aws.String(region),
		DisableSSL:       aws.Bool(!useSSL),
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return nil, fmt.Errorf("set up aws session: %v", err)
	}

	s3Client := s3.New(sess)

	return &dinner.ObjectStore{
		Client:   s3Client,
		Uploader: s3manager.NewUploader(sess),
		Bucket:   bucket,
	}, nil
}

func setupUpstreamProxy(upstream string) (*httputil.ReverseProxy, error) {
	upstreamURL, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(upstreamURL), nil
}

func httpServer(h http.Handler, addr string) *http.Server {
	httpServer := &http.Server{
		ReadTimeout:  dinner.MaxExecutionTime,
		WriteTimeout: dinner.MaxExecutionTime,
	}
	httpServer.Addr = addr
	httpServer.Handler = h

	return httpServer
}

func mustListenAndServe(srv *http.Server) {
	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func awaitShutdown() {
	stop := make(chan os.Signal, 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}

func shutdown(ctx context.Context, srv *http.Server) error {
	err := srv.Shutdown(ctx)
	if err != nil {
		return err
	}

	return nil
}
