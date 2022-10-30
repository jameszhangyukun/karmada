package helper

import (
	"encoding/json"
	"fmt"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	luajson "layeh.com/gopher-json"
)

func ConvertLuaResultToUnstructured(luaResult lua.LValue) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	jsonBytes, err := luajson.Encode(luaResult)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, u)
	if err != nil {
		return nil, err
	}
	return u, err
}

func ConvertLuaResultToReplicaRequirements(luaResult lua.LValue) (*workv1alpha2.ReplicaRequirements, error) {
	u := &workv1alpha2.ReplicaRequirements{}
	jsonBytes, err := luajson.Encode(luaResult)
	if err != nil {
		klog.Errorf("lua json Encode error %v", err)
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, u)
	if err != nil {
		klog.Errorf(" json Unmarshal error %v", err)
		return nil, err
	}
	return u, err
}

func ConvertLuaResultToInt(luaResult lua.LValue) (int32, error) {
	var value int32
	jsonBytes, err := luajson.Encode(luaResult)
	if err != nil {
		return 0, fmt.Errorf("encode json error %s", err.Error())
	}
	err = json.Unmarshal(jsonBytes, &value)

	if err != nil {
		return 0, fmt.Errorf("unmarshal json error %s", err.Error())
	}
	return value, nil
}

func ConvertLuaResultToBool(luaResult lua.LValue) (bool, error) {
	var value bool
	jsonBytes, err := luajson.Encode(luaResult)
	if err != nil {
		return false, fmt.Errorf("encode json error %s", err.Error())
	}
	err = json.Unmarshal(jsonBytes, &value)

	if err != nil {
		return false, fmt.Errorf("unmarshal json error %s", err.Error())
	}
	return value, nil
}
