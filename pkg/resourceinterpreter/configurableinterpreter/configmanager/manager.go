package configmanager

import (
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

var resourceInterpreterCustomizationsGVR = schema.GroupVersionResource{
	Group:    configv1alpha1.GroupVersion.Group,
	Version:  configv1alpha1.GroupVersion.Version,
	Resource: "resourceinterpretercustomizations",
}

// InterpreterConfigManager  collect the resource interpreter customization configuration.
type InterpreterConfigManager struct {
	initialSynced *atomic.Value
	lister        cache.GenericLister
	ConfigCache   map[schema.GroupVersionKind]configv1alpha1.LocalValueRetention
}

// HasSynced return true when the manager is synced with existing configuration.
func (configManager *InterpreterConfigManager) HasSynced() bool {
	if configManager.initialSynced.Load().(bool) {
		return true
	}

	configuration, err := configManager.lister.List(labels.Everything())
	klog.Infof("InterpreterConfigManager HasSynced %v ", configuration)
	klog.Infof("InterpreterConfigManager HasSynced %d ", len(configuration))
	klog.Infof("InterpreterConfigManager err %v", err)
	if err == nil && len(configuration) == 0 {
		// the empty list we initially stored is valid to use.
		// Setting initialSynced to true, so subsequent checks
		// would be able to take the fast path on the atomic boolean in a
		// cluster without any webhooks configured.
		configManager.initialSynced.Store(true)
		// the informer has synced, and we don't have any items
		return true
	}
	fmt.Printf("sycn resource error %v", err.Error())
	return false
}

// NewInterpreterConfigManager return a new interpreterConfigManager with resourceinterpretercustomizations handlers.
func NewInterpreterConfigManager(inform genericmanager.SingleClusterInformerManager) InterpreterConfigManager {
	manager := InterpreterConfigManager{
		lister:        inform.Lister(resourceInterpreterCustomizationsGVR),
		initialSynced: &atomic.Value{},
		ConfigCache:   make(map[schema.GroupVersionKind]configv1alpha1.LocalValueRetention, 0),
	}
	manager.initialSynced.Store(false)
	configHandlers := fedinformer.NewHandlerOnEvents(
		func(obj interface{}) { manager.addResourceInterpreterCustomizations(obj) },
		func(oldObj, newObj interface{}) { manager.updateResourceInterpreterCustomizations(oldObj, newObj) },
		func(obj interface{}) { manager.deleteResourceInterpreterCustomizations(obj) })
	inform.ForResource(resourceInterpreterCustomizationsGVR, configHandlers)

	return manager
}

func (configManager *InterpreterConfigManager) addResourceInterpreterCustomizations(obj interface{}) {
	configurations, err := configManager.lister.List(labels.Everything())
	fmt.Printf(" configurations %v  \n", configurations)
	fmt.Printf("configurations err %v \n", err)
	fmt.Printf("obj %v", obj)
	resourceInterpreterCustomization, ok := obj.(*configv1alpha1.ResourceInterpreterCustomization)
	if !ok {
		klog.Errorf("Cannot convert to resourceInterpreterCustomization: %v", obj)
		return
	}
	klog.Infof("Receiving add event for resourceInterpreterCustomization %s", resourceInterpreterCustomization.Name)
	cacheKey := schema.GroupVersionKind{
		Version: resourceInterpreterCustomization.Spec.APIVersion,
		Kind:    resourceInterpreterCustomization.Spec.Kind,
	}
	configManager.ConfigCache[cacheKey] = resourceInterpreterCustomization.Spec.Retention
}

func (configManager *InterpreterConfigManager) updateResourceInterpreterCustomizations(_, newObj interface{}) {
	configurations, err := configManager.lister.List(labels.Everything())
	fmt.Printf(" configurations %v  \n", configurations)
	fmt.Printf("configurations err %v \n", err)
	fmt.Printf("obj %v", newObj)
	resourceInterpreterCustomization, ok := newObj.(*configv1alpha1.ResourceInterpreterCustomization)
	if !ok {
		klog.Errorf("Cannot convert to resourceInterpreterCustomization: %v", newObj)
		return
	}
	klog.Infof("Receiving update event for resourceInterpreterCustomization %s", resourceInterpreterCustomization.Name)
	cacheKey := schema.GroupVersionKind{
		Version: resourceInterpreterCustomization.Spec.APIVersion,
		Kind:    resourceInterpreterCustomization.Spec.Kind,
	}
	configManager.ConfigCache[cacheKey] = resourceInterpreterCustomization.Spec.Retention
}

func (configManager *InterpreterConfigManager) deleteResourceInterpreterCustomizations(obj interface{}) {
	configurations, err := configManager.lister.List(labels.Everything())
	fmt.Printf(" configurations %v  \n", configurations)
	fmt.Printf("configurations err %v \n", err)
	fmt.Printf("obj %v", obj)
	var resourceInterpreterCustomization *configv1alpha1.ResourceInterpreterCustomization
	switch t := obj.(type) {
	case *configv1alpha1.ResourceInterpreterCustomization:
		resourceInterpreterCustomization = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		resourceInterpreterCustomization, ok = t.Obj.(*configv1alpha1.ResourceInterpreterCustomization)
		if !ok {
			klog.Errorf("Cannot convert to configv1alpha1.ResourceInterpreterCustomization: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to configv1alpha1.ResourceInterpreterCustomization %v", t)
		return
	}
	klog.Infof("Receiving delete event for resourceInterpreterCustomization %s", resourceInterpreterCustomization.Name)
	cacheKey := schema.GroupVersionKind{
		Version: resourceInterpreterCustomization.Spec.APIVersion,
		Kind:    resourceInterpreterCustomization.Spec.Kind,
	}
	delete(configManager.ConfigCache, cacheKey)
}
