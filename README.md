# Node Feature Discovery

[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/node-feature-discovery)](https://goreportcard.com/report/github.com/kubernetes-sigs/node-feature-discovery)
[![Prow Build](https://prow.k8s.io/badge.svg?jobs=post-node-feature-discovery-push-images)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/logs/post-node-feature-discovery-push-images)
[![Prow E2E-Test](https://prow.k8s.io/badge.svg?jobs=postsubmit-node-feature-discovery-e2e-test)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/logs/postsubmit-node-feature-discovery-e2e-test)

Welcome to Node Feature Discovery – a Kubernetes add-on for detecting hardware
features and system configuration!

### See our [Documentation][documentation] for detailed instructions and reference

#### Quick-start – the short-short version

```bash
$ kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref=v0.12.0
  namespace/node-feature-discovery created
  customresourcedefinition.apiextensions.k8s.io/nodefeaturerules.nfd.k8s-sigs.io created
  serviceaccount/nfd-master created
  clusterrole.rbac.authorization.k8s.io/nfd-master created
  clusterrolebinding.rbac.authorization.k8s.io/nfd-master created
  configmap/nfd-worker-conf created
  service/nfd-master created
  deployment.apps/nfd-master created
  daemonset.apps/nfd-worker created

$ kubectl -n node-feature-discovery get all
  NAME                              READY   STATUS    RESTARTS   AGE
  pod/nfd-master-555458dbbc-sxg6w   1/1     Running   0          56s
  pod/nfd-worker-mjg9f              1/1     Running   0          17s
...

$ kubectl get no -o json | jq .items[].metadata.labels
  {
    "kubernetes.io/arch": "amd64",
    "kubernetes.io/os": "linux",
    "feature.node.kubernetes.io/cpu-cpuid.ADX": "true",
    "feature.node.kubernetes.io/cpu-cpuid.AESNI": "true",
...

```

[documentation]: https://kubernetes-sigs.github.io/node-feature-discovery
