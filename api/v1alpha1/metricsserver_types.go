/*
Copyright 2025.

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

// MetricsServerSpec defines the desired state of MetricsServer.
type MetricsServerSpec struct {
	// Image specifies the container image to use for metrics-server
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="registry.k8s.io/metrics-server/metrics-server:v0.7.2"
	Image string `json:"image,omitempty"`

	// Replicas is the number of replicas for the metrics-server deployment
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines the compute resources for the metrics-server container
	// +kubebuilder:validation:Optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Args specifies additional command-line arguments for metrics-server
	// +kubebuilder:validation:Optional
	Args []string `json:"args,omitempty"`

	// KubeletInsecureTLS disables TLS verification when connecting to kubelets
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	KubeletInsecureTLS *bool `json:"kubeletInsecureTLS,omitempty"`

	// NodeSelector defines node selection constraints for metrics-server pods
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations defines tolerations for metrics-server pods
	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity defines affinity constraints for metrics-server pods
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// ServiceMonitor enables creation of a ServiceMonitor for Prometheus monitoring
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	ServiceMonitor *bool `json:"serviceMonitor,omitempty"`

	// PodDisruptionBudget enables creation of a PodDisruptionBudget
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	PodDisruptionBudget *bool `json:"podDisruptionBudget,omitempty"`

	// HostNetwork enables host networking for metrics-server pods
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	HostNetwork *bool `json:"hostNetwork,omitempty"`

	// PriorityClassName specifies the priority class for metrics-server pods
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="system-cluster-critical"
	PriorityClassName *string `json:"priorityClassName,omitempty"`

	// ServiceLabels specifies additional labels for the metrics-server service
	// +kubebuilder:validation:Optional
	ServiceLabels map[string]string `json:"serviceLabels,omitempty"`

	// ServiceAnnotations specifies additional annotations for the metrics-server service
	// +kubebuilder:validation:Optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// PodLabels specifies additional labels for metrics-server pods
	// +kubebuilder:validation:Optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// PodAnnotations specifies additional annotations for metrics-server pods
	// +kubebuilder:validation:Optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// ServiceAccountAnnotations specifies additional annotations for the service account
	// +kubebuilder:validation:Optional
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations,omitempty"`
}

// MetricsServerStatus defines the observed state of MetricsServer.
type MetricsServerStatus struct {
	// Conditions represent the latest available observations of the MetricsServer's state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ReadyReplicas is the number of ready replicas
	// +kubebuilder:validation:Optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available replicas
	// +kubebuilder:validation:Optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ms
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MetricsServer is the Schema for the metrics-servers API.
type MetricsServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetricsServerSpec   `json:"spec,omitempty"`
	Status MetricsServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MetricsServerList contains a list of MetricsServer.
type MetricsServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetricsServer{}, &MetricsServerList{})
}

// GetReplicas returns the number of replicas with default
func (m *MetricsServer) GetReplicas() int32 {
	if m.Spec.Replicas == nil {
		return 1
	}
	return *m.Spec.Replicas
}

// GetImage returns the image with default
func (m *MetricsServer) GetImage() string {
	if m.Spec.Image == "" {
		return "registry.k8s.io/metrics-server/metrics-server:v0.7.2"
	}
	return m.Spec.Image
}

// GetKubeletInsecureTLS returns whether to use insecure TLS with default
func (m *MetricsServer) GetKubeletInsecureTLS() bool {
	if m.Spec.KubeletInsecureTLS == nil {
		return true
	}
	return *m.Spec.KubeletInsecureTLS
}

// GetServiceMonitor returns whether to create ServiceMonitor with default
func (m *MetricsServer) GetServiceMonitor() bool {
	if m.Spec.ServiceMonitor == nil {
		return false
	}
	return *m.Spec.ServiceMonitor
}

// GetPodDisruptionBudget returns whether to create PodDisruptionBudget with default
func (m *MetricsServer) GetPodDisruptionBudget() bool {
	if m.Spec.PodDisruptionBudget == nil {
		return false
	}
	return *m.Spec.PodDisruptionBudget
}

// GetHostNetwork returns whether to use host networking with default
func (m *MetricsServer) GetHostNetwork() bool {
	if m.Spec.HostNetwork == nil {
		return false
	}
	return *m.Spec.HostNetwork
}

// GetPriorityClassName returns the priority class name with default
func (m *MetricsServer) GetPriorityClassName() string {
	if m.Spec.PriorityClassName == nil {
		return "system-cluster-critical"
	}
	return *m.Spec.PriorityClassName
}
