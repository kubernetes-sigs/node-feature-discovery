---
title: "NFD Operator"
parent: "Deployment"
layout: default
nav_order: 4
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
