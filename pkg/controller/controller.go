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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	objcache "github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/metrics"

	shareclientv1alpha1 "github.com/openshift/client-go/sharedresource/clientset/versioned"
	shareinformer "github.com/openshift/client-go/sharedresource/informers/externalversions"
)

const (
	DefaultResyncDuration = 10 * time.Minute
)

type Controller struct {
	kubeRestConfig *rest.Config

	kubeClient *kubernetes.Clientset

	cfgMapWorkqueue          workqueue.RateLimitingInterface
	secretWorkqueue          workqueue.RateLimitingInterface
	sharedConfigMapWorkqueue workqueue.RateLimitingInterface
	sharedSecretWorkqueue    workqueue.RateLimitingInterface

	cfgMapInformer          cache.SharedIndexInformer
	secInformer             cache.SharedIndexInformer
	sharedConfigMapInformer cache.SharedIndexInformer
	sharedSecretInformer    cache.SharedIndexInformer

	sharedConfigMapInformerFactory shareinformer.SharedInformerFactory
	sharedSecretInformerFactory    shareinformer.SharedInformerFactory
	informerFactory                informers.SharedInformerFactory

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

	shareClient, err := shareclientv1alpha1.NewForConfig(kubeRestConfig)
	if err != nil {
		return nil, err
	}

	tweakListOptions := internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
		ignored := []string{}
		for _, ns := range ignoredNamespaces {
			klog.V(4).Infof("namespace '%s' is being ignored", ns)
			ignored = append(ignored, fmt.Sprintf("metadata.namespace!=%s", ns))
		}
		options.FieldSelector = strings.Join(ignored, ",")
	})
	informerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient,
		DefaultResyncDuration, informers.WithTweakListOptions(tweakListOptions))

	klog.V(5).Infof("configured share relist %v", shareRelist)
	shareInformerFactory := shareinformer.NewSharedInformerFactoryWithOptions(shareClient,
		shareRelist)

	c := &Controller{
		kubeClient:     kubeClient,
		kubeRestConfig: kubeRestConfig,
		sharedConfigMapWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),
			"shared-configmap-changes"),
		sharedSecretWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),
			"shared-secret-changes"),
		informerFactory:                informerFactory,
		sharedConfigMapInformerFactory: shareInformerFactory,
		sharedSecretInformerFactory:    shareInformerFactory,
		sharedConfigMapInformer:        shareInformerFactory.Sharedresource().V1alpha1().SharedConfigMaps().Informer(),
		sharedSecretInformer:           shareInformerFactory.Sharedresource().V1alpha1().SharedSecrets().Informer(),
		listers:                        client.GetListers(),
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

		c.cfgMapInformer.AddEventHandler(c.configMapEventHandler())
		c.secInformer.AddEventHandler(c.secretEventHandler())
	}

	client.SetSharedConfigMapsLister(c.sharedConfigMapInformerFactory.Sharedresource().V1alpha1().SharedConfigMaps().Lister())
	client.SetSharedSecretsLister(c.sharedSecretInformerFactory.Sharedresource().V1alpha1().SharedSecrets().Lister())
	c.sharedConfigMapInformer.AddEventHandler(c.sharedConfigMapEventHandler())
	c.sharedSecretInformer.AddEventHandler(c.sharedSecretEventHandler())

	return c, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	if c.cfgMapWorkqueue != nil && c.secretWorkqueue != nil {
		defer c.cfgMapWorkqueue.ShutDown()
		defer c.secretWorkqueue.ShutDown()
	}
	defer c.sharedConfigMapWorkqueue.ShutDown()
	defer c.sharedSecretWorkqueue.ShutDown()

	c.informerFactory.Start(stopCh)
	c.sharedConfigMapInformerFactory.Start(stopCh)
	c.sharedSecretInformerFactory.Start(stopCh)

	if c.cfgMapInformer != nil && !cache.WaitForCacheSync(stopCh, c.cfgMapInformer.HasSynced) {
		return fmt.Errorf("failed to wait for ConfigMap informer cache to sync")
	}
	if c.secInformer != nil && !cache.WaitForCacheSync(stopCh, c.secInformer.HasSynced) {
		return fmt.Errorf("failed to wait for Secrets informer cache to sync")
	}
	if !cache.WaitForCacheSync(stopCh, c.sharedConfigMapInformer.HasSynced) {
		return fmt.Errorf("failed to wait for sharedconfigmap caches to sync")
	}
	if !cache.WaitForCacheSync(stopCh, c.sharedSecretInformer.HasSynced) {
		return fmt.Errorf("failed to wait for sharedsecret caches to sync")
	}

	if c.cfgMapWorkqueue != nil {
		go wait.Until(c.configMapEventProcessor, time.Second, stopCh)
	}
	if c.secretWorkqueue != nil {
		go wait.Until(c.secretEventProcessor, time.Second, stopCh)
	}
	go wait.Until(c.sharedConfigMapEventProcessor, time.Second, stopCh)
	go wait.Until(c.sharedSecretEventProcessor, time.Second, stopCh)

	// start the Prometheus metrics serner
	klog.Info("Starting the metrics server")
	server, err := metrics.BuildServer(metrics.MetricsPort)
	if err != nil {
		return err
	}
	go metrics.RunServer(server, stopCh)

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
func (c *Controller) configMapEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
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
			case cache.DeletedFinalStateUnknown:
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
		objcache.DelConfigMap(cm)
	case client.AddObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		objcache.UpsertConfigMap(cm)
	case client.UpdateObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		objcache.UpsertConfigMap(cm)
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
func (c *Controller) secretEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
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
			case cache.DeletedFinalStateUnknown:
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
		objcache.DelSecret(secret)
	case client.AddObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		objcache.UpsertSecret(secret)
	case client.UpdateObjectAction:
		// again, add vs. update distinctions upheld for now, even though the path is common, in case
		// host filesystem interactions changes such that different methods for add vs. update are needed
		objcache.UpsertSecret(secret)
	default:
		return fmt.Errorf("unexpected secret event action: %s", event.Verb)
	}
	return nil
}

