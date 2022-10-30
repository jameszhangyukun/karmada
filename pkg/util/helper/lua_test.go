package helper

import (
	"encoding/json"
	"fmt"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"reflect"
	"testing"
)

func TestGetReplicas(t *testing.T) {
	var replicas int32 = 1
	//quantity := *resource.NewQuantity(1000, resource.BinarySI)
	vm := VM{UseOpenLibs: false}
	tests := []struct {
		name      string
		deploy    *appsv1.Deployment
		luaScript string
		expected  bool
	}{
		//{
		//	name: "Test GetReplica",
		//	deploy: &appsv1.Deployment{
		//		TypeMeta: metav1.TypeMeta{
		//			Kind:       "Deployment",
		//			APIVersion: "v1",
		//		},
		//		ObjectMeta: metav1.ObjectMeta{
		//			Namespace: "foo",
		//			Name:      "bar",
		//		},
		//		Spec: appsv1.DeploymentSpec{
		//			Replicas: &replicas,
		//			Template: corev1.PodTemplateSpec{
		//				Spec: corev1.PodSpec{
		//					Containers: []corev1.Container{
		//						{
		//							Resources: corev1.ResourceRequirements{
		//								Limits: corev1.ResourceList{
		//									"memory": quantity,
		//								},
		//							},
		//						},
		//					},
		//					NodeSelector: map[string]string{"test": "test"},
		//				},
		//			},
		//		},
		//	},
		//	expected:  true,
		//	luaScript: "function GetReplicas(desiredObj)\n  \tprint(\"Hello World.\")\n nodeClaim = {}\n resourceRequest = {}\n result = {}\n\n replica = desiredObj.spec.replicas\n result.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits\n\n nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector\n nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations\n result.nodeClaim = nodeClaim\n\n return replica, result\nend",
		//},
		{
			name: "Test GetReplica",
			deploy: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{},
							},
						},
					},
				},
			},
			expected:  true,
			luaScript: "function GetReplicas(desiredObj)\n  \tprint(\"Hello World.\")\n nodeClaim = {}\n resourceRequest = {}\n result = {}\n\n replica = desiredObj.spec.replicas\n result.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits\n\n nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector\n nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations\n result.nodeClaim = {} result.nodeClaim = nil\n\n return replica, {}\nend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toUnstructured, _ := ToUnstructured(tt.deploy)
			replicas, requires, err := vm.GetReplicas(toUnstructured, tt.luaScript)
			klog.Infof("replicas %v", replicas)
			klog.Infof("requires %v", requires)
			if err != nil {
				t.Errorf(err.Error())
			}
		})
	}
}

func TestReviseDeploymentReplica(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		replica     int32
		expected    *unstructured.Unstructured
		expectError bool
		luaScript   string
	}{
		{
			name: "Deployment .spec.replicas accessor error, expected int64",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			replica:     3,
			expectError: true,
			luaScript:   "function ReviseReplica(desiredObj, desiredReplica)\n  desiredObj.spec.replicas = desiredReplica\n  return desiredObj\n end",
		},
		{
			name: "revise deployment replica",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replica: 3,
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
					},
				},
			},
			expectError: false,
			luaScript:   "function ReviseReplica(desiredObj, desiredReplica)\n  desiredObj.spec.replicas = desiredReplica\n  return desiredObj\n end",
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := vm.ReviseReplica(tt.object, int64(tt.replica), tt.luaScript)
			deploy := &appsv1.Deployment{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), deploy)
			if err == nil && *deploy.Spec.Replicas == tt.replica {
				t.Log("Success Test")
			}
			if err != nil {
				t.Errorf(err.Error())
			}

		})
	}
}

func TestAggregateDeploymentStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"replicas":            0,
		"readyReplicas":       0,
		"updatedReplicas":     0,
		"availableReplicas":   1,
		"unavailableReplicas": 0,
	}
	raw, _ := BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	oldDeploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
	}
	oldDeploy.Status = appsv1.DeploymentStatus{
		Replicas: 0, ReadyReplicas: 1, UpdatedReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: 0}

	newDeploy := &appsv1.Deployment{Status: appsv1.DeploymentStatus{Replicas: 0, ReadyReplicas: 0, UpdatedReplicas: 0, AvailableReplicas: 2, UnavailableReplicas: 0}}
	oldObj, _ := ToUnstructured(oldDeploy)
	newObj, _ := ToUnstructured(newDeploy)

	var aggregateItem []map[string]interface{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := make(map[string]interface{})
		if err := json.Unmarshal(item.Status.Raw, &temp); err != nil {
		}
		aggregateItem = append(aggregateItem, temp)
	}

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []map[string]interface{}
		expectedObj           *unstructured.Unstructured
		luaScript             string
	}{
		{
			name:                  "update deployment status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregateItem,
			expectedObj:           newObj,
			luaScript:             "function AggregateStatus(desiredObj, statusItems)\n    for i = 1, #statusItems do\n      desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + statusItems[i].readyReplicas\n    end\n    return desiredObj\n end",
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		actualObj, _ := vm.AggregateStatus(tt.curObj, tt.aggregatedStatusItems, tt.luaScript)
		actualDeploy := appsv1.DeploymentStatus{}
		err := ConvertToTypedObject(actualObj.Object["status"], &actualDeploy)
		expectDeploy := appsv1.DeploymentStatus{}

		err = ConvertToTypedObject(tt.expectedObj.Object["status"], &expectDeploy)
		if err != nil {
			fmt.Printf(err.Error())
		}
		if reflect.DeepEqual(expectDeploy, actualDeploy) {
			fmt.Printf("Success \n")
		}
	}
}

