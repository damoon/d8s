package wedding

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
)

const (
	// MaxExecutionTime is the longest time allowed for a command to run.
	MaxExecutionTime = 30 * time.Minute

	apiVersion     = "1.40"
	buildkitImage  = "moby/buildkit:v0.7.2-rootless"
	skopeoImage    = "mrliptontea/skopeo:1.2.0"
	buildMemory    = "2147483648" // 2Gi default
	buildCPUQuota  = 100_000      // results in 1 cpu
	buildCPUPeriod = 100_000      // 100ms is the default of docker
	skopeoMemory   = "100Mi"
	skopeoCPU      = "200m"
)

// Service runs the wedding server.
type Service struct {
	router           http.Handler
	objectStore      *ObjectStore
	namespace        string
	kubernetesClient *kubernetes.Clientset
}

// NewService creates a new service server and initiates the routes.
func NewService(gitHash, gitRef string, objectStore *ObjectStore, kubernetesClient *kubernetes.Clientset, namespace string) *Service {
	srv := &Service{
		objectStore:      objectStore,
		namespace:        namespace,
		kubernetesClient: kubernetesClient,
	}

	srv.routes(gitHash, gitRef)

	return srv
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Service) routes(gitHash, gitRef string) {
	router := mux.NewRouter()
	router.HandleFunc("/_ping", ping).Methods(http.MethodGet)
	router.HandleFunc("/session", ignored).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/version", versionHandler(gitHash, gitRef)).Methods(http.MethodGet)
	router.HandleFunc("/{apiVersion}/info", ignored).Methods(http.MethodGet)

	router.HandleFunc("/{apiVersion}/build", s.build).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/{name:.+}/tag", s.tagImage).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/{name:.+}/push", s.pushImage).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/{name:.+}/json", s.inspect).Methods(http.MethodGet)
	router.HandleFunc("/{apiVersion}/images/create", s.pullImage).Methods(http.MethodPost)

	router.HandleFunc("/{apiVersion}/containers/prune", containersPrune).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/json", imagesJSON).Methods(http.MethodGet)
	router.HandleFunc("/{apiVersion}/build/prune", buildPrune).Methods(http.MethodPost)

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("This function is not supported by wedding."))
		log.Printf("501 - Not Implemented: %s %s", r.Method, r.URL)
	})

	s.router = loggingMiddleware(router)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "kube-probe/") {
			log.Printf("HTTP Request %s %s\n", r.Method, r.URL)
		}

		next.ServeHTTP(w, r)
	})
}

func ignored(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
