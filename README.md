# Metrics Server Operator

A Kubernetes operator for deploying and managing [metrics-server](https://github.com/kubernetes-sigs/metrics-server) using a declarative CRD-based approach. This operator replaces the need for manual Helm deployments or Ansible playbooks by providing a native Kubernetes resource to manage metrics-server installations.

[![CI](https://github.com/vexxhost/metrics-server-operator/actions/workflows/ci.yml/badge.svg)](https://github.com/vexxhost/metrics-server-operator/actions/workflows/ci.yml)
[![Release](https://github.com/vexxhost/metrics-server-operator/actions/workflows/release.yml/badge.svg)](https://github.com/vexxhost/metrics-server-operator/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vexxhost/metrics-server-operator)](https://goreportcard.com/report/github.com/vexxhost/metrics-server-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Description

The Metrics Server Operator provides a Kubernetes-native way to deploy and manage metrics-server instances. It handles the complexity of deploying all required resources including:

- **Deployment** - The metrics-server pods with proper security context and resource limits
- **Service** - ClusterIP service for metrics-server communication  
- **ServiceAccount** - Dedicated service account with minimal required permissions
- **RBAC** - ClusterRole and ClusterRoleBinding with least-privilege access
- **APIService** - Registration of the `metrics.k8s.io/v1beta1` API
- **PodDisruptionBudget** - Optional PDB for high availability deployments

### Key Features

- 🔒 **Security-first** - Follows Pod Security Standards with locked-down RBAC
- 🏗️ **Production-ready** - Comprehensive health checks, monitoring, and observability
- 🧪 **Well-tested** - Extensive unit tests and e2e test coverage
- 📊 **Configurable** - Flexible configuration options for different environments
- 🔄 **GitOps-friendly** - Declarative configuration that works with GitOps workflows
- 🏢 **Enterprise-ready** - Multi-architecture support (amd64/arm64) and air-gapped environments

## Quick Start

### Prerequisites

- Kubernetes cluster v1.24+ (for Pod Security Standards support)
- kubectl configured to access your cluster
- Cluster admin permissions for initial setup

### Installation

Install the operator using the latest release:

```bash
kubectl apply -f https://github.com/vexxhost/metrics-server-operator/releases/latest/download/install.yaml
```

### Deploy metrics-server

Create a `MetricsServer` resource:

```yaml
apiVersion: observability.vexxhost.dev/v1alpha1
kind: MetricsServer
metadata:
  name: default-metrics-server
spec:
  # Use defaults - deploys to kube-system namespace
  image: "registry.k8s.io/metrics-server/metrics-server:v0.7.2"
  replicas: 1
  kubeletInsecureTLS: true  # Required for most clusters
```

Apply the configuration:

```bash
kubectl apply -f metricsserver.yaml
```

Verify the deployment:

```bash
# Check the MetricsServer status
kubectl get metricsserver default-metrics-server

# Verify metrics-server is working
kubectl top nodes
kubectl top pods -A
```

## Configuration Examples

### Minimal Configuration

```yaml
apiVersion: observability.vexxhost.dev/v1alpha1
kind: MetricsServer
metadata:
  name: simple
spec: {}  # Uses all defaults
```

### Production Configuration

```yaml
apiVersion: observability.vexxhost.dev/v1alpha1
kind: MetricsServer
metadata:
  name: production
spec:
  image: "registry.k8s.io/metrics-server/metrics-server:v0.7.2"
  replicas: 2
  kubeletInsecureTLS: false  # Use secure TLS if supported
  
  resources:
    requests:
      cpu: 10m      # Very conservative for production
      memory: 32Mi  # Minimal memory footprint
    limits:
      cpu: 100m     # Prevents resource spikes
      memory: 128Mi # Conservative memory limit
  
  args:
    - --metric-resolution=30s
    - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
    - --v=2
  
  # Pod placement constraints
  nodeSelector:
    kubernetes.io/os: linux
  
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule
  
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchLabels:
                app.kubernetes.io/component: metrics-server
            topologyKey: kubernetes.io/hostname
  
  # High availability features
  podDisruptionBudget: true
  
  # Monitoring integration
  serviceMonitor: true  # Creates ServiceMonitor for Prometheus
  
  # Custom labels and annotations
  serviceLabels:
    monitoring: "enabled"
  
  serviceAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "10250"
    prometheus.io/scheme: "https"
  
  podLabels:
    environment: "production"
  
  podAnnotations:
    cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
```

## ⚠️ Important: Singleton Constraint

**Only one MetricsServer instance is allowed per cluster.** This constraint exists because metrics-server registers the cluster-wide `v1beta1.metrics.k8s.io` APIService, which can only have one backend service.

### Enforcement

The operator enforces this constraint at two levels:

**1. API Level (Validating Webhook)**:
- 🚫 **Immediate rejection**: Additional MetricsServer creation requests are rejected with an error
- ⚡ **Fast feedback**: Users get immediate feedback without creating invalid resources
- 🛡️ **Fail-safe**: Webhook validates before any resource is stored in etcd

**2. Controller Level (Reconciliation)**:
- ✅ **First instance**: Deploys successfully and remains healthy
- ❌ **Additional instances**: Marked as `Degraded` with reason `SingletonViolation` (backup enforcement)
- 🔄 **After deletion**: New instances can be created once the existing one is removed

### Example Violation

```bash
# First instance - works fine
kubectl apply -f - <<EOF
apiVersion: observability.vexxhost.dev/v1alpha1
kind: MetricsServer
metadata:
  name: primary
spec: {}
EOF

# Second instance - will be rejected immediately by webhook
kubectl apply -f - <<EOF
apiVersion: observability.vexxhost.dev/v1alpha1
kind: MetricsServer
metadata:
  name: secondary  # ❌ This will fail at API level
spec: {}
EOF

# Output will show immediate rejection:
# error validating data: ValidationError(MetricsServer): singleton constraint violation: 
# MetricsServer instance 'primary' already exists. Only one MetricsServer is allowed per cluster...
```

### Migration Strategy

To replace an existing MetricsServer:

1. **Delete the old instance**: `kubectl delete metricsserver old-name`
2. **Wait for deletion to complete**: `kubectl wait --for=delete metricsserver old-name --timeout=60s`
3. **Create the new instance**: The webhook will now allow the new instance to be created

## Configuration Reference

### MetricsServerSpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | `registry.k8s.io/metrics-server/metrics-server:v0.7.2` | Container image for metrics-server |
| `replicas` | int32 | `1` | Number of replicas for the deployment |
| `kubeletInsecureTLS` | bool | `true` | Skip TLS verification when connecting to kubelets |
| `priorityClassName` | string | `system-cluster-critical` | Priority class for metrics-server pods |
| `hostNetwork` | bool | `false` | Enable host networking for pods |
| `resources` | [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core) | See defaults | Resource limits and requests |
| `args` | []string | See defaults | Additional command-line arguments |
| `nodeSelector` | map[string]string | `{}` | Node selection constraints |
| `tolerations` | [][Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) | `[]` | Pod tolerations |
| `affinity` | [Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core) | `nil` | Pod affinity constraints |
| `podDisruptionBudget` | bool | `false` | Create a PodDisruptionBudget |
| `serviceMonitor` | bool | `false` | Create a ServiceMonitor for Prometheus |
| `serviceLabels` | map[string]string | `{}` | Additional labels for the service |
| `serviceAnnotations` | map[string]string | `{}` | Additional annotations for the service |
| `podLabels` | map[string]string | `{}` | Additional labels for pods |
| `podAnnotations` | map[string]string | `{}` | Additional annotations for pods |
| `serviceAccountAnnotations` | map[string]string | `{}` | Additional annotations for the service account |

### Default Arguments

The operator sets these default arguments for metrics-server:

- `--cert-dir=/tmp`
- `--secure-port=10250`
- `--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname`
- `--kubelet-use-node-status-port`
- `--metric-resolution=15s`
- `--kubelet-insecure-tls` (if `kubeletInsecureTLS: true`)

Additional arguments can be provided via the `args` field.

### Default Resource Limits

The operator uses conservative, production-ready resource defaults to prevent runaway resource consumption:

**Metrics-Server Defaults:**
- **CPU Request:** `10m` (minimal baseline)
- **Memory Request:** `32Mi` (small memory footprint)
- **CPU Limit:** `100m` (prevents CPU spikes)
- **Memory Limit:** `128Mi` (conservative memory ceiling)

**Operator Defaults:**
- **CPU Request:** `10m` (minimal baseline)
- **Memory Request:** `32Mi` (small memory footprint)  
- **CPU Limit:** `100m` (prevents CPU spikes)
- **Memory Limit:** `128Mi` (conservative memory ceiling)
- **Ephemeral Storage:** `128Mi` request, `256Mi` limit

These defaults are suitable for most production clusters and can be overridden via the `resources` field if higher limits are needed for large clusters.

## Development

### Prerequisites

- Go 1.23+
- Docker 17.03+
- kubectl v1.11.3+
- Access to a Kubernetes v1.24+ cluster
- [operator-sdk](https://sdk.operatorframework.io/docs/installation/) v1.34+

### Local Development

```bash
# Clone the repository
git clone https://github.com/vexxhost/metrics-server-operator.git
cd metrics-server-operator

# Install dependencies
go mod download

# Generate manifests and code
make manifests generate

# Run tests
make test

# Install CRDs
make install

# Run the operator locally (connects to your current kubectl context)
make run
```

### Building and Deployment

```bash
# Build and push the container image
make docker-build docker-push IMG=your-registry/metrics-server-operator:tag

# Deploy to cluster
make deploy IMG=your-registry/metrics-server-operator:tag

# Create a sample MetricsServer
kubectl apply -f config/samples/core_v1alpha1_metricsserver.yaml
```

### Testing with Kind

Create a local Kubernetes cluster for testing:

```bash
# Install Kind
go install sigs.k8s.io/kind@latest

# Create cluster
kind create cluster --name metrics-server-test

# Load your image (if testing local builds)
kind load docker-image your-registry/metrics-server-operator:tag --name metrics-server-test

# Deploy and test
make install
make deploy IMG=your-registry/metrics-server-operator:tag
kubectl apply -f config/samples/core_v1alpha1_metricsserver.yaml

# Verify metrics are working
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=metrics-server -n kube-system --timeout=300s
kubectl top nodes

# Cleanup
kind delete cluster --name metrics-server-test
```

### Running Tests

```bash
# Unit tests
make test

# E2E tests (requires a running cluster)
make test-e2e

# Test with coverage
make test COVERAGE=true
```

## Troubleshooting

### Common Issues

**1. APIService not available**

```bash
# Check APIService status
kubectl get apiservice v1beta1.metrics.k8s.io -o yaml

# Check metrics-server pod logs
kubectl logs -l app.kubernetes.io/component=metrics-server -n kube-system
```

**2. Kubelet connection issues**

If you see TLS errors, ensure `kubeletInsecureTLS: true` is set:

```yaml
spec:
  kubeletInsecureTLS: true
```

**3. Resource not found errors**

Ensure the CRDs are installed:

```bash
kubectl get crd metricsservers.observability.vexxhost.dev
```

**4. Singleton constraint violation**

With the validating webhook enabled, creation of additional MetricsServer instances is blocked at the API level:

```bash
# If you see this error during kubectl apply:
# error validating data: ValidationError(MetricsServer): singleton constraint violation

# List all MetricsServer instances
kubectl get metricsserver

# Delete the existing instance to create a new one
kubectl delete metricsserver existing-instance-name
kubectl wait --for=delete metricsserver existing-instance-name --timeout=60s

# Now you can create the new instance
kubectl apply -f your-new-metricsserver.yaml
```

If your MetricsServer shows `Degraded=True` with `Reason=SingletonViolation` (backup enforcement):

```bash
# Check which instance is conflicting
kubectl get metricsserver YOUR-INSTANCE -o yaml | grep -A 5 "conditions:"

# Delete the unwanted instance
kubectl delete metricsserver unwanted-instance-name
```

Only one MetricsServer is allowed per cluster due to APIService constraints.

**5. Webhook issues**

If the validating webhook is not working (e.g., you can create multiple MetricsServer instances):

```bash
# Check if the webhook is registered
kubectl get validatingwebhookconfigurations

# Check webhook service and endpoints
kubectl get service metrics-server-operator-webhook-service -n metrics-server-operator-system
kubectl get endpoints metrics-server-operator-webhook-service -n metrics-server-operator-system

# Check operator pod is ready and webhook server is running
kubectl get pods -n metrics-server-operator-system
kubectl logs deployment/metrics-server-operator-controller-manager -n metrics-server-operator-system | grep webhook
```

**6. RBAC permission errors**

Check that the operator has proper cluster permissions:

```bash
kubectl auth can-i '*' '*' --as=system:serviceaccount:metrics-server-operator-system:metrics-server-operator-controller-manager
```

### Debug Commands

```bash
# Check operator logs
kubectl logs -f deployment/metrics-server-operator-controller-manager -n metrics-server-operator-system

# Check MetricsServer status
kubectl get metricsserver -o yaml

# Check all created resources
kubectl get all -n kube-system -l app.kubernetes.io/managed-by=metrics-server-operator

# Test metrics endpoint directly
kubectl get --raw "/apis/metrics.k8s.io/v1beta1/nodes"
```

## Migration from Helm/Ansible

If you're currently using Helm or Ansible to deploy metrics-server, here's how to migrate:

### From Helm

1. **Uninstall existing Helm deployment**:
   ```bash
   helm uninstall metrics-server -n kube-system
   ```

2. **Install the operator**:
   ```bash
   kubectl apply -f https://github.com/vexxhost/metrics-server-operator/releases/latest/download/install.yaml
   ```

3. **Create MetricsServer resource** with equivalent configuration to your Helm values.

### From Ansible (atmosphere.common)

This operator is designed as a direct replacement for the `metrics_server` role from `atmosphere.common`. The default configuration provides the same behavior:

```yaml
apiVersion: observability.vexxhost.dev/v1alpha1
kind: MetricsServer
metadata:
  name: default
spec:
  image: "registry.k8s.io/metrics-server/metrics-server:v0.7.2"
  kubeletInsecureTLS: true
```

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

### Code of Conduct

This project adheres to the Contributor Covenant [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

For security issues, please see our [Security Policy](SECURITY.md).

## License

Copyright 2025 Vexxhost, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Acknowledgments

- [Kubernetes SIG Instrumentation](https://github.com/kubernetes-sigs/metrics-server) for metrics-server
- [Operator SDK](https://sdk.operatorframework.io/) for the operator framework
- [Kubebuilder](https://kubebuilder.io/) for the controller-runtime framework