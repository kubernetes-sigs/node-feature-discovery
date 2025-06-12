---
title: "Get started"
layout: default
nav_order: 1
has_children: true
---

# Node Feature Discovery

Welcome to Node Feature Discovery -- a Kubernetes add-on for detecting hardware
features and system configuration!

Continue to:

- **[Introduction](introduction.md)** for more details on the
  project.

- **[Quick start](quick-start.md)** for quick step-by-step
  instructions on how to get NFD running on your cluster.

## Quick-start -- the short-short version

```bash
$ kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
  namespace/node-feature-discovery created
  serviceaccount/nfd-master created
  clusterrole.rbac.authorization.k8s.io/nfd-master created
  clusterrolebinding.rbac.authorization.k8s.io/nfd-master created
  configmap/nfd-worker-conf created
  deployment.apps/nfd-master created
  daemonset.apps/nfd-worker created

$ kubectl -n node-feature-discovery get all
  NAME                              READY   STATUS    RESTARTS   AGE
  pod/nfd-master-555458dbbc-sxg6w   1/1     Running   0          56s
  pod/nfd-worker-mjg9f              1/1     Running   0          17s
...

$ kubectl get nodes -o json | jq '.items[].metadata.labels'
  {
    "kubernetes.io/arch": "amd64",
    "kubernetes.io/os": "linux",
    "feature.node.kubernetes.io/cpu-cpuid.ADX": "true",
    "feature.node.kubernetes.io/cpu-cpuid.AESNI": "true",
...

```
