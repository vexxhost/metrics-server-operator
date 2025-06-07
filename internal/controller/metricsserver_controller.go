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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/vexxhost/metrics-server-operator/api/v1alpha1"
	"github.com/vexxhost/metrics-server-operator/internal/builder"
)

// MetricsServerReconciler reconciles a MetricsServer object
type MetricsServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=observability.vexxhost.dev,resources=metricsservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.vexxhost.dev,resources=metricsservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observability.vexxhost.dev,resources=metricsservers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes/metrics,verbs=get
// +kubebuilder:rbac:groups="",resources=pods;nodes;nodes/stats;namespaces;configmaps,verbs=get;list
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=pods;nodes,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MetricsServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the MetricsServer instance
	ms := &corev1alpha1.MetricsServer{}
	err := r.Get(ctx, req.NamespacedName, ms)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("MetricsServer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MetricsServer")
		return ctrl.Result{}, err
	}

	// Check if the MetricsServer instance is marked to be deleted
	if ms.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(ms, corev1alpha1.FinalizerName) {
			// Run finalization logic for MetricsServer
			if err := r.finalizeMetricsServer(ctx, ms); err != nil {
				return ctrl.Result{}, err
			}

			// Remove finalizer and update the CR
			controllerutil.RemoveFinalizer(ms, corev1alpha1.FinalizerName)
			err := r.Update(ctx, ms)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(ms, corev1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(ms, corev1alpha1.FinalizerName)
		err = r.Update(ctx, ms)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check for singleton constraint (only one MetricsServer allowed per cluster)
	if err := r.validateSingletonConstraint(ctx, ms); err != nil {
		// Set degraded condition
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             corev1alpha1.ReasonSingletonViolation,
			Message:            err.Error(),
			ObservedGeneration: ms.Generation,
		})
		// Update status and requeue
		if updateErr := r.Status().Update(ctx, ms); updateErr != nil {
			log.Error(updateErr, "Failed to update MetricsServer status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, err
	}

	// Initialize status conditions if not present
	if ms.Status.Conditions == nil {
		ms.Status.Conditions = []metav1.Condition{}
	}

	// Update progressing condition
	meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.ConditionTypeProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             corev1alpha1.ReasonReconciling,
		Message:            "Reconciling MetricsServer resources",
		ObservedGeneration: ms.Generation,
	})

	// Update status
	ms.Status.ObservedGeneration = ms.Generation
	if err := r.Status().Update(ctx, ms); err != nil {
		log.Error(err, "Failed to update MetricsServer status")
		return ctrl.Result{}, err
	}

	// Reconcile all resources
	if err := r.reconcileResources(ctx, ms); err != nil {
		// Update degraded condition
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             corev1alpha1.ReasonFailed,
			Message:            fmt.Sprintf("Failed to reconcile resources: %v", err),
			ObservedGeneration: ms.Generation,
		})
		if statusErr := r.Status().Update(ctx, ms); statusErr != nil {
			log.Error(statusErr, "Failed to update MetricsServer status")
		}
		return ctrl.Result{}, err
	}

	// Check deployment status
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      corev1alpha1.DefaultDeploymentName,
		Namespace: corev1alpha1.DefaultNamespace,
	}, deployment); err != nil {
		return ctrl.Result{}, err
	}

	// Update status based on deployment
	ms.Status.ReadyReplicas = deployment.Status.ReadyReplicas
	ms.Status.AvailableReplicas = deployment.Status.AvailableReplicas

	// Update conditions based on deployment status
	if deployment.Status.ReadyReplicas == ms.GetReplicas() {
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             corev1alpha1.ReasonDeploymentReady,
			Message:            "MetricsServer deployment is ready",
			ObservedGeneration: ms.Generation,
		})
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             corev1alpha1.ReasonReady,
			Message:            "MetricsServer is available",
			ObservedGeneration: ms.Generation,
		})
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             corev1alpha1.ReasonReady,
			Message:            "MetricsServer resources reconciled successfully",
			ObservedGeneration: ms.Generation,
		})
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeDegraded,
			Status:             metav1.ConditionFalse,
			Reason:             corev1alpha1.ReasonReady,
			Message:            "MetricsServer is healthy",
			ObservedGeneration: ms.Generation,
		})
	} else {
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             corev1alpha1.ReasonDeploymentNotReady,
			Message:            fmt.Sprintf("Deployment has %d/%d replicas ready", deployment.Status.ReadyReplicas, ms.GetReplicas()),
			ObservedGeneration: ms.Generation,
		})
		meta.SetStatusCondition(&ms.Status.Conditions, metav1.Condition{
			Type:               corev1alpha1.ConditionTypeAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             corev1alpha1.ReasonDeploymentNotReady,
			Message:            "MetricsServer is not yet available",
			ObservedGeneration: ms.Generation,
		})
	}

	// Update status
	if err := r.Status().Update(ctx, ms); err != nil {
		log.Error(err, "Failed to update MetricsServer status")
		return ctrl.Result{}, err
	}

	// Requeue after 30 seconds to check status
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileResources reconciles all resources for MetricsServer
func (r *MetricsServerReconciler) reconcileResources(ctx context.Context, ms *corev1alpha1.MetricsServer) error {
	// Reconcile ServiceAccount
	sa := builder.BuildServiceAccount(ms)
	if err := r.reconcileResource(ctx, ms, sa); err != nil {
		return fmt.Errorf("failed to reconcile ServiceAccount: %w", err)
	}

	// Reconcile ClusterRole
	clusterRole := builder.BuildClusterRole(ms)
	if err := r.reconcileClusterScopedResource(ctx, ms, clusterRole); err != nil {
		return fmt.Errorf("failed to reconcile ClusterRole: %w", err)
	}

	// Reconcile ClusterRole for aggregated reader
	clusterRoleReader := builder.BuildClusterRoleAggregatedReader(ms)
	if err := r.reconcileClusterScopedResource(ctx, ms, clusterRoleReader); err != nil {
		return fmt.Errorf("failed to reconcile ClusterRole aggregated reader: %w", err)
	}

	// Reconcile ClusterRoleBinding
	clusterRoleBinding := builder.BuildClusterRoleBinding(ms)
	if err := r.reconcileClusterScopedResource(ctx, ms, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to reconcile ClusterRoleBinding: %w", err)
	}

	// Reconcile RoleBinding for auth reader
	roleBindingAuthReader := builder.BuildRoleBindingAuthReader(ms)
	if err := r.reconcileResource(ctx, ms, roleBindingAuthReader); err != nil {
		return fmt.Errorf("failed to reconcile RoleBinding auth reader: %w", err)
	}

	// Reconcile ClusterRoleBinding for auth delegator
	clusterRoleBindingAuthDelegator := builder.BuildClusterRoleBindingAuthDelegator(ms)
	if err := r.reconcileClusterScopedResource(ctx, ms, clusterRoleBindingAuthDelegator); err != nil {
		return fmt.Errorf("failed to reconcile ClusterRoleBinding auth delegator: %w", err)
	}

	// Reconcile Service
	service := builder.BuildService(ms)
	if err := r.reconcileResource(ctx, ms, service); err != nil {
		return fmt.Errorf("failed to reconcile Service: %w", err)
	}

	// Reconcile Deployment
	deployment := builder.BuildDeployment(ms)
	if err := r.reconcileResource(ctx, ms, deployment); err != nil {
		return fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	// Reconcile APIService
	apiService := builder.BuildAPIService(ms)
	if err := r.reconcileClusterScopedResource(ctx, ms, apiService); err != nil {
		return fmt.Errorf("failed to reconcile APIService: %w", err)
	}

	// Reconcile PodDisruptionBudget if enabled
	if ms.GetPodDisruptionBudget() {
		pdb := builder.BuildPodDisruptionBudget(ms)
		if err := r.reconcileResource(ctx, ms, pdb); err != nil {
			return fmt.Errorf("failed to reconcile PodDisruptionBudget: %w", err)
		}
	}

	return nil
}

// reconcileResource reconciles a namespaced resource
func (r *MetricsServerReconciler) reconcileResource(ctx context.Context, ms *corev1alpha1.MetricsServer, obj client.Object) error {
	log := logf.FromContext(ctx)

	// Set MetricsServer instance as the owner of the resource
	if err := controllerutil.SetControllerReference(ms, obj, r.Scheme); err != nil {
		return err
	}

	// Check if the resource already exists
	found := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
		err = r.Create(ctx, obj)
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	// Update resource if needed
	if needsUpdate(found, obj) {
		log.Info("Updating resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
		obj.SetResourceVersion(found.GetResourceVersion())
		err = r.Update(ctx, obj)
		if err != nil {
			return err
		}
	}

	return nil
}

// reconcileClusterScopedResource reconciles a cluster-scoped resource
func (r *MetricsServerReconciler) reconcileClusterScopedResource(ctx context.Context, ms *corev1alpha1.MetricsServer, obj client.Object) error {
	log := logf.FromContext(ctx)

	// Add labels to identify ownership
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[corev1alpha1.LabelInstance] = ms.Name
	obj.SetLabels(labels)

	// Check if the resource already exists
	found := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, types.NamespacedName{Name: obj.GetName()}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating cluster-scoped resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
		err = r.Create(ctx, obj)
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	// Check if this instance owns the resource
	foundLabels := found.GetLabels()
	if foundLabels[corev1alpha1.LabelInstance] != ms.Name {
		return fmt.Errorf("cluster-scoped resource %s is owned by another MetricsServer instance", obj.GetName())
	}

	// Update resource if needed
	if needsUpdate(found, obj) {
		log.Info("Updating cluster-scoped resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
		obj.SetResourceVersion(found.GetResourceVersion())
		err = r.Update(ctx, obj)
		if err != nil {
			return err
		}
	}

	return nil
}

// needsUpdate checks if a resource needs to be updated
func needsUpdate(current, desired client.Object) bool {
	switch current.(type) {
	case *corev1.ServiceAccount:
		currentSA := current.(*corev1.ServiceAccount)
		desiredSA := desired.(*corev1.ServiceAccount)
		return !equality.Semantic.DeepEqual(currentSA.Labels, desiredSA.Labels) ||
			!equality.Semantic.DeepEqual(currentSA.Annotations, desiredSA.Annotations)

	case *rbacv1.ClusterRole:
		currentCR := current.(*rbacv1.ClusterRole)
		desiredCR := desired.(*rbacv1.ClusterRole)
		return !equality.Semantic.DeepEqual(currentCR.Labels, desiredCR.Labels) ||
			!equality.Semantic.DeepEqual(currentCR.Rules, desiredCR.Rules)

	case *rbacv1.ClusterRoleBinding:
		currentCRB := current.(*rbacv1.ClusterRoleBinding)
		desiredCRB := desired.(*rbacv1.ClusterRoleBinding)
		return !equality.Semantic.DeepEqual(currentCRB.Labels, desiredCRB.Labels) ||
			!equality.Semantic.DeepEqual(currentCRB.RoleRef, desiredCRB.RoleRef) ||
			!equality.Semantic.DeepEqual(currentCRB.Subjects, desiredCRB.Subjects)

	case *rbacv1.RoleBinding:
		currentRB := current.(*rbacv1.RoleBinding)
		desiredRB := desired.(*rbacv1.RoleBinding)
		return !equality.Semantic.DeepEqual(currentRB.Labels, desiredRB.Labels) ||
			!equality.Semantic.DeepEqual(currentRB.RoleRef, desiredRB.RoleRef) ||
			!equality.Semantic.DeepEqual(currentRB.Subjects, desiredRB.Subjects)

	case *corev1.Service:
		currentSvc := current.(*corev1.Service)
		desiredSvc := desired.(*corev1.Service)
		// Don't update ClusterIP
		desiredSvc.Spec.ClusterIP = currentSvc.Spec.ClusterIP
		return !equality.Semantic.DeepEqual(currentSvc.Labels, desiredSvc.Labels) ||
			!equality.Semantic.DeepEqual(currentSvc.Annotations, desiredSvc.Annotations) ||
			!equality.Semantic.DeepEqual(currentSvc.Spec, desiredSvc.Spec)

	case *appsv1.Deployment:
		currentDep := current.(*appsv1.Deployment)
		desiredDep := desired.(*appsv1.Deployment)
		return !equality.Semantic.DeepEqual(currentDep.Labels, desiredDep.Labels) ||
			!equality.Semantic.DeepEqual(currentDep.Spec, desiredDep.Spec)

	case *apiregistrationv1.APIService:
		currentAPI := current.(*apiregistrationv1.APIService)
		desiredAPI := desired.(*apiregistrationv1.APIService)
		return !equality.Semantic.DeepEqual(currentAPI.Labels, desiredAPI.Labels) ||
			!equality.Semantic.DeepEqual(currentAPI.Spec, desiredAPI.Spec)

	case *policyv1.PodDisruptionBudget:
		currentPDB := current.(*policyv1.PodDisruptionBudget)
		desiredPDB := desired.(*policyv1.PodDisruptionBudget)
		return !equality.Semantic.DeepEqual(currentPDB.Labels, desiredPDB.Labels) ||
			!equality.Semantic.DeepEqual(currentPDB.Spec, desiredPDB.Spec)

	default:
		return false
	}
}

// finalizeMetricsServer performs cleanup when MetricsServer is deleted
func (r *MetricsServerReconciler) finalizeMetricsServer(ctx context.Context, ms *corev1alpha1.MetricsServer) error {
	log := logf.FromContext(ctx)
	log.Info("Running finalizer for MetricsServer", "Name", ms.Name)

	// Clean up cluster-scoped resources that have our instance label
	clusterResources := []client.Object{
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: corev1alpha1.DefaultClusterRoleName}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: corev1alpha1.DefaultClusterRoleAggregatedReaderName}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: corev1alpha1.DefaultClusterRoleBindingName}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s:system:auth-delegator", corev1alpha1.DefaultServiceAccountName)}},
		&apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{Name: corev1alpha1.DefaultAPIServiceName}},
	}

	for _, obj := range clusterResources {
		err := r.Get(ctx, types.NamespacedName{Name: obj.GetName()}, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}

		// Check if this instance owns the resource
		labels := obj.GetLabels()
		if labels != nil && labels[corev1alpha1.LabelInstance] == ms.Name {
			log.Info("Deleting cluster-scoped resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
			if err := r.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricsServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add APIService to scheme
	if err := apiregistrationv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.MetricsServer{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Watches(
			&rbacv1.ClusterRole{},
			handler.EnqueueRequestsFromMapFunc(r.findMetricsServerForClusterResource),
		).
		Watches(
			&rbacv1.ClusterRoleBinding{},
			handler.EnqueueRequestsFromMapFunc(r.findMetricsServerForClusterResource),
		).
		Watches(
			&apiregistrationv1.APIService{},
			handler.EnqueueRequestsFromMapFunc(r.findMetricsServerForClusterResource),
		).
		Named("metrics-server").
		Complete(r)
}

// findMetricsServerForClusterResource returns requests for MetricsServer that owns cluster-scoped resources
func (r *MetricsServerReconciler) findMetricsServerForClusterResource(ctx context.Context, obj client.Object) []ctrl.Request {
	labels := obj.GetLabels()
	if labels == nil {
		return nil
	}

	instanceName, ok := labels[corev1alpha1.LabelInstance]
	if !ok {
		return nil
	}

	// Find the MetricsServer with this name
	msList := &corev1alpha1.MetricsServerList{}
	if err := r.List(ctx, msList); err != nil {
		return nil
	}

	for _, ms := range msList.Items {
		if ms.Name == instanceName {
			return []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name: ms.Name,
					},
				},
			}
		}
	}

	return nil
}

// validateSingletonConstraint ensures only one MetricsServer instance exists per cluster
// since metrics-server registers the cluster-wide v1beta1.metrics.k8s.io APIService
func (r *MetricsServerReconciler) validateSingletonConstraint(ctx context.Context, current *corev1alpha1.MetricsServer) error {
	msList := &corev1alpha1.MetricsServerList{}
	if err := r.List(ctx, msList); err != nil {
		return fmt.Errorf("failed to list existing MetricsServer instances: %w", err)
	}

	// Count existing instances (excluding the current one)
	var existingInstances []string
	for _, ms := range msList.Items {
		// Skip the current instance and deleted instances
		if ms.Name == current.Name || ms.GetDeletionTimestamp() != nil {
			continue
		}
		existingInstances = append(existingInstances, ms.Name)
	}

	if len(existingInstances) > 0 {
		return fmt.Errorf("only one MetricsServer instance is allowed per cluster due to APIService constraints. Existing instances: %v. Please delete other instances before creating this one", existingInstances)
	}

	return nil
}
