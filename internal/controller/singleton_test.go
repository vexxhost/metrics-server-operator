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

package controller

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "github.com/vexxhost/metrics-server-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateSingletonConstraint(t *testing.T) {
	// Setup the test scheme
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = corev1alpha1.AddToScheme(s)

	tests := []struct {
		name            string
		existingServers []*corev1alpha1.MetricsServer
		currentServer   *corev1alpha1.MetricsServer
		expectError     bool
		errorContains   string
	}{
		{
			name:            "no existing servers - should pass",
			existingServers: []*corev1alpha1.MetricsServer{},
			currentServer: &corev1alpha1.MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "first"},
			},
			expectError: false,
		},
		{
			name: "same server only - should pass",
			existingServers: []*corev1alpha1.MetricsServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "same"}},
			},
			currentServer: &corev1alpha1.MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "same"},
			},
			expectError: false,
		},
		{
			name: "different server exists - should fail",
			existingServers: []*corev1alpha1.MetricsServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "existing"}},
			},
			currentServer: &corev1alpha1.MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "new"},
			},
			expectError:   true,
			errorContains: "only one MetricsServer instance is allowed per cluster",
		},
		{
			name: "multiple different servers exist - should fail",
			existingServers: []*corev1alpha1.MetricsServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "existing1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "existing2"}},
			},
			currentServer: &corev1alpha1.MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "new"},
			},
			expectError:   true,
			errorContains: "existing1",
		},
		{
			name: "deleted server exists - should pass",
			existingServers: []*corev1alpha1.MetricsServer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting",
						DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
						Finalizers:        []string{corev1alpha1.FinalizerName},
					},
				},
			},
			currentServer: &corev1alpha1.MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "new"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with existing servers
			objs := make([]runtime.Object, len(tt.existingServers))
			for i, server := range tt.existingServers {
				objs[i] = server
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithRuntimeObjects(objs...).
				Build()

			// Create reconciler
			r := &MetricsServerReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			// Test validation
			err := r.validateSingletonConstraint(context.TODO(), tt.currentServer)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}
