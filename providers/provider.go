package providers

import (
	"fmt"

	"github.com/Sirupsen/logrus"
)

type Provider interface {
	Init(rootDomainName string) error
	GetName() string
	HealthCheck() error
	AddRecord(record DnsRecord) error
	RemoveRecord(record DnsRecord) error
	UpdateRecord(record DnsRecord) error
	GetRecords() ([]DnsRecord, error)
	GetRecord(fqdn string) (*DnsRecord, error)
}

var (
	providers = make(map[string]Provider)
)

func GetProvider(name, rootDomainName string) (Provider, error) {
	if provider, ok := providers[name]; ok {
		if err := provider.Init(rootDomainName); err != nil {
			return nil, err
		}
		return provider, nil
	}
	return nil, fmt.Errorf("No such provider '%s'", name)
}

func RegisterProvider(name string, provider Provider) {
	if _, exists := providers[name]; exists {
		logrus.Errorf("Provider '%s' tried to register twice", name)
	}
	providers[name] = provider
}

type DnsRecord struct {
	Fqdn    string
	Records []string
	Type    string
	TTL     int
}

// Fqdn ensures that the name is a fqdn adding a trailing dot if necessary.
func Fqdn(name string) string {
	n := len(name)
	if n == 0 || name[n-1] == '.' {
		return name
	}
	return name + "."
}

// UnFqdn converts the fqdn into a name removing the trailing dot.
func UnFqdn(name string) string {
	n := len(name)
	if n != 0 && name[n-1] == '.' {
		return name[:n-1]
	}
	return name
}
