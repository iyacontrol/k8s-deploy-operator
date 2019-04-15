package main

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/k8s-deploy-operator/pkg/apis/k8sdeployoperator/v1"
	clientset "k8s.io/k8s-deploy-operator/pkg/client/clientset/versioned"
	sscheme "k8s.io/k8s-deploy-operator/pkg/client/clientset/versioned/scheme"
	informers "k8s.io/k8s-deploy-operator/pkg/client/informers/externalversions"
	listers "k8s.io/k8s-deploy-operator/pkg/client/listers/k8sdeployoperator/v1"
)

const controllerAgentName = "k8s_deploy_controller"

var policy = metav1.DeletePropagationBackground
var deleteOptions = &metav1.DeleteOptions{
	PropagationPolicy: &policy,
}

// Controller is the controller implementation for Canary resources
type Controller struct {
	kubeclientset kubernetes.Interface
	cdclientset   clientset.Interface

	cdLister listers.CanaryLister
	cdSynced cache.InformerSynced

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced

	queue    workqueue.RateLimitingInterface
	recorder record.EventRecorder
}

// NewController returns a new instance of a controller
func NewController(
	kubeclientset kubernetes.Interface,
	cdclientset clientset.Interface,

	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cdInformerFactory informers.SharedInformerFactory) *Controller {

	cdInformer := cdInformerFactory.K8sdeployoperator().V1().Canaries()
	sscheme.AddToScheme(scheme.Scheme)

	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	c := &Controller{
		kubeclientset: kubeclientset,
		cdclientset:   cdclientset,

		cdLister: cdInformer.Lister(),
		cdSynced: cdInformer.Informer().HasSynced,

		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,

		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Canaries"),
		recorder: recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when EventProvider resources change
	cdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			glog.Info("UpdateFunc called with objects: %v, %v", old, new)
			oldCrd := old.(*v1.Canary)
			newCrd := new.(*v1.Canary)

			if oldCrd.ResourceVersion == newCrd.ResourceVersion {
				// Periodic resync will send update events for all known Objects.
				// Two different versions of the same Objects will always have different RVs.
				return
			}

			if reflect.DeepEqual(oldCrd.Spec, newCrd.Spec) {
				return
			}

			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(key)
			}

		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting K8s Deploy controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.cdSynced, c.deploymentsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process the resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, 30*time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.queue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.queue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	glog.Infof("\nReceived: namespace: %v, name: %v\n", namespace, name)

	cd, err := c.cdLister.Canaries(namespace).Get(name)
	if err != nil {
		return fmt.Errorf("error getting resource: %v", err)
	}
	glog.Infof("k8sdeploy: %v", cd)

	canaryDeployName := name + "-canary"

	switch cd.Spec.Stage {
	case K8sDeployStageCanary:
		deploy, err := c.deploymentsLister.Deployments(namespace).Get(name)
		if err != nil {
			return err
		}

		canary := &appsv1.Deployment{
			TypeMeta: deploy.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:        canaryDeployName,
				Namespace:   namespace,
				Labels:      deploy.Labels,
				Annotations: deploy.Annotations,
			},
			Spec: deploy.Spec,
		}

		canary.Spec.Template.Spec.Containers[cd.Spec.Index].Image = cd.Spec.Image

		_, err = c.kubeclientset.AppsV1().Deployments(namespace).Create(canary)
		if err != nil {
			return err
		}

		glog.Infof("create canary deployment name: %s", canaryDeployName)

	case K8sDeployStageRollBack:
		err := c.kubeclientset.AppsV1().Deployments(namespace).Delete(canaryDeployName, deleteOptions)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		glog.Infof("delete canary deployment name: %s", canaryDeployName)

	case K8sDeployStageRollup:
		err := c.kubeclientset.AppsV1().Deployments(namespace).Delete(canaryDeployName, deleteOptions)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		glog.Infof("delete canary deployment name: %s", canaryDeployName)

		deploy, err := c.deploymentsLister.Deployments(namespace).Get(name)
		if err != nil {
			return err
		}

		deploy.Spec.Template.Spec.Containers[cd.Spec.Index].Image = cd.Spec.Image

		_, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(deploy)
		if err != nil {
			return err
		}

		glog.Infof("update container image : %s of  deployment name: %s", cd.Spec.Image, name)

	default:
		glog.Errorf("cannot handle operation %v", cd.Spec.Stage)
		return nil
	}
	return nil
}
