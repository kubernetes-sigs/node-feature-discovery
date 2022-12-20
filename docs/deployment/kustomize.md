---
title: "Kustomize"
layout: default
sort: 2
---

# Deploymenet with Kustomize
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

[Kustomize](https://github.com/kubernetes-sigs/kustomize) provides easy
deployment of NFD. Customization of the deployment is done by maintaining
declarative overlays on top of the base overlays in NFD.

To follow the deployment instructions here,
[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl) v1.21 or
later is required.

The kustomize overlays provided in the repo can be used directly:

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
```

This will required RBAC rules and deploy nfd-master (as a deployment) and
nfd-worker (as daemonset) in the `node-feature-discovery` namespace.

**NOTE:** nfd-topology-updater is not deployed as part of the `default` overlay.
Please refer to the [Master Worker Topologyupdater](#master-worker-topologyupdater)
and [Topologyupdater](#topologyupdater) below.

Alternatively you can clone the repository and customize the deployment by
creating your own overlays. For example, to deploy the
[minimal](image-variants#minimal) image. See [kustomize][kustomize] for more
information about managing deployment configurations.

## Overlays

The NFD repository hosts a set of overlays for different usages and deployment
scenarios under
[`deployment/overlays`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays)

- [`default`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/default):
  default deployment of nfd-worker as a daemonset, described above
- [`default-combined`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/default-combined)
  see [Master-worker pod](#master-worker-pod) below
- [`default-job`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/default-job):
  see [Worker one-shot](#worker-one-shot) below
- [`master-worker-topologyupdater`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/master-worker-topologyupdater):
  see [Master Worker Topologyupdater](#master-worker-topologyupdater) below
- [`topologyupdater`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/topologyupdater):
  see [Topology Updater](#topologyupdater) below
- [`prune`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/prune):
  clean up the cluster after uninstallation, see
  [Removing feature labels](uninstallation#removing-feature-labels)
- [`samples/cert-manager`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/samples/cert-manager):
  an example for supplementing the default deployment with cert-manager for TLS
  authentication, see
  [Automated TLS certificate management using cert-manager](tls)
  for details
- [`samples/custom-rules`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/samples/custom-rules):
  an example for spicing up the default deployment with a separately managed
  configmap of custom labeling rules, see
  [Custom feature source](../usage/features#custom) for more information about
  custom node labels

### Master-worker pod

You can also run nfd-master and nfd-worker inside the same pod

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default-combined?ref={{ site.release }}

```

This creates a DaemonSet that runs nfd-worker and nfd-master in the same Pod.
In this case no nfd-master is run on the master node(s), but, the worker nodes
are able to label themselves which may be desirable e.g. in single-node setups.

**NOTE:** nfd-topology-updater is not deployed by the default-combined overlay.
To enable nfd-topology-updater in this scenario,the users must customize the
deployment themselves.

### Worker one-shot

Feature discovery can alternatively be configured as a one-shot job.
The `default-job` overlay may be used to achieve this:

```bash
NUM_NODES=$(kubectl get no -o jsonpath='{.items[*].metadata.name}' | wc -w)
kubectl kustomize https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default-job?ref={{ site.release }} | \
    sed s"/NUM_NODES/$NUM_NODES/" | \
    kubectl apply -f -
```

The example above launches as many jobs as there are non-master nodes. Note that
this approach does not guarantee running once on every node. For example,
tainted, non-ready nodes or some other reasons in Job scheduling may cause some
node(s) will run extra job instance(s) to satisfy the request.

### Master Worker Topologyupdater

NFD-Master, nfd-worker and nfd-topology-updater can be configured to be
deployed as separate pods. The `master-worker-topologyupdater` overlay may be
used to achieve this:

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/master-worker-topologyupdater?ref={{ site.release }}

```

### Topologyupdater

In order to deploy just nfd-topology-updater (without nfd-master and nfd-worker)
use the `topologyupdater` overlay:

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/topologyupdater?ref={{ site.release }}

```

NFD-Topology-Updater can be configured along with the `default` overlay
(which deploys nfd-worker and nfd-master) where all the software components
are deployed as separate pods;

```bash

kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/topologyupdater?ref={{ site.release }}

```

## Uninstallation

Simplest way is to invoke `kubectl delete` on the overlay that was used for
deployment.  Beware that this will also delete the namespace that NFD is
running in. For example, in case the default overlay from the repo was used:

```bash
kubectl delete -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
```

Alternatively you can delete create objects one-by-one, depending on the type
of deployment, for example:

```bash
NFD_NS=node-feature-discovery
kubectl -n $NFD_NS delete ds nfd-worker
kubectl -n $NFD_NS delete deploy nfd-master
kubectl -n $NFD_NS delete svc nfd-master
kubectl -n $NFD_NS delete sa nfd-master
kubectl delete clusterrole nfd-master
kubectl delete clusterrolebinding nfd-master
```

<!-- Links -->
[kustomize]: https://github.com/kubernetes-sigs/kustomize
