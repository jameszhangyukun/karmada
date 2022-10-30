package helper

import (
	"context"
	"encoding/json"
	"fmt"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	luajson "layeh.com/gopher-json"
	"time"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// VM Defines a struct that implements the luaVM
type VM struct {
	// UseOpenLibs flag to enable open libraries. Libraries are disabled by default while running, but enabled during testing to allow the use of print statements
	UseOpenLibs bool
}

func (vm VM) GetReplicas(obj *unstructured.Unstructured, script string) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return 0, nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	l.SetContext(ctx)

	objectValue := decodeValue(l, obj.Object)
	err = l.DoString(script)
	replicasLuaFunc := l.GetGlobal("GetReplicas")
	l.Push(replicasLuaFunc)
	l.Push(objectValue)
	l.Call(1, 2)
	fmt.Println("Execute success ")
	replicaRequirementResult := l.Get(l.GetTop())
	l.Pop(1)

	requires = &workv1alpha2.ReplicaRequirements{}
	if replicaRequirementResult.Type() == lua.LTTable {
		requires, err = ConvertLuaResultToReplicaRequirements(replicaRequirementResult)
		if err != nil {
			klog.Errorf("ConvertLuaResultToReplicaRequirements err %v", err.Error())
			return 0, nil, err
		}
	} else if replicaRequirementResult.Type() == lua.LTNil {
		requires = nil
	} else {
		return 0, nil, fmt.Errorf("expect the returned requires type is table but got %s", replicaRequirementResult.Type())
	}

	luaReplica := l.Get(l.GetTop())
	if luaReplica.Type() == lua.LTNumber {
		replica, err = ConvertLuaResultToInt(luaReplica)
		if err != nil {
			return 0, nil, err
		}
	} else {
		return 0, nil, fmt.Errorf("expect the returned replica type is number but got %s", luaReplica.Type())
	}
	return
}

func (vm VM) ReviseReplica(object *unstructured.Unstructured, replica int64, script string) (*unstructured.Unstructured, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	objectValue := decodeValue(l, object.Object)
	replicaValue := decodeValue(l, replica)
	err = l.DoString(script)
	reviseReplicaLuaFunc := l.GetGlobal("ReviseReplica")
	if reviseReplicaLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function ReviseReplica pleace check the function name")
	}
	l.Push(reviseReplicaLuaFunc)
	l.Push(objectValue)
	l.Push(replicaValue)

	l.Call(2, 1)
	luaResult := l.Get(l.GetTop())
	reviseReplicaResult := &unstructured.Unstructured{}
	if luaResult.Type() == lua.LTTable {
		reviseReplicaResult, err = ConvertLuaResultToUnstructured(luaResult)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}

	return reviseReplicaResult, nil
}

func (vm VM) setLib(l *lua.LState) error {
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		// load our 'safe' version of the OS library
		{lua.OsLibName, OpenSafeOs},
	} {
		if err := l.CallByParam(lua.P{
			Fn:      l.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			return err
		}
	}
	return nil
}

func (vm VM) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured, script string) (retained *unstructured.Unstructured, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	desiredObjectValue := decodeValue(l, desired.Object)
	observedObjectValue := decodeValue(l, observed.Object)
	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	retainLuaFunc := l.GetGlobal("Retain")
	if retainLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function Retatin pleace check the function ")
	}
	l.Push(retainLuaFunc)
	l.Push(desiredObjectValue)
	l.Push(observedObjectValue)

	l.Call(2, 1)
	luaResult := l.Get(l.GetTop())
	retainResult := &unstructured.Unstructured{}
	if luaResult.Type() == lua.LTTable {
		retainResult, err = ConvertLuaResultToUnstructured(luaResult)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}

	return retainResult, nil
}

func (vm VM) AggregateStatus(object *unstructured.Unstructured, item []map[string]interface{}, script string) (*unstructured.Unstructured, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}

	objectValue := decodeValue(l, object.Object)
	itemValue := decodeValue(l, item)
	retainLuaFunc := l.GetGlobal("AggregateStatus")
	if retainLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function AggregateStatus pleace check the function ")
	}
	l.Push(retainLuaFunc)
	l.Push(objectValue)
	l.Push(itemValue)

	l.Call(2, 1)
	luaResult := l.Get(l.GetTop())
	aggregateStatus := &unstructured.Unstructured{}
	if luaResult.Type() == lua.LTTable {
		aggregateStatus, err = ConvertLuaResultToUnstructured(luaResult)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}

	return aggregateStatus, nil
}

