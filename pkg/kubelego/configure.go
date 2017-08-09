package kubelego

import (
	"github.com/Shopify/kube-lego/pkg/ingress"
	"github.com/Shopify/kube-lego/pkg/kubelego_const"
	"github.com/Shopify/kube-lego/pkg/utils"

	"fmt"
	"strings"
)

func (kl *KubeLego) TlsFilterHosts(tlsSlice []kubelego.Tls) []kubelego.Tls {

	output := []kubelego.Tls{}
	for _, elm := range tlsSlice {
		hosts := []string{}

		for _, host := range elm.Hosts() {
			if utils.RegexpSliceMatchString(kl.LegoHostFilterRegexps(), host) {
				kl.Log().Infof("ignoring host %s because it matched a regexp", host)
				continue
			}

			hosts = append(hosts, host)
		}

		elm.SetHosts(hosts)

		output = append(output, elm)
	}

	return output
}

func (kl *KubeLego) TlsIgnoreDuplicatedSecrets(tlsSlice []kubelego.Tls) []kubelego.Tls {

	tlsBySecret := map[string][]kubelego.Tls{}

	for _, elm := range tlsSlice {
		key := fmt.Sprintf(
			"%s/%s",
			elm.SecretMetadata().Namespace,
			elm.SecretMetadata().Name,
		)
		tlsBySecret[key] = append(
			tlsBySecret[key],
			elm,
		)
	}

	output := []kubelego.Tls{}
	for key, slice := range tlsBySecret {
		if len(slice) == 1 {
			output = append(output, slice...)
			continue
		}

		texts := []string{}
		for _, elem := range slice {
			texts = append(
				texts,
				fmt.Sprintf(
					"ingress %s/%s (hosts: %s)",
					elem.IngressMetadata().Namespace,
					elem.IngressMetadata().Name,
					strings.Join(elem.Hosts(), ", "),
				),
			)
		}
		kl.Log().Warnf(
			"the secret %s is used multiple times. These linked TLS ingress elements where ignored: %s",
			key,
			strings.Join(texts, ", "),
		)
	}

	return output
}

func (kl *KubeLego) processProvider(ings []kubelego.Ingress) (err error) {

	for providerName, provider := range kl.legoIngressProvider {
		err := provider.Reset()
		if err != nil {
			provider.Log().Error(err)
			continue
		}

		for _, ing := range ings {
			if providerName == ing.IngressProvider() {
				err = provider.Process(ing)
				if err != nil {
					provider.Log().Error(err)
				}
			}
		}

		err = provider.Finalize()
		if err != nil {
			provider.Log().Error(err)
		}
	}
	return nil
}

func (kl *KubeLego) reconfigure(ingressesAll []kubelego.Ingress) error {
	tlsSlice := []kubelego.Tls{}
	ingresses := []kubelego.Ingress{}

	// filter ingresses, collect tls names
	for _, ing := range ingressesAll {
		if ing.Ignore() {
			continue
		}
		tlsSlice = append(tlsSlice, ing.Tls()...)
		ingresses = append(ingresses, ing)
	}

	// setup providers
	kl.processProvider(ingresses)

	// normify tls config
	tlsSlice = kl.TlsIgnoreDuplicatedSecrets(tlsSlice)

	// filter hosts
	tlsSlice = kl.TlsFilterHosts(tlsSlice)

	// process certificate validity
	kl.Log().Info("process certificate requests for ingresses")
	errs := kl.TlsProcessHosts(tlsSlice)
	if len(errs) > 0 {
		errsStr := []string{}
		for _, err := range errs {
			errsStr = append(errsStr, fmt.Sprintf("%s", err))
		}
		kl.Log().Error("Error while processing certificate requests: ", strings.Join(errsStr, ", "))

		// request a rerun of reconfigure
		kl.workQueue.Add(true)
	}

	return nil
}

func (kl *KubeLego) Reconfigure() error {
	ingressesAll, err := ingress.All(kl)
	if err != nil {
		return err
	}

	return kl.reconfigure(ingressesAll)
}

func (kl *KubeLego) TlsProcessHosts(tlsSlice []kubelego.Tls) []error {
	errs := []error{}
	for _, tlsElem := range tlsSlice {
		err := tlsElem.Process()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
