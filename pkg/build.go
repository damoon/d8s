package wedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helpText = `
wedding builds only support these arguments: context, tag, buildargs, cachefrom, cpuperiod, cpuquota, dockerfile, memory, labels, and target
%s`
)

type buildConfig struct {
	buildArgs       map[string]string
	labels          map[string]string
	cacheRepo       string
	cpuMilliseconds int
	dockerfile      string
	memoryBytes     int
	target          string
	tags            []string
	noCache         bool
	registryAuth    dockerConfig
	contextFilePath string
}

// ObjectStore manages access to a S3 compatible file store.
type ObjectStore struct {
	Client *s3.S3
	Bucket string
}

func (s Service) build(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg, err := buildParameters(r)
	if err != nil {
		printBuildHelpText(w, err)
		return
	}

	err = s.objectStore.storeContext(ctx, r, cfg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("store context: %v", err)))
		log.Printf("execute build: %v", err)
		return
	}
	defer func() {
		s.objectStore.deleteContext(ctx, cfg)
	}()

	err = s.executeBuild(ctx, cfg, w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("execute build: %v", err)))
		log.Printf("execute build: %v", err)
		return
	}

	w.Write([]byte(`{"aux":{"ID":"sha256:42341736246f8e99122d49e4c0e414f0a3e5f69a024e72a2ac1a39a2093d483f"}}`))
}

func buildParameters(r *http.Request) (*buildConfig, error) {
	cfg := &buildConfig{}

	asserts := map[string]string{
		// "buildargs":    "{}",
		// "cachefrom":    "[]",
		"cgroupparent": "",
		// "cpuperiod":    "0",
		// "cpuquota":     "100000",
		"cpusetcpus": "",
		"cpusetmems": "",
		"cpushares":  "0",
		// "dockerfile":   "use-case-1%2FDockerfile",
		// "labels": "{}",
		// "memory":       "1000",
		"memswap": "0",
		// "networkmode": "default", // needs two ignored values
		// "rm":      "1", // needs two ignored values
		"shmsize": "0",
		// "target":       "",
		"ulimits": "null",
		// "version": "1", // needs two ignored values
	}

	for k, v := range asserts {
		if r.URL.Query().Get(k) != v {
			return cfg, fmt.Errorf("unsupported argument %s set to '%s'", k, r.URL.Query().Get(k))
		}
	}

	networkmode := r.URL.Query().Get("networkmode")
	if networkmode != "default" && networkmode != "" { // docker uses "default", tilt uses ""
		return cfg, fmt.Errorf("unsupported argument networkmode set to '%s'", networkmode)
	}

	version := r.URL.Query().Get("version")
	if version != "1" && version != "2" { // docker uses "1", tilt uses "2"
		return cfg, fmt.Errorf("unsupported argument version set to '%s'", version)
	}

	rm := r.URL.Query().Get("rm")
	if rm != "1" && rm != "0" { // docker uses "1", tilt uses 02"
		return cfg, fmt.Errorf("unsupported argument rm set to '%s'", rm)
	}

	err := json.Unmarshal([]byte(r.URL.Query().Get("buildargs")), &cfg.buildArgs)
	if err != nil {
		return cfg, fmt.Errorf("decode buildargs: %v", err)
	}

	err = json.Unmarshal([]byte(r.URL.Query().Get("labels")), &cfg.labels)
	if err != nil {
		return cfg, fmt.Errorf("decode labels: %v", err)
	}

	// cache repo
	cachefrom := []string{}
	err = json.Unmarshal([]byte(r.URL.Query().Get("cachefrom")), &cachefrom)
	if err != nil {
		return cfg, fmt.Errorf("decode cachefrom: %v", err)
	}

	if len(cachefrom) > 1 {
		return cfg, fmt.Errorf("wedding only supports one cachefrom image")
	}
	if len(cachefrom) == 1 {
		cfg.cacheRepo = cachefrom[0]
	}

	// TODO set default cache from tag

	// cpu limit
	cpuperiod, err := strconv.Atoi(r.URL.Query().Get("cpuperiod"))
	if err != nil {
		return cfg, fmt.Errorf("parse cpu period to int: %v", err)
	}
	if cpuperiod == 0 {
		cpuperiod = 100_000 // results in 1 cpu
	}

	cpuquota, err := strconv.Atoi(r.URL.Query().Get("cpuquota"))
	if err != nil {
		return cfg, fmt.Errorf("parse cpu quota to int: %v", err)
	}
	if cpuperiod == 0 {
		cpuperiod = 100_000 // 100ms is the default of docker
	}

	cfg.cpuMilliseconds = int(1000 * float64(cpuquota) / float64(cpuperiod))

	// Dockerfile
	cfg.dockerfile = r.URL.Query().Get("dockerfile")
	if cfg.dockerfile == "" {
		cfg.dockerfile = "Dockerfile"
	}

	// memory limit
	memoryArg := r.URL.Query().Get("memory")
	if memoryArg == "" {
		memoryArg = "2147483648" // 2Gi default
	}
	memory, err := strconv.Atoi(memoryArg)
	if err != nil {
		return cfg, fmt.Errorf("parse cpu quota to int: %v", err)
	}
	cfg.memoryBytes = memory

	// target
	cfg.target = r.URL.Query().Get("target")

	// image tag
	cfg.tags = r.URL.Query()["t"]

	// disable cache
	nocache := r.URL.Query().Get("nocache")
	cfg.noCache = nocache == "1"

	// registry authentitation
	dockerCfg, err := xRegistryConfig(r.Header.Get("X-Registry-Config")).toDockerConfig()
	if err != nil {
		return cfg, fmt.Errorf("extract registry config: %v", err)
	}

	cfg.registryAuth = dockerCfg

	return cfg, nil
}

