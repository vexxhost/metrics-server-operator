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

package builder

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"

	corev1alpha1 "github.com/vexxhost/metrics-server-operator/api/v1alpha1"
)

// BuildServiceAccount creates a ServiceAccount for MetricsServer
func BuildServiceAccount(ms *corev1alpha1.MetricsServer) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        corev1alpha1.DefaultServiceAccountName,
			Namespace:   corev1alpha1.DefaultNamespace,
			Labels:      buildLabels(ms),
			Annotations: ms.Spec.ServiceAccountAnnotations,
		},
	}
	return sa
}

// BuildClusterRole creates a ClusterRole for MetricsServer
func BuildClusterRole(ms *corev1alpha1.MetricsServer) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   corev1alpha1.DefaultClusterRoleName,
			Labels: buildLabels(ms),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/metrics"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "nodes", "nodes/stats", "namespaces", "configmaps"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
}

// BuildClusterRoleAggregatedReader creates an aggregated ClusterRole for viewing metrics
func BuildClusterRoleAggregatedReader(ms *corev1alpha1.MetricsServer) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   corev1alpha1.DefaultClusterRoleAggregatedReaderName,
			Labels: buildLabelsWithAggregation(ms),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"metrics.k8s.io"},
				Resources: []string{"pods", "nodes"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
}

// BuildClusterRoleBinding creates a ClusterRoleBinding for MetricsServer
func BuildClusterRoleBinding(ms *corev1alpha1.MetricsServer) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   corev1alpha1.DefaultClusterRoleBindingName,
			Labels: buildLabels(ms),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     corev1alpha1.DefaultClusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      corev1alpha1.DefaultServiceAccountName,
				Namespace: corev1alpha1.DefaultNamespace,
			},
		},
	}
}

// BuildRoleBindingAuthReader creates a RoleBinding for auth delegation
func BuildRoleBindingAuthReader(ms *corev1alpha1.MetricsServer) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      corev1alpha1.DefaultRoleBindingAuthReaderName,
			Namespace: corev1alpha1.DefaultNamespace,
			Labels:    buildLabels(ms),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "extension-apiserver-authentication-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      corev1alpha1.DefaultServiceAccountName,
				Namespace: corev1alpha1.DefaultNamespace,
			},
		},
	}
}

// BuildClusterRoleBindingAuthDelegator creates a ClusterRoleBinding for auth delegation
func BuildClusterRoleBindingAuthDelegator(ms *corev1alpha1.MetricsServer) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s:system:auth-delegator", corev1alpha1.DefaultServiceAccountName),
			Labels: buildLabels(ms),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      corev1alpha1.DefaultServiceAccountName,
				Namespace: corev1alpha1.DefaultNamespace,
			},
		},
	}
}

