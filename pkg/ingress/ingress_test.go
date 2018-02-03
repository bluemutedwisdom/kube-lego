package ingress

import (
	"regexp"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	k8sExtensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"

	kubelego "github.com/jetstack/kube-lego/pkg/kubelego_const"
	"github.com/jetstack/kube-lego/pkg/mocks"
)

func TestFilterTlsHosts(t *testing.T) {
	ctrlMock := gomock.NewController(t)
	defer ctrlMock.Finish()

	ing := &Ingress{
		IngressApi: &k8sExtensions.Ingress{
			Spec: k8sExtensions.IngressSpec{
				TLS: []k8sExtensions.IngressTLS{
					k8sExtensions.IngressTLS{
						Hosts:      []string{"domain1", "sub.custom-domain.ca"},
						SecretName: "secret1",
					},
					k8sExtensions.IngressTLS{
						Hosts:      []string{"*.domain.com"},
						SecretName: "secret2",
					},
				},
			},
		},
	}
	ing.kubelego = mocks.DummyKubeLego(ctrlMock)

	assert.Equal(t, []string{"domain1", "sub.custom-domain.ca"}, ing.Tls()[0].Hosts())
	assert.Equal(t, []string{"*.domain.com"}, ing.Tls()[1].Hosts())

	filters := []*regexp.Regexp{}

	ing.FilterTlsHosts(filters)
	assert.Equal(t, []string{"domain1", "sub.custom-domain.ca"}, ing.Tls()[0].Hosts())
	assert.Equal(t, []string{"*.domain.com"}, ing.Tls()[1].Hosts())

	subDomainsFilter, err := regexp.Compile(".*\\.custom-domain\\.ca$")
	assert.Nil(t, err)
	filters = append(filters, subDomainsFilter)
	ing.FilterTlsHosts(filters)
	assert.Equal(t, []string{"domain1"}, ing.Tls()[0].Hosts())
	assert.Equal(t, []string{"*.domain.com"}, ing.Tls()[1].Hosts())

	wildcardDomainsFilter, err := regexp.Compile("^\\*\\..*")
	assert.Nil(t, err)
	filters = append(filters, wildcardDomainsFilter)
	ing.FilterTlsHosts(filters)
	assert.Equal(t, []string{"domain1"}, ing.Tls()[0].Hosts())
	assert.Equal(t, []string{}, ing.Tls()[1].Hosts())
}

func TestIsSupportedIngressClass(t *testing.T) {
	supportedClass := []string{"nginx", "gce", "custom"}
	out, err := IsSupportedIngressClass(supportedClass, "Nginx")
	assert.Equal(t, "nginx", out)
	assert.Nil(t, err)

	out, err = IsSupportedIngressClass(supportedClass, "customlb")
	assert.NotNil(t, err)

	out, err = IsSupportedIngressClass(supportedClass, "gce")
	assert.Equal(t, "gce", out)
	assert.Nil(t, err)

}

func TestIngress_Tls(t *testing.T) {
	ing := &Ingress{
		IngressApi: &k8sExtensions.Ingress{
			Spec: k8sExtensions.IngressSpec{
				TLS: []k8sExtensions.IngressTLS{
					k8sExtensions.IngressTLS{
						Hosts:      []string{"domain1", "domain2"},
						SecretName: "secret1",
					},
					k8sExtensions.IngressTLS{
						Hosts:      []string{"domain3"},
						SecretName: "secret2",
					},
				},
			},
		},
	}

	assert.Equal(t, 2, len(ing.Tls()))

	found := 0

	for _, tls := range ing.Tls() {
		if tls.SecretMetadata().Name == "secret1" {
			found++
			assert.Equal(t, []string{"domain1", "domain2"}, tls.Hosts())
		}
		if tls.SecretMetadata().Name == "secret2" {
			found++
			assert.Equal(t, []string{"domain3"}, tls.Hosts())
		}
	}

	assert.Equal(t, 2, found)
}

// ensure ingress provider compatability
func TestIngress_Provider(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// fake kubelego
	mockKL := mocks.NewMockKubeLego(ctrl)
	mockKL.EXPECT().LegoDefaultIngressClass().AnyTimes().Return("default-class")
	mockKL.EXPECT().LegoDefaultIngressProvider().AnyTimes().Return("default-provider")

	// ing with no annotations
	ingNo := &Ingress{
		IngressApi: &k8sExtensions.Ingress{},
		kubelego:   mockKL,
	}
	assert.Equal(t, "default-provider", ingNo.IngressProvider())

	// ing with class gce|nginx annotations
	ingClass := &Ingress{
		IngressApi: &k8sExtensions.Ingress{},
		kubelego:   mockKL,
	}

	ingClass.IngressApi.Annotations = map[string]string{
		kubelego.AnnotationIngressClass: "gce",
	}
	assert.Equal(t, "gce", ingClass.IngressProvider())

	ingClass.IngressApi.Annotations = map[string]string{
		kubelego.AnnotationIngressClass: "nginx",
	}
	assert.Equal(t, "nginx", ingClass.IngressProvider())

	ingClass.IngressApi.Annotations = map[string]string{
		kubelego.AnnotationIngressClass: "my-class",
	}
	assert.Equal(t, "default-provider", ingClass.IngressProvider())

}