func (c *Controller) addSharedConfigMapToQueue(s *sharev1alpha1.SharedConfigMap, verb client.ObjectAction) {
	event := client.Event{
		Object: s,
		Verb:   verb,
	}
	c.sharedConfigMapWorkqueue.Add(event)
}

func (c *Controller) addSharedSecretToQueue(s *sharev1alpha1.SharedSecret, verb client.ObjectAction) {
	event := client.Event{
		Object: s,
		Verb:   verb,
	}
	c.sharedSecretWorkqueue.Add(event)
}

func (c *Controller) sharedConfigMapEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			switch v := o.(type) {
			case *sharev1alpha1.SharedConfigMap:
				c.addSharedConfigMapToQueue(v, client.AddObjectAction)
			default:
				//log unrecognized type
			}
		},
		UpdateFunc: func(o, n interface{}) {
			switch v := n.(type) {
			case *sharev1alpha1.SharedConfigMap:
				c.addSharedConfigMapToQueue(v, client.UpdateObjectAction)
			default:
				//log unrecognized type
			}
		},
		DeleteFunc: func(o interface{}) {
			switch v := o.(type) {
			case cache.DeletedFinalStateUnknown:
				switch vv := v.Obj.(type) {
				case *sharev1alpha1.SharedConfigMap:
					// log recovered deleted obj from tombstone via vv.GetName()
					c.addSharedConfigMapToQueue(vv, client.DeleteObjectAction)
				default:
					// log  error decoding obj tombstone
				}
			case *sharev1alpha1.SharedConfigMap:
				c.addSharedConfigMapToQueue(v, client.DeleteObjectAction)
			default:
				//log unrecognized type
			}
		},
	}
}

