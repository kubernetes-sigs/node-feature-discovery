---
title: "Quick start"
layout: default
sort: 2
---

# Quick start

Minimal steps to deploy latest released version of NFD in your cluster.

## Installation

Deploy with kustomize -- creates a new namespace, service and required RBAC
rules and deploys nfd-master and nfd-worker daemons.

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
```

## Verify

Wait until NFD master and NFD worker are running.

```bash
$ kubectl -n node-feature-discovery get ds,deploy
NAME                         DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
daemonset.apps/nfd-worker    2         2         2       2            2           <none>          10s

NAME                         READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/nfd-master   1/1     1            1           17s

```

Check that NFD feature labels have been created

```bash
$ kubectl get no -o json | jq .items[].metadata.labels
{
  "beta.kubernetes.io/arch": "amd64",
  "beta.kubernetes.io/os": "linux",
  "feature.node.kubernetes.io/cpu-cpuid.ADX": "true",
  "feature.node.kubernetes.io/cpu-cpuid.AESNI": "true",
  "feature.node.kubernetes.io/cpu-cpuid.AVX": "true",
...
```

## Use node labels

Create a pod targeting a distinguishing feature (select a valid feature from
the list printed on the previous step)

```bash
$ cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: feature-dependent-pod
spec:
  containers:
  - image: k8s.gcr.io/pause
    name: pause
  nodeSelector:
    # Select a valid feature
    feature.node.kubernetes.io/cpu-cpuid.AESNI: 'true'
EOF
pod/feature-dependent-pod created
```

See that the pod is running on a desired node

```bash
$ kubectl get po feature-dependent-pod -o wide
NAME                    READY   STATUS    RESTARTS   AGE   IP          NODE     NOMINATED NODE   READINESS GATES
feature-dependent-pod   1/1     Running   0          23s   10.36.0.4   node-2   <none>           <none>
```

## Additional Optional Installation Steps

In order to deploy nfd-master and nfd-topology-updater daemons
use `topologyupdater` overlay.

Deploy with kustomize -- creates a new namespace, service and required RBAC
rules and nfd-master and nfd-topology-updater daemons.

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/topologyupdater?ref={{ site.release }}
```

**NOTE:**

[PodResource API][podresource-api] is a prerequisite for nfd-topology-updater.

Preceding Kubernetes v1.23, the `kubelet` must be started with the following flag:

`--feature-gates=KubeletPodResourcesGetAllocatable=true`

Starting Kubernetes v1.23, the `GetAllocatableResources` is enabled by default
through `KubeletPodResourcesGetAllocatable` [feature gate][feature-gate].

## Verify

Wait until NFD master and NFD topologyupdater are running.

```bash
$ kubectl -n node-feature-discovery get ds,deploy
NAME                                  DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
daemonset.apps/nfd-topology-updater   2         2         2       2            2           <none>          5s

NAME                         READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/nfd-master   1/1     1            1           17s

```

Check that the NodeResourceTopology CR instances are created

```bash
$ kubectl get noderesourcetopologies.topology.node.k8s.io
NAME                 AGE
kind-control-plane   23s
kind-worker          23s
```

## Show the CR instances

```bash
$ kubectl describe noderesourcetopologies.topology.node.k8s.io kind-control-plane
Name:         kind-control-plane
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  topology.node.k8s.io/v1alpha1
Kind:         NodeResourceTopology
...
Topology Policies:
  SingleNUMANodeContainerLevel
Zones:
    Name:             node-0
    Costs:
      node-0:  10
      node-1:  20
    Resources:
        Name:         Cpu
        Allocatable:  3
        Capacity:     3
        Available:    3
        Name:         vendor/nic1
        Allocatable:  2
        Capacity:     2
        Available:    2
        Name:         vendor/nic2
        Allocatable:  2
        Capacity:     2
        Available:    2
    Type:             Node
    Name:             node-1
    Costs:
      node-0:  20
      node-1:  10
    Resources:
        Name:         Cpu
        Allocatable:  4
        Capacity:     4
        Available:    4
        Name:         vendor/nic1
        Allocatable:  2
        Capacity:     2
        Available:    2
        Name:         vendor/nic2
        Allocatable:  2
        Capacity:     2
        Available:    2
    Type:             Node
Events:               <none>
```

The CR instances created can be used to gain insight into the allocatable
resources along with the granularity of those resources at a per-zone level
(represented by node-0 and node-1 in the above example) or can be used by an
external entity (e.g. topology-aware scheduler plugin) to take an action based
on the gathered information.

<!-- Links -->
[podresource-api]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#monitoring-device-plugin-resources
[feature-gate]: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates
