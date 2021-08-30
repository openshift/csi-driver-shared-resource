package controller

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	storagev1alpha1 "github.com/openshift/api/storage/v1alpha1"
	storageclient "github.com/openshift/client-go/storage/clientset/versioned"
	storageinformer "github.com/openshift/client-go/storage/informers/externalversions"

	"github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
)

const (
	DefaultResyncDuration = 10 * time.Minute
)

type Controller struct {
	kubeRestConfig *rest.Config

	kubeClient *kubernetes.Clientset

	cfgMapWorkqueue workqueue.RateLimitingInterface
	secretWorkqueue workqueue.RateLimitingInterface
	shareWorkqueue  workqueue.RateLimitingInterface

	cfgMapInformer kcache.SharedIndexInformer
	secInformer    kcache.SharedIndexInformer
	shareInformer  kcache.SharedIndexInformer

	shareInformerFactory storageinformer.SharedInformerFactory
	informerFactory      informers.SharedInformerFactory

	listers *client.Listers
}

// NewController instantiate a new controller with relisting interval, and optional refresh-resources
// mode. Refresh-resources mode means the controller will keep watching for ConfigMaps and Secrets
// for future changes, when disabled it only loads the resource contents before mounting the volume.
func NewController(shareRelist time.Duration, refreshResources bool, ignoredNamespaces []string) (*Controller, error) {
	kubeRestConfig, err := client.GetConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeRestConfig)
	if err != nil {
		return nil, err
	}

	shareClient, err := storageclient.NewForConfig(kubeRestConfig)
	if err != nil {
		return nil, err
	}

	// NOTE, not specifying a namespace defaults to metav1.NamespaceAll in
	// informers.NewSharedInformerFactoryWithOptions, but we restrict OpenShift
	// "system" namespaces with chatty configmaps like the leaderelection related ones
	// that are updated every few seconds
	tweakListOptions := internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
		ignored := []string{}
		for _, ns := range ignoredNamespaces {
			klog.V(4).Infof("namespace %q is being ignored", ns)
			ignored = append(ignored, fmt.Sprintf("metadata.namespace!=%s", ns))
		}
		options.FieldSelector = strings.Join(ignored, ",")
	})
	informerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient,
		DefaultResyncDuration, informers.WithTweakListOptions(tweakListOptions))

	klog.V(5).Infof("configured share relist %v", shareRelist)
	shareInformerFactory := storageinformer.NewSharedInformerFactoryWithOptions(shareClient,
		shareRelist)

	c := &Controller{
		kubeClient:     kubeClient,
		kubeRestConfig: kubeRestConfig,
		shareWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),
			"shared-resource-share-changes"),
		informerFactory:      informerFactory,
		shareInformerFactory: shareInformerFactory,
		shareInformer:        shareInformerFactory.Storage().V1alpha1().SharedResources().Informer(),
		listers:              client.GetListers(),
	}

	if refreshResources {
		c.cfgMapWorkqueue = workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), "shared-resource-configmap-changes")
		c.secretWorkqueue = workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), "shared-resource-secret-changes")
		c.cfgMapInformer = informerFactory.Core().V1().ConfigMaps().Informer()
		c.secInformer = informerFactory.Core().V1().Secrets().Informer()

		client.SetConfigMapsLister(c.informerFactory.Core().V1().ConfigMaps().Lister())
		client.SetSecretsLister(c.informerFactory.Core().V1().Secrets().Lister())
	}

	client.SetSharesLister(c.shareInformerFactory.Storage().V1alpha1().SharedResources().Lister())

	if refreshResources {
		c.cfgMapInformer.AddEventHandler(c.configMapEventHandler())
		c.secInformer.AddEventHandler(c.secretEventHandler())
	}
	c.shareInformer.AddEventHandler(c.shareEventHandler())

	return c, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	if c.cfgMapWorkqueue != nil && c.secretWorkqueue != nil {
		defer c.cfgMapWorkqueue.ShutDown()
		defer c.secretWorkqueue.ShutDown()
	}
	defer c.shareWorkqueue.ShutDown()

	c.informerFactory.Start(stopCh)
	c.shareInformerFactory.Start(stopCh)

	if c.cfgMapInformer != nil && !kcache.WaitForCacheSync(stopCh, c.cfgMapInformer.HasSynced) {
		return fmt.Errorf("failed to wait for ConfigMap informer cache to sync")
	}
	if c.secInformer != nil && !kcache.WaitForCacheSync(stopCh, c.secInformer.HasSynced) {
		return fmt.Errorf("failed to wait for Secrets informer cache to sync")
	}
	if !kcache.WaitForCacheSync(stopCh, c.shareInformer.HasSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	if c.cfgMapWorkqueue != nil {
		go wait.Until(c.configMapEventProcessor, time.Second, stopCh)
	}
	if c.secretWorkqueue != nil {
		go wait.Until(c.secretEventProcessor, time.Second, stopCh)
	}
	go wait.Until(c.shareEventProcessor, time.Second, stopCh)

	<-stopCh

	return nil
}

