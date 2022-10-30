package configmanager

import (
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CustomAccessor provides a common interface to get webhook configuration.
type CustomAccessor interface {
	GetRetention() *configv1alpha1.LocalValueRetention
	GetReplicaResource() *configv1alpha1.ReplicaResourceRequirement
	GetReplicaRevision() *configv1alpha1.ReplicaRevision
	GetStatusReflection() *configv1alpha1.StatusReflection
	GetStatusAggregation() *configv1alpha1.StatusAggregation
	GetHealthInterpretation() *configv1alpha1.HealthInterpretation
	GetDependencyInterpretation() *configv1alpha1.HealthInterpretation
	GetConfigurationName() string
	GetConfigurationTargetGVK() schema.GroupVersionKind
}

type resourceCustomAccessor struct {
	retention                *configv1alpha1.LocalValueRetention
	replicaResource          *configv1alpha1.ReplicaResourceRequirement
	replicaRevision          *configv1alpha1.ReplicaRevision
	statusReflection         *configv1alpha1.StatusReflection
	statusAggregation        *configv1alpha1.StatusAggregation
	healthInterpretation     *configv1alpha1.HealthInterpretation
	dependencyInterpretation *configv1alpha1.DependencyInterpretation
	configurationName        string
	configurationTargetGVK   schema.GroupVersionKind
}

// NewResourceCustomAccessorAccessor create an accessor for webhook.
func NewResourceCustomAccessorAccessor(customization *configv1alpha1.ResourceInterpreterCustomization) CustomAccessor {
	return &resourceCustomAccessor{
		retention:                customization.Spec.Customizations.Retention,
		replicaResource:          customization.Spec.Customizations.ReplicaResource,
		replicaRevision:          customization.Spec.Customizations.ReplicaRevision,
		statusReflection:         customization.Spec.Customizations.StatusReflection,
		statusAggregation:        customization.Spec.Customizations.StatusAggregation,
		healthInterpretation:     customization.Spec.Customizations.HealthInterpretation,
		dependencyInterpretation: customization.Spec.Customizations.DependencyInterpretation,
		configurationName:        customization.Name,
		configurationTargetGVK: schema.GroupVersionKind{
			Version: customization.Spec.Target.APIVersion,
			Kind:    customization.Spec.Target.Kind,
		},
	}
}

func (a *resourceCustomAccessor) GetReplicaRevision() *configv1alpha1.ReplicaRevision {
	return a.replicaRevision
}

// GetRetention gets a string that uniquely identifies the webhook.
func (a *resourceCustomAccessor) GetRetention() *configv1alpha1.LocalValueRetention {
	return a.retention
}

func (a *resourceCustomAccessor) GetReplicaResource() *configv1alpha1.ReplicaResourceRequirement {
	return a.replicaResource
}

func (a *resourceCustomAccessor) GetStatusReflection() *configv1alpha1.StatusReflection {
	return a.statusReflection
}

func (a *resourceCustomAccessor) GetStatusAggregation() *configv1alpha1.StatusAggregation {
	return a.statusAggregation
}

func (a *resourceCustomAccessor) GetHealthInterpretation() *configv1alpha1.HealthInterpretation {
	return a.healthInterpretation
}

func (a *resourceCustomAccessor) GetDependencyInterpretation() *configv1alpha1.HealthInterpretation {
	return a.healthInterpretation
}

func (a *resourceCustomAccessor) GetConfigurationName() string {
	return a.configurationName
}

func (a *resourceCustomAccessor) GetConfigurationTargetGVK() schema.GroupVersionKind {
	return a.configurationTargetGVK
}
