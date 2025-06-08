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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive
	. "github.com/onsi/gomega"    //nolint:revive

	"github.com/vexxhost/metrics-server-operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "metrics-server-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "metrics-server-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "metrics-server-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "metrics-server-operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		// Retry deployment in case cert-manager webhook isn't fully ready
		Eventually(func(g Gomega) {
			cmd := exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "Failed to deploy controller-manager")
		}, 60*time.Second, 5*time.Second).Should(Succeed())

		By("waiting for webhook certificate secret to be created")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "secret",
				"webhook-server-cert", "-n", namespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "Webhook certificate secret not found")
		}, 120*time.Second, 5*time.Second).Should(Succeed())
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("cleaning up metrics clusterrolebinding")
		cmd = exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			// Delete if exists first to avoid conflicts
			cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=metrics-server-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		It("should successfully deploy and reconcile a MetricsServer instance", func() {
			sampleFile := "config/samples/core_v1alpha1_metricsserver.yaml"

			By("waiting for webhook certificate secret to be created")
			verifyWebhookCertificate := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secret",
					"webhook-server-cert", "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyWebhookCertificate, 60*time.Second).Should(Succeed())

			By("waiting for controller pod to be fully ready")
			verifyControllerReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods",
					"-l", "control-plane=controller-manager",
					"-n", namespace,
					"-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyControllerReady, 120*time.Second).Should(Succeed())

			By("waiting for webhook service to be ready")
			verifyWebhookReady := func(g Gomega) {
				// Check if endpoints exist
				cmd := exec.Command("kubectl", "get", "endpoints",
					"metrics-server-operator-webhook-service", "-n", namespace,
					"-o", "jsonpath={.subsets[*].addresses[*].ip}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Webhook endpoint has no addresses")
			}
			Eventually(verifyWebhookReady, 60*time.Second).Should(Succeed())

			By("waiting for webhook server to accept connections")
			// Add additional wait time for webhook server initialization
			// Based on test observations, the webhook needs significant time to be ready
			time.Sleep(30 * time.Second)

			// Test webhook connectivity with a dry-run - allow for initial connection failures
			verifyWebhookConnectivity := func(g Gomega) {
				cmd := exec.Command("kubectl", "apply", "--dry-run=server", "-f", sampleFile)
				output, err := utils.Run(cmd)
				// The dry-run might fail initially if webhook is not ready
				if err != nil {
					// Check if it's a connection refused error
					errStr := err.Error()
					if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "dial tcp") {
						// This is expected during startup, fail the assertion to retry
						g.Expect(err).NotTo(HaveOccurred(), "Webhook not ready yet: %s", errStr)
						return
					}
					// For other errors, check if it's a validation error (which means webhook is working)
					if strings.Contains(errStr, "validation") || strings.Contains(errStr, "invalid") {
						// Webhook is working but rejecting the request - this is fine
						return
					}
				}
				// If no error or validation error, webhook is working
				g.Expect(output).To(ContainSubstring("metricsserver.observability.vexxhost.dev/metrics-server-sample"))
			}
			// Increase timeout and retry interval for webhook readiness
			Eventually(verifyWebhookConnectivity, 300*time.Second, 10*time.Second).Should(Succeed())

			By("creating a sample MetricsServer")
			cmd := exec.Command("kubectl", "apply", "-f", sampleFile)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply MetricsServer sample")

			By("validating that the MetricsServer status shows as ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "metricsserver", "metrics-server-sample",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "MetricsServer should be ready")
			}, 5*time.Minute).Should(Succeed())

			By("verifying that the metrics-server deployment was created")
			cmd = exec.Command("kubectl", "get", "deployment", "metrics-server", "-n", "kube-system")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "metrics-server deployment should exist")

			By("verifying that the metrics-server service was created")
			cmd = exec.Command("kubectl", "get", "service", "metrics-server", "-n", "kube-system")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "metrics-server service should exist")

			By("verifying that the APIService was created and is available")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apiservice", "v1beta1.metrics.k8s.io",
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "APIService should be available")
			}, 3*time.Minute).Should(Succeed())

			By("testing that node metrics are accessible")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "top", "nodes")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("CPU"), "Node metrics should be available")
			}, 2*time.Minute).Should(Succeed())

			By("verifying reconciliation metrics")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"}`,
					"metrics-server"),
			))

			By("cleaning up the MetricsServer instance")
			cmd = exec.Command("kubectl", "delete", "-f", sampleFile)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete MetricsServer sample")

			By("verifying that resources are cleaned up")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "metrics-server", "-n", "kube-system")
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "metrics-server deployment should be deleted")
			}, 2*time.Minute).Should(Succeed())
		})

		It("should handle MetricsServer updates correctly", func() {
			sampleFile := "config/samples/core_v1alpha1_metricsserver.yaml"

			By("creating a sample MetricsServer")
			cmd := exec.Command("kubectl", "apply", "-f", sampleFile)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply MetricsServer sample")

			By("waiting for initial deployment to be ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "metrics-server", "-n", "kube-system",
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Initial deployment should have 1 ready replica")
			}, 3*time.Minute).Should(Succeed())

			By("updating the MetricsServer to use 2 replicas")
			cmd = exec.Command("kubectl", "patch", "metricsserver", "metrics-server-sample",
				"--type=merge", "-p", `{"spec":{"replicas":2}}`)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch MetricsServer")

			By("verifying that the deployment was updated to 2 replicas")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "metrics-server", "-n", "kube-system",
					"-o", "jsonpath={.spec.replicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"), "Deployment should be updated to 2 replicas")
			}, 2*time.Minute).Should(Succeed())

			By("waiting for both replicas to be ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "metrics-server", "-n", "kube-system",
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"), "Both replicas should be ready")
			}, 3*time.Minute).Should(Succeed())

			By("cleaning up the MetricsServer instance")
			cmd = exec.Command("kubectl", "delete", "-f", sampleFile)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete MetricsServer sample")
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
