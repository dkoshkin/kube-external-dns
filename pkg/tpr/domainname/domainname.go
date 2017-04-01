package domainname

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type DomainNameSpec struct {
	ServiceName string `json:"serviceName"`
	Record      Record `json:"record"`
}

type Record struct {
	FQDN      string   `json:"fqdn"`
	Endpoints []string `json:"endpoints"`
	Type      string   `json:"type"`
	TTL       int      `json:"ttl"`
}

type DomainName struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata"`

	Spec DomainNameSpec `json:"spec"`
}

type DomainNameList struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ListMeta `json:"metadata"`

	Items []DomainName `json:"items"`
}

// Required to satisfy Object interface
func (e *DomainName) GetObjectKind() schema.ObjectKind {
	return &e.TypeMeta
}

// Required to satisfy ObjectMetaAccessor interface
func (e *DomainName) GetObjectMeta() metav1.Object {
	return &e.Metadata
}

// Required to satisfy Object interface
func (el *DomainNameList) GetObjectKind() schema.ObjectKind {
	return &el.TypeMeta
}

// Required to satisfy ListMetaAccessor interface
func (el *DomainNameList) GetListMeta() metav1.List {
	return &el.Metadata
}

// The code below is used only to work around a known problem with third-party
// resources and ugorji. If/when these issues are resolved, the code below
// should no longer be required.

type DomainNameCopy DomainName
type DomainNameListCopy DomainNameList

func (e *DomainName) UnmarshalJSON(data []byte) error {
	tmp := DomainNameCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := DomainName(tmp)
	*e = tmp2
	return nil
}

func (el *DomainNameList) UnmarshalJSON(data []byte) error {
	tmp := DomainNameListCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := DomainNameList(tmp)
	*el = tmp2
	return nil
}
