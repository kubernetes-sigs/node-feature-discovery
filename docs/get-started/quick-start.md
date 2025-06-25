---
title: "Quick start"
parent: "Get started"
layout: default
nav_order: 2
---

# Quick start

Minimal steps to deploy latest released version of NFD in your cluster.

## Installation

Deploy with kustomize -- creates a new namespace, service and required RBAC
rules and deploys nfd-master and nfd-worker daemons.

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}"
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
$ kubectl get no -o json | jq ".items[].metadata.labels"
{
  "kubernetes.io/arch": "amd64",
  "kubernetes.io/os": "linux",
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
  - image: registry.k8s.io/pause
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

### Deploy nfd-topology-updater

To deploy nfd-topology-updater use the `topologyupdater` kustomize
overlay.

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/topologyupdater?ref={{ site.release }}"
```

### Verify nfd-topology-updater

Wait until nfd-topology-updater is running.

```bash
$ kubectl -n node-feature-discovery get ds
NAME                                  DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
daemonset.apps/nfd-topology-updater   2         2         2       2            2           <none>          5s

```

Check that the NodeResourceTopology objects are created

```bash
$ kubectl get noderesourcetopologies.topology.node.k8s.io
NAME                 AGE
kind-control-plane   23s
kind-worker          23s
```
