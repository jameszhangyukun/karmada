package configurableinterpreter

import (
	"fmt"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"testing"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestReplica(t *testing.T) {
	quantity := *resource.NewQuantity(1000, resource.BinarySI)
	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"memory": quantity,
								},
							},
						},
					},
					NodeSelector: map[string]string{"test": "test"},
				},
			},
		},
	}
	deploy.Status.Replicas = 10
	var configurableInterpreter ConfigurableInterpreter
	obj, _ := helper.ToUnstructured(deploy)
	replicas, requirements, err := configurableInterpreter.GetReplicas(obj)
	if err != nil {
		t.Errorf(err.Error())
	}
	if replicas != 10 {
		t.Errorf("Error replica error replica != 10")
	}
	if requirements.NodeClaim.NodeSelector["test"] != "test" {
		t.Errorf("requirements.NodeClaim.NodeSelector[test] not equal test")
	}
	t.Logf("Success test")

}

func TestReviseDeploymentReplica(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		replica     int32
		expected    *unstructured.Unstructured
		expectError bool
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
						"replicas": int64(3),
					},
				},
			},
			expectError: false,
		},
	}
	var configurableInterpreter ConfigurableInterpreter

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := configurableInterpreter.ReviseReplica(tt.object, int64(tt.replica))
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

func TestRetainPod(t *testing.T) {

	observedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-busybox",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "busybox", Image: "busybox:latest", Command: []string{"sleep", "1000"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name: "TestMount",
						},
					}},
			},
			ServiceAccountName: "TestCase",
			NodeName:           "testNode",
			Volumes: []corev1.Volume{
				{
					Name:         "TestVolume",
					VolumeSource: corev1.VolumeSource{},
				},
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
	observedPodUnstructured, _ := helper.ToUnstructured(observedPod)
	desiredPod := observedPod.DeepCopy()
	desiredPod.Spec.NodeName = "DesiredPod"
	desiredPod.Spec.Containers[0].VolumeMounts[0].Name = "DesiredMountName"
	desiredPodUnstructured, _ := helper.ToUnstructured(desiredPod)

	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		desired *unstructured.Unstructured
	}{
		{
			name:    "TestCase1",
			object:  observedPodUnstructured,
			desired: desiredPodUnstructured,
		},
	}
	var configurableInterpreter ConfigurableInterpreter

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := configurableInterpreter.Retain(tt.desired, tt.object)
			resPod := &corev1.Pod{}
			fmt.Printf("%v", res)
			err = helper.ConvertToTypedObject(observedPodUnstructured, resPod)
			if err == nil && observedPod.Spec.NodeName == resPod.Spec.NodeName {
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
		"replicas":            1,
		"readyReplicas":       1,
		"updatedReplicas":     1,
		"availableReplicas":   1,
		"unavailableReplicas": 0,
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	oldDeploy := &appsv1.Deployment{TypeMeta: metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	},
	}
	newDeploy := &appsv1.Deployment{Status: appsv1.DeploymentStatus{Replicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2}}
	oldObj, _ := helper.ToUnstructured(oldDeploy)
	newObj, _ := helper.ToUnstructured(newDeploy)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update deployment status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
	}
	var configurableInterpreter ConfigurableInterpreter

	for _, tt := range tests {
		actualObj, _ := configurableInterpreter.AggregateStatus(tt.curObj, tt.aggregatedStatusItems)
		actualDeploy := appsv1.DeploymentStatus{}
		err := helper.ConvertToTypedObject(actualObj.Object["status"], &actualDeploy)
		expectDeploy := appsv1.DeploymentStatus{}

		err = helper.ConvertToTypedObject(tt.expectedObj.Object["status"], &expectDeploy)
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
	newObj, _ := helper.ToUnstructured(newDeploy)

	tests := []struct {
		name        string
		curObj      *unstructured.Unstructured
		expectedObj bool
	}{
		{
			name:        "update deployment status",
			curObj:      newObj,
			expectedObj: true,
		},
	}
	var configurableInterpreter ConfigurableInterpreter

	for _, tt := range tests {
		flag, _ := configurableInterpreter.InterpretHealth(tt.curObj)

		if reflect.DeepEqual(flag, tt.expectedObj) {
			fmt.Printf("Success \n")
		}
	}

}

func Test_getEntireStatus(t *testing.T) {
	testMap := map[string]interface{}{"key": "value"}
	wantRawExtension, _ := helper.BuildStatusRawExtension(testMap)
	type args struct {
		object *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			"object doesn't have status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
			},
			nil,
			false,
		},
		{
			"object have wrong format status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"status": "a string",
					},
				},
			},
			nil,
			true,
		},
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
		},
	}
	var configurableInterpreter ConfigurableInterpreter

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configurableInterpreter.ReflectStatus(tt.args.object)
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

func TestPodDeploy(t *testing.T) {

	newPod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}
	newObj, _ := helper.ToUnstructured(&newPod)

	tests := []struct {
		name   string
		curObj *unstructured.Unstructured
	}{
		{
			name:   "get pod deploy",
			curObj: newObj,
		},
	}
	var configurableInterpreter ConfigurableInterpreter

	for _, tt := range tests {
		res, _ := configurableInterpreter.GetDependencies(tt.curObj)

		fmt.Printf("%v", res)
	}

}
