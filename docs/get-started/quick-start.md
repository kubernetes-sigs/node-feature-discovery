---
title: "Quick Start"
layout: default
sort: 2
---

# Quick Start

Minimal steps to deploy latest released version of NFD in your cluster.

## Installation

Deploy nfd-master -- creates a new namespace, service and required RBAC rules

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/nfd-master.yaml.template
```

Deploy nfd-worker as a daemonset

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/nfd-worker-daemonset.yaml.template
```

## Verify

Wait until NFD master and worker are running.

```bash
$ kubectl -n node-feature-discovery get ds,deploy
NAME                        DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
daemonset.apps/nfd-worker   3         3         3       3            3           <none>          5s
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

## Use Node Labels

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
