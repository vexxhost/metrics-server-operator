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

const (
	// DefaultNamespace is the default namespace for metrics-server deployment
	DefaultNamespace = "kube-system"

	// DefaultServiceAccountName is the default service account name
	DefaultServiceAccountName = "metrics-server"

	// DefaultServiceName is the default service name
	DefaultServiceName = "metrics-server"

	// DefaultDeploymentName is the default deployment name
	DefaultDeploymentName = "metrics-server"

	// DefaultAPIServiceName is the default API service name
	DefaultAPIServiceName = "v1beta1.metrics.k8s.io"

	// DefaultClusterRoleName is the default cluster role name
	DefaultClusterRoleName = "system:metrics-server"

	// DefaultClusterRoleBindingName is the default cluster role binding name
	DefaultClusterRoleBindingName = "system:metrics-server"

	// DefaultClusterRoleAggregatedReaderName is the default aggregated reader cluster role name
	DefaultClusterRoleAggregatedReaderName = "system:metrics-server-aggregated-reader"

	// DefaultRoleBindingAuthReaderName is the default auth reader role binding name
	DefaultRoleBindingAuthReaderName = "metrics-server-auth-reader"

	// LabelManagedBy is the label key for managed-by
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// LabelManagedByValue is the label value for managed-by
	LabelManagedByValue = "metrics-server-operator"

	// LabelInstance is the label key for instance
	LabelInstance = "app.kubernetes.io/instance"

	// LabelComponent is the label key for component
	LabelComponent = "app.kubernetes.io/component"

	// LabelComponentValue is the label value for component
	LabelComponentValue = "metrics-server"

	// LabelName is the label key for name
	LabelName = "app.kubernetes.io/name"

	// LabelNameValue is the label value for name
	LabelNameValue = "metrics-server"

	// ConditionTypeReady is the condition type for ready status
	ConditionTypeReady = "Ready"

	// ConditionTypeProgressing is the condition type for progressing status
	ConditionTypeProgressing = "Progressing"

	// ConditionTypeDegraded is the condition type for degraded status
	ConditionTypeDegraded = "Degraded"

	// ConditionTypeAvailable is the condition type for available status
	ConditionTypeAvailable = "Available"

	// ReasonReconciling is the reason for reconciling condition
	ReasonReconciling = "Reconciling"

	// ReasonReady is the reason for ready condition
	ReasonReady = "Ready"

	// ReasonFailed is the reason for failed condition
	ReasonFailed = "Failed"

	// ReasonProgressing is the reason for progressing condition
	ReasonProgressing = "Progressing"

	// ReasonDeploymentNotReady is the reason when deployment is not ready
	ReasonDeploymentNotReady = "DeploymentNotReady"

	// ReasonDeploymentReady is the reason when deployment is ready
	ReasonDeploymentReady = "DeploymentReady"

	// ReasonAPIServiceNotReady is the reason when API service is not ready
	ReasonAPIServiceNotReady = "APIServiceNotReady"

	// ReasonAPIServiceReady is the reason when API service is ready
	ReasonAPIServiceReady = "APIServiceReady"

	// ReasonSingletonViolation is the reason when multiple MetricsServer instances exist
	ReasonSingletonViolation = "SingletonViolation"

	// FinalizerName is the finalizer name for MetricsServer resources
	FinalizerName = "metrics-server.core.vexxhost.com/finalizer"
)
