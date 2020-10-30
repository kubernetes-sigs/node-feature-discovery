# Node Feature Discovery

[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/node-feature-discovery)](https://goreportcard.com/report/github.com/kubernetes-sigs/node-feature-discovery)
[![Prow Build](https://prow.k8s.io/badge.svg?jobs=pull-node-feature-discovery-build-image)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/pr-logs/directory/pull-node-feature-discovery-build-image)

Welcome to Node Feature Discovery – a Kubernetes add-on for detecting hardware
features and system configuration!

### See our [Documentation][documentation] for detailed instructions and reference

#### Quick-start – the short-short version

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/v0.6.0/nfd-master.yaml.template
  namespace/node-feature-discovery created
...

kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/v0.6.0/nfd-worker-daemonset.yaml.template
  daemonset.apps/nfd-worker created

kubectl -n node-feature-discovery get all
  NAME                              READY   STATUS    RESTARTS   AGE
  pod/nfd-master-555458dbbc-sxg6w   1/1     Running   0          56s
  pod/nfd-worker-mjg9f              1/1     Running   0          17s
...

$ kubectl get no -o json | jq .items[].metadata.labels
  {
    "beta.kubernetes.io/arch": "amd64",
    "beta.kubernetes.io/os": "linux",
    "feature.node.kubernetes.io/cpu-cpuid.ADX": "true",
    "feature.node.kubernetes.io/cpu-cpuid.AESNI": "true",
...

```

[documentation]: https://kubernetes-sigs.github.io/node-feature-discovery
