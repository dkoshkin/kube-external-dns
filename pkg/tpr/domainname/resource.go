package domainname

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/dkoshkin/kube-external-dns/pkg/tpr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
)

type DomainNameResource struct {
	tpr.Metadata
	TPRClient *rest.RESTClient
}

func (r *DomainNameResource) Meta() tpr.Metadata {
	return r.Metadata
}

// Resource strips "-" and pluralizes the string
func (r *DomainNameResource) Resource() string {
	resource := fmt.Sprint(strings.Replace(r.Meta().Type, "-", "", -1) + "s")
	return resource
}

// New
func New(client *rest.RESTClient) *DomainNameResource {
	return &DomainNameResource{
		Metadata: tpr.Metadata{
			Type:        "domain-name",
			Group:       "koshk.in",
			Description: "An external DNS record configured by koshkin/kube-external-dns service",
			Version:     "v1",
		},
		TPRClient: client,
	}
}

// Get
func (r *DomainNameResource) Get(name string, namesapce string) (*DomainName, error) {
	var record DomainName
	if err := r.TPRClient.Get().Resource(r.Resource()).Namespace(namesapce).Name(name).Do().Into(&record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *DomainNameResource) GetAll(namesapce string) (*DomainNameList, error) {
	var recordList DomainNameList
	if err := r.TPRClient.Get().Resource(r.Resource()).Namespace(namesapce).Do().Into(&recordList); err != nil {
		return nil, err
	}
	return &recordList, nil
}

func (r *DomainNameResource) CreateOrUpdate(record *DomainName, namesapce string) (*DomainName, error) {
	foundRecord, err := r.Get(record.Metadata.Name, namesapce)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Infof("%s: DomainName resource does not exists, will be creating it", record.Metadata.GetName())
			var result DomainName
			// TPR not found, create new record
			if err := r.TPRClient.Post().Resource(r.Resource()).Namespace(api.NamespaceDefault).Body(record).Do().Into(&result); err != nil {
				return &result, fmt.Errorf("%s: error creating %s DomainName resource: %v", record.Metadata.GetName(), r.Resource(), err)
			}
			return &result, nil
		}
		return nil, fmt.Errorf("%s: error determining if ExternalDNS record exists: %v", record.Metadata.Name, err)
	}

	// update record
	if foundRecord != nil {
		logrus.Infof("%s: DomainName resource already exists, will be updating it", record.Metadata.Name)
		logrus.Info("TOOD")
		return foundRecord, nil
	}

	return nil, fmt.Errorf("%s: error determining if DomainName resource exists", record.Metadata.Name)
}

func (r *DomainNameResource) Delete(name string, namesapce string) error {
	foundRecord, err := r.Get(name, namesapce)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Infof("%s: DomainName resource does not exists, will not be deleting anything", name)
			return nil
		}
		return fmt.Errorf("%s: error determining if DomainName resource exists: %v", name, err)
	}

	if foundRecord != nil {
		logrus.Infof("%s: DomainName resource exists, will be deleting it", name)
		if err := r.TPRClient.Delete().Resource(r.Resource()).Namespace(namesapce).Name(name).Do().Error(); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("%s: error determining if DomainName resource exists", name)
}