func TestHealthDeploymentStatus(t *testing.T) {
	var cnt int32 = 2
	newDeploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &cnt,
		},

		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2}}
	newObj, _ := ToUnstructured(newDeploy)

	tests := []struct {
		name        string
		curObj      *unstructured.Unstructured
		expectedObj bool
		luaScript   string
	}{
		{
			name:        "TestHealthDeploymentStatus",
			curObj:      newObj,
			expectedObj: true,
			luaScript:   "function InterpretHealth(observedObj)\n    return (observedObj.status.updatedReplicas == observedObj.spec.replicas) and (observedObj.metadata.generation == observedObj.status.observedGeneration)\nend ",
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		flag, _ := vm.InterpretHealth(tt.curObj, tt.luaScript)

		if reflect.DeepEqual(flag, tt.expectedObj) {
			fmt.Printf("Success \n")
		}
	}

}

func TestRetainDeployment(t *testing.T) {
	tests := []struct {
		name        string
		desiredObj  *unstructured.Unstructured
		observedObj *unstructured.Unstructured
		expectError bool
		luaScript   string
	}{
		{
			name: "Deployment .spec.replicas accessor error, expected int64",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
					},
				},
			},
			expectError: true,
			luaScript:   "function Retain(desiredObj, observedObj)\n  desiredObj = observedObj\n  return desiredObj\n end",
		},
		{
			name: "revise deployment replica",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
					},
				},
			},
			expectError: false,
			luaScript:   "function Retain(desiredObj, observedObj)\n  desiredObj = observedObj\n  return desiredObj\n end",
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := vm.Retain(tt.desiredObj, tt.observedObj, tt.luaScript)
			deploy := &appsv1.Deployment{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), deploy)
			if err == nil && reflect.DeepEqual(deploy, tt.observedObj) {
				t.Log("Success Test")
			}
			if err != nil {
				t.Errorf(err.Error())
			}

		})
	}
}

func TestStatusReflection(t *testing.T) {
	testMap := map[string]interface{}{"key": "value"}
	wantRawExtension, _ := BuildStatusRawExtension(testMap)
	type args struct {
		object *unstructured.Unstructured
	}
	tests := []struct {
		name      string
		args      args
		want      *runtime.RawExtension
		wantErr   bool
		luaScript string
	}{
		{
			"object have correct format status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"status": testMap,
					},
				},
			},
			wantRawExtension,
			false,
			"function ReflectStatus (observedObj)\n    print(observedObj == nil)\n    print(observedObj.status == nil)\n    if observedObj.status == nil then\n        return false, nil\n    end\n    return true, observedObj.status\nend\n",
		},
	}

	vm := VM{UseOpenLibs: false}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vm.ReflectStatus(tt.args.object, tt.luaScript)
			if (err != nil) != tt.wantErr {
				t.Errorf("reflectWholeStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reflectWholeStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDeployPodDependencies(t *testing.T) {

	newDeploy := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "test",
				},
			},
		},
	}
	newObj, _ := ToUnstructured(&newDeploy)

	tests := []struct {
		name      string
		curObj    *unstructured.Unstructured
		luaScript string
	}{
		{
			name:      "Get GetDeployPodDependencies",
			curObj:    newObj,
			luaScript: "\nfunction GetDependencies(desiredObj)\ndependentSas = {}\nrefs = {}\n\nif desiredObj.spec.template.spec.serviceAccountName ~= \"\" and desiredObj.spec.template.spec.serviceAccountName ~= \"default\" then\n    dependentSas[desiredObj.spec.template.spec.serviceAccountName] = true\nend\n\nlocal idx = 1\n\nfor key, value in pairs(dependentSas) do\n    dependObj = {}\n    dependObj.apiVersion = \"v1\"\n    dependObj.kind = \"ServiceAccount\"\n    dependObj.name = key\n    dependObj.namespace = desiredObj.namespace\n    refs[idx] = {}\n    refs[idx] = dependObj\n    idx = idx + 1\nend\n\nreturn refs\nend",
		},
	}

	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		res, _ := vm.GetDependencies(tt.curObj, tt.luaScript)
		fmt.Printf("%v", res)
	}

}
