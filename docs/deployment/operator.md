---
title: "NFD Operator"
layout: default
sort: 4
---

# Deployment with NFD Operator
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

The [Node Feature Discovery Operator][nfd-operator] automates installation,
configuration and updates of NFD using a specific NodeFeatureDiscovery custom
resource. This also provides good support for managing NFD as a dependency of
other operators.

## Deployment

Deployment using the
[Node Feature Discovery Operator][nfd-operator]
is recommended to be done via
[operatorhub.io](https://operatorhub.io/operator/nfd-operator).

> **NOTE:** The latest NFD Operator release on operatorhub.io (v0.6.0, 2023)
> does not work with current NFD versions. It starts nfd-worker with the
> `-server` flag (removed in NFD v0.17), probes nfd-master with
> `grpc_health_probe` (not shipped since NFD v0.15), and does not install the
> `nfd.k8s-sigs.io` NodeFeature CRDs or RBAC that current NFD requires. Its own
> deployment also references the sidecar image
> `gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0`, which is no longer available, so
> the operator pod never becomes ready. Until a new operator release is
> published, deploy NFD with [Helm](helm.md) or [Kustomize](kustomize.md).
> With the v0.6.0 operator, set `spec.operand.image` to an NFD release it
> supports, e.g. `registry.k8s.io/nfd/node-feature-discovery:v0.12.1-minimal`.

1. You need to have
   [OLM][OLM]
   installed. If you don't, take a look at the
   [latest release](https://github.com/operator-framework/operator-lifecycle-manager/releases/latest)
   for detailed instructions.
1. Install the operator:

   ```bash
   kubectl create -f https://operatorhub.io/install/nfd-operator.yaml
   ```

1. Create `NodeFeatureDiscovery` object (in `nfd` namespace here):

   ```bash
   cat << EOF | kubectl apply -f -
   apiVersion: v1
   kind: Namespace
   metadata:
     name: nfd
   ---
   apiVersion: nfd.kubernetes.io/v1
   kind: NodeFeatureDiscovery
   metadata:
     name: my-nfd-deployment
     namespace: nfd
   spec:
     operand:
       image: {{ site.container_image }}
       imagePullPolicy: IfNotPresent
   EOF
   ```

## Uninstallation

If you followed the deployment instructions above you can uninstall NFD with:

```bash
kubectl -n nfd delete NodeFeatureDiscovery my-nfd-deployment
```

Optionally, you can also remove the namespace:

```bash
kubectl delete ns nfd
```

See the [node-feature-discovery-operator][nfd-operator] and [OLM][OLM] project
documentation for instructions for uninstalling the operator and operator
lifecycle manager, respectively.

<!-- Links -->
[nfd-operator]: https://github.com/kubernetes-sigs/node-feature-discovery-operator
[OLM]: https://github.com/operator-framework/operator-lifecycle-manager
