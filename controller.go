package main

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	crdv1beta1 "github.com/kulong0105/controller-demo/pkg/apis/stable/v1beta1"
	informers "github.com/kulong0105/controller-demo/pkg/client/informers/externalversions/stable/v1beta1"
)

type Controller struct {
	informer  informers.CronTabInformer
	workqueue workqueue.RateLimitingInterface
}

func NewController(informer informers.CronTabInformer) *Controller {

	controller := &Controller{
		informer:  informer,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CronTab"),
	}

	klog.Info("setting up crontab event handlers")

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCronTab,
		UpdateFunc: func(old, new interface{}) {
			oldJob := old.(*crdv1beta1.CronTab)
			newJob := new.(*crdv1beta1.CronTab)

			if oldJob.ResourceVersion == newJob.ResourceVersion {
				return
			}

			controller.enqueueCronTab(new)
		},
		DeleteFunc: controller.enqueueCronTabForDelete,
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("starting crontab control loop")
	klog.Info("waiting to informer caches to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.informer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")
	}

	klog.Info("starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("started workers")

	<-stopCh

	klog.Info("shutting down workers")
	return nil
}

// runWorker always call processNextWorkItem get item from workqueue
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%': ‘%s'", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.Infof("Successfully syncd '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// get CronTab object from Informer cache
func (c *Controller) syncHandler(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	crontab, err := c.informer.Lister().CronTabs(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warningf("[CronTabCRD] %s %s does not exist in local cache, will delete it from CronTab ...", namespace, name)
			klog.Infof("CronTabCRD] delete crontab: %s %s", namespace, name)
			return nil
		}

		runtime.HandleError(fmt.Errorf("failed to get crontab by: %s %s", namespace, name))
		return err
	}

	klog.Infof("[CronTabCRD] try to process crontab: %#v ...", crontab)
	return nil
}

func (c *Controller) enqueueCronTab(obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.workqueue.AddRateLimited(key)
}

func (c *Controller) enqueueCronTabForDelete(obj interface{}) {
	var key string
	var err error

	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.workqueue.AddRateLimited(key)
}
