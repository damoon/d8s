package wedding

import (
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
)

const apiVersion = "1.40"

// Service runs the wedding server.
type Service struct {
	router           http.Handler
	objectStore      *ObjectStore
	namespace        string
	kubernetesClient *kubernetes.Clientset
}

// NewService creates a new service server and initiates the routes.
func NewService(objectStore *ObjectStore, kubernetesClient *kubernetes.Clientset, namespace string) *Service {
	srv := &Service{
		objectStore:      objectStore,
		namespace:        namespace,
		kubernetesClient: kubernetesClient,
	}

	srv.routes()

	return srv
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Service) routes() {
	router := mux.NewRouter()
	router.HandleFunc("/_ping", ping).Methods(http.MethodGet)
	router.HandleFunc("/session", ignored).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/version", version).Methods(http.MethodGet)

	router.HandleFunc("/{apiVersion}/build", s.build).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/{name:.+}/tag", s.tagImage).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/{name:.+}/push", s.pushImage).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/create", s.pullImage).Methods(http.MethodPost)

	router.HandleFunc("/{apiVersion}/containers/prune", containersPrune).Methods(http.MethodPost)
	router.HandleFunc("/{apiVersion}/images/json", imagesJSON).Methods(http.MethodGet)
	router.HandleFunc("/{apiVersion}/build/prune", buildPrune).Methods(http.MethodPost)

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream(w, "This function is not supported by wedding.")
		w.WriteHeader(http.StatusNotImplemented)
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
