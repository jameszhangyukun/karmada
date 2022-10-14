package configurableinterpreter

import (
	"encoding/json"
	"fmt"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"

	"io/ioutil"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"os"
)

type ConfigurableInterpreter struct {
}

// HookEnabled tells if any hook exist for specific resource type and operation type.
func (e *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	switch operationType {

	// TODO(RainbowMango): more cases should be added here
	}

	klog.V(4).Infof("Default interpreter is not enabled for kind %q with operation %q.", kind, operationType)
	return false
}

type ReplicaResult struct {
	Replica int `json:"replica"`
	workv1alpha2.ReplicaRequirements
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (e *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	luaScript := getMockLua("replica.lua")
	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunLua(object, luaScript)

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
func getMockLua(scriptFile string) string {
	data, err := ioutil.ReadFile("./lua/" + scriptFile)
	if err != nil {
		fmt.Printf("%v", err)
		if os.IsNotExist(err) {
			return ""
		}
		return ""
	}
	return string(data)
}

// ReviseReplica revises the replica of the given object.
func (e *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	luaScript := getMockLua("revisereplica.lua")
	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunRevisereplicaWithLua(luaScript, object, replica)

	if err != nil {
		return nil, err
	}
	result := luaResult.Get(-1)
	replicaRequirement := &unstructured.Unstructured{}

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, replicaRequirement)
		if err != nil {
			return nil, err
		}
	}

	return replicaRequirement, nil
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (e *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	luaScript := getMockLua("retain.lua")
	vm := helper.VM{UseOpenLibs: false}
	luaResult, err := vm.RunRetainWithLua(luaScript, desired, observed)

	if err != nil {
		return nil, err
	}
	result := luaResult.Get(-1)
	replicaRequirement := &unstructured.Unstructured{}

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, replicaRequirement)
		if err != nil {
			return nil, err
		}
	}

	return replicaRequirement, nil
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (e *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	var aggregateItem []map[string]interface{}
	luaScript := getMockLua("aggregate.lua")
	vm := helper.VM{UseOpenLibs: false}

	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := make(map[string]interface{})
		if err := json.Unmarshal(item.Status.Raw, &temp); err != nil {
			fmt.Errorf(err.Error())
			return nil, err
		}

		aggregateItem = append(aggregateItem, temp)
	}

	luaResult, err := vm.RunAggregateWithLua(luaScript, object, aggregateItem)

	if err != nil {
		fmt.Errorf(err.Error())
		return nil, err
	}
	result := luaResult.Get(-1)
	replicaRequirement := &unstructured.Unstructured{}

	if result.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, replicaRequirement)
		if err != nil {
			return nil, err
		}
	}

	return replicaRequirement, nil
}

// GetDependencies returns the dependent resources of the given object.
func (e *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	luaScript := getMockLua("getPodDependency.lua")
	vm := helper.VM{UseOpenLibs: false}

	luaResult, err := vm.RunReflectStatusWithLua(luaScript, object)

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
func (e *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	luaScript := getMockLua("reflectwholestatus.lua")
	vm := helper.VM{UseOpenLibs: false}

	luaResult, err := vm.RunReflectStatusWithLua(luaScript, object)

	if err != nil {
		return nil, err
	}
	existResult := luaResult.Get(1)
	result := luaResult.Get(2)
	var exist bool
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
func (e *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (bool, error) {
	luaScript := getMockLua("health.lua")
	vm := helper.VM{UseOpenLibs: false}

	luaResult, err := vm.RunHealthWithLua(luaScript, object)

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
