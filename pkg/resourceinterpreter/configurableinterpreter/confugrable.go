package configurableinterpreter

import (
	"encoding/json"
	"fmt"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/configmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// ConfigurableInterpreter interpret custom resource with webhook configuration.
type ConfigurableInterpreter struct {
	// configManager caches all re configurations.
	configManager configmanager.ConfigManager
}

// NewConfigurableInterpreter return a new ConfigurableInterpreter.
func NewConfigurableInterpreter(informer genericmanager.SingleClusterInformerManager) (*ConfigurableInterpreter, error) {
	return &ConfigurableInterpreter{
		configManager: configmanager.NewInterpreterConfigManager(informer),
	}, nil
}

func (c *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	if !c.configManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return false
	}
	_, exist := c.configManager.CustomAccessors()[kind]
	return exist
}

type ReplicaResult struct {
	Replica int `json:"replica"`
	workv1alpha2.ReplicaRequirements
}

func (c *ConfigurableInterpreter) CheckConfigurableExists(kind schema.GroupVersionKind) bool {
	klog.Infof("ConfigurableInterpreter Execute CheckConfigurableExists")
	klog.Infof("ConfigurableInterpreter value %v", c.configManager.CustomAccessors())
	_, ok := c.configManager.CustomAccessors()[kind]
	return ok
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {

	klog.Infof("ConfigurableInterpreter Execute ReviseReplica")
	customAccessor := c.configManager.CustomAccessors()[object.GroupVersionKind()]
	if customAccessor.GetReplicaResource() == nil || len(customAccessor.GetReplicaResource().LuaScript) == 0 {
		return 0, nil, fmt.Errorf("customized interpreter operation GetReplicas  for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetReplicaResource().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}

	replicas, requires, err := luavm.GetReplicas(object, luaScript)
	klog.Infof("Execute Result %v %v %v", requires, *requires, err)
	return replicas, requires, err
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	klog.Infof("ConfigurableInterpreter Execute ReviseReplica")
	customAccessor := c.configManager.CustomAccessors()[object.GroupVersionKind()]
	if customAccessor.GetReplicaRevision() == nil || len(customAccessor.GetReplicaRevision().LuaScript) == 0 {
		return nil, fmt.Errorf("customized interpreter operation ReviseReplica  for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetReplicaRevision().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}

	reviseReplica, err := luavm.ReviseReplica(object, replica, luaScript)
	klog.Infof("Execute ReviseReplica Result %v %v %v", reviseReplica, err)
	return reviseReplica, err
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	klog.Infof("ConfigurableInterpreter Execute Retain")
	customAccessor := c.configManager.CustomAccessors()[desired.GroupVersionKind()]
	if customAccessor.GetRetention() == nil || len(customAccessor.GetRetention().LuaScript) == 0 {
		return nil, fmt.Errorf("customized interpreter operation Retain  for %q not found", desired.GroupVersionKind())
	}

	luaScript := customAccessor.GetRetention().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}

	retainedReplica, err := luavm.Retain(desired, observed, luaScript)
	klog.Infof("Execute Retain Result %v %v %v", retainedReplica, err)
	return retainedReplica, err

}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	klog.Infof("ConfigurableInterpreter Execute AggregateStatus")
	customAccessor := c.configManager.CustomAccessors()[object.GroupVersionKind()]
	if customAccessor.GetStatusAggregation() == nil || len(customAccessor.GetStatusAggregation().LuaScript) == 0 {
		return nil, fmt.Errorf("customized interpreter AggregateStatus Retain  for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetStatusAggregation().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}
	var aggregateItem []map[string]interface{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := make(map[string]interface{})
		if err := json.Unmarshal(item.Status.Raw, &temp); err != nil {
			return nil, err
		}
		aggregateItem = append(aggregateItem, temp)
	}
	aggregateStatusResult, err := luavm.AggregateStatus(object, aggregateItem, luaScript)
	klog.Infof("Execute AggregateStatus Result %v %v %v", aggregateStatusResult, err)
	return aggregateStatusResult, err
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	klog.Infof("ConfigurableInterpreter Execute GetDependencies")

	customAccessor := c.configManager.CustomAccessors()[object.GroupVersionKind()]
	if customAccessor.GetDependencyInterpretation() == nil || len(customAccessor.GetDependencyInterpretation().LuaScript) == 0 {
		return nil, fmt.Errorf("customized interpreter GetDependencies Retain  for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetDependencyInterpretation().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}

	getDependenciesResult, err := luavm.GetDependencies(object, luaScript)
	klog.Infof("Execute GetDependencies Result %v %v %v", getDependenciesResult, err)
	return getDependenciesResult, err
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	klog.Infof("ConfigurableInterpreter Execute ReflectStatus")
	customAccessor := c.configManager.CustomAccessors()[object.GroupVersionKind()]
	if customAccessor.GetStatusReflection() == nil || len(customAccessor.GetStatusReflection().LuaScript) == 0 {
		return nil, fmt.Errorf("customized interpreter GetDependencies Retain  for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetStatusReflection().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}

	getReflectStatusResult, err := luavm.ReflectStatus(object, luaScript)
	klog.Infof("Execute ReflectStatus Result %v %v %v", getReflectStatusResult, err)
	return getReflectStatusResult, err

}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (bool, error) {
	klog.Infof("ConfigurableInterpreter Execute InterpretHealth")
	customAccessor := c.configManager.CustomAccessors()[object.GroupVersionKind()]
	if customAccessor.GetHealthInterpretation() == nil || len(customAccessor.GetHealthInterpretation().LuaScript) == 0 {
		return false, fmt.Errorf("customized interpreter GetHealthInterpretation   for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetHealthInterpretation().LuaScript
	klog.Infof("lua script %s", luaScript)

	luavm := helper.VM{UseOpenLibs: false}

	getGetHealthInterpretationResult, err := luavm.InterpretHealth(object, luaScript)
	klog.Infof("Execute ReflectStatus Result %v %v %v", getGetHealthInterpretationResult, err)
	return getGetHealthInterpretationResult, err
}
