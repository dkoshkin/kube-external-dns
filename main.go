package main

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/Sirupsen/logrus"
	"github.com/dkoshkin/kube-external-dns/dns"
	"github.com/gorilla/mux"

	_ "github.com/dkoshkin/kube-external-dns/providers/cloudflare"
)

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	watchlist := cache.NewListWatchFromClient(
		clientset.Core().RESTClient(),
		"services",
		v1.NamespaceAll,
		fields.Everything())

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Service{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				serv := obj.(*v1.Service)
				logrus.Infof("%s: service add event", serv.Name)
				if err := dns.HandleUpsertEvent(serv); err != nil {
					logrus.Error(err)
				}
			},
			DeleteFunc: func(obj interface{}) {
				serv := obj.(*v1.Service)
				logrus.Infof("%s: service delete event", serv.Name)
				if err := dns.HandleDeleteEvent(serv); err != nil {
					logrus.Error(err)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				serv := newObj.(*v1.Service)
				logrus.Infof("%s: service update event", serv.Name)
				if err := dns.HandleUpsertEvent(serv); err != nil {
					logrus.Error(err)
				}
			},
		},
	)

	go controller.Run(wait.NeverStop)

	//Keep alive
	logrus.Fatal(http.ListenAndServe(":8080", newRouter()))
}

// Set via linker flag
var version string
var buildDate string

func newRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/healthz", HealthzHandler)
	return r
}

// HealthzHandler always returns Ok for now
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Ok\n%s\n%s\n", version, buildDate)
}
