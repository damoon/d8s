package d8s

import (
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/sync/semaphore"
)

const (
	// MaxExecutionTime is the longest time allowed for a command to run.
	MaxExecutionTime = 30 * time.Minute

	apiVersion     = "1.40"
	buildkitImage  = "moby/buildkit:v0.9.3-rootless"
	skopeoImage    = "ghcr.io/utopia-planitia/skopeo-image@sha256:130836bd82e5f3a856f659e22f0e9d97c545ff0d955807b806595ec4874d5f37"
	buildMemory    = "2147483648" // 2Gi default
	buildCPUQuota  = 100_000      // results in 1 cpu
	buildCPUPeriod = 100_000      // 100ms is the default of docker
	skopeoMemory   = "100Mi"
	skopeoCPU      = "200m"
)

var (
	semBuild  = semaphore.NewWeighted(1)
	semSkopeo = semaphore.NewWeighted(5)
)

// Service runs the dinner server.
type Service struct {
	router      http.Handler
	objectStore *ObjectStore
	upstream    *httputil.ReverseProxy
}

// NewService creates a new service server and initiates the routes.
func NewService(gitHash, gitRef string, objectStore *ObjectStore, upstream *httputil.ReverseProxy) *Service {
	srv := &Service{
		objectStore: objectStore,
		upstream:    upstream,
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

	router.HandleFunc("/{apiVersion}/build", s.build).Methods(http.MethodPost)

	router.HandleFunc("/_chunks", s.chunkExists).Methods(http.MethodGet)
	router.HandleFunc("/_chunks", s.addChunk).Methods(http.MethodPost)

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.upstream.ServeHTTP(w, r)
	})

	s.router = loggingMiddleware(router)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "kube-probe/") {
			//	log.Printf("HTTP Request %s %s\n", r.Method, r.URL)
		}

		next.ServeHTTP(w, r)
	})
}

func ignored(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func missing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}
