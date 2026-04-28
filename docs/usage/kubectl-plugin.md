---
title: "Kubectl plugin"
layout: default
sort: 10
---

# Kubectl plugin
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

> ***Developer Preview*** This feature is currently in developer preview and
> subject to change. It is not recommended to use it in production
> environments.

## Overview

The `kubectl` plugin `kubectl nfd` can be used to validate/dryrun and test
NodeFeatureRule and NodeFeatureGroup objects. It can be installed with the
following command:

```bash
git clone https://github.com/kubernetes-sigs/node-feature-discovery
cd node-feature-discovery
make build-kubectl-nfd
KUBECTL_PATH=/usr/local/bin/
mv ./bin/kubectl-nfd ${KUBECTL_PATH}
```

### Validate

The plugin can be used to validate a NodeFeatureRule or NodeFeatureGroup object.
The kind is detected automatically from the file content:

```bash
kubectl nfd validate -f <nodefeaturerule-or-nodefeaturegroup.yaml>
```

You can use the example files to try it out:

```bash
$ kubectl nfd validate -f examples/nodefeaturegroup.yaml
Validating examples/nodefeaturegroup.yaml
Validating rule:  kernel version
Validating rule:  veth kernel module
examples/nodefeaturegroup.yaml is valid
```

### Test

The plugin can be used to test a NodeFeatureRule or NodeFeatureGroup object
against a node. The kind is detected automatically from the file content:

```bash
kubectl nfd test -f <nodefeaturerule-or-nodefeaturegroup.yaml> -n <node-name>
```

### DryRun

The plugin can be used to dry run a NodeFeatureRule or NodeFeatureGroup object
against a NodeFeature file. The kind is detected automatically from the file
content:

```bash
kubectl get -n node-feature-discovery nodefeature <nodename> -o yaml > <nodefeature.yaml>
kubectl nfd dryrun -f <nodefeaturerule-or-nodefeaturegroup.yaml> -n <nodefeature.yaml>
```

For example, using the example files:

```bash
$ kubectl nfd dryrun -f examples/nodefeaturerule.yaml -n examples/nodefeature.yaml
Evaluating "examples/nodefeaturerule.yaml" against NodeFeature "examples/nodefeature.yaml"
Processing rule:  my sample rule
*** Labels ***
vendor.io/my-sample-feature=true
"examples/nodefeaturerule.yaml" is valid for NodeFeature "examples/nodefeature.yaml"
```

```bash
$ kubectl nfd dryrun -f examples/nodefeaturegroup.yaml -n examples/nodefeature.yaml
Evaluating "examples/nodefeaturegroup.yaml" against NodeFeature "examples/nodefeature.yaml"
Processing rule:  kernel version
Rule "kernel version" did not match
Processing rule:  veth kernel module
Rule "veth kernel module" did not match
"examples/nodefeaturegroup.yaml" is valid for NodeFeature "examples/nodefeature.yaml"
```
