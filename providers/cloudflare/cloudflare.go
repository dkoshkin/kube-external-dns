package cloudflare

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	api "github.com/crackcomm/cloudflare"
	"github.com/dkoshkin/kube-external-dns/providers"
	"golang.org/x/net/context"
)

type CloudflareProvider struct {
	client *api.Client
	zone   *api.Zone
	ctx    context.Context
	root   string
}

func init() {
	logrus.Info("Registering 'cloudflare' provider")
	providers.RegisterProvider("cloudflare", &CloudflareProvider{})
}

func (c *CloudflareProvider) Init(rootDomainName string) error {
	var email, apiKey string
	if email = os.Getenv("CLOUDFLARE_EMAIL"); len(email) == 0 {
		return fmt.Errorf("CLOUDFLARE_EMAIL is not set")
	}

	if apiKey = os.Getenv("CLOUDFLARE_KEY"); len(apiKey) == 0 {
		return fmt.Errorf("CLOUDFLARE_KEY is not set")
	}

	c.client = api.New(&api.Options{
		Email: email,
		Key:   apiKey,
	})

	c.ctx = context.Background()
	c.root = providers.UnFqdn(rootDomainName)

	if err := c.setZone(); err != nil {
		return fmt.Errorf("Failed to set zone for root domain %s: %v", c.root, err)
	}

	logrus.Infof("Configured %s with zone '%s'", c.GetName(), c.root)
	return nil
}

func (*CloudflareProvider) GetName() string {
	return "CloudFlare"
}

func (c *CloudflareProvider) HealthCheck() error {
	_, err := c.client.Zones.Details(c.ctx, c.zone.ID)
	return err
}

func (c *CloudflareProvider) AddRecord(record providers.DnsRecord) error {
	for _, rec := range record.Records {
		r := c.prepareRecord(record)
		r.Content = rec
		err := c.client.Records.Create(c.ctx, r)
		if err != nil {
			return fmt.Errorf("CloudFlare API call has failed: %v", err)
		}
	}

	return nil
}

func (c *CloudflareProvider) UpdateRecord(record providers.DnsRecord) error {
	if err := c.RemoveRecord(record); err != nil {
		return err
	}

	return c.AddRecord(record)
}

func (c *CloudflareProvider) RemoveRecord(record providers.DnsRecord) error {
	records, err := c.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		err := c.client.Records.Delete(c.ctx, c.zone.ID, rec.ID)
		if err != nil {
			return fmt.Errorf("CloudFlare API call has failed: %v", err)
		}
	}

	return nil
}

func (c *CloudflareProvider) GetRecords() ([]providers.DnsRecord, error) {
	var records []providers.DnsRecord
	result, err := c.client.Records.List(c.ctx, c.zone.ID)
	if err != nil {
		return records, fmt.Errorf("CloudFlare API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range result {
		fqdn := providers.Fqdn(rec.Name)
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
			record := providers.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}

	return records, nil
}

func (c *CloudflareProvider) GetRecord(fqdn string) (*providers.DnsRecord, error) {
	records, err := c.GetRecords()
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		// need to sanitize
		if r.Fqdn == providers.Fqdn(fqdn) {
			logrus.Info(r)
			return &r, nil
		}
	}

	return nil, nil
}

func (c *CloudflareProvider) setZone() error {
	zones, err := c.client.Zones.List(c.ctx)
	if err != nil {
		return fmt.Errorf("CloudFlare API call has failed: %v", err)
	}

	for _, zone := range zones {
		if zone.Name == c.root {
			c.zone = zone
			break
		}
	}
	if c.zone == nil {
		return fmt.Errorf("Zone %s does not exist", c.root)
	}

	return nil
}

func (c *CloudflareProvider) prepareRecord(record providers.DnsRecord) *api.Record {
	name := providers.UnFqdn(record.Fqdn)
	return &api.Record{
		Type:   record.Type,
		Name:   name,
		TTL:    sanitizeTTL(record.TTL),
		ZoneID: c.zone.ID,
	}
}

func (c *CloudflareProvider) findRecords(record providers.DnsRecord) ([]*api.Record, error) {
	var records []*api.Record
	result, err := c.client.Records.List(c.ctx, c.zone.ID)
	if err != nil {
		return records, fmt.Errorf("CloudFlare API call has failed: %v", err)
	}

	name := providers.UnFqdn(record.Fqdn)
	for _, rec := range result {
		if rec.Name == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

// TTL must be between 120 and 86400 seconds
func sanitizeTTL(ttl int) int {
	if ttl < 120 {
		logrus.Warn("Setting TTL to 120 seconds")
		ttl = 120
	} else if ttl > 86400 {
		logrus.Warn("Adjusting TTL to 86400 seconds")
		ttl = 86400
	}
	return ttl
}
