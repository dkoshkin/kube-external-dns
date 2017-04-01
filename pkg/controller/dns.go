package dns

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"

	dnsprovider "github.com/dkoshkin/kube-external-dns/pkg/provider/dns"
)

var providerAnnotation = "external.dns.koshk.in/provider"
var rootDomainAnnotation = "external.dns.koshk.in/root-domain"
var subDomainAnnotation = "external.dns.koshk.in/sub-domain" //optional: name.namespace.$domain
//var ttlAnnotation = "external.dns.koshk.in/TTL"              //optional: 120 seconds

// UpsertToDNSProvider will create or update a record if exists, in an external DNS provider
func UpsertToDNSProvider(service *v1.Service) (changed bool, record *dnsprovider.DnsRecord, err error) {
	mngr, err := GetManager(service)
	if err != nil {
		return false, nil, err
	}
	// nothing to do
	if mngr == nil {
		return false, nil, nil
	}

	fqdn := mngr.DNSRecord.Fqdn
	name := mngr.ServiceName
	found, err := mngr.GetRecord()
	// check if record already exists
	if err != nil {
		return false, nil, fmt.Errorf("%s: could not determine if record '%s' exists: %v", name, fqdn, err)
	}
	if found == nil {
		logrus.Infof("%s: is not already set, will be creating a new record", name)
		err := mngr.InsertRecord()
		return err == nil, mngr.DNSRecord, err
	}
	if !slicesSimilar(found.Records, mngr.DNSRecord.Records) {
		logrus.Warnf("%s: is set but contains different records, will be updating it", name)
		err := mngr.UpdateRecord()
		return err == nil, mngr.DNSRecord, err
	}

	logrus.Infof("%s: is already configured, nothing to do", name)

	return false, mngr.DNSRecord, nil
}

// DeleteToDNSProvider will delete a record
func DeleteToDNSProvider(service *v1.Service) (changed bool, record *dnsprovider.DnsRecord, err error) {
	mngr, err := GetManager(service)
	if err != nil {
		return false, nil, err
	}
	// nothing to do
	if mngr == nil {
		return false, nil, nil
	}

	// check if record exists
	name := mngr.ServiceName
	fqdn := mngr.DNSRecord.Fqdn
	found, err := mngr.GetRecord()
	if err != nil {
		return false, nil, fmt.Errorf("%s: could not determine if record '%s' exists: %v", name, fqdn, err)
	}
	if found == nil {
		return false, nil, fmt.Errorf("%s: expected record but it was not found", name)
	}

	logrus.Infof("%s: record found, will be deleting it", name)
	err = mngr.DeleteRecord()
	return err == nil, mngr.DNSRecord, err
}

// GetManager parses the v1.Service object and returns a DNS manager
func GetManager(service *v1.Service) (*DNSController, error) {
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

	dnsProvider, err := dnsprovider.GetProvider(providerStr, rootDomain)
	if err != nil {
		return nil, fmt.Errorf("%s: error getting provider: %v", service.Name, err)
	}
	subDomain := fmt.Sprintf("%s.%s", service.Name, service.Namespace)
	// allow to overwire default subDomain
	if subDomainStr := annotations[subDomainAnnotation]; len(subDomainStr) > 0 {
		subDomain = subDomainStr
	}

	fqdn := fmt.Sprintf("%s.%s", subDomain, rootDomain)

	mngr := DNSController{
		ServiceName: service.Name,
		Provider:    dnsProvider,
		DNSRecord: &dnsprovider.DnsRecord{
			Fqdn:    fqdn,
			Records: records,
			Type:    "A", // always default to "A" for now,
			TTL:     0,   // set it to 0 and let the provider default to minimum
		},
	}

	return &mngr, nil
}

// DNSController handles creating, updating and deleting DNS records
type DNSController struct {
	ServiceName string
	Provider    dnsprovider.Provider
	DNSRecord   *dnsprovider.DnsRecord
}

func (mngr *DNSController) GetRecord() (*dnsprovider.DnsRecord, error) {
	fqdn := mngr.DNSRecord.Fqdn
	return mngr.Provider.GetRecord(fqdn)
}

func (mngr *DNSController) InsertRecord() error {
	return mngr.Provider.AddRecord(*mngr.DNSRecord)
}

func (mngr *DNSController) UpdateRecord() error {
	return mngr.Provider.UpdateRecord(*mngr.DNSRecord)
}

func (mngr *DNSController) DeleteRecord() error {
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
