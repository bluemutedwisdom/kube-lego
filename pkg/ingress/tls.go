package ingress

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/kube-lego/pkg/kubelego_const"
	"github.com/Shopify/kube-lego/pkg/secret"
	"github.com/Shopify/kube-lego/pkg/utils"

	"github.com/Sirupsen/logrus"
	k8sApi "k8s.io/client-go/pkg/api/v1"
	k8sExtensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

var _ kubelego.Tls = &Tls{}

type Tls struct {
	*k8sExtensions.IngressTLS
	ingress kubelego.Ingress
	secret  kubelego.Secret
}

func (t Tls) SecretMetadata() (meta *k8sApi.ObjectMeta) {
	return &k8sApi.ObjectMeta{
		Namespace: t.ingress.Object().Namespace,
		Name:      t.SecretName,
	}
}

func (t Tls) IngressMetadata() (meta *k8sApi.ObjectMeta) {
	return &k8sApi.ObjectMeta{
		Namespace: t.ingress.Object().Namespace,
		Name:      t.ingress.Object().Name,
	}
}

func (t *Tls) Secret() kubelego.Secret {
	if t.secret != nil {
		return t.secret
	}
	meta := t.SecretMetadata()
	t.secret = secret.New(t.ingress.KubeLego(), meta.Namespace, meta.Name)
	return t.secret
}

func (t *Tls) Hosts() []string {
	return utils.StringSliceLowerCase(t.IngressTLS.Hosts)
}

func (t *Tls) Log() *logrus.Entry {
	return t.ingress.Log().WithField("context", "ingress_tls")
}

func (i *Tls) newCertNeeded() bool {
	if len(i.Hosts()) == 0 {
		i.Log().Info("no host associated with ingress")
		return false
	}

	tlsSecret := i.Secret()
	logger := i.Log().WithFields(logrus.Fields{
		"secret_name": tlsSecret.Object().Name,
		"domains":     i.Hosts(),
	})

	if !tlsSecret.Exists() {
		i.Log().Info("no cert associated with ingress")
		return true
	}

	if !tlsSecret.TlsDomainsInclude(i.Hosts()) {
		logger.Info("cert does not cover all domains")
		return true
	}

	expireTime, err := tlsSecret.TlsExpireTime()
	if err != nil {
		logger.Warn("error while reading expiry time: ", err)
		return true
	}
	minimumValidity := i.ingress.KubeLego().LegoMinimumValidity()
	logger = logger.WithFields(logrus.Fields{
		"expire_time":      expireTime.String(),
		"minimum_validity": minimumValidity.String(),
	})
	timeLeft := expireTime.Sub(time.Now())
	if timeLeft < minimumValidity {
		logger.Infof("cert expires soon so renew")
		return true
	} else {
		logger.Infof("cert expires in %.1f days, no renewal needed", timeLeft.Hours()/24)
	}

	return false
}

func (i *Tls) Process() error {

	if !i.newCertNeeded() {
		i.Log().Debug("no cert request needed")
		return nil
	}

	err := i.RequestCert()
	if err != nil {
		i.Log().Error("failed to request certificate", err)
	}

	return err
}

func (i *Tls) RequestCert() error {
	// sanity check
	if i.IngressTLS.SecretName == "" {
		return errors.New("Ingress has an empty secretName. Skipping certificate retrieval")
	}

	i.Log().Infof("requesting certificate for %s", strings.Join(i.Hosts(), ","))

	certData, err := i.ingress.KubeLego().AcmeClient().ObtainCertificate(
		i.Hosts(),
	)
	if err != nil {
		return err
	}

	s := i.Secret()
	s.Object().Annotations = map[string]string{
		kubelego.AnnotationEnabled: "true",
	}
	s.Object().Type = k8sApi.SecretTypeTLS

	s.Object().Data = certData

	return s.Save()
}
