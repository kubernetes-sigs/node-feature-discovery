---
title: "Kubectl plugin"
parent: "Usage"
layout: default
nav_order: 10
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
NodeFeatureRule objects. It can be installed with the following command:

```bash
git clone https://github.com/kubernetes-sigs/node-feature-discovery
cd node-feature-discovery
make build-kubectl-nfd
KUBECTL_PATH=/usr/local/bin/
mv ./bin/kubectl-nfd ${KUBECTL_PATH}
```

### Validate

The plugin can be used to validate a NodeFeatureRule object:

```bash
kubectl nfd validate -f <nodefeaturerule.yaml>
```

### Test

The plugin can be used to test a NodeFeatureRule object against a node:

```bash
kubectl nfd test -f <nodefeaturerule.yaml> -n <node-name>
```

### DryRun

The plugin can be used to DryRun a NodeFeatureRule object against a NodeFeature
file:

```bash
kubectl get -n node-feature-discovery nodefeature <nodename> -o yaml > <nodefeature.yaml>
kubectl nfd dryrun -f <nodefeaturerule.yaml> -n <nodefeature.yaml>
```

Or you can use the example NodeFeature file(it is a minimal NodeFeature file):

```bash
$ kubectl nfd dryrun -f examples/nodefeaturerule.yaml -n examples/nodefeature.yaml
Evaluating NodeFeatureRule "examples/nodefeaturerule.yaml" against NodeFeature "examples/nodefeature.yaml"
Processing rule:  my sample rule
*** Labels ***
vendor.io/my-sample-feature=true
NodeFeatureRule "examples/nodefeaturerule.yaml" is valid for NodeFeature "examples/nodefeature.yaml"
```
