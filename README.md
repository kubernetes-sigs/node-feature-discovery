# Node Feature Discovery

[![Go Report Card](https://goreportcard.com/badge/sigs.k8s.io/node-feature-discovery)](https://goreportcard.com/report/sigs.k8s.io/node-feature-discovery)
[![Prow Build](https://prow.k8s.io/badge.svg?jobs=post-node-feature-discovery-push-images)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/logs/post-node-feature-discovery-push-images)
[![Prow E2E-Test](https://prow.k8s.io/badge.svg?jobs=postsubmit-node-feature-discovery-e2e-test)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/logs/postsubmit-node-feature-discovery-e2e-test)

Welcome to Node Feature Discovery – a Kubernetes add-on for detecting hardware
features and system configuration!

## See our [Documentation][documentation] for detailed instructions and reference

### Quick-start – the short-short version

#### Helm

```bash
helm install -n node-feature-discovery --create-namespace nfd oci://registry.k8s.io/nfd/charts/node-feature-discovery --version 0.18.2
```

#### Kustomize

Alternatively, you can deploy using kubectl and kustomize:

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref=v0.18.2"
```

#### Verify the deployment

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

[documentation]: https://kubernetes-sigs.github.io/node-feature-discovery
