package wedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	cpuMilliseconds int
	dockerfile      string
	memoryBytes     int
	target          string
	tags            []string
	registryAuth    dockerConfig
	contextFilePath string
}

// ObjectStore manages access to a S3 compatible file store.
type ObjectStore struct {
	Client   *s3.S3
	Uploader *s3manager.Uploader
	Bucket   string
}

func (s Service) build(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg, err := buildParameters(r)
	if err != nil {
		printBuildHelpText(w, err)
		return
	}

	err = s.objectStore.storeContext(ctx, r.Body, cfg)
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
		log.Printf("execute build: %v", err)
		return
	}
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
		// "dockerfile":   "Dockerfile",
		// "labels": "{}",
		// "memory":       "1000",
		"memswap": "0",
		// "networkmode": "default", // needs two ignored values
		// "rm":      "1", // needs two ignored values
		"shmsize": "0",
		// "target":       "",
		"ulimits": "null",
		// "version": "1", // needs two ignored values
		"nocache": "",
	}

	for k, v := range asserts {
		if r.URL.Query().Get(k) != v {
			return cfg, fmt.Errorf("unsupported argument %s set to '%s'", k, r.URL.Query().Get(k))
		}
	}

	cachefrom := r.URL.Query().Get("cachefrom")
	if cachefrom != "[]" && cachefrom != "null" { // docker uses "[]", tilt uses "null" by default
		return cfg, fmt.Errorf("unsupported argument cachefrom set to '%s'", cachefrom)
	}

	networkmode := r.URL.Query().Get("networkmode")
	if networkmode != "default" && networkmode != "" { // docker uses "default", tilt uses "" by default
		return cfg, fmt.Errorf("unsupported argument networkmode set to '%s'", networkmode)
	}

	version := r.URL.Query().Get("version")
	if version != "1" && version != "2" { // docker uses "1", tilt uses "2" by default
		return cfg, fmt.Errorf("unsupported argument version set to '%s'", version)
	}

	rm := r.URL.Query().Get("rm")
	if rm != "1" && rm != "0" { // docker uses "1", tilt uses "0" by default
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

	// cpu limit
	cpuquota, err := strconv.Atoi(r.URL.Query().Get("cpuquota"))
	if err != nil {
		return cfg, fmt.Errorf("parse cpu quota to int: %v", err)
	}
	if cpuquota == 0 {
		cpuquota = buildCPUQuota
	}

	cpuperiod, err := strconv.Atoi(r.URL.Query().Get("cpuperiod"))
	if err != nil {
		return cfg, fmt.Errorf("parse cpu period to int: %v", err)
	}
	if cpuperiod == 0 {
		cpuperiod = buildCPUPeriod
	}

	cfg.cpuMilliseconds = int(1000 * float64(cpuquota) / float64(cpuperiod))

	// Dockerfile
	cfg.dockerfile = r.URL.Query().Get("dockerfile")
	if cfg.dockerfile == "" {
		cfg.dockerfile = "Dockerfile"
	}

	// memory limit
	memoryArg := r.URL.Query().Get("memory")
	if memoryArg == "" || memoryArg == "0" {
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

func (o ObjectStore) storeContext(ctx context.Context, r io.Reader, cfg *buildConfig) error {
	path := fmt.Sprintf("%d.tar", time.Now().UnixNano())
	cfg.contextFilePath = path

	_, err := o.Uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(o.Bucket),
		Key:         aws.String(path),
		ContentType: aws.String("application/x-tar"),
		Body:        r,
	})
	if err != nil {
		return fmt.Errorf("upload build context to bucket: %v", err)
	}

	return nil
}

func (o ObjectStore) presignContext(cfg *buildConfig) (string, error) {

	objectRequest, _ := o.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(o.Bucket),
		Key:    aws.String(cfg.contextFilePath),
	})

	url, err := objectRequest.Presign(MaxExecutionTime)
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

	destination := "--output type=image,push=true,name=wedding-registry:5000/digests"
	if imageNames != "" {
		destination = fmt.Sprintf(`--output type=image,push=true,\"name=%s\"`, imageNames)
	}

	dockerfileName := filepath.Base(cfg.dockerfile)
	dockerfileDir := filepath.Dir(cfg.dockerfile)

	target := ""
	if cfg.target != "" {
		target = fmt.Sprintf("--opt target=%s", cfg.target)
	}

	buildargs := ""
	for k, v := range cfg.buildArgs {
		buildargs += fmt.Sprintf("--opt build-arg:%s='%s' ", k, v)
	}

	labels := ""
	for k, v := range cfg.labels {
		buildargs += fmt.Sprintf("--opt label:%s='%s' ", k, v)
	}

	buildScript := fmt.Sprintf(`
set -euo pipefail

echo download bulid context
mkdir ~/context && cd ~/context
wget -O - "%s" | tar -xf -

set -x
buildctl-daemonless.sh \
 build \
 --frontend dockerfile.v0 \
 --local context=. \
 --local dockerfile=%s \
 --opt filename=%s \
 %s \
 %s \
 %s \
 %s \
 --export-cache=type=registry,ref=wedding-registry:5000/cache-repo,mode=max \
 --import-cache=type=registry,ref=wedding-registry:5000/cache-repo
`, presignedContextURL, dockerfileDir, dockerfileName, buildargs, labels, target, destination)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-build-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: buildkitImage,
					Name:  "buildkit",
					Command: []string{
						"timeout",
						strconv.Itoa(int(MaxExecutionTime / time.Second)),
					},
					Args: []string{
						"sh",
						"-c",
						buildScript,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/home/user/.docker",
							Name:      "docker-config",
						},
						{
							MountPath: "/home/user/.config/buildkit",
							Name:      "buildkitd-config",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", cfg.cpuMilliseconds)),
							corev1.ResourceMemory: resource.MustParse(strconv.Itoa(cfg.memoryBytes)),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", cfg.cpuMilliseconds)),
							corev1.ResourceMemory: resource.MustParse(strconv.Itoa(cfg.memoryBytes)),
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
				{
					Name: "buildkitd-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "buildkitd-config",
							},
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	o := &output{w: w}
	d := &digestParser{w: o}
	err = s.executePod(ctx, pod, d)
	if err != nil {
		o.Errorf("execute build: %v", err)
		return err
	}

	err = d.publish(w)
	if err != nil {
		return err
	}

	return nil
}

type digestParser struct {
	buf bytes.Buffer
	w   io.Writer
}

func (d *digestParser) publish(w io.Writer) error {
	patterns := regexp.
		MustCompile(`exporting manifest (sha256:[0-9a-f]+)`).
		FindStringSubmatch(d.buf.String())

	if len(patterns) != 2 || patterns[1] == "" {
		return fmt.Errorf("digest not found")
	}

	_, err := w.Write([]byte(fmt.Sprintf(`{"aux":{"ID":"%s"}}`, patterns[1])))
	if err != nil {
		return err
	}

	return nil
}

func (d *digestParser) Write(bb []byte) (int, error) {
	_, err := d.buf.Write(bb)
	if err != nil {
		return 0, err
	}

	return d.w.Write(bb)
}