func (c *Controller) addConfigMapToQueue(cm *corev1.ConfigMap, verb client.ObjectAction) {
	event := client.Event{
		Object: cm,
		Verb:   verb,
	}
	c.cfgMapWorkqueue.Add(event)
}

// as the actions we have to take *MAY* vary to a significant enough degree between add, update, and delete,
// especially if we move off of vanilla os.MkdirAll / os.Create, we propagate that distinction down the line
func (c *Controller) configMapEventHandler() kcache.ResourceEventHandlerFuncs {
	return kcache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			switch v := o.(type) {
			case *corev1.ConfigMap:
				c.addConfigMapToQueue(v, client.AddObjectAction)
			default:
				//log unrecognized type
			}
		},
		UpdateFunc: func(o, n interface{}) {
			switch v := n.(type) {
			case *corev1.ConfigMap:
				c.addConfigMapToQueue(v, client.UpdateObjectAction)
			default:
				//log unrecognized type
			}
		},
		DeleteFunc: func(o interface{}) {
			switch v := o.(type) {
			case kcache.DeletedFinalStateUnknown:
				switch vv := v.Obj.(type) {
				case *corev1.ConfigMap:
					// log recovered deleted obj from tombstone via vv.GetName()
					c.addConfigMapToQueue(vv, client.DeleteObjectAction)
				default:
					// log  error decoding obj tombstone
				}
			case *corev1.ConfigMap:
				c.addConfigMapToQueue(v, client.DeleteObjectAction)
			default:
				//log unrecognized type
			}
		},
	}
}

func (c *Controller) configMapEventProcessor() {
	for {
		obj, shutdown := c.cfgMapWorkqueue.Get()
		if shutdown {
			return
		}

		func() {
			defer c.cfgMapWorkqueue.Done(obj)

			event, ok := obj.(client.Event)
			if !ok {
				c.cfgMapWorkqueue.Forget(obj)
				return
			}

			if err := c.syncConfigMap(event); err != nil {
				c.cfgMapWorkqueue.AddRateLimited(obj)
			} else {
				c.cfgMapWorkqueue.Forget(obj)
			}
		}()
	}
}

func (c *Controller) syncConfigMap(event client.Event) error {
	obj := event.Object.DeepCopyObject()
	cm, ok := obj.(*corev1.ConfigMap)
	if cm == nil || !ok {
		msg := fmt.Sprintf("unexpected object vs. configmap: %v", event.Object.GetObjectKind().GroupVersionKind())
		fmt.Print(msg)
		return fmt.Errorf(msg)
	}
	klog.V(5).Infof("verb %s obj namespace %s configmap name %s", event.Verb, cm.Namespace, cm.Name)
	switch event.Verb {
	case client.DeleteObjectAction:
		cache.DelConfigMap(cm)
	case client.AddObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		cache.UpsertConfigMap(cm)
	case client.UpdateObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		cache.UpsertConfigMap(cm)
	default:
		return fmt.Errorf("unexpected configmap event action: %s", event.Verb)
	}
	return nil
}

func (c *Controller) addSecretToQueue(s *corev1.Secret, verb client.ObjectAction) {
	event := client.Event{
		Object: s,
		Verb:   verb,
	}
	c.secretWorkqueue.Add(event)
}

// as the actions we have to take vary to a significant enough degree between add, update, and delete
// we propagate that distinction down the line
func (c *Controller) secretEventHandler() kcache.ResourceEventHandlerFuncs {
	return kcache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			switch v := o.(type) {
			case *corev1.Secret:
				c.addSecretToQueue(v, client.AddObjectAction)
			default:
				//log unrecognized type
			}
		},
		UpdateFunc: func(o, n interface{}) {
			switch v := n.(type) {
			case *corev1.Secret:
				c.addSecretToQueue(v, client.UpdateObjectAction)
			default:
				//log unrecognized type
			}
		},
		DeleteFunc: func(o interface{}) {
			switch v := o.(type) {
			case kcache.DeletedFinalStateUnknown:
				switch vv := v.Obj.(type) {
				case *corev1.Secret:
					// log recovered deleted obj from tombstone via vv.GetName()
					c.addSecretToQueue(vv, client.DeleteObjectAction)
				default:
					// log  error decoding obj tombstone
				}
			case *corev1.Secret:
				c.addSecretToQueue(v, client.DeleteObjectAction)
			default:
				//log unrecognized type
			}
		},
	}
}

func (c *Controller) secretEventProcessor() {
	for {
		obj, shutdown := c.secretWorkqueue.Get()
		if shutdown {
			return
		}

		func() {
			defer c.secretWorkqueue.Done(obj)

			event, ok := obj.(client.Event)
			if !ok {
				c.secretWorkqueue.Forget(obj)
				return
			}

			if err := c.syncSecret(event); err != nil {
				c.secretWorkqueue.AddRateLimited(obj)
			} else {
				c.secretWorkqueue.Forget(obj)
			}
		}()
	}
}

func (c *Controller) syncSecret(event client.Event) error {
	obj := event.Object.DeepCopyObject()
	secret, ok := obj.(*corev1.Secret)
	if secret == nil || !ok {
		return fmt.Errorf("unexpected object vs. secret: %v", event.Object.GetObjectKind().GroupVersionKind())
	}
	// since we don't mutate we do not copy
	klog.V(5).Infof("verb %s obj namespace %s secret name %s", event.Verb, secret.Namespace, secret.Name)
	switch event.Verb {
	case client.DeleteObjectAction:
		cache.DelSecret(secret)
	case client.AddObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		cache.UpsertSecret(secret)
	case client.UpdateObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		cache.UpsertSecret(secret)
	default:
		return fmt.Errorf("unexpected secret event action: %s", event.Verb)
	}
	return nil
}

func (c *Controller) addShareToQueue(s *storagev1alpha1.SharedResource, verb client.ObjectAction) {
	event := client.Event{
		Object: s,
		Verb:   verb,
	}
	c.shareWorkqueue.Add(event)
}

func (c *Controller) shareEventHandler() kcache.ResourceEventHandlerFuncs {
	return kcache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			switch v := o.(type) {
			case *storagev1alpha1.SharedResource:
				c.addShareToQueue(v, client.AddObjectAction)
			default:
				//log unrecognized type
			}
		},
		UpdateFunc: func(o, n interface{}) {
			switch v := n.(type) {
			case *storagev1alpha1.SharedResource:
				c.addShareToQueue(v, client.UpdateObjectAction)
			default:
				//log unrecognized type
			}
		},
		DeleteFunc: func(o interface{}) {
			switch v := o.(type) {
			case kcache.DeletedFinalStateUnknown:
				switch vv := v.Obj.(type) {
				case *storagev1alpha1.SharedResource:
					// log recovered deleted obj from tombstone via vv.GetName()
					c.addShareToQueue(vv, client.DeleteObjectAction)
				default:
					// log  error decoding obj tombstone
				}
			case *storagev1alpha1.SharedResource:
				c.addShareToQueue(v, client.DeleteObjectAction)
			default:
				//log unrecognized type
			}
		},
	}
}

func (c *Controller) shareEventProcessor() {
	for {
		obj, shutdown := c.shareWorkqueue.Get()
		if shutdown {
			return
		}

		func() {
			defer c.shareWorkqueue.Done(obj)

			event, ok := obj.(client.Event)
			if !ok {
				c.shareWorkqueue.Forget(obj)
				return
			}

			if err := c.syncShare(event); err != nil {
				c.shareWorkqueue.AddRateLimited(obj)
			} else {
				c.shareWorkqueue.Forget(obj)
			}
		}()
	}
}

func (c *Controller) syncShare(event client.Event) error {
	obj := event.Object.DeepCopyObject()
	share, ok := obj.(*storagev1alpha1.SharedResource)
	if share == nil || !ok {
		return fmt.Errorf("unexpected object vs. share: %v", event.Object.GetObjectKind().GroupVersionKind())
	}
	klog.V(4).Infof("verb %s share name %s", event.Verb, share.Name)
	switch event.Verb {
	case client.DeleteObjectAction:
		cache.DelShare(share)
	case client.AddObjectAction:
		cache.AddShare(share)
	case client.UpdateObjectAction:
		cache.UpdateShare(share)
	default:
		return fmt.Errorf("unexpected share event action: %s", event.Verb)
	}

	return nil
}
