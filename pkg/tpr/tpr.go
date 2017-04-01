package tpr

import (
	"fmt"

	"github.com/Sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

type ThirdPartyResource interface {
	Meta() Metadata
}

type Metadata struct {
	Type        string
	Group       string
	Version     string
	Description string
}

// Initialize
func Initialize(clientset *kubernetes.Clientset, resource ThirdPartyResource) error {
	meta := resource.Meta()
	if meta.Type == "" {
		return fmt.Errorf("Resource Type cannot be empty")
	}
	if meta.Group == "" {
		return fmt.Errorf("Resource Group cannot be empty")
	}
	if meta.Version == "" {
		return fmt.Errorf("Resource Version cannot be empty")
	}
	if meta.Description == "" {
		return fmt.Errorf("Resource Description cannot be empty")
	}
	// must be in this format
	name := fmt.Sprint(meta.Type + "." + meta.Group)
	if _, err := clientset.ExtensionsV1beta1().ThirdPartyResources().Get(name, metav1.GetOptions{}); err != nil {
		logrus.Infof("%s: attempting to initialize TPR", name)
		if errors.IsNotFound(err) {
			tpr := &v1beta1.ThirdPartyResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Versions: []v1beta1.APIVersion{
					{Name: meta.Version},
				},
				Description: meta.Description,
			}

			if _, err := clientset.ExtensionsV1beta1().ThirdPartyResources().Create(tpr); err != nil {
				return fmt.Errorf("%s: error initializing TPR: %v", name, err)
			}
			logrus.Infof("%s: TPR initialized", name)
		} else {
			return fmt.Errorf("%s: error determining if TPR exists: %v", name, err)
		}
	} else {
		logrus.Infof("%s: TPR is already initialized, nothing to do", name)
	}

	return nil
}

// GetClient
func GetClient(baseConfig *rest.Config, types ...runtime.Object) *rest.RESTClient {
	groupversion := schema.GroupVersion{
		Group:   "koshk.in",
		Version: "v1",
	}

	baseConfig.GroupVersion = &groupversion
	baseConfig.APIPath = "/apis"
	baseConfig.ContentType = runtime.ContentTypeJSON
	baseConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	schemeBuilder := runtime.NewSchemeBuilder(
		func(scheme *runtime.Scheme) error {
			scheme.AddKnownTypes(
				groupversion,
				types...,
			)
			return nil
		})
	metav1.AddToGroupVersion(api.Scheme, groupversion)
	schemeBuilder.AddToScheme(api.Scheme)

	client, err := rest.RESTClientFor(baseConfig)
	if err != nil {
		panic(err.Error())
	}

	return client
}
