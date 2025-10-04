---
title: "Quick start"
layout: default
sort: 2
---

# Quick start

Minimal steps to deploy latest released version of NFD in your cluster.

## Installation

NFD installation consists of CRDs, RBAC rules, Deployments of the nfd-master
and nfd-gc daemons and DaemonSet of the nfd-worker daemon.

### Helm

```bash
helm install -n node-feature-discovery --create-namespace nfd {{ site.helm_oci_repo }} --version {{ site.helm_chart_version }}
```

### Kustomize

Alternatively, NFD can be deploy with kubectl/kustomize.

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}"
```

## Verify

Wait until NFD pods are running.

```bash
$ kubectl -n node-feature-discovery get ds,deploy
NAME                                               DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
daemonset.apps/nfd-node-feature-discovery-worker   2         2         2       2            2           <none>          20s

NAME                                                READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/nfd-node-feature-discovery-gc       1/1     1            1           20s
deployment.apps/nfd-node-feature-discovery-master   1/1     1            1           20s

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

#### Deploy nfd-topology-updater with Helm

```bash
helm upgrade --install -n node-feature-discovery --create-namespace nfd {{ site.helm_oci_repo }} --version {{ site.helm_chart_version }} --set topologyUpdater.enable=true
```

#### Deploy nfd-topology-updater with Kustomize

There's a separate overlay that deploys nfd-topology-updater in addition to the
default NFD components.

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/topologyupdater?ref={{ site.release }}"
```

### Verify nfd-topology-updater

Wait until nfd-topology-updater is running.

```bash
$ kubectl -n node-feature-discovery get ds
NAME                                                         DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
daemonset.apps/nfd-node-feature-discovery-topology-updater   2         2         2       2            2           <none>          20s
...
```

Check that the NodeResourceTopology objects are created

```bash
$ kubectl get noderesourcetopologies.topology.node.k8s.io
NAME                 AGE
kind-control-plane   23s
kind-worker          23s
```
