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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/vexxhost/metrics-server-operator/api/v1alpha1"
)

var _ = Describe("MetricsServer Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-metrics-server"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name: resourceName,
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind MetricsServer")
			metricsserver := &corev1alpha1.MetricsServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, metricsserver)
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1alpha1.MetricsServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
					Spec: corev1alpha1.MetricsServerSpec{
						Replicas: ptr.To(int32(1)),
						Image:    "registry.k8s.io/metrics-server/metrics-server:v0.7.2",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance MetricsServer")
			resource := &corev1alpha1.MetricsServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			// Wait for deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Clean up cluster-scoped resources
			clusterResources := []types.NamespacedName{
				{Name: corev1alpha1.DefaultClusterRoleName},
				{Name: corev1alpha1.DefaultClusterRoleAggregatedReaderName},
				{Name: corev1alpha1.DefaultClusterRoleBindingName},
				{Name: corev1alpha1.DefaultServiceAccountName + ":system:auth-delegator"},
				{Name: corev1alpha1.DefaultAPIServiceName},
			}

			for _, name := range clusterResources {
				// Clean up ClusterRole
				cr := &rbacv1.ClusterRole{}
				if err := k8sClient.Get(ctx, name, cr); err == nil {
					_ = k8sClient.Delete(ctx, cr)
				}

				// Clean up ClusterRoleBinding
				crb := &rbacv1.ClusterRoleBinding{}
				if err := k8sClient.Get(ctx, name, crb); err == nil {
					_ = k8sClient.Delete(ctx, crb)
				}

				// Clean up APIService
				api := &apiregistrationv1.APIService{}
				if err := k8sClient.Get(ctx, name, api); err == nil {
					_ = k8sClient.Delete(ctx, api)
				}
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &MetricsServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the MetricsServer has finalizer")
			metricsserver := &corev1alpha1.MetricsServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, metricsserver)
				if err != nil {
					return false
				}
				for _, finalizer := range metricsserver.Finalizers {
					if finalizer == corev1alpha1.FinalizerName {
						return true
					}
				}
				return false
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			By("Checking that ServiceAccount was created")
			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      corev1alpha1.DefaultServiceAccountName,
					Namespace: corev1alpha1.DefaultNamespace,
				}, sa)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking that ClusterRole was created")
			cr := &rbacv1.ClusterRole{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: corev1alpha1.DefaultClusterRoleName,
				}, cr)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking that ClusterRoleBinding was created")
			crb := &rbacv1.ClusterRoleBinding{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: corev1alpha1.DefaultClusterRoleBindingName,
				}, crb)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking that Service was created")
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      corev1alpha1.DefaultServiceName,
					Namespace: corev1alpha1.DefaultNamespace,
				}, svc)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking that Deployment was created")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      corev1alpha1.DefaultDeploymentName,
					Namespace: corev1alpha1.DefaultNamespace,
				}, deployment)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking that APIService was created")
			apiService := &apiregistrationv1.APIService{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: corev1alpha1.DefaultAPIServiceName,
				}, apiService)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Verifying ServiceAccount has correct labels")
			Expect(sa.Labels[corev1alpha1.LabelManagedBy]).To(Equal(corev1alpha1.LabelManagedByValue))
			Expect(sa.Labels[corev1alpha1.LabelInstance]).To(Equal(resourceName))

			By("Verifying Deployment has correct spec")
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("registry.k8s.io/metrics-server/metrics-server:v0.7.2"))
			Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(corev1alpha1.DefaultServiceAccountName))

			By("Checking MetricsServer status conditions")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, metricsserver)
				if err != nil {
					return false
				}
				return meta.IsStatusConditionTrue(metricsserver.Status.Conditions, corev1alpha1.ConditionTypeProgressing)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("should handle MetricsServer deletion", func() {
			By("Reconciling to create resources")
			controllerReconciler := &MetricsServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for resources to be created")
			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      corev1alpha1.DefaultServiceAccountName,
					Namespace: corev1alpha1.DefaultNamespace,
				}, sa)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Deleting the MetricsServer")
			metricsserver := &corev1alpha1.MetricsServer{}
			err = k8sClient.Get(ctx, typeNamespacedName, metricsserver)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Delete(ctx, metricsserver)).To(Succeed())

			By("Reconciling the deletion")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the MetricsServer is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, metricsserver)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("should update resources when spec changes", func() {
			By("Initial reconciliation")
			controllerReconciler := &MetricsServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for initial deployment")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      corev1alpha1.DefaultDeploymentName,
					Namespace: corev1alpha1.DefaultNamespace,
				}, deployment)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Updating MetricsServer spec")
			metricsserver := &corev1alpha1.MetricsServer{}
			err = k8sClient.Get(ctx, typeNamespacedName, metricsserver)
			Expect(err).NotTo(HaveOccurred())

			metricsserver.Spec.Replicas = ptr.To(int32(2))
			metricsserver.Spec.Image = "registry.k8s.io/metrics-server/metrics-server:v0.7.1"
			Expect(k8sClient.Update(ctx, metricsserver)).To(Succeed())

			By("Reconciling after update")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployment was updated")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      corev1alpha1.DefaultDeploymentName,
					Namespace: corev1alpha1.DefaultNamespace,
				}, deployment)
				if err != nil {
					return false
				}
				return *deployment.Spec.Replicas == 2 &&
					deployment.Spec.Template.Spec.Containers[0].Image == "registry.k8s.io/metrics-server/metrics-server:v0.7.1"
			}, time.Second*15, time.Millisecond*500).Should(BeTrue())
		})

		It("should prevent multiple MetricsServer instances (singleton constraint)", func() {
			By("creating the first MetricsServer instance")
			firstMS := &corev1alpha1.MetricsServer{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.vexxhost.com/v1alpha1",
					Kind:       "MetricsServer",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "first-metrics-server",
				},
				Spec: corev1alpha1.MetricsServerSpec{
					Image:    "registry.k8s.io/metrics-server/metrics-server:v0.7.2",
					Replicas: ptr.To(int32(1)),
				},
			}

			Expect(k8sClient.Create(ctx, firstMS)).Should(Succeed())

			// Create reconciler for this test
			controllerReconciler := &MetricsServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Trigger reconciliation for first instance
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "first-metrics-server"},
			})
			Expect(err).NotTo(HaveOccurred())

			By("creating a second MetricsServer instance")
			secondMS := &corev1alpha1.MetricsServer{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.vexxhost.com/v1alpha1",
					Kind:       "MetricsServer",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "second-metrics-server",
				},
				Spec: corev1alpha1.MetricsServerSpec{
					Image:    "registry.k8s.io/metrics-server/metrics-server:v0.7.2",
					Replicas: ptr.To(int32(1)),
				},
			}

			Expect(k8sClient.Create(ctx, secondMS)).Should(Succeed())

			By("verifying the second instance is rejected due to singleton constraint")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "second-metrics-server"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only one MetricsServer instance is allowed per cluster"))
			Expect(err.Error()).To(ContainSubstring("first-metrics-server"))

			By("checking that the second instance has a degraded condition")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "second-metrics-server"}, secondMS)
				if err != nil {
					return false
				}
				for _, condition := range secondMS.Status.Conditions {
					if condition.Type == corev1alpha1.ConditionTypeDegraded &&
						condition.Status == metav1.ConditionTrue &&
						condition.Reason == corev1alpha1.ReasonSingletonViolation {
						return true
					}
				}
				return false
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			By("verifying that after deleting the first instance, the second can reconcile")
			Expect(k8sClient.Delete(ctx, firstMS)).Should(Succeed())

			// Wait for deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "first-metrics-server"}, &corev1alpha1.MetricsServer{})
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Now the second instance should reconcile successfully
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "second-metrics-server"},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
