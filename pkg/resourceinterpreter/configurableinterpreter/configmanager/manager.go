package configmanager

import (
	"fmt"
	"github.com/karmada-io/karmada/pkg/util/helper"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	configuration *atomic.Value
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
		configuration: &atomic.Value{},
	}
	manager.configuration.Store(make(map[schema.GroupVersionKind]*configv1alpha1.ResourceInterpreterCustomization))
	manager.initialSynced.Store(false)
	configHandlers := fedinformer.NewHandlerOnEvents(
		func(_ interface{}) { manager.updateConfiguration() },
		func(_, _ interface{}) { manager.updateConfiguration() },
		func(_ interface{}) { manager.updateConfiguration() })
	inform.ForResource(resourceInterpreterCustomizationsGVR, configHandlers)
	return manager
}

func (configManager *InterpreterConfigManager) updateConfiguration() {

	configurations, err := configManager.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
		return
	}
	configs := make(map[schema.GroupVersionKind]*configv1alpha1.ResourceInterpreterCustomization, 0)

	for _, c := range configurations {
		unstructuredConfig, err := helper.ToUnstructured(c)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: %w", err)
			return
		}
		config := &configv1alpha1.ResourceInterpreterCustomization{}
		err = helper.ConvertToTypedObject(unstructuredConfig, config)
		key := schema.GroupVersionKind{
			Version: config.Spec.APIVersion,
			Kind:    config.Spec.Kind,
		}
		configs[key] = config
	}

	configManager.configuration.Store(configs)
	configManager.initialSynced.Store(true)
}

func (configManager *InterpreterConfigManager) GetResourceInterpreterCustomization(gvk schema.GroupVersionKind) *configv1alpha1.ResourceInterpreterCustomization {
	m := configManager.configuration.Load().(map[schema.GroupVersionKind]*configv1alpha1.ResourceInterpreterCustomization)
	return m[gvk]
}
