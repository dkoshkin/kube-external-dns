package digitalocean

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	api "github.com/digitalocean/godo"
	"golang.org/x/oauth2"

	"github.com/dkoshkin/kube-external-dns/pkg/provider/dns"
	"github.com/juju/ratelimit"
)

type DigitalOceanProvider struct {
	client         *api.Client
	rootDomainName string
	limiter        *ratelimit.Bucket
}

const TTL = 120

func init() {
	dns.RegisterProvider("digitalocean", &DigitalOceanProvider{})
}

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func (p *DigitalOceanProvider) Init(rootDomainName string) error {
	var pat string
	if pat = os.Getenv("DO_PAT"); len(pat) == 0 {
		return fmt.Errorf("DO_PAT is not set")
	}

	tokenSource := &TokenSource{
		AccessToken: pat,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	p.client = api.NewClient(oauthClient)

	// DO's API is rate limited at 5000/hour.
	doqps := (float64)(5000.0 / 3600.0)
	p.limiter = ratelimit.NewBucketWithRate(doqps, 100)

	p.rootDomainName = dns.UnFqdn(rootDomainName)

	// Retrieve email address associated with this PAT.
	p.limiter.Wait(1)
	acct, _, err := p.client.Account.Get(oauth2.NoContext)
	if err != nil {
		return err
	}

	// Now confirm that domain is accessible under this PAT.
	p.limiter.Wait(1)
	domains, _, err := p.client.Domains.Get(oauth2.NoContext, p.rootDomainName)
	if err != nil {
		return err
	}

	// DO's TTLs are domain-wide.
	logrus.Infof("Configured %s with email %s and domain %s", p.GetName(), acct.Email, domains.Name)
	return nil
}

func (p *DigitalOceanProvider) GetName() string {
	return "DigitalOcean"
}

func (p *DigitalOceanProvider) HealthCheck() error {
	p.limiter.Wait(1)
	_, _, err := p.client.Domains.Get(oauth2.NoContext, p.rootDomainName)
	return err
}

func (p *DigitalOceanProvider) AddRecord(record dns.DnsRecord) error {
	for _, r := range record.Records {
		createRequest := &api.DomainRecordEditRequest{
			Type: record.Type,
			Name: record.Fqdn,
			Data: r,
		}

		logrus.Debugf("Creating record: %v", createRequest)
		p.limiter.Wait(1)
		_, _, err := p.client.Domains.CreateRecord(oauth2.NoContext, p.rootDomainName, createRequest)
		if err != nil {
			return fmt.Errorf("API call has failed: %v", err)
		}
	}

	return nil
}

func (p *DigitalOceanProvider) UpdateRecord(record dns.DnsRecord) error {
	if err := p.RemoveRecord(record); err != nil {
		return err
	}

	return p.AddRecord(record)
}

func (p *DigitalOceanProvider) RemoveRecord(record dns.DnsRecord) error {
	// We need to fetch paginated results to get all records
	doRecords, err := p.fetchDoRecords()
	if err != nil {
		return fmt.Errorf("RemoveRecord: %v", err)
	}

	for _, rec := range doRecords {
		// DO records don't have fully-qualified names like ours
		fqdn := p.nameToFqdn(rec.Name)
		if fqdn == record.Fqdn && rec.Type == record.Type {
			p.limiter.Wait(1)
			logrus.Debugf("Deleting record: %v", rec)
			_, err := p.client.Domains.DeleteRecord(oauth2.NoContext, p.rootDomainName, rec.ID)
			if err != nil {
				return fmt.Errorf("API call has failed: %v", err)
			}
		}
	}

	return nil
}

func (p *DigitalOceanProvider) GetRecords() ([]dns.DnsRecord, error) {
	dnsRecords := []dns.DnsRecord{}
	recordMap := map[string]map[string][]string{}
	doRecords, err := p.fetchDoRecords()
	if err != nil {
		return nil, fmt.Errorf("GetRecords: %v", err)
	}

	for _, rec := range doRecords {
		fqdn := p.nameToFqdn(rec.Name)
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Data)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Data}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Data}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			// DigitalOcean does not have per-record TTLs.
			dnsRecord := dns.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: TTL}
			dnsRecords = append(dnsRecords, dnsRecord)
		}
	}

	return dnsRecords, nil
}

func (c *DigitalOceanProvider) GetRecord(fqdn string) (*dns.DnsRecord, error) {
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

// fetchDoRecords retrieves all records for the root domain from Digital Ocean.
func (p *DigitalOceanProvider) fetchDoRecords() ([]api.DomainRecord, error) {
	doRecords := []api.DomainRecord{}
	opt := &api.ListOptions{
		// Use the maximum of 200 records per page
		PerPage: 200,
	}
	for {
		p.limiter.Wait(1)
		records, resp, err := p.client.Domains.Records(oauth2.NoContext, p.rootDomainName, opt)
		if err != nil {
			return nil, fmt.Errorf("API call has failed: %v", err)
		}

		if len(records) > 0 {
			doRecords = append(doRecords, records...)
		}

		if resp.Links == nil || resp.Links.IsLastPage() || len(records) == 0 {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, fmt.Errorf("Failed to get current page: %v", err)
		}

		opt.Page = page + 1
	}

	logrus.Debugf("Fetched %d DO records", len(doRecords))
	return doRecords, nil
}

func (p *DigitalOceanProvider) nameToFqdn(name string) string {
	var fqdn string
	if name == "@" {
		fqdn = p.rootDomainName
	} else {
		names := []string{name, p.rootDomainName}
		fqdn = strings.Join(names, ".")
	}

	return dns.Fqdn(fqdn)
}