func (c *Controller) sharedSecretEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			switch v := o.(type) {
			case *sharev1alpha1.SharedSecret:
				c.addSharedSecretToQueue(v, client.AddObjectAction)
			default:
				//log unrecognized type
			}
		},
		UpdateFunc: func(o, n interface{}) {
			switch v := n.(type) {
			case *sharev1alpha1.SharedSecret:
				c.addSharedSecretToQueue(v, client.UpdateObjectAction)
			default:
				//log unrecognized type
			}
		},
		DeleteFunc: func(o interface{}) {
			switch v := o.(type) {
			case cache.DeletedFinalStateUnknown:
				switch vv := v.Obj.(type) {
				case *sharev1alpha1.SharedSecret:
					// log recovered deleted obj from tombstone via vv.GetName()
					c.addSharedSecretToQueue(vv, client.DeleteObjectAction)
				default:
					// log  error decoding obj tombstone
				}
			case *sharev1alpha1.SharedSecret:
				c.addSharedSecretToQueue(v, client.DeleteObjectAction)
			default:
				//log unrecognized type
			}
		},
	}
}

func (c *Controller) sharedConfigMapEventProcessor() {
	for {
		obj, shutdown := c.sharedConfigMapWorkqueue.Get()
		if shutdown {
			return
		}

		func() {
			defer c.sharedConfigMapWorkqueue.Done(obj)

			event, ok := obj.(client.Event)
			if !ok {
				c.sharedConfigMapWorkqueue.Forget(obj)
				return
			}

			if err := c.syncSharedConfigMap(event); err != nil {
				c.sharedConfigMapWorkqueue.AddRateLimited(obj)
			} else {
				c.sharedConfigMapWorkqueue.Forget(obj)
			}
		}()
	}
}

func (c *Controller) sharedSecretEventProcessor() {
	for {
		obj, shutdown := c.sharedSecretWorkqueue.Get()
		if shutdown {
			return
		}

		func() {
			defer c.sharedSecretWorkqueue.Done(obj)

			event, ok := obj.(client.Event)
			if !ok {
				c.sharedSecretWorkqueue.Forget(obj)
				return
			}

			if err := c.syncSharedSecret(event); err != nil {
				c.sharedSecretWorkqueue.AddRateLimited(obj)
			} else {
				c.sharedSecretWorkqueue.Forget(obj)
			}
		}()
	}
}

func (c *Controller) syncSharedConfigMap(event client.Event) error {
	obj := event.Object.DeepCopyObject()
	share, ok := obj.(*sharev1alpha1.SharedConfigMap)
	if share == nil || !ok {
		return fmt.Errorf("unexpected object vs. shared configmap: %v", event.Object.GetObjectKind().GroupVersionKind())
	}
	klog.V(4).Infof("verb %s share name %s", event.Verb, share.Name)
	switch event.Verb {
	case client.DeleteObjectAction:
		objcache.DelSharedConfigMap(share)
	case client.AddObjectAction:
		objcache.AddSharedConfigMap(share)
	case client.UpdateObjectAction:
		objcache.UpdateSharedConfigMap(share)
	default:
		return fmt.Errorf("unexpected share event action: %s", event.Verb)
	}

	return nil
}

func (c *Controller) syncSharedSecret(event client.Event) error {
	obj := event.Object.DeepCopyObject()
	share, ok := obj.(*sharev1alpha1.SharedSecret)
	if share == nil || !ok {
		return fmt.Errorf("unexpected object vs. shared secret: %v", event.Object.GetObjectKind().GroupVersionKind())
	}
	klog.V(4).Infof("verb %s share name %s", event.Verb, share.Name)
	switch event.Verb {
	case client.DeleteObjectAction:
		objcache.DelSharedSecret(share)
	case client.AddObjectAction:
		objcache.AddSharedSecret(share)
	case client.UpdateObjectAction:
		objcache.UpdateSharedSecret(share)
	default:
		return fmt.Errorf("unexpected share event action: %s", event.Verb)
	}

	return nil
}
