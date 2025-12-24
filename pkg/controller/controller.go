package controller

import (
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	sharev1alpha1 "github.com/openshift/api/sharedresource/v1alpha1"
	shareinformer "github.com/openshift/client-go/sharedresource/informers/externalversions"

	objcache "github.com/openshift/csi-driver-shared-resource/pkg/cache"
	"github.com/openshift/csi-driver-shared-resource/pkg/client"
	"github.com/openshift/csi-driver-shared-resource/pkg/metrics"
)

const (
	DefaultResyncDuration = 10 * time.Minute
)

type sharedObjectInformer struct {
	stopCh          chan struct{}
	informerFactory informers.SharedInformerFactory
	informer        cache.SharedIndexInformer
}

type Controller struct {
	kubeClient kubernetes.Interface

	cfgMapWorkqueue          workqueue.TypedRateLimitingInterface[any]
	secretWorkqueue          workqueue.TypedRateLimitingInterface[any]
	sharedConfigMapWorkqueue workqueue.TypedRateLimitingInterface[any]
	sharedSecretWorkqueue    workqueue.TypedRateLimitingInterface[any]

	secretWatchObjs    sync.Map
	configMapWatchObjs sync.Map

	sharedConfigMapInformer cache.SharedIndexInformer
	sharedSecretInformer    cache.SharedIndexInformer

	sharedConfigMapInformerFactory shareinformer.SharedInformerFactory
	sharedSecretInformerFactory    shareinformer.SharedInformerFactory

	listers *client.Listers

	refreshResources bool
}

// NewController instantiate a new controller with relisting interval, and optional refresh-resources
// mode. Refresh-resources mode means the controller will keep watching for ConfigMaps and Secrets
// for future changes, when disabled it only loads the resource contents before mounting the volume.
func NewController(shareRelist time.Duration, refreshResources bool) (*Controller, error) {
	kubeClient := client.GetClient()
	shareClient := client.GetShareClient()

	klog.V(5).Infof("configured share relist %v", shareRelist)
	shareInformerFactory := shareinformer.NewSharedInformerFactoryWithOptions(shareClient,
		shareRelist)

	c := &Controller{
		kubeClient: kubeClient,
		sharedConfigMapWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any](),
			"shared-configmap-changes"),
		sharedSecretWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any](),
			"shared-secret-changes"),
		secretWatchObjs:                sync.Map{},
		configMapWatchObjs:             sync.Map{},
		sharedConfigMapInformerFactory: shareInformerFactory,
		sharedSecretInformerFactory:    shareInformerFactory,
		sharedConfigMapInformer:        shareInformerFactory.Sharedresource().V1alpha1().SharedConfigMaps().Informer(),
		sharedSecretInformer:           shareInformerFactory.Sharedresource().V1alpha1().SharedSecrets().Informer(),
		listers:                        client.GetListers(),
		refreshResources:               refreshResources,
	}

	c.cfgMapWorkqueue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[any](), "shared-resource-configmap-changes")
	c.secretWorkqueue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[any](), "shared-resource-secret-changes")

	client.SetSharedConfigMapsLister(c.sharedConfigMapInformerFactory.Sharedresource().V1alpha1().SharedConfigMaps().Lister())
	client.SetSharedSecretsLister(c.sharedSecretInformerFactory.Sharedresource().V1alpha1().SharedSecrets().Lister())
	c.sharedConfigMapInformer.AddEventHandler(c.sharedConfigMapEventHandler())
	c.sharedSecretInformer.AddEventHandler(c.sharedSecretEventHandler())

	return c, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer c.cfgMapWorkqueue.ShutDown()
	defer c.secretWorkqueue.ShutDown()
	defer c.sharedConfigMapWorkqueue.ShutDown()
	defer c.sharedSecretWorkqueue.ShutDown()

	c.sharedConfigMapInformerFactory.Start(stopCh)
	c.sharedSecretInformerFactory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.sharedConfigMapInformer.HasSynced) {
		return fmt.Errorf("failed to wait for sharedconfigmap caches to sync")
	}
	if !cache.WaitForCacheSync(stopCh, c.sharedSecretInformer.HasSynced) {
		return fmt.Errorf("failed to wait for sharedsecret caches to sync")
	}

	go wait.Until(c.configMapEventProcessor, time.Second, stopCh)
	go wait.Until(c.secretEventProcessor, time.Second, stopCh)
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

	c.secretWatchObjs.Range(func(key, value interface{}) bool {
		informerObj := value.(sharedObjectInformer)
		close(informerObj.stopCh)
		return true
	})
	c.configMapWatchObjs.Range(func(key, value interface{}) bool {
		informerObj := value.(sharedObjectInformer)
		close(informerObj.stopCh)
		return true
	})

	return nil
}

func (c *Controller) RegisterSecretInformer(namespace string) error {
	val, ok := c.secretWatchObjs.LoadOrStore(namespace, sharedObjectInformer{})
	if !ok {
		informerObj := val.(sharedObjectInformer)
		informerObj.informerFactory = informers.NewSharedInformerFactoryWithOptions(c.kubeClient, DefaultResyncDuration, informers.WithNamespace(namespace))
		informerObj.informer = informerObj.informerFactory.Core().V1().Secrets().Informer()
		informerObj.informer.AddEventHandler(c.secretEventHandler())
		client.SetSecretsLister(namespace, informerObj.informerFactory.Core().V1().Secrets().Lister())
		informerObj.stopCh = make(chan struct{})
		informerObj.informerFactory.Start(informerObj.stopCh)
		if !cache.WaitForCacheSync(informerObj.stopCh, informerObj.informer.HasSynced) {
			return fmt.Errorf("failed to wait for Secrets informer cache for namespace %s to sync", namespace)
		}
		c.secretWatchObjs.Store(namespace, informerObj)
	}
	return nil
}

