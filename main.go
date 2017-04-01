package main

import (
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	dnscontroller "github.com/dkoshkin/kube-external-dns/pkg/controller"
	dnsprovider "github.com/dkoshkin/kube-external-dns/pkg/provider/dns"
	"github.com/dkoshkin/kube-external-dns/pkg/server"
	"github.com/dkoshkin/kube-external-dns/pkg/tpr"
	dnstpr "github.com/dkoshkin/kube-external-dns/pkg/tpr/domainname"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Only required to authenticate against GKE clusters
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	_ "github.com/dkoshkin/kube-external-dns/pkg/provider/dns/cloudflare"
	_ "github.com/dkoshkin/kube-external-dns/pkg/provider/dns/digitalocean"
	_ "github.com/dkoshkin/kube-external-dns/pkg/provider/dns/dnsimple"
	_ "github.com/dkoshkin/kube-external-dns/pkg/provider/dns/route53"
)

// Set via linker flag
var version string
var buildDate string

func main() {
	// creates a kubeconfig
	config, err := buildKubecConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// get k8s client with TPRs based on default config
	tprClient := tpr.GetClient(config, &dnstpr.DomainName{}, &dnstpr.DomainNameList{})
	// initialize the TPR with the API server
	domainNameTPR := dnstpr.New(tprClient)
	err = tpr.Initialize(clientset, domainNameTPR)
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
				service := obj.(*v1.Service)
				logrus.Infof("%s: service add event", service.Name)
				changed, providerRecord, err := dnscontroller.UpsertToDNSProvider(service)
				if err != nil {
					logrus.Error(err)
				}
				if changed {
					logrus.Infof("%s: provider DNS record changed succesfully", service.Name)
					// Update DomainName TPR
					if err != createOrUpdateDomainNameTPR(service, providerRecord, domainNameTPR) {
						logrus.Errorf("%s: DomainName TPR could not updated: %v", service.Name, err)
					}
					logrus.Infof("%s: DomainName TPR changed succesfully", service.Name)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				service := newObj.(*v1.Service)
				logrus.Infof("%s: service update event", service.Name)
				changed, providerRecord, err := dnscontroller.UpsertToDNSProvider(service)
				if err != nil {
					logrus.Error(err)
				}
				if changed {
					logrus.Infof("%s: provider DNS record changed succesfully", service.Name)
					// Update DomainName TPR
					if err != createOrUpdateDomainNameTPR(service, providerRecord, domainNameTPR) {
						logrus.Errorf("%s: DomainName TPR could not updated: %v", service.Name, err)
					}
					logrus.Infof("%s: DomainName TPR changed succesfully", service.Name)
				}
			},
			DeleteFunc: func(obj interface{}) {
				service := obj.(*v1.Service)
				logrus.Infof("%s: service delete event", service.Name)
				changed, _, err := dnscontroller.DeleteToDNSProvider(service)
				if err != nil {
					logrus.Error(err)
				}
				if changed {
					logrus.Infof("%s: provider DNS record deleted succesfully", service.Name)
					// Delete TPR
					err := domainNameTPR.Delete(service.Name, service.Namespace)
					if err != nil {
						logrus.Errorf("%s: DomainName TPR could not deleted: %v", service.Name, err)
					}
					logrus.Infof("%s: DomainName TPR deleted succesfully", service.Name)
				}
			},
		},
	)

	go controller.Run(wait.NeverStop)

	//Keep alive
	logrus.Fatal(http.ListenAndServe(":8080", server.NewRouter(version, buildDate)))
}

func createOrUpdateDomainNameTPR(service *v1.Service, providerRecord *dnsprovider.DnsRecord, domainNameTPR *dnstpr.DomainNameResource) error {
	tpr := &dnstpr.DomainName{
		Metadata: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
		},
		Spec: dnstpr.DomainNameSpec{
			ServiceName: service.Name,
			Record: dnstpr.Record{
				FQDN:      providerRecord.Fqdn,
				Endpoints: providerRecord.Records,
				Type:      providerRecord.Type,
				TTL:       providerRecord.TTL,
			},
		},
	}
	_, err := domainNameTPR.CreateOrUpdate(tpr, service.Namespace)

	return err
}

func buildKubecConfig() (*rest.Config, error) {
	// use the provided file or setup an in-cluster config
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
