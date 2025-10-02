---
title: "Get started"
layout: default
sort: 1
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

### Helm

```bash
helm install -n node-feature-discovery --create-namespace nfd {{ site.helm_oci_repo }} --version {{ site.helm_chart_version }}
```

### Kustomize

Alternatively, you can deploy using kubectl and kustomize:

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
```

### Verify the deployment

```bash
$ kubectl -n node-feature-discovery get all
  NAME                              READY   STATUS    RESTARTS   AGE
  pod/nfd-gc-565fc85d9b-94jpj       1/1     Running   0          18s
  pod/nfd-master-6796d89d7b-qccrq   1/1     Running   0          18s
  pod/nfd-worker-nwdp6              1/1     Running   0          18s
...

$ kubectl get no -o json | jq ".items[].metadata.labels"
  {
    "kubernetes.io/arch": "amd64",
    "kubernetes.io/os": "linux",
    "feature.node.kubernetes.io/cpu-cpuid.ADX": "true",
    "feature.node.kubernetes.io/cpu-cpuid.AESNI": "true",
...
```
