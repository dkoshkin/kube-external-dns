package dnsimple

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/dkoshkin/kube-external-dns/pkg/provider/dns"
	"github.com/juju/ratelimit"
	api "github.com/weppos/go-dnsimple/dnsimple"
)

type DNSimpleProvider struct {
	client  *api.Client
	root    string
	limiter *ratelimit.Bucket
}

func init() {
	logrus.Info("Registering 'dnsimple' provider")
	dns.RegisterProvider("dnsimple", &DNSimpleProvider{})
}

func (d *DNSimpleProvider) Init(rootDomainName string) error {
	var email, apiToken string
	if email = os.Getenv("DNSIMPLE_EMAIL"); len(email) == 0 {
		return fmt.Errorf("DNSIMPLE_EMAIL is not set")
	}

	if apiToken = os.Getenv("DNSIMPLE_TOKEN"); len(apiToken) == 0 {
		return fmt.Errorf("DNSIMPLE_TOKEN is not set")
	}

	d.root = dns.UnFqdn(rootDomainName)
	d.client = api.NewClient(apiToken, email)
	d.limiter = ratelimit.NewBucketWithRate(1.5, 5)

	domains, _, err := d.client.Domains.List()
	if err != nil {
		return fmt.Errorf("Failed to list zones: %v", err)
	}

	found := false
	for _, domain := range domains {
		if domain.Name == d.root {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Zone for '%s' not found", d.root)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

func (*DNSimpleProvider) GetName() string {
	return "DNSimple"
}

func (d *DNSimpleProvider) HealthCheck() error {
	d.limiter.Wait(1)
	_, _, err := d.client.Users.User()
	return err
}

func (d *DNSimpleProvider) parseName(record dns.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", d.root))
	return name
}

func (d *DNSimpleProvider) AddRecord(record dns.DnsRecord) error {
	name := d.parseName(record)
	for _, rec := range record.Records {
		recordInput := api.Record{
			Name:    name,
			TTL:     record.TTL,
			Type:    record.Type,
			Content: rec,
		}
		d.limiter.Wait(1)
		_, _, err := d.client.Domains.CreateRecord(d.root, recordInput)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleProvider) findRecords(record dns.DnsRecord) ([]api.Record, error) {
	var records []api.Record

	d.limiter.Wait(1)
	resp, _, err := d.client.Domains.ListRecords(d.root, "", "")
	if err != nil {
		return records, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	name := d.parseName(record)
	for _, rec := range resp {
		if rec.Name == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

func (d *DNSimpleProvider) UpdateRecord(record dns.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *DNSimpleProvider) RemoveRecord(record dns.DnsRecord) error {
	records, err := d.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		d.limiter.Wait(1)
		_, err := d.client.Domains.DeleteRecord(d.root, rec.Id)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleProvider) GetRecords() ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord

	d.limiter.Wait(1)
	recordResp, _, err := d.client.Domains.ListRecords(d.root, "", "")
	if err != nil {
		return records, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range recordResp {
		var fqdn string
		if rec.Name == "" {
			fqdn = d.root + "."
		} else {
			fqdn = fmt.Sprintf("%s.%s.", rec.Name, d.root)
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.Type] = rec.TTL
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Content)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Content}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Content}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			ttl := recordTTLs[fqdn][recordType]
			record := dns.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}
	return records, nil
}

func (c *DNSimpleProvider) GetRecord(fqdn string) (*dns.DnsRecord, error) {
	records, err := c.GetRecords()
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		// need to sanitize
		if r.Fqdn == dns.Fqdn(fqdn) {
			logrus.Info(r)
			return &r, nil
		}
	}

	return nil, nil
}
