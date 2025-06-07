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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var metricsserverlog = logf.Log.WithName("metricsserver-webhook")

// webhookClient is a global client for webhook operations
var webhookClient client.Client

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *MetricsServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	// Store the client for webhook use
	webhookClient = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-observability-vexxhost-dev-v1alpha1-metricsserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.vexxhost.dev,resources=metricsservers,verbs=create,versions=v1alpha1,name=vmetricsserver.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MetricsServer) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	metricsserverlog.Info("validate create", "name", r.Name)

	// Check for existing MetricsServer instances
	msList := &MetricsServerList{}
	if err := webhookClient.List(ctx, msList); err != nil {
		return nil, fmt.Errorf("failed to list existing MetricsServer instances: %w", err)
	}

	// Count existing instances (excluding deleted instances)
	existingInstances := make([]string, 0, len(msList.Items))
	for _, existingMS := range msList.Items {
		// Skip deleted instances
		if existingMS.GetDeletionTimestamp() != nil {
			continue
		}
		existingInstances = append(existingInstances, existingMS.Name)
	}

	if len(existingInstances) > 0 {
		return admission.Warnings{
			"Only one MetricsServer instance is allowed per cluster due to APIService constraints.",
		}, fmt.Errorf("singleton constraint violation: MetricsServer instance '%s' already exists. Only one MetricsServer is allowed per cluster due to the cluster-wide v1beta1.metrics.k8s.io APIService registration. Please delete the existing instance before creating a new one", existingInstances[0])
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MetricsServer) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	metricsserverlog.Info("validate update", "name", r.Name)

	// Updates to existing instances are always allowed
	// The singleton constraint only applies to creation
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MetricsServer) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	metricsserverlog.Info("validate delete", "name", r.Name)

	// Deletions are always allowed
	return nil, nil
}
