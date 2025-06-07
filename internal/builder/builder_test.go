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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	corev1alpha1 "github.com/vexxhost/metrics-server-operator/api/v1alpha1"
)

func TestBuildServiceAccount(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
	}

	sa := BuildServiceAccount(ms)

	if sa.Name != corev1alpha1.DefaultServiceAccountName {
		t.Errorf("Expected ServiceAccount name %s, got %s", corev1alpha1.DefaultServiceAccountName, sa.Name)
	}

	if sa.Namespace != corev1alpha1.DefaultNamespace {
		t.Errorf("Expected ServiceAccount namespace %s, got %s", corev1alpha1.DefaultNamespace, sa.Namespace)
	}

	if sa.Labels[corev1alpha1.LabelManagedBy] != corev1alpha1.LabelManagedByValue {
		t.Errorf("Expected managed-by label %s, got %s",
			corev1alpha1.LabelManagedByValue, sa.Labels[corev1alpha1.LabelManagedBy])
	}
}

func TestBuildClusterRole(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
	}

	cr := BuildClusterRole(ms)

	if cr.Name != corev1alpha1.DefaultClusterRoleName {
		t.Errorf("Expected ClusterRole name %s, got %s", corev1alpha1.DefaultClusterRoleName, cr.Name)
	}

	expectedRules := 2
	if len(cr.Rules) != expectedRules {
		t.Errorf("Expected %d rules, got %d", expectedRules, len(cr.Rules))
	}

	// Check for nodes/metrics rule
	found := false
	for _, rule := range cr.Rules {
		for _, resource := range rule.Resources {
			if resource == "nodes/metrics" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Expected nodes/metrics resource in ClusterRole rules")
	}
}

func TestBuildService(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
		Spec: corev1alpha1.MetricsServerSpec{
			ServiceLabels: map[string]string{
				"custom": "label",
			},
			ServiceAnnotations: map[string]string{
				"custom": "annotation",
			},
		},
	}

	svc := BuildService(ms)

	if svc.Name != corev1alpha1.DefaultServiceName {
		t.Errorf("Expected Service name %s, got %s", corev1alpha1.DefaultServiceName, svc.Name)
	}

	if svc.Namespace != corev1alpha1.DefaultNamespace {
		t.Errorf("Expected Service namespace %s, got %s", corev1alpha1.DefaultNamespace, svc.Namespace)
	}

	if svc.Labels["custom"] != "label" {
		t.Error("Expected custom service label to be set")
	}

	if svc.Annotations["custom"] != "annotation" {
		t.Error("Expected custom service annotation to be set")
	}

	if len(svc.Spec.Ports) != 1 {
		t.Errorf("Expected 1 port, got %d", len(svc.Spec.Ports))
	}

	if svc.Spec.Ports[0].Port != 443 {
		t.Errorf("Expected port 443, got %d", svc.Spec.Ports[0].Port)
	}
}

func TestBuildDeployment(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
		Spec: corev1alpha1.MetricsServerSpec{
			Replicas: ptr.To(int32(2)),
			Image:    "custom-image:latest",
			Args:     []string{"--custom-arg"},
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("400Mi"),
				},
			},
			PodLabels: map[string]string{
				"custom": "pod-label",
			},
			PodAnnotations: map[string]string{
				"custom": "pod-annotation",
			},
		},
	}

	deployment := BuildDeployment(ms)

	if deployment.Name != corev1alpha1.DefaultDeploymentName {
		t.Errorf("Expected Deployment name %s, got %s", corev1alpha1.DefaultDeploymentName, deployment.Name)
	}

	if *deployment.Spec.Replicas != 2 {
		t.Errorf("Expected 2 replicas, got %d", *deployment.Spec.Replicas)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]
	if container.Image != "custom-image:latest" {
		t.Errorf("Expected image custom-image:latest, got %s", container.Image)
	}

	// Check for required args
	hasInsecureTLS := false
	hasCustomArg := false
	for _, arg := range container.Args {
		if arg == "--kubelet-insecure-tls" {
			hasInsecureTLS = true
		}
		if arg == "--custom-arg" {
			hasCustomArg = true
		}
	}
	if !hasInsecureTLS {
		t.Error("Expected --kubelet-insecure-tls arg by default")
	}
	if !hasCustomArg {
		t.Error("Expected custom arg to be included")
	}

	// Check resources
	cpuRequest := container.Resources.Requests[corev1.ResourceCPU]
	if cpuRequest.String() != "200m" {
		t.Errorf("Expected CPU request 200m, got %s", cpuRequest.String())
	}

	// Check pod labels
	if deployment.Spec.Template.Labels["custom"] != "pod-label" {
		t.Error("Expected custom pod label to be set")
	}

	// Check pod annotations
	if deployment.Spec.Template.Annotations["custom"] != "pod-annotation" {
		t.Error("Expected custom pod annotation to be set")
	}
}

func TestBuildAPIService(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
	}

	apiService := BuildAPIService(ms)

	if apiService.Name != corev1alpha1.DefaultAPIServiceName {
		t.Errorf("Expected APIService name %s, got %s", corev1alpha1.DefaultAPIServiceName, apiService.Name)
	}

	if apiService.Spec.Group != "metrics.k8s.io" {
		t.Errorf("Expected group metrics.k8s.io, got %s", apiService.Spec.Group)
	}

	if apiService.Spec.Version != "v1beta1" {
		t.Errorf("Expected version v1beta1, got %s", apiService.Spec.Version)
	}

	if apiService.Spec.Service.Name != corev1alpha1.DefaultServiceName {
		t.Errorf("Expected service reference %s, got %s", corev1alpha1.DefaultServiceName, apiService.Spec.Service.Name)
	}

	if !apiService.Spec.InsecureSkipTLSVerify {
		t.Error("Expected InsecureSkipTLSVerify to be true")
	}
}

