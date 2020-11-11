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
	router           *mux.Router
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
	s.router = mux.NewRouter()
	s.router.HandleFunc("/_ping", ping).Methods(http.MethodGet)
	s.router.HandleFunc("/session", ignored).Methods(http.MethodPost)
	s.router.HandleFunc("/v"+apiVersion+"/version", version).Methods(http.MethodGet)

	s.router.HandleFunc("/v"+apiVersion+"/build", s.buildHandler()).Methods(http.MethodPost)
	s.router.HandleFunc("/v"+apiVersion+"/images/{name}/tag", tagImage).Methods(http.MethodPost)
	s.router.HandleFunc("/v"+apiVersion+"/images/{name:.+}/push", pushImage).Methods(http.MethodPost)

	s.router.HandleFunc("/v"+apiVersion+"/containers/prune", containersPrune).Methods(http.MethodPost)
	s.router.HandleFunc("/v"+apiVersion+"/images/json", imagesJSON).Methods(http.MethodGet)
	s.router.HandleFunc("/v"+apiVersion+"/build/prune", buildPrune).Methods(http.MethodPost)

	s.router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("NOT FOUND: %v", r)
	})
}

func ignored(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func logReqResp(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "kube-probe/") {
			log.Printf("req: %v\n", r)
		}

		fn(w, r)
	}
}
