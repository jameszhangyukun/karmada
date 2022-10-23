package configurableinterpreter

import (
	"encoding/json"
	"fmt"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/configmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"

	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// ConfigurableInterpreter interpret custom resource with webhook configuration.
type ConfigurableInterpreter struct {
	// configManager caches all re configurations.
	configManager configmanager.InterpreterConfigManager
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
	_, exist := c.configManager.ConfigCache[kind]
	return exist
}

type ReplicaResult struct {
	Replica int `json:"replica"`
	workv1alpha2.ReplicaRequirements
}

func (c *ConfigurableInterpreter) CheckConfigurableExists(kind schema.GroupVersionKind) bool {
	_, exist := c.configManager.ConfigCache[kind]
	return exist
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {

	localValueRetention, exist := c.configManager.ConfigCache[object.GroupVersionKind()]
	if !exist {
		return 0, nil, fmt.Errorf("customized interpreter for %q not found", object.GroupVersionKind())
	}
	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunLua(object, localValueRetention.RetentionLua)

	if err != nil {
		return 0, nil, err
	}
	result := luaResult.Get(-1)

	replicaRequirement := &ReplicaResult{}
	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return 0, nil, err
		}
		err = json.Unmarshal(jsonBytes, replicaRequirement)
		if err != nil {
			return 0, nil, err
		}
	}

	return int32(replicaRequirement.Replica), &replicaRequirement.ReplicaRequirements, nil
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	localValueRetention, exist := c.configManager.ConfigCache[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("customized interpreter for %q not found", object.GroupVersionKind())
	}
	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunRevisereplicaWithLua(localValueRetention.RetentionLua, object, replica)

	if err != nil {
		return nil, err
	}
	result := luaResult.Get(-1)
	unstructuredResult := &unstructured.Unstructured{}

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, unstructuredResult)
		if err != nil {
			return nil, err
		}
	}

	return unstructuredResult, nil
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	localValueRetention, exist := c.configManager.ConfigCache[desired.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("customized interpreter for %q not found",
			desired.GroupVersionKind())
	}
	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunRetainWithLua(localValueRetention.RetentionLua, desired, observed)

	if err != nil {
		return nil, err
	}
	result := luaResult.Get(-1)
	retainResult := &unstructured.Unstructured{}

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, retainResult)
		if err != nil {
			return nil, err
		}
	}

	return retainResult, nil
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	localValueRetention, exist := c.configManager.ConfigCache[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("customized interpreter for %q not found", object.GroupVersionKind())
	}
	var aggregateItem []map[string]interface{}
	vm := helper.VM{UseOpenLibs: false}

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

	luaResult, err := vm.RunAggregateWithLua(localValueRetention.RetentionLua, object, aggregateItem)

	if err != nil {
		return nil, err
	}
	result := luaResult.Get(-1)
	aggregateStatusResult := &unstructured.Unstructured{}

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, aggregateStatusResult)
		if err != nil {
			return nil, err
		}
	}

	return aggregateStatusResult, nil
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	localValueRetention, exist := c.configManager.ConfigCache[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("customized interpreter for %q not found", object.GroupVersionKind())
	}
	vm := helper.VM{UseOpenLibs: false}

	luaResult, err := vm.RunReflectStatusWithLua(localValueRetention.RetentionLua, object)

	if err != nil {
		return nil, err
	}

	result := luaResult.Get(-1)

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, &dependencies)
		if err != nil {
			return nil, err
		}
	}
	return dependencies, nil
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	localValueRetention, exist := c.configManager.ConfigCache[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("customized interpreter for %q not found", object.GroupVersionKind())
	}
	vm := helper.VM{UseOpenLibs: false}

	luaResult, err := vm.RunReflectStatusWithLua(localValueRetention.RetentionLua, object)

	if err != nil {
		return nil, err
	}
	existResult := luaResult.Get(1)
	result := luaResult.Get(2)
	if existResult.Type() == lua.LTBool {
		jsonBytes, err := luajson.Encode(existResult)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(jsonBytes, &exist)
		if err != nil {
			return nil, err
		}
	}
	if exist {
		resultMap := make(map[string]interface{})
		if result.Type() == lua.LTTable {
			jsonBytes, err := luajson.Encode(result)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(jsonBytes, &resultMap)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("Failed to get status field from ")
		}
		return helper.BuildStatusRawExtension(resultMap)
	}
	return nil, nil
}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (bool, error) {
	localValueRetention, exist := c.configManager.ConfigCache[object.GroupVersionKind()]
	if !exist {
		return false, fmt.Errorf("customized interpreter for %q not found", object.GroupVersionKind())
	}

	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunHealthWithLua(localValueRetention.RetentionLua, object)

	if err != nil {
		return false, err
	}
	result := luaResult.Get(-1)
	var health bool
	if result.Type() == lua.LTBool {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return false, err
		}

		err = json.Unmarshal(jsonBytes, &health)
		if err != nil {
			return false, err
		}
	}

	return health, nil
}
