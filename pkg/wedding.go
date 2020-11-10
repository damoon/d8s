package wedding

import (
	"log"
	"net/http"
	"strings"

	"k8s.io/client-go/kubernetes"
)

const apiVersion = "1.40"

// Service runs the wedding server.
type Service struct {
	router           *http.ServeMux
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
	s.router = http.NewServeMux()
	s.router.HandleFunc("/v"+apiVersion+"/images/json", imagesJSON)
	s.router.HandleFunc("/v"+apiVersion+"/containers/prune", containersPrune)
	s.router.HandleFunc("/v"+apiVersion+"/build/prune", buildPrune)
	s.router.HandleFunc("/v"+apiVersion+"/version", version)
	s.router.HandleFunc("/v"+apiVersion+"/build", logReqResp(s.buildHandler()))
	s.router.HandleFunc("/_ping", ping)
	s.router.HandleFunc("/", logReqResp(unsupported))
}

func logReqResp(fn http.HandlerFunc) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.Header.Get("User-Agent"), "kube-probe/") {
			log.Printf("req: %v\n", req)
		}

		fn(res, req)
	}
}
