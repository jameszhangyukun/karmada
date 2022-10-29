package configmanager

import (
	"fmt"
	"sync/atomic"

	"github.com/karmada-io/karmada/pkg/util/helper"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

// ConfigManager can list custom resource interpreter.
type ConfigManager interface {
	CustomAccessors() map[schema.GroupVersionKind]CustomAccessor
	HasSynced() bool
}

// interpreterConfigManager  collect the resource interpreter customization configuration.
type interpreterConfigManager struct {
	initialSynced *atomic.Value
	lister        cache.GenericLister
	configuration *atomic.Value
}

// CustomAccessors return all configured resource interpreter webhook.
func (configManager *interpreterConfigManager) CustomAccessors() map[schema.GroupVersionKind]CustomAccessor {
	return configManager.configuration.Load().(map[schema.GroupVersionKind]CustomAccessor)
}

// HasSynced return true when the manager is synced with existing configuration.
func (configManager *interpreterConfigManager) HasSynced() bool {
	if configManager.initialSynced.Load().(bool) {
		return true
	}

	configuration, err := configManager.lister.List(labels.Everything())
	klog.Infof("interpreterConfigManager HasSynced %v", configuration)
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
func NewInterpreterConfigManager(inform genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{
		lister:        inform.Lister(resourceInterpreterCustomizationsGVR),
		initialSynced: &atomic.Value{},
		configuration: &atomic.Value{},
	}
	manager.configuration.Store(make(map[schema.GroupVersionKind]CustomAccessor))
	manager.initialSynced.Store(false)
	configHandlers := fedinformer.NewHandlerOnEvents(
		func(_ interface{}) { manager.updateConfiguration() },
		func(_, _ interface{}) { manager.updateConfiguration() },
		func(_ interface{}) { manager.updateConfiguration() })
	inform.ForResource(resourceInterpreterCustomizationsGVR, configHandlers)
	return manager
}

func (configManager *interpreterConfigManager) updateConfiguration() {
	configurations, err := configManager.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
		return
	}
	configs := make(map[schema.GroupVersionKind]CustomAccessor, 0)

	for _, c := range configurations {
		unstructuredConfig, err := helper.ToUnstructured(c)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: %w", err)
			return
		}
		config := &configv1alpha1.ResourceInterpreterCustomization{}
		err = helper.ConvertToTypedObject(unstructuredConfig, config)
		key := schema.GroupVersionKind{
			Version: config.Spec.Target.APIVersion,
			Kind:    config.Spec.Target.Kind,
		}
		configs[key] = NewResourceCustomAccessorAccessor(config)
	}

	configManager.configuration.Store(configs)
	configManager.initialSynced.Store(true)
}