// BuildService creates a Service for MetricsServer
func BuildService(ms *corev1alpha1.MetricsServer) *corev1.Service {
	labels := buildLabels(ms)
	for k, v := range ms.Spec.ServiceLabels {
		labels[k] = v
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        corev1alpha1.DefaultServiceName,
			Namespace:   corev1alpha1.DefaultNamespace,
			Labels:      labels,
			Annotations: ms.Spec.ServiceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: buildSelectorLabels(ms),
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromString("https"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	return svc
}

// BuildDeployment creates a Deployment for MetricsServer
func BuildDeployment(ms *corev1alpha1.MetricsServer) *appsv1.Deployment {
	labels := buildLabels(ms)
	selectorLabels := buildSelectorLabels(ms)

	podLabels := make(map[string]string)
	for k, v := range selectorLabels {
		podLabels[k] = v
	}
	for k, v := range ms.Spec.PodLabels {
		podLabels[k] = v
	}

	args := []string{
		"--cert-dir=/tmp",
		"--secure-port=10250",
		"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
		"--kubelet-use-node-status-port",
		"--metric-resolution=15s",
	}

	if ms.GetKubeletInsecureTLS() {
		args = append(args, "--kubelet-insecure-tls")
	}

	args = append(args, ms.Spec.Args...)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      corev1alpha1.DefaultDeploymentName,
			Namespace: corev1alpha1.DefaultNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(ms.GetReplicas()),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: ms.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: corev1alpha1.DefaultServiceAccountName,
					PriorityClassName:  ms.GetPriorityClassName(),
					HostNetwork:        ms.GetHostNetwork(),
					NodeSelector:       ms.Spec.NodeSelector,
					Tolerations:        ms.Spec.Tolerations,
					Affinity:           ms.Spec.Affinity,
					Containers: []corev1.Container{
						{
							Name:            "metrics-server",
							Image:           ms.GetImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            args,
							Ports: []corev1.ContainerPort{
								{
									Name:          "https",
									ContainerPort: 10250,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem:   ptr.To(true),
								RunAsNonRoot:             ptr.To(true),
								RunAsUser:                ptr.To(int64(1000)),
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "tmp",
									MountPath: "/tmp",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/livez",
										Port:   intstr.FromString("https"),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								PeriodSeconds:    10,
								FailureThreshold: 3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromString("https"),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								InitialDelaySeconds: 20,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							Resources: buildResources(ms),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return deployment
}

// BuildAPIService creates an APIService for MetricsServer
func BuildAPIService(ms *corev1alpha1.MetricsServer) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name:   corev1alpha1.DefaultAPIServiceName,
			Labels: buildLabels(ms),
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:                "metrics.k8s.io",
			Version:              "v1beta1",
			GroupPriorityMinimum: 100,
			VersionPriority:      100,
			Service: &apiregistrationv1.ServiceReference{
				Name:      corev1alpha1.DefaultServiceName,
				Namespace: corev1alpha1.DefaultNamespace,
			},
			InsecureSkipTLSVerify: true,
		},
	}
}

// BuildPodDisruptionBudget creates a PodDisruptionBudget for MetricsServer
func BuildPodDisruptionBudget(ms *corev1alpha1.MetricsServer) *policyv1.PodDisruptionBudget {
	minAvailable := intstr.FromInt(1)
	if ms.GetReplicas() == 1 {
		minAvailable = intstr.FromInt(0)
	}

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      corev1alpha1.DefaultDeploymentName,
			Namespace: corev1alpha1.DefaultNamespace,
			Labels:    buildLabels(ms),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: buildSelectorLabels(ms),
			},
		},
	}
}

// buildLabels creates standard labels for all resources
func buildLabels(ms *corev1alpha1.MetricsServer) map[string]string {
	labels := map[string]string{
		corev1alpha1.LabelManagedBy: corev1alpha1.LabelManagedByValue,
		corev1alpha1.LabelInstance:  ms.Name,
		corev1alpha1.LabelComponent: corev1alpha1.LabelComponentValue,
		corev1alpha1.LabelName:      corev1alpha1.LabelNameValue,
	}
	return labels
}

// buildSelectorLabels creates selector labels for pods
func buildSelectorLabels(ms *corev1alpha1.MetricsServer) map[string]string {
	return map[string]string{
		corev1alpha1.LabelInstance:  ms.Name,
		corev1alpha1.LabelComponent: corev1alpha1.LabelComponentValue,
	}
}

// buildLabelsWithAggregation creates labels for aggregated cluster roles
func buildLabelsWithAggregation(ms *corev1alpha1.MetricsServer) map[string]string {
	labels := buildLabels(ms)
	labels["rbac.authorization.k8s.io/aggregate-to-view"] = "true"
	labels["rbac.authorization.k8s.io/aggregate-to-edit"] = "true"
	labels["rbac.authorization.k8s.io/aggregate-to-admin"] = "true"
	return labels
}

// buildResources creates resource requirements with defaults
func buildResources(ms *corev1alpha1.MetricsServer) corev1.ResourceRequirements {
	if ms.Spec.Resources != nil {
		return *ms.Spec.Resources
	}

	// Default resources - production-ready, conservative limits
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}
}