func TestBuildPodDisruptionBudget(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
		Spec: corev1alpha1.MetricsServerSpec{
			Replicas: ptr.To(int32(1)),
		},
	}

	pdb := BuildPodDisruptionBudget(ms)

	if pdb.Name != corev1alpha1.DefaultDeploymentName {
		t.Errorf("Expected PDB name %s, got %s", corev1alpha1.DefaultDeploymentName, pdb.Name)
	}

	// For single replica, minAvailable should be 0
	if pdb.Spec.MinAvailable.IntVal != 0 {
		t.Errorf("Expected minAvailable 0 for single replica, got %d", pdb.Spec.MinAvailable.IntVal)
	}

	// Test with multiple replicas
	ms.Spec.Replicas = ptr.To(int32(3))
	pdb = BuildPodDisruptionBudget(ms)
	if pdb.Spec.MinAvailable.IntVal != 1 {
		t.Errorf("Expected minAvailable 1 for multiple replicas, got %d", pdb.Spec.MinAvailable.IntVal)
	}
}

func TestMetricsServerGetMethods(t *testing.T) {
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
	}

	// Test default values
	if ms.GetReplicas() != 1 {
		t.Errorf("Expected default replicas 1, got %d", ms.GetReplicas())
	}

	if ms.GetImage() != "registry.k8s.io/metrics-server/metrics-server:v0.7.2" {
		t.Errorf("Expected default image, got %s", ms.GetImage())
	}

	if !ms.GetKubeletInsecureTLS() {
		t.Error("Expected default kubeletInsecureTLS to be true")
	}

	if ms.GetServiceMonitor() {
		t.Error("Expected default serviceMonitor to be false")
	}

	if ms.GetPodDisruptionBudget() {
		t.Error("Expected default podDisruptionBudget to be false")
	}

	if ms.GetHostNetwork() {
		t.Error("Expected default hostNetwork to be false")
	}

	if ms.GetPriorityClassName() != "system-cluster-critical" {
		t.Errorf("Expected default priorityClassName system-cluster-critical, got %s", ms.GetPriorityClassName())
	}

	// Test custom values
	ms.Spec.Replicas = ptr.To(int32(3))
	ms.Spec.Image = "custom:latest"
	ms.Spec.KubeletInsecureTLS = ptr.To(false)
	ms.Spec.ServiceMonitor = ptr.To(true)
	ms.Spec.PodDisruptionBudget = ptr.To(true)
	ms.Spec.HostNetwork = ptr.To(true)
	ms.Spec.PriorityClassName = ptr.To("custom-priority")

	if ms.GetReplicas() != 3 {
		t.Errorf("Expected custom replicas 3, got %d", ms.GetReplicas())
	}

	if ms.GetImage() != "custom:latest" {
		t.Errorf("Expected custom image, got %s", ms.GetImage())
	}

	if ms.GetKubeletInsecureTLS() {
		t.Error("Expected custom kubeletInsecureTLS to be false")
	}

	if !ms.GetServiceMonitor() {
		t.Error("Expected custom serviceMonitor to be true")
	}

	if !ms.GetPodDisruptionBudget() {
		t.Error("Expected custom podDisruptionBudget to be true")
	}

	if !ms.GetHostNetwork() {
		t.Error("Expected custom hostNetwork to be true")
	}

	if ms.GetPriorityClassName() != "custom-priority" {
		t.Errorf("Expected custom priorityClassName, got %s", ms.GetPriorityClassName())
	}
}

func TestBuildResources(t *testing.T) {
	// Test default resources
	ms := &corev1alpha1.MetricsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-metrics-server",
		},
	}

	resources := buildResources(ms)

	// Check default requests
	cpuRequest := resources.Requests[corev1.ResourceCPU]
	if cpuRequest.String() != "10m" {
		t.Errorf("Expected default CPU request 10m, got %s", cpuRequest.String())
	}

	memoryRequest := resources.Requests[corev1.ResourceMemory]
	if memoryRequest.String() != "32Mi" {
		t.Errorf("Expected default memory request 32Mi, got %s", memoryRequest.String())
	}

	// Check default limits
	cpuLimit := resources.Limits[corev1.ResourceCPU]
	if cpuLimit.String() != "100m" {
		t.Errorf("Expected default CPU limit 100m, got %s", cpuLimit.String())
	}

	memoryLimit := resources.Limits[corev1.ResourceMemory]
	if memoryLimit.String() != "128Mi" {
		t.Errorf("Expected default memory limit 128Mi, got %s", memoryLimit.String())
	}

	// Test custom resources
	customResources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("400Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("800Mi"),
		},
	}

	ms.Spec.Resources = customResources
	resources = buildResources(ms)

	// Check custom requests
	cpuRequest = resources.Requests[corev1.ResourceCPU]
	if cpuRequest.String() != "200m" {
		t.Errorf("Expected custom CPU request 200m, got %s", cpuRequest.String())
	}

	memoryRequest = resources.Requests[corev1.ResourceMemory]
	if memoryRequest.String() != "400Mi" {
		t.Errorf("Expected custom memory request 400Mi, got %s", memoryRequest.String())
	}

	// Check custom limits
	cpuLimit = resources.Limits[corev1.ResourceCPU]
	if cpuLimit.String() != "500m" {
		t.Errorf("Expected custom CPU limit 500m, got %s", cpuLimit.String())
	}

	memoryLimit = resources.Limits[corev1.ResourceMemory]
	if memoryLimit.String() != "800Mi" {
		t.Errorf("Expected custom memory limit 800Mi, got %s", memoryLimit.String())
	}
}