func (c *Controller) UnregisterSecretInformer(namespace string) {
	val, ok := c.secretWatchObjs.Load(namespace)
	if ok {
		informerObj := val.(sharedObjectInformer)
		close(informerObj.stopCh)
		c.secretWatchObjs.Delete(namespace)
	}
}

func (c *Controller) PruneSecretInformers(namespaces map[string]struct{}) {
	c.secretWatchObjs.Range(func(key, value interface{}) bool {
		ns := key.(string)
		if _, ok := namespaces[ns]; !ok {
			klog.V(2).Infof("unregistering secret informer for namespace %s", ns)
			c.UnregisterSecretInformer(ns)
		}
		return true
	})
}

func (c *Controller) RegisterConfigMapInformer(namespace string) error {
	val, ok := c.configMapWatchObjs.LoadOrStore(namespace, sharedObjectInformer{})
	if !ok {
		informerObj := val.(sharedObjectInformer)
		informerObj.informerFactory = informers.NewSharedInformerFactoryWithOptions(c.kubeClient, DefaultResyncDuration, informers.WithNamespace(namespace))
		informerObj.informer = informerObj.informerFactory.Core().V1().ConfigMaps().Informer()
		informerObj.informer.AddEventHandler(c.configMapEventHandler())
		client.SetConfigMapsLister(namespace, informerObj.informerFactory.Core().V1().ConfigMaps().Lister())
		informerObj.stopCh = make(chan struct{})
		informerObj.informerFactory.Start(informerObj.stopCh)
		if !cache.WaitForCacheSync(informerObj.stopCh, informerObj.informer.HasSynced) {
			return fmt.Errorf("failed to wait for ConfigMaps informer cache for namespace %s to sync", namespace)
		}
		c.configMapWatchObjs.Store(namespace, informerObj)
	}
	return nil
}

func (c *Controller) UnregisterConfigMapInformer(namespace string) {
	val, ok := c.configMapWatchObjs.Load(namespace)
	if ok {
		informerObj := val.(sharedObjectInformer)
		close(informerObj.stopCh)
		c.configMapWatchObjs.Delete(namespace)
	}
}

func (c *Controller) PruneConfigMapInformers(namespaces map[string]struct{}) {
	c.configMapWatchObjs.Range(func(key, value interface{}) bool {
		ns := key.(string)
		if _, ok := namespaces[ns]; !ok {
			klog.V(2).Infof("unregistering configmap informer for namespace %s", ns)
			c.UnregisterConfigMapInformer(ns)
		}
		return true
	})

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
		return fmt.Errorf("%s", msg)
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
	var err error
	switch event.Verb {
	case client.DeleteObjectAction:
		objcache.DelSharedConfigMap(share)
		if c.refreshResources {
			c.PruneConfigMapInformers(objcache.NamespacesWithSharedConfigMaps())

		}
	case client.AddObjectAction:
		if c.refreshResources {
			// this must be before AddShareConfigMap so we wait until the information cache has synched
			err = c.RegisterConfigMapInformer(share.Spec.ConfigMapRef.Namespace)
		}
		objcache.AddSharedConfigMap(share)
		if c.refreshResources {
			c.PruneConfigMapInformers(objcache.NamespacesWithSharedConfigMaps())
		}
	case client.UpdateObjectAction:
		if c.refreshResources {
			// this must be before UpdateShareConfigMap so we wait until the informer cache has synched
			err = c.RegisterConfigMapInformer(share.Spec.ConfigMapRef.Namespace)
		}
		objcache.UpdateSharedConfigMap(share)
		if c.refreshResources {
			c.PruneConfigMapInformers(objcache.NamespacesWithSharedConfigMaps())
		}
	default:
		return fmt.Errorf("unexpected share event action: %s", event.Verb)
	}

	return err
}

func (c *Controller) syncSharedSecret(event client.Event) error {
	obj := event.Object.DeepCopyObject()
	share, ok := obj.(*sharev1alpha1.SharedSecret)
	if share == nil || !ok {
		return fmt.Errorf("unexpected object vs. shared secret: %v", event.Object.GetObjectKind().GroupVersionKind())
	}
	klog.V(4).Infof("verb %s share name %s", event.Verb, share.Name)
	var err error
	switch event.Verb {
	case client.DeleteObjectAction:
		objcache.DelSharedSecret(share)
		if c.refreshResources {
			c.PruneSecretInformers(objcache.NamespacesWithSharedSecrets())
		}
	case client.AddObjectAction:
		if c.refreshResources {
			// this must be before AddShareSecret so we wait until the informer cache has synched
			err = c.RegisterSecretInformer(share.Spec.SecretRef.Namespace)
		}
		objcache.AddSharedSecret(share)
		if c.refreshResources {
			c.PruneSecretInformers(objcache.NamespacesWithSharedSecrets())
		}
	case client.UpdateObjectAction:
		if c.refreshResources {
			// this must be before UpdateShareSecret so we wait until the informer cache has synched
			err = c.RegisterSecretInformer(share.Spec.SecretRef.Namespace)
		}
		objcache.UpdateSharedSecret(share)
		if c.refreshResources {
			c.PruneSecretInformers(objcache.NamespacesWithSharedSecrets())
		}
	default:
		return fmt.Errorf("unexpected share event action: %s", event.Verb)
	}

	return err
}
