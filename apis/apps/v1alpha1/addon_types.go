/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddonSpec defines the desired state of Addon
type AddonSpec struct {
	// Description is the description of the addon.
	Description string `json:"description,omitempty"`

	// Type is the type of the addon.
	Type string `json:"type,omitempty"`

	// Namespace is the namespace to install the helm chart. if it's empty, will be installed to the same namespace as kubeblocks.
	Namespace string `json:"namespace,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// Helm defines the helm chart spec for the addon
	Helm *HelmTypeSpec `json:"helm,omitempty"`

	// Selector defines the selectors for the addon
	Selector *AddonSelector `json:"selector,omitempty"`

	// DefaultInstallValues is the addon default install parameters.
	DefaultInstallValues map[string]string `json:"defaultInstallValues,omitempty"`

	// Install defines the install spec for the addon
	Install *InstallSpec `json:"install,omitempty"`

	// Dependencies lists the names of other Addons that must be installed before this one.
	Dependencies []string `json:"dependencies,omitempty"`
}

// HelmTypeSpec defines the helm chart spec for the addon.
type HelmTypeSpec struct {
	// ChartLocationURL is the location of the helm chart.
	ChartLocationURL string `json:"chartLocationURL,omitempty"`

	// ChartName is the name of the helm chart.
	ChartName string `json:"chartName,omitempty"`

	// ChartVersion is the version of the helm chart.
	ChartVersion string `json:"chartVersion,omitempty"`

	// ValuesFiles is the list of helm values files.
	ValuesFiles []string `json:"valuesFiles,omitempty"`

	// Values is the helm chart values in raw yaml.
	Values string `json:"values,omitempty"`
}

// AddonSelector defines the selectors for the addon
type AddonSelector struct {
	// LabelSelector is the label selector for the addon
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// NamespaceSelector is the namespace selector for the addon
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// Labels is the selector to match addon labels.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is the selector to match addon annotations.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// InstallSpec defines the install spec for the addon
type InstallSpec struct {
	// HelmInstallValues defines the helm install values for the addon
	HelmInstallValues map[string]string `json:"helmInstallValues,omitempty"`

	// Enabled defines whether the addon is enabled
	Enabled bool `json:"enabled,omitempty"`

	// Resources is a list of resources to be installed.
	Resources []string `json:"resources,omitempty"`

	// ServiceAccount is the service account name to be used for install.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// Environment is the environment variables to be set for install.
	Environment map[string]string `json:"environment,omitempty"`
}

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// Phase is the phase of the addon
	Phase AddonPhase `json:"phase,omitempty"`

	// Conditions is an array of current observed addon conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// HelmRelease is the helm release name of the addon.
	HelmRelease string `json:"helmRelease,omitempty"`

	// ObservedGeneration is the most recent generation observed for this addon.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// AddonPhase is a label for the phase of an addon at the current time.
type AddonPhase string

// These are the valid phases of an addon.
const (
	// AddonDisabled means the addon is disabled
	AddonDisabled AddonPhase = "Disabled"
	// AddonEnabled means the addon is enabled
	AddonEnabled AddonPhase = "Enabled"
	// AddonFailed means the addon is failed
	AddonFailed AddonPhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ad
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",JSONPath=".spec.namespace",description="namespace for installed"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="addon type"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="addon phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Addon is the Schema for the addons API
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AddonList contains a list of Addon
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}

// Ensure Addon implements webhook.Validator (compile-time check)
var _ interface {
	ValidateCreate() error
	ValidateUpdate(old interface{}) error
	ValidateDelete() error
} = &Addon{}
