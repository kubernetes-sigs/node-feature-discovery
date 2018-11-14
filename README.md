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

To try out stand-alone, one can run a Docker container where node-feature-discovery is already set as entry point.
Such run is useful for checking features-detection part, but labeling part is expected to fail.
It is recommended to use --no-publish and --oneshot to achieve clean run in stand-alone case.

```
node-feature-discovery.

  Usage:
  node-feature-discovery [--no-publish] [--sources=<sources>] [--label-whitelist=<pattern>]
     [--oneshot | --sleep-interval=<seconds>] [--config=<path>]
     [--options=<config>]
  node-feature-discovery -h | --help
  node-feature-discovery --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --config=<path>             Config file to use.
                              [Default: /etc/kubernetes/node-feature-discovery/node-feature-discovery.conf]
  --options=<config>          Specify config options from command line. Config
                              options are specified in the same format as in the
                              config file (i.e. json or yaml). These options
                              will override settings read from the config file.
                              [Default: ]
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpuid,iommu,kernel,memory,network,pci,pstate,rdt,selinux,storage]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server. [Default: ]
  --oneshot                   Label once and exit.
  --sleep-interval=<seconds>  Time to sleep between re-labeling. Non-positive
                              value implies no re-labeling (i.e. infinite
                              sleep). [Default: 60s]
```
**NOTE** Some feature sources need certain directories and/or files from the
host mounted inside the NFD container. Thus, you need to provide Docker with the
correct `--volume` options in order for them to work correctly when run
stand-alone directly with `docker run`. See the
[template spec](https://github.com/kubernetes-incubator/node-feature-discovery/blob/master/node-feature-discovery-daemonset.yaml.template)
for up-to-date information about the required volume mounts.

## Feature discovery

### Feature sources

The current set of feature sources are the following:

- [CPUID][cpuid] for x86/Arm64 CPU details
- IOMMU
- Kernel
- Memory
- Network
- Pstate ([Intel P-State driver][intel-pstate])
- RDT ([Intel Resource Director Technology][intel-rdt])
- Selinux
- Storage

### Feature labels

The published node labels encode a few pieces of information:

- A "namespace" (e.g. `node.alpha.kubernetes-incubator.io`).
- The version of this discovery code that wrote the label, according to
  `git describe --tags --dirty --always`.
- The source for each label (e.g. `cpuid`).
- The name of the discovered feature as it appears in the underlying
  source, (e.g. `AESNI` from cpuid).
- The value of the discovered feature.

Feature label names adhere to the following pattern:
```
<namespace>/nfd-<source name>-<feature name>[.<attribute name>]
```
The last component (i.e. `attribute-name`) is optional, and only used if a
feature logically has sub-hierarchy, e.g. `sriov.capable` and
`sriov.configure` from the `network` source.

_Note: only features that are available on a given node are labeled, so
the only label value published for features is the string `"true"`._

```json
{
  "node.alpha.kubernetes-incubator.io/node-feature-discovery.version": "v0.3.0",
  "node.alpha.kubernetes-incubator.io/nfd-cpuid-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-iommu-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-kernel-version.<version component>": "<version number>",
  "node.alpha.kubernetes-incubator.io/nfd-memory-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-network-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-pci-<device label>.present": "true",
  "node.alpha.kubernetes-incubator.io/nfd-pstate-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-rdt-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-selinux-<feature-name>": "true",
  "node.alpha.kubernetes-incubator.io/nfd-storage-<feature-name>": "true"
}
```

The `--sources` flag controls which sources to use for discovery.

_Note: Consecutive runs of node-feature-discovery will update the labels on a
given node. If features are not discovered on a consecutive run, the corresponding
label will be removed. This includes any restrictions placed on the consecutive run,
such as restricting discovered features with the --label-whitelist option._

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

### IOMMU Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| enabled        | IOMMU is present and enabled in the kernel

### Kernel Features

| Feature | Attribute | Description                                            |
| ------- | --------- | ------------------------------------------------------ |
| version | full      | Full kernel version as reported by `/proc/sys/kernel/osrelease` (e.g. '4.5.6-7-g123abcde')
| <br>    | major     | First component of the kernel version (e.g. '4')
| <br>    | minor     | Second component of the kernel version (e.g. '5')
| <br>    | revision  | Third component of the kernel version (e.g. '6')

### Memory Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| numa           | Multiple memory nodes i.e. NUMA architecture detected

### Network Features

| Feature | Attribute  | Description                                           |
| ------- | ---------- | ----------------------------------------------------- |
| sriov   | capable    | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled Network Interface Card(s) present
| <br>    | configured | SR-IOV virtual functions have been configured

### PCI Features

| Feature              | Attribute | Description                               |
| -------------------- | --------- | ----------------------------------------- |
| &lt;device label&gt; | present   | PCI device is detected

The `<device label>` part is composed of raw PCI IDs, separated by dashes.
The set of fields used in `<device label>` is configurable, valid fields being
`class`, `vendor`, `device`, `subsystem_vendor` and `subsystem_device`.
Defaults fields are `class` and `vendor`. An example label using the default
label fields:
```
node.alpha.kubernetes-incubator.io/nfd-pci-1200_8086.present=true
```

Also  the set of PCI device classes that the feature source detects is
configurable. By default, device classes (0x)03, (0x)0b40 and (0x)12, i.e.
GPUs, co-processors and accelerator cards are deteted.

See [configuration options](#configuration-options)
for more information on NFD config.

### RDT (Intel Resource Director Technology) Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| RDTMON         | Intel RDT Monitoring Technology
| RDTCMT         | Intel Cache Monitoring (CMT)
| RDTMBM         | Intel Memory Bandwidth Monitoring (MBM)
| RDTL3CA        | Intel L3 Cache Allocation Technology
| RDTL2CA        | Intel L2 Cache Allocation Technology
| RDTMBA         | Intel Memory Bandwidth Allocation (MBA) Technology

### Selinux Features

| Feature name       | Description                                                                         |
| :--------------:   | :---------------------------------------------------------------------------------: |
| selinux            | selinux is enabled on the node

### Storage Features

| Feature name       | Description                                                                         |
| :--------------:   | :---------------------------------------------------------------------------------: |
| nonrotationaldisk  | Non-rotational disk, like SSD, is present in the node

## Getting started
### System requirements

1. Linux (x86_64/Arm64)
1. [kubectl] [kubectl-setup] (properly set up and configured to work with your
   Kubernetes cluster)
1. [Docker] [docker-down] (only required to build and push docker images)

### Usage

Feature discovery is preferably run as a Kubernetes DaemonSet. There is an
example spec that can be used as a template, or, as is when just trying out the
service:
```
kubectl create -f rbac.yaml
kubectl create -f node-feature-discovery-daemonset.yaml.template
```

When the job runs, it contacts the Kubernetes API server to add labels
to the node to advertise hardware features.

If you have RBAC authorization enabled (as is the default e.g. with clusters initialized with kubeadm) you need to configure the appropriate ClusterRoles, ClusterRoleBindings and a ServiceAccount in order for NFD to create node labels. The provided templates will configure these for you.

When run as a daemonset, nodes are re-labeled at an interval specified using
the `--sleep-interval` option. In the [template](https://github.com/kubernetes-incubator/node-feature-discovery/blob/master/node-feature-discovery-daemonset.yaml.template#L26) the default interval is set to 60s
which is also the default when no `--sleep-interval` is specified.

Feature discovery can alternatively be configured as a one-shot job. There is
an example script in this repo that demonstrates how to deploy the job in the cluster.

```
./label-nodes.sh
```

The label-nodes.sh script tries to launch as many jobs as there are Ready nodes.
Note that this approach does not guarantee running once on every node.
For example, if some node is tainted NoSchedule or fails to start a job for some other reason, then some other node will run extra job instance(s) to satisfy the request and the tainted/failed node does not get labeled.

[![asciicast](https://asciinema.org/a/11wir751y89617oemwnsgli4a.svg)](https://asciinema.org/a/11wir751y89617oemwnsgli4a)

### Configuration options

NFD supports a configuration file. The default location is
`/etc/kubernetes/node-feature-discovery/node-feature-discovery.conf`, but,
this can be changed by specifying the`--config` command line flag. The file is
read inside the Docker image, and thus, Volumes and VolumeMounts are needed to
make your configuration available for NFD. The preferred method is to use a
ConfigMap.
For example, create a config map using the example config as a template:
```
cp node-feature-discovery.conf.example node-feature-discovery.conf
vim node-feature-discovery.conf  # edit the configuration
kubectl create configmap node-feature-discovery-config --from-file=node-feature-discovery.conf
```
Then, configure Volumes and VolumeMounts in the Pod spec (just the relevant
snippets shown below):
```
...
  containers:
      volumeMounts:
        - name: node-feature-discovery-config
          mountPath: "/etc/kubernetes/node-feature-discovery/"
...
  volumes:
    - name: node-feature-discovery-config
      configMap:
        name: node-feature-discovery-config
...
```
You could also use other types of volumes, of course. That is, hostPath if
different config for different nodes would be required, for example.

The (empty-by-default)
[example config](https://github.com/kubernetes-incubator/node-feature-discovery/blob/master/node-feature-discovery.conf.example)
is used as a config in the NFD Docker image. Thus, this can be used as a default
configuration in custom-built images.

Configuration options can also be specified via the `--options` command line
flag, in which case no mounts need to be used. The same format as in the config
file must be used, i.e. JSON (or YAML). For example:
```
--options='{"sources": { "pci": { "deviceClassWhitelist": ["12"] } } }'
```
Configuration options specified from the command line will override those read
from the config file.

Currently, the only available configuration options are related to the
[PCI feature source](#pci-features).

## Building from source

Download the source code.

```
git clone https://github.com/kubernetes-incubator/node-feature-discovery
```

**Build the container image:**

```
cd <project-root>
make
```

**NOTE: Our default docker image is hosted in quay.io. To override the 
`QUAY_REGISTRY_USER` use the `-e` option as follows: 
`QUAY_REGISTRY_USER=<my-username> make image -e`**

You can also specify a build tool different from Docker, for example:
```
make IMAGE_BUILD_CMD="buildah bud"
```

Push the container image (optional, this example with Docker)

```
docker push <quay-domain-name>/<registry-user>/<image-name>:<version>
```

**Change the job spec to use your custom image (optional):**

To use your published image from the step above instead of the
`quay.io/kubernetes_incubator/node-feature-discovery` image, edit `image` attribute in the file
[node-feature-discovery-job.yaml.template](node-feature-discovery-job.yaml.template#L23)
to the new location (`<quay-domain-name>/<registry-user>/<image-name>[:<version>]`).

## Targeting Nodes with Specific Features

Nodes with specific features can be targeted using the `nodeSelector` field. The
following example shows how to target nodes with Intel TurboBoost enabled.

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    env: test
  name: golang-test
spec:
  containers:
    - image: golang
      name: go1
  nodeSelector:
    node.alpha.kubernetes-incubator.io/nfd-pstate-turbo: 'true'
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
