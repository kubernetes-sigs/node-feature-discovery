---
title: "Kubectl plugin cmdline reference"
parent: "Reference"
layout: default
nav_order: 8
---

# Commandline flags of kubectl-nfd (plugin)
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

To quickly view available command line flags execute `kubectl nfd -help`.

### -h, -help

Print usage and exit.

## Validate

Validate a NodeFeatureRule file.

### -f / --nodefeature-file

The `--nodefeature-file` flag specifies the path to the NodeFeatureRule file
to validate.

## Test

Test a NodeFeatureRule file against a node without applying it.

### -k, --kubeconfig

The `--kubeconfig` flag specifies the path to the kubeconfig file to use for
CLI requests.

### -s, --namespace

The `--namespace` flag specifies the namespace to use for CLI requests.
Default: `default`.

### -n, --nodename

The `--nodename` flag specifies the name of the node to test the
NodeFeatureRule against.

### -f, --nodefeaturerule-file

The `--nodefeaturerule-file` flag specifies the path to the NodeFeatureRule file
to test.

## DryRun

Process a NodeFeatureRule file against a NodeFeature file.

### -f, --nodefeaturerule-file

The `--nodefeaturerule-file` flag specifies the path to the NodeFeatureRule file
to test.

### -n, --nodefeature-file

The `--nodefeature-file` flag specifies the path to the NodeFeature file to test.
