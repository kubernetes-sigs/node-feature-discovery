# Node Feature Discovery

[![Go Report Card](https://goreportcard.com/badge/sigs.k8s.io/node-feature-discovery)](https://goreportcard.com/report/sigs.k8s.io/node-feature-discovery)
[![Prow Build](https://prow.k8s.io/badge.svg?jobs=post-node-feature-discovery-push-images)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/logs/post-node-feature-discovery-push-images)
[![Prow E2E-Test](https://prow.k8s.io/badge.svg?jobs=postsubmit-node-feature-discovery-e2e-test)](https://prow.k8s.io/job-history/gs/kubernetes-jenkins/logs/postsubmit-node-feature-discovery-e2e-test)

Node Feature Discovery (NFD) detects hardware features and system configuration
on each node in a Kubernetes cluster and advertises them as node labels,
annotations, and extended resources. Schedulers, operators, and controllers can
then use those labels to place workloads on nodes with the right CPU
instructions, GPUs, network adapters, or kernel features.

For full documentation see the **[Documentation][documentation]** site.

> **Using a release?** Read the docs for your version, not this branch.
> The `main` branch reflects work in progress and may not match your deployment.
>
> | NFD version | Docs |
> |---|---|
> | v0.19 (latest) | [v0.19 docs][documentation] |
> | v0.18 | [v0.18 docs](https://kubernetes-sigs.github.io/node-feature-discovery/v0.18) |

## How it works

NFD runs three components:

| Component | Runs as | What it does |
|---|---|---|
| `nfd-worker` | DaemonSet (every node) | Discovers hardware and OS features; publishes them as `NodeFeature` objects |
| `nfd-master` | Deployment | Reads `NodeFeature` objects, evaluates rules, writes labels and annotations to Node objects, and maintains `NodeFeatureGroup` status |
| `nfd-gc` | Deployment | Removes stale `NodeFeature` and `NodeResourceTopology` objects when nodes are deleted |

## Quick start

> Requires Kubernetes v1.24 or later.

### Helm

```bash
helm install -n node-feature-discovery --create-namespace nfd \
  oci://registry.k8s.io/nfd/charts/node-feature-discovery --version 0.19.0
```

### Kustomize

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref=v0.19.0"
```

### Verify

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
}
```

`nfd-worker` has labeled every node with the features it detected. You can now
use those labels in a Pod's `nodeSelector`:

```yaml
nodeSelector:
  feature.node.kubernetes.io/cpu-cpuid.AESNI: "true"
```

## Compatibility

NFD is compatible with Kubernetes v1.24 and later. The two most recent NFD
minor releases are supported. See the [versions reference][versions] for the
full deprecation policy.

| NFD version | Kubernetes |
|---|---|
| v0.19.x | v1.24+ |
| v0.18.x | v1.24+ |

## Community

- Slack: [#node-feature-discovery](https://kubernetes.slack.com/messages/node-feature-discovery)
  on [kubernetes.slack.com](https://slack.k8s.io/)
- Mailing list: [SIG-Node](https://groups.google.com/a/kubernetes.io/g/sig-node)
- Issues: [GitHub issue tracker](https://github.com/kubernetes-sigs/node-feature-discovery/issues)

NFD is a [SIG-Node](https://github.com/kubernetes/community/blob/master/sig-node/README.md)
subproject under the [Kubernetes SIGs](https://github.com/kubernetes-sigs) organization.

[documentation]: https://kubernetes-sigs.github.io/node-feature-discovery/v0.19
[versions]: https://kubernetes-sigs.github.io/node-feature-discovery/v0.19/reference/versions