func (vm VM) InterpretHealth(object *unstructured.Unstructured, script string) (bool, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return false, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return false, err
	}
	getInterpretHealthLuaFunc := l.GetGlobal("InterpretHealth")
	if getInterpretHealthLuaFunc.Type() == lua.LTNil {
		return false, fmt.Errorf("can't get function InterpretHealth pleace check the function ")
	}
	objectValue := decodeValue(l, object.Object)
	l.Push(getInterpretHealthLuaFunc)
	l.Push(objectValue)

	l.Call(1, 1)

	var health bool
	luaResult := l.Get(l.GetTop())
	if luaResult.Type() == lua.LTBool {
		health, err = ConvertLuaResultToBool(luaResult)
		if err != nil {
			return false, err
		}
		return health, nil
	} else {
		return false, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}
}

func (vm VM) ReflectStatus(object *unstructured.Unstructured, script string) (status *runtime.RawExtension, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	getReflectStatusLuaFunc := l.GetGlobal("ReflectStatus")
	if getReflectStatusLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function ReflectStatus pleace check the function ")
	}
	objectValue := decodeValue(l, object.Object)
	l.Push(getReflectStatusLuaFunc)
	l.Push(objectValue)
	l.Call(1, 2)

	luaStatusResult := l.Get(l.GetTop())
	l.Pop(1)
	if luaStatusResult.Type() != lua.LTTable {
		return nil, fmt.Errorf("expect the returned replica type is table but got %s", luaStatusResult.Type())
	}

	luaExistResult := l.Get(l.GetTop())
	var exist bool
	if luaExistResult.Type() == lua.LTBool {
		exist, err = ConvertLuaResultToBool(luaExistResult)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("expect the returned replica type is bool but got %s", luaExistResult.Type())
	}

	if exist {
		resultMap := make(map[string]interface{})
		jsonBytes, err := luajson.Encode(luaStatusResult)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, &resultMap)
		if err != nil {
			return nil, err
		}
		return BuildStatusRawExtension(resultMap)
	}
	return nil, err

}

func (vm VM) GetDependencies(object *unstructured.Unstructured, script string) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	getDependenciesLuaFunc := l.GetGlobal("GetDependencies")
	if getDependenciesLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function Retatin pleace check the function ")
	}
	objectValue := decodeValue(l, object.Object)
	l.Push(getDependenciesLuaFunc)
	l.Push(objectValue)

	l.Call(1, 1)

	luaResult := l.Get(l.GetTop())
	if luaResult.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(luaResult)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, &dependencies)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}
	return
}

// Took logic from the link below and added the int, int32, and int64 types since the value would have type int64
// while actually running in the controller and it was not reproducible through testing.
// https://github.com/layeh/gopher-json/blob/97fed8db84274c421dbfffbb28ec859901556b97/json.go#L154
func decodeValue(L *lua.LState, value interface{}) lua.LValue {
	switch converted := value.(type) {
	case bool:
		return lua.LBool(converted)
	case float64:
		return lua.LNumber(converted)
	case string:
		return lua.LString(converted)
	case json.Number:
		return lua.LString(converted)
	case int:
		return lua.LNumber(converted)
	case int32:
		return lua.LNumber(converted)
	case int64:
		return lua.LNumber(converted)
	case []interface{}:
		arr := L.CreateTable(len(converted), 0)
		for _, item := range converted {
			arr.Append(decodeValue(L, item))
		}
		return arr
	case []map[string]interface{}:
		arr := L.CreateTable(len(converted), 0)
		for _, item := range converted {
			arr.Append(decodeValue(L, item))
		}
		return arr
	case map[string]interface{}:
		tbl := L.CreateTable(0, len(converted))
		for key, item := range converted {
			tbl.RawSetH(lua.LString(key), decodeValue(L, item))
		}
		return tbl
	case nil:
		return lua.LNil
	}

	return lua.LNil
}
