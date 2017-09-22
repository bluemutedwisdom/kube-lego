package kubelego

import (
	"reflect"
	"time"

	"github.com/Shopify/kube-lego/pkg/ingress"

	k8sMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	k8sExtensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/Sirupsen/logrus"
)

func ingressListFunc(c *kubernetes.Clientset, ns string) func(k8sMeta.ListOptions) (runtime.Object, error) {
	return func(opts k8sMeta.ListOptions) (runtime.Object, error) {
		return c.Extensions().Ingresses(ns).List(opts)
	}
}

func ingressWatchFunc(c *kubernetes.Clientset, ns string) func(options k8sMeta.ListOptions) (watch.Interface, error) {
	return func(options k8sMeta.ListOptions) (watch.Interface, error) {
		return c.Extensions().Ingresses(ns).Watch(options)
	}
}

func (kl *KubeLego) requestReconfigureForIngress(obj interface{}, event string) {
	ingressApi := obj.(*k8sExtensions.Ingress)
	logger := kl.Log().WithFields(logrus.Fields{
		"ingress":   ingressApi.Name,
		"namespace": ingressApi.Namespace,
	})

	logger.Debugf("%s event triggered", event)

	if err := ingress.IgnoreIngress(ingressApi); err != nil {
		kl.Log().WithFields(logrus.Fields{
			"ingress":   ingressApi.Name,
			"namespace": ingressApi.Namespace,
		}).Info("ignoring as ", err)

		return
	}

	kl.requestReconfigure()
}

func (kl *KubeLego) requestReconfigure() {
	kl.workQueue.Add(true)
}

func (kl *KubeLego) WatchReconfigure() {

	kl.workQueue = workqueue.New()

	// handle worker shutdown
	go func() {
		<-kl.stopCh
		kl.workQueue.ShutDown()
	}()

	go func() {
		kl.waitGroup.Add(1)
		defer kl.waitGroup.Done()
		for {
			item, quit := kl.workQueue.Get()
			if quit {
				return
			}
			kl.Log().Debugf("worker: begin processing %v", item)
			kl.Reconfigure()
			kl.Log().Debugf("worker: done processing %v", item)
			kl.workQueue.Done(item)
		}
	}()
}

func (kl *KubeLego) WatchEvents() {

	kl.Log().Debugf("start watching ingress objects")

	resyncPeriod := 60 * time.Second

	ingEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			kl.requestReconfigureForIngress(obj, "CREATE")
		},
		DeleteFunc: func(obj interface{}) {
			kl.requestReconfigureForIngress(obj, "DELETE")
		},
		UpdateFunc: func(old, cur interface{}) {
			oldIng := old.(*k8sExtensions.Ingress)
			upIng := cur.(*k8sExtensions.Ingress)

			//ignore resource version in equality check
			oldIng.ResourceVersion = ""
			upIng.ResourceVersion = ""

			if !reflect.DeepEqual(oldIng, upIng) {
				kl.requestReconfigureForIngress(cur, "UPDATE")
			}
		},
	}

	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  ingressListFunc(kl.kubeClient, kl.legoWatchNamespace),
			WatchFunc: ingressWatchFunc(kl.kubeClient, kl.legoWatchNamespace),
		},
		&k8sExtensions.Ingress{},
		resyncPeriod,
		ingEventHandler,
	)

	go controller.Run(kl.stopCh)
}
