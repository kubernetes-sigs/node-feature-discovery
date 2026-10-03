---
title: "E2E-test config reference"
layout: default
sort: 5
published: false
---

# Configuration file reference of end-to-end tests
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

This section describes the end-to-end test configuration file. The file is
passed to the e2e tests with the `-nfd.e2e-config` flag, which `make e2e-test`
sets from the `E2E_TEST_CONFIG` variable. The tests run in the `test/e2e`
directory, so use an absolute path:

```bash
make e2e-test KUBECONFIG=$HOME/.kube/config E2E_TEST_CONFIG=$PWD/e2e-config.yaml
```

Without a configuration file the tests that depend on `defaultFeatures` are
skipped and the `kubelet` defaults are used. A configuration file must contain
the `defaultFeatures` section, otherwise the e2e tests panic when reading the
file. `defaultFeatures` and `kubelet` are both top-level sections:

```yaml
defaultFeatures:
  labelWhitelist:
    - "feature.node.kubernetes.io/kernel-version.major"
  annotationWhitelist:
    - "nfd.node.kubernetes.io/feature-labels"
  nodes:
    - name: workers
      nodeNameRegexp: "-worker"
      expectedLabelValues:
        "feature.node.kubernetes.io/kernel-version.major": "6"
      expectedLabelKeys:
        - "feature.node.kubernetes.io/kernel-version.full"
      expectedAnnotationKeys:
        - "nfd.node.kubernetes.io/feature-labels"
kubelet:
  configPath: "/var/lib/kubelet/config.yaml"
  podResourcesSocketPath: "/var/lib/kubelet/pod-resources/kubelet.sock"
```

## defaultFeatures

The `defaultFeatures` section configures the test that deploys nfd-worker with
the default feature sources and checks the labels and annotations of the nodes.
For each node, the test uses the first entry of `defaultFeatures.nodes` whose
`nodeNameRegexp` matches the node name. Nodes without a matching entry are
skipped.

### defaultFeatures.labelWhitelist

`defaultFeatures.labelWhitelist` is the list of label keys that are checked
strictly: if a node has a label from this list that starts with
`feature.node.kubernetes.io` and that is not listed in `expectedLabelValues` or
`expectedLabelKeys` of the node's entry, the test fails. Other labels are only
checked if they are expected.

Default: *empty*

### defaultFeatures.annotationWhitelist

`defaultFeatures.annotationWhitelist` is the list of annotation keys that are
checked strictly: if a node has an annotation from this list that starts with
`nfd.node.kubernetes.io` and that is not listed in `expectedAnnotationValues`
or `expectedAnnotationKeys` of the node's entry, the test fails.

Default: *empty*

### defaultFeatures.nodes

`defaultFeatures.nodes` is the list of expected labels and annotations, per
group of nodes. Each entry has the following fields:

- `name`: name of the entry, only used in the test log.
- `nodeNameRegexp`: regular expression (Go syntax) that is matched against the
  node name. The match is not anchored, and an empty value matches all nodes.
- `expectedLabelValues`: map of labels that must exist on the node with the
  given value.
- `expectedLabelKeys`: list of labels that must exist on the node, with any
  value.
- `expectedAnnotationValues`: map of annotations that must exist on the node
  with the given value.
- `expectedAnnotationKeys`: list of annotations that must exist on the node,
  with any value.

Default: *empty*

## kubelet

The `kubelet` section configures the host paths that the nfd-topology-updater
tests mount into the nfd-topology-updater pods.

### kubelet.configPath

`kubelet.configPath` is the path of the kubelet configuration file on the
nodes.

Default: `/var/lib/kubelet/config.yaml`

### kubelet.podResourcesSocketPath

`kubelet.podResourcesSocketPath` is the path of the kubelet pod resources
socket on the nodes.

Default: `/var/lib/kubelet/pod-resources/kubelet.sock`
