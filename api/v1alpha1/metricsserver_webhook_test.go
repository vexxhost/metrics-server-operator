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
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetricsServerWebhook_ValidateCreate(t *testing.T) {
	// Setup the test scheme
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = AddToScheme(s)

	tests := []struct {
		name            string
		existingServers []*MetricsServer
		newServer       *MetricsServer
		expectError     bool
		errorContains   string
		expectWarnings  bool
	}{
		{
			name:            "no existing servers - should pass",
			existingServers: []*MetricsServer{},
			newServer: &MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "first"},
			},
			expectError:    false,
			expectWarnings: false,
		},
		{
			name: "existing server exists - should fail",
			existingServers: []*MetricsServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "existing"}},
			},
			newServer: &MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "new"},
			},
			expectError:     true,
			errorContains:   "singleton constraint violation",
			expectWarnings:  true,
		},
		{
			name: "deleted server exists - should pass",
			existingServers: []*MetricsServer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting",
						DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
						Finalizers:        []string{FinalizerName},
					},
				},
			},
			newServer: &MetricsServer{
				ObjectMeta: metav1.ObjectMeta{Name: "new"},
			},
			expectError:    false,
			expectWarnings: false,
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

			// Set the global webhook client
			webhookClient = fakeClient

			// Test validation
			warnings, err := tt.newServer.ValidateCreate(context.TODO(), tt.newServer)

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

			if tt.expectWarnings {
				if len(warnings) == 0 {
					t.Error("Expected warnings but got none")
				}
			} else {
				if len(warnings) > 0 {
					t.Errorf("Expected no warnings but got: %v", warnings)
				}
			}
		})
	}
}

func TestMetricsServerWebhook_ValidateUpdate(t *testing.T) {
	ms := &MetricsServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	// Updates should always be allowed
	warnings, err := ms.ValidateUpdate(context.TODO(), ms, ms)

	if err != nil {
		t.Errorf("Expected no error for update, got: %s", err.Error())
	}

	if len(warnings) > 0 {
		t.Errorf("Expected no warnings for update, got: %v", warnings)
	}
}

func TestMetricsServerWebhook_ValidateDelete(t *testing.T) {
	ms := &MetricsServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	// Deletes should always be allowed
	warnings, err := ms.ValidateDelete(context.TODO(), ms)

	if err != nil {
		t.Errorf("Expected no error for delete, got: %s", err.Error())
	}

	if len(warnings) > 0 {
		t.Errorf("Expected no warnings for delete, got: %v", warnings)
	}
}