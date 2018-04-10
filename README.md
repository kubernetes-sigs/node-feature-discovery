# Node feature discovery for [Kubernetes](https://kubernetes.io)

[![Build Status](https://api.travis-ci.org/kubernetes-incubator/node-feature-discovery.svg?branch=master)](https://travis-ci.com/kubernetes-incubator/node-feature-discovery)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/node-feature-discovery)](https://goreportcard.com/report/github.com/kubernetes-incubator/node-feature-discovery)

- [Overview](#overview)
- [Command line interface](#command-line-interface)
- [Feature discovery](#feature-discovery)
  - [Feature sources](#feature-sources)
  - [Feature labels](#feature-labels)
- [Getting started](#getting-started)
  - [System requirements](#system-requirements)
  - [Usage](#usage)
- [Building from source](#building-from-source)
- [Targeting nodes with specific features](#targeting-nodes-with-specific-features)
- [References](#references)
- [License](#license)
- [Demo](#demo)

_**NOTE:** We are gathering evidence in order to graduate from the Kubernetes incubator. If you are a user of the project, please add yourself to [this list](https://github.com/kubernetes-incubator/node-feature-discovery/wiki/Users) with as much detail as you are comfortable providing (name and email optional)._

## Overview

This software enables node feature discovery for Kubernetes. It detects
hardware features available on each node in a Kubernetes cluster, and advertises
those features using node labels.

This project uses GitHub [milestones](https://github.com/kubernetes-incubator/node-feature-discovery/milestones) for release planning.

## Command line interface

```
node-feature-discovery.

  Usage:
  node-feature-discovery [--no-publish --sources=<sources> --label-whitelist=<pattern>]
  node-feature-discovery -h | --help
  node-feature-discovery --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpuid,rdt,pstate,memory,network,storage,selinux]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server. [Default: ]
```

## Feature discovery

### Feature sources

The current set of feature sources are the following:

- [CPUID][cpuid] for x86/Arm64 CPU details
- [Intel Resource Director Technology][intel-rdt]
- [Intel P-State driver][intel-pstate]
- Memory
- Network
- Storage
- Selinux

### Feature labels

The published node labels encode a few pieces of information:

- A "namespace" (e.g. `node.alpha.kubernetes-incubator.io/nfd`).
- The version of this discovery code that wrote the label, according to
  `git describe --tags --dirty --always`.
- The source for each label (e.g. `cpuid`).
- The name of the discovered feature as it appears in the underlying
  source, (e.g. `AESNI` from cpuid).

_Note: only features that are available on a given node are labeled, so
the only label value published for features is the string `"true"`._

```json
{
  "node.alpha.kubernetes-incubator.io/node-feature-discovery.version": "v0.1.0",
  "node.alpha.kubernetes-incubator.io/nfd-cpuid-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-rdt-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-pstate-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-memory-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-network-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-storage-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-selinux-<feature-name>": "true"
}
```

The `--sources` flag controls which sources to use for discovery.

_Note: Consecutive runs of node-feature-discovery will update the labels on a
given node. If features are not discovered on a consecutive run, the corresponding
label will be removed. This includes any restrictions placed on the consecutive run,
such as restricting discovered features with the --label-whitelist option._

### Intel Resource Director Technology (RDT) Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| RDTMON         | Intel Cache Monitoring Technology (CMT) and Intel Memory Bandwidth Monitoring (MBM)
| RDTL3CA        | Intel L3 Cache Allocation Technology
| RDTL2CA        | Intel L2 Cache Allocation Technology

### X86 CPUID Features (Partial List)

| Feature name   | Description                                                  |
| :------------: | :----------------------------------------------------------: |
| ADX            | Multi-Precision Add-Carry Instruction Extensions (ADX)
| AESNI          | Advanced Encryption Standard (AES) New Instructions (AES-NI)
| AVX            | Advanced Vector Extensions (AVX)
| AVX2           | Advanced Vector Extensions 2 (AVX2)
| BMI1           | Bit Manipulation Instruction Set 1 (BMI)
| BMI2           | Bit Manipulation Instruction Set 2 (BMI2)
| SSE4.1         | Streaming SIMD Extensions 4.1 (SSE4.1)
| SSE4.2         | Streaming SIMD Extensions 4.2 (SSE4.2)
| SGX            | Software Guard Extensions (SGX)

### Memory Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| numa           | Multiple memory nodes i.e. NUMA architecture detected

### Arm64 CPUID Features (Partial List)

| Feature name   | Description                                                  |
| :------------: | :----------------------------------------------------------: |
| AES            | Announcing the Advanced Encryption Standard
| EVSTRM         | Event Stream Frequency Features
| FPHP           | Half Precision(16bit) Floating Point Data Processing Instructions
| ASIMDHP        | Half Precision(16bit) Asimd Data Processing Instructions
| ATOMICS        | Atomic Instructions to the A64
| ASIMRDM        | Support for Rounding Double Multiply Add/Subtract
| PMULL          | Optional Cryptographic and CRC32 Instructions
| JSCVT          | Perform Conversion to Match Javascript
| DCPOP          | Persistent Memory Support

### Network Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| [SRIOV][sriov] | Single Root Input/Output Virtualization (SR-IOV) enabled Network Interface Card

### Storage Features

| Feature name       | Description                                                                         |
| :--------------:   | :---------------------------------------------------------------------------------: |
| nonrotationaldisk  | Non-rotational disk, like SSD, is present in the node

### Selinux Features

| Feature name       | Description                                                                         |
| :--------------:   | :---------------------------------------------------------------------------------: |
| selinux            | selinux is enabled on the node

## Getting started
### System requirements

1. Linux (x86_64/Arm64)
1. [kubectl] [kubectl-setup] (properly set up and configured to work with your
   Kubernetes cluster)
1. [Docker] [docker-down] (only required to build and push docker images)

### Usage

Feature discovery is done as a one-shot job. There is an example script in this
repo that demonstrates how to deploy the job to unlabeled nodes.

```
./label-nodes.sh
```

The discovery script will launch a job on each unlabeled node in the
cluster. When the job runs, it contacts the Kubernetes API server to add labels
to the node to advertise hardware features (initially, from `cpuid`, RDT, p-state and network).

[![asciicast](https://asciinema.org/a/11wir751y89617oemwnsgli4a.png)](https://asciinema.org/a/11wir751y89617oemwnsgli4a)

## Building from source

Download the source code.

```
git clone https://github.com/kubernetes-incubator/node-feature-discovery
```

**Build the Docker image:**

```
cd <project-root>
make
```

**NOTE: Our default docker image is hosted in quay.io. To override the 
`QUAY_REGISTRY_USER` use the `-e` option as follows: 
`QUAY_REGISTRY_USER=<my-username> make docker -e`**

Push the Docker Image (optional)

```
docker push <quay-domain-name>/<registry-user>/<image-name>:<version>
```

**Change the job spec to use your custom image (optional):**

To use your published image from the step above instead of the
`quay.io/kubernetes_incubator/node-feature-discovery` image, edit line 40 in the file
[node-feature-discovery-job.json.template](node-feature-discovery-job.json.template)
to the new location (`<quay-domain-name>/<registry-user>/<image-name>[:<version>]`).

## Targeting Nodes with Specific Features

Nodes with specific features can be targeted using the `nodeSelector` field. The
following example shows how to target nodes with Intel TurboBoost enabled.

```json
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "labels": {
            "env": "test"
        },
        "name": "golang-test"
    },
    "spec": {
        "containers": [
            {
                "image": "golang",
                "name": "go1",
            }
        ],
        "nodeSelector": {
                "node.alpha.kubernetes-incubator.io/nfd-pstate-turbo": "true"
        }
    }
}
```

For more details on targeting nodes, see [node selection][node-sel].

## References

Github issues

- [#28310](https://github.com/kubernetes/kubernetes/issues/28310)
- [#28311](https://github.com/kubernetes/kubernetes/issues/28311)
- [#28312](https://github.com/kubernetes/kubernetes/issues/28312)

[Design proposal](https://docs.google.com/document/d/1uulT2AjqXjc_pLtDu0Kw9WyvvXm-WAZZaSiUziKsr68/edit)

## Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The project was established 2016-08-29. The incubator team for the project is:

- Sponsor: Dawn Chen (@dchen1107)
- Champion: David Oppenheimer (@davidopp)
- SIG: sig-node

## License

This is open source software released under the [Apache 2.0 License](LICENSE).

## Demo

A demo on the benefits of using node feature discovery can be found in [demo](demo/). 

<!-- Links -->
[cpuid]: http://man7.org/linux/man-pages/man4/cpuid.4.html
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
[sriov]: http://www.intel.com/content/www/us/en/pci-express/pci-sig-sr-iov-primer-sr-iov-technology-paper.html
[docker-down]: https://docs.docker.com/engine/installation
[golang-down]: https://golang.org/dl
[gcc-down]: https://gcc.gnu.org
[kubectl-setup]: https://coreos.com/kubernetes/docs/latest/configure-kubectl.html
[node-sel]: http://kubernetes.io/docs/user-guide/node-selection
