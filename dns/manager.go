package dns

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/dkoshkin/kube-external-dns/providers"
	"k8s.io/client-go/pkg/api/v1"
)

var providerAnnotation = "kube.external.dns.io/provider"
var rootDomainAnnotation = "kube.external.dns.io/root-domain"
var subDomainAnnotation = "kube.external.dns.io/sub-domain" //optional: name.namespace.$domain
//var ttlAnnotation = "kube.external.dns.io/TTL"              //optional: 300 seconds

// HandleUpsertEvent will create or update a record if exists
func HandleUpsertEvent(service *v1.Service) error {
	mngr, err := GetManager(service)
	if err != nil {
		return err
	}
	// nothing to do
	if mngr == nil {
		return nil
	}

	fqdn := mngr.DNSRecord.Fqdn
	name := mngr.ServiceName
	found, err := mngr.GetRecord()
	// check if record already exists
	if err != nil {
		return fmt.Errorf("%s: could not determine if record '%s' exists: %v", name, fqdn, err)
	}
	if found == nil {
		logrus.Infof("%s: is not already set, will be creating a new record", name)
		return mngr.InsertRecord()
	}
	if !slicesSimilar(found.Records, mngr.DNSRecord.Records) {
		logrus.Warnf("%s: is set but contains different records, will be updating it", name)
		return mngr.UpdateRecord()
	}

	logrus.Infof("%s: is already configured, nothing to do", name)

	return nil
}

// HandleDeleteEvent will delete a record
func HandleDeleteEvent(service *v1.Service) error {
	mngr, err := GetManager(service)
	if err != nil {
		return err
	}
	// nothing to do
	if mngr == nil {
		return nil
	}

	// check if record exists
	name := mngr.ServiceName
	fqdn := mngr.DNSRecord.Fqdn
	found, err := mngr.GetRecord()
	if err != nil {
		return fmt.Errorf("%s: could not determine if record '%s' exists: %v", name, fqdn, err)
	}
	if found == nil {
		return fmt.Errorf("%s: expected record but it was not found", name)
	}

	logrus.Infof("%s: record found, will be deleting it", name)
	return mngr.DeleteRecord()
}

// GetManager parses the v1.Service object and returns a DNS manager
func GetManager(service *v1.Service) (*DNSManager, error) {
	if service == nil {
		logrus.Warn("service object is nil")
		return nil, nil
	}
	// get details from annotations
	annotations := service.Annotations
	providerStr, ok := annotations[providerAnnotation]
	if !ok {
		logrus.Infof("%s: service resource does not have the annotation '%s'", service.Name, providerAnnotation)
		return nil, nil
	}
	rootDomain := annotations[rootDomainAnnotation]
	if len(rootDomain) == 0 {
		return nil, fmt.Errorf("%s: service resource annotation '%s' cannot be empty", service.Name, providerAnnotation)
	}
	var records []string
	// 	TODO use real LB IPs
	// if len(service.Spec.ClusterIP) > 0 {
	// 	records = append(records, service.Spec.ClusterIP)
	// }
	if service.Status.LoadBalancer.Ingress != nil {
		lbRecords := service.Status.LoadBalancer.Ingress
		for _, r := range lbRecords {
			if len(r.IP) > 0 {
				records = append(records, r.IP)
			}
		}
	}
	if len(records) == 0 {
		logrus.Warnf("%s: service does not have valid IP records, this could mean its just not ready yet", service.Name)
		return nil, nil
	}

	provider, err := providers.GetProvider(providerStr, rootDomain)
	if err != nil {
		return nil, fmt.Errorf("%s: error getting provider: %v", service.Name, err)
	}
	subDomain := fmt.Sprintf("%s.%s", service.Name, service.Namespace)
	// allow to overwire default subDomain
	if subDomainStr := annotations[subDomainAnnotation]; len(subDomainStr) > 0 {
		subDomain = subDomainStr
	}

	fqdn := fmt.Sprintf("%s.%s", subDomain, rootDomain)

	mngr := DNSManager{
		ServiceName: service.Name,
		Provider:    provider,
		DNSRecord: &providers.DnsRecord{
			Fqdn:    fqdn,
			Records: records,
			Type:    "A", // always default to "A" for now,
			TTL:     0,   // set it to 0 and let the provider default to minimum
		},
	}

	return &mngr, nil
}

// DNSManager handles creating, updating and deleting DNS records
type DNSManager struct {
	ServiceName string
	Provider    providers.Provider
	DNSRecord   *providers.DnsRecord
}

func (mngr *DNSManager) GetRecord() (*providers.DnsRecord, error) {
	fqdn := mngr.DNSRecord.Fqdn
	return mngr.Provider.GetRecord(fqdn)
}

func (mngr *DNSManager) InsertRecord() error {
	return mngr.Provider.AddRecord(*mngr.DNSRecord)
}

func (mngr *DNSManager) UpdateRecord() error {
	return mngr.Provider.UpdateRecord(*mngr.DNSRecord)
}

func (mngr *DNSManager) DeleteRecord() error {
	return mngr.Provider.RemoveRecord(*mngr.DNSRecord)
}

func slicesSimilar(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		diff[_x]++
	}
	for _, _y := range y {
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y]--
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	if len(diff) == 0 {
		return true
	}
	return false
}