func printBuildHelpText(w http.ResponseWriter, err error) {
	txt := fmt.Sprintf(helpText, err)

	w.WriteHeader(http.StatusBadRequest)

	_, err = w.Write([]byte(txt))
	if err != nil {
		log.Printf("print help text: %v", err)
	}
}

func (o ObjectStore) storeContext(ctx context.Context, r *http.Request, cfg *buildConfig) error {

	// BUG: possible OOM: loading all context into memory
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read context: %v", err)
	}

	path := fmt.Sprintf("%d.tar", time.Now().UnixNano())

	ioutil.WriteFile(path, b, os.ModePerm)

	file, err := os.Open(path)
	defer file.Close()

	put := &s3.PutObjectInput{
		Bucket:      aws.String(o.Bucket),
		Key:         aws.String(path),
		ContentType: aws.String("application/x-tar"),
		Body:        file,
	}

	_, err = o.Client.PutObjectWithContext(ctx, put)
	if err != nil {
		return fmt.Errorf("upload context to bucket: %v", err)
	}

	cfg.contextFilePath = path

	return nil

}

func (o ObjectStore) presignContext(cfg *buildConfig) (string, error) {

	objectRequest, _ := o.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(o.Bucket),
		Key:    aws.String(cfg.contextFilePath),
	})

	url, err := objectRequest.Presign(10 * time.Minute)
	if err != nil {
		return "", fmt.Errorf("presign GET %s: %v", cfg.contextFilePath, err)
	}

	return url, nil
}

func (o ObjectStore) deleteContext(ctx context.Context, cfg *buildConfig) error {
	_, err := o.Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(o.Bucket),
		Key:    aws.String(cfg.contextFilePath),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s Service) executeBuild(ctx context.Context, cfg *buildConfig, w http.ResponseWriter) error {

	stream(w, fmt.Sprintf("%v", cfg))

	presignedContextURL, err := s.objectStore.presignContext(cfg)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-docker-config-",
		},
		StringData: map[string]string{
			"config.json": cfg.registryAuth.mustToJSON(),
		},
	}

	secretClient := s.kubernetesClient.CoreV1().Secrets(s.namespace)

	secret, err = secretClient.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		streamf(w, "Secret creation failed: %v\n", err)
		return fmt.Errorf("create secret: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = secretClient.Delete(ctx, secret.Name, metav1.DeleteOptions{})
		if err != nil {
			streamf(w, "Secret deletetion failed: %v\n", err)
			log.Printf("delete secret: %v", err)
		}
	}()

	imageNames := ""
	for idx, tag := range cfg.tags {
		if idx != 0 {
			imageNames += ","
		}
		imageNames += fmt.Sprintf("wedding-registry:5000/images/%s", tag)
	}

	output := "--output type=image,push=true,name=wedding-registry:5000/digests"
	if imageNames != "" {
		output = fmt.Sprintf("--output type=image,push=true,\"name=%s\"", imageNames)
	}

	// TODO add timeout for script
	buildScript := fmt.Sprintf(`
set -euo pipefail

mkdir ~/context && cd ~/context

mkdir -p ~/.config/buildkit/
echo "
[registry.\"wedding-registry:5000\"]
http = true
insecure = true
" > ~/.config/buildkit/buildkitd.toml

echo Downloading context
wget -O - "%s" | tar -xf - # --quiet

export BUILDKITD_FLAGS="--oci-worker-no-process-sandbox"
export BUILDCTL_CONNECT_RETRIES_MAX=100
buildctl-daemonless.sh \
 build \
 --frontend dockerfile.v0 \
 --local context=. \
 --local dockerfile=. \
 --opt filename=Dockerfile \
 %s \
 --export-cache=type=registry,ref=wedding-registry:5000/cache-repo,mode=max \
 --import-cache=type=registry,ref=wedding-registry:5000/cache-repo
`, presignedContextURL, output)

	stream(w, buildScript)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-build-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "moby/buildkit:v0.7.2-rootless",
					Name:  "buildkit",
					Command: []string{
						"sh",
						"-c",
						buildScript,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/home/user/.docker",
							Name:      "docker-config",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "docker-config",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	err = s.executePod(ctx, pod, w)
	if err != nil {
		return err
	}

	return nil
}
