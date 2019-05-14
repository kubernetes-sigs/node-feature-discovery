# Node feature discovery for [Kubernetes](https://kubernetes.io)

[![Build Status](https://api.travis-ci.org/kubernetes-sigs/node-feature-discovery.svg?branch=master)](https://travis-ci.org/kubernetes-sigs/node-feature-discovery)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/node-feature-discovery)](https://goreportcard.com/report/github.com/kubernetes-sigs/node-feature-discovery)

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

## Overview

This software enables node feature discovery for Kubernetes. It detects
hardware features available on each node in a Kubernetes cluster, and advertises
those features using node labels.

NFD consists of two software components:
1. **nfd-master** is responsible for labeling Kubernetes node objects
2. **nfd-worker** is detects features and communicates them to nfd-master.
   One instance of nfd-worker is supposed to be run on each node of the cluster

## Command line interface

You can run NFD in stand-alone Docker containers e.g. for testing
purposes. This is useful for checking features-detection.

### NFD-Master

When running as a standalone container labeling is expected to fail because
Kubernetes API is not available. Thus, it is recommended to use `--no-publish`
command line flag. E.g.
```
$ docker run --rm --name=nfd-test <NFD_CONTAINER_IMAGE> nfd-master --no-publish
2019/02/01 14:48:21 Node Feature Discovery Master <NFD_VERSION>
2019/02/01 14:48:21 gRPC server serving on port: 8080
```

Command line flags of nfd-master:
```
$ docker run --rm <NFD_CONTAINER_IMAGE> nfd-master --help
...
nfd-master.

  Usage:
  nfd-master [--no-publish] [--label-whitelist=<pattern>] [--port=<port>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
     [--verify-node-name] [--extra-label-ns=<list>]
  nfd-master -h | --help
  nfd-master --version

  Options:
  -h --help                       Show this screen.
  --version                       Output version and exit.
  --port=<port>                   Port on which to listen for connections.
                                  [Default: 8080]
  --ca-file=<path>                Root certificate for verifying connections
                                  [Default: ]
  --cert-file=<path>              Certificate used for authenticating connections
                                  [Default: ]
  --key-file=<path>               Private key matching --cert-file
                                  [Default: ]
  --verify-node-name              Verify worker node name against CN from the TLS
                                  certificate. Only has effect when TLS authentication
                                  has been enabled.
  --no-publish                    Do not publish feature labels
  --label-whitelist=<pattern>     Regular expression to filter label names to
                                  publish to the Kubernetes API server. [Default: ]
  --extra-label-ns=<list>         Comma separated list of allowed extra label namespaces
                                  [Default: ]
```

### NFD-Worker

In order to run nfd-worker as a "stand-alone" container against your
standalone nfd-master you need to run them in the same network namespace:
```
$ docker run --rm --network=container:nfd-test <NFD_CONTAINER_IMAGE> nfd-worker
2019/02/01 14:48:56 Node Feature Discovery Worker <NFD_VERSION>
...
```
If you just want to try out feature discovery without connecting to nfd-master,
pass the `--no-publish` flag to nfd-worker.

Command line flags of nfd-worker:
```
$ docker run --rm <CONTAINER_IMAGE_ID> nfd-worker --help
...
nfd-worker.

  Usage:
  nfd-worker [--no-publish] [--sources=<sources>] [--label-whitelist=<pattern>]
     [--oneshot | --sleep-interval=<seconds>] [--config=<path>]
     [--options=<config>] [--server=<server>] [--server-name-override=<name>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
  nfd-worker -h | --help
  nfd-worker --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --config=<path>             Config file to use.
                              [Default: /etc/kubernetes/node-feature-discovery/nfd-worker.conf]
  --options=<config>          Specify config options from command line. Config
                              options are specified in the same format as in the
                              config file (i.e. json or yaml). These options
                              will override settings read from the config file.
                              [Default: ]
  --ca-file=<path>            Root certificate for verifying connections
                              [Default: ]
  --cert-file=<path>          Certificate used for authenticating connections
                              [Default: ]
  --key-file=<path>           Private key matching --cert-file
                              [Default: ]
  --server=<server>           NFD server address to connecto to.
                              [Default: localhost:8080]
  --server-name-override=<name> Name (CN) expect from server certificate, useful
                              in testing
                              [Default: ]
  --sources=<sources>         Comma separated list of feature sources.
                              [Default: cpu,iommu,kernel,local,memory,network,pci,storage,system]
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
[template spec](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-worker-daemonset.yaml.template)
for up-to-date information about the required volume mounts.

## Feature discovery

### Feature sources

The current set of feature sources are the following:

- CPU
- IOMMU
- Kernel
- Memory
- Network
- PCI
- Storage
- System
- Local (hooks for user-specific features)

### Feature labels

The published node labels encode a few pieces of information:

- Namespace, i.e. `feature.node.kubernetes.io`
- The source for each label (e.g. `cpu`).
- The name of the discovered feature as it appears in the underlying
  source, (e.g. `cpuid.AESNI` from cpu).
- The value of the discovered feature.

Feature label names adhere to the following pattern:
```
<namespace>/<source name>-<feature name>[.<attribute name>]
```
The last component (i.e. `attribute-name`) is optional, and only used if a
feature logically has sub-hierarchy, e.g. `sriov.capable` and
`sriov.configure` from the `network` source.


```json
{
  "feature.node.kubernetes.io/cpu-<feature-name>": "true",
  "feature.node.kubernetes.io/iommu-<feature-name>": "true",
  "feature.node.kubernetes.io/kernel-<feature name>": "<feature value>",
  "feature.node.kubernetes.io/memory-<feature-name>": "true",
  "feature.node.kubernetes.io/network-<feature-name>": "true",
  "feature.node.kubernetes.io/pci-<device label>.present": "true",
  "feature.node.kubernetes.io/storage-<feature-name>": "true",
  "feature.node.kubernetes.io/system-<feature name>": "<feature value>",
  "feature.node.kubernetes.io/<file name>-<feature name>": "<feature value>"
}
```

The `--sources` flag controls which sources to use for discovery.

_Note: Consecutive runs of nfd-worker will update the labels on a
given node. If features are not discovered on a consecutive run, the corresponding
label will be removed. This includes any restrictions placed on the consecutive run,
such as restricting discovered features with the --label-whitelist option._

### CPU Features

| Feature name            | Attribute          | Description                   |
| ----------------------- | ------------------ | ----------------------------- |
| cpuid                   | &lt;cpuid flag&gt; | CPU capability is supported
| hardware_multithreading | <br>               | Hardware multithreading, such as Intel HTT, enabled (number of locical CPUs is greater than physical CPUs)
| power                   | sst_bf.enabled     | Intel SST-BF ([Intel Speed Select Technology][intel-sst] - Base frequency) enabled
| [pstate][intel-pstate]  | turbo              | Turbo frequencies are enabled in Intel pstate driver
| [rdt][intel-rdt]        | RDTMON             | Intel RDT Monitoring Technology
| <br>                    | RDTCMT             | Intel Cache Monitoring (CMT)
| <br>                    | RDTMBM             | Intel Memory Bandwidth Monitoring (MBM)
| <br>                    | RDTL3CA            | Intel L3 Cache Allocation Technology
| <br>                    | RDTL2CA            | Intel L2 Cache Allocation Technology
| <br>                    | RDTMBA             | Intel Memory Bandwidth Allocation (MBA) Technology

The (sub-)set of CPUID attributes to publish is configurable via the
`attributeBlacklist` and `attributeWhitelist` cpuid options of the cpu source.
If whitelist is specified, only whitelisted attributes will be published. With
blacklist, only blacklisted attributes are filtered out. `attributeWhitelist`
has priority over `attributeBlacklist`.  For examples and more information
about configurability, see [Configuration Options](#configuration-options).
By default, the following CPUID flags have been blacklisted:
BMI1, BMI2, CLMUL, CMOV, CX16, ERMS, F16C, HTT, LZCNT, MMX, MMXEXT, NX, POPCNT,
RDRAND, RDSEED, RDTSCP, SGX, SSE, SSE2, SSE3, SSE4.1, SSE4.2 and SSSE3.

**NOTE** The cpuid features advertise *supported* CPU capabilities, that is, a
capability might be supported but not enabled.


#### X86 CPUID Attributes (Partial List)

| Attribute | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| ADX       | Multi-Precision Add-Carry Instruction Extensions (ADX)
| AESNI     | Advanced Encryption Standard (AES) New Instructions (AES-NI)
| AVX       | Advanced Vector Extensions (AVX)
| AVX2      | Advanced Vector Extensions 2 (AVX2)

#### Arm64 CPUID Attribute (Partial List)

| Attribute | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| AES       | Announcing the Advanced Encryption Standard
| EVSTRM    | Event Stream Frequency Features
| FPHP      | Half Precision(16bit) Floating Point Data Processing Instructions
| ASIMDHP   | Half Precision(16bit) Asimd Data Processing Instructions
| ATOMICS   | Atomic Instructions to the A64
| ASIMRDM   | Support for Rounding Double Multiply Add/Subtract
| PMULL     | Optional Cryptographic and CRC32 Instructions
| JSCVT     | Perform Conversion to Match Javascript
| DCPOP     | Persistent Memory Support

### IOMMU Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| enabled        | IOMMU is present and enabled in the kernel

### Kernel Features

| Feature | Attribute           | Description                                  |
| ------- | ------------------- | -------------------------------------------- |
| config  | &lt;option name&gt; | Kernel config option is enabled (set 'y' or 'm').<br> Default options are `NO_HZ`, `NO_HZ_IDLE`, `NO_HZ_FULL` and `PREEMPT`
| selinux | enabled             | Selinux is enabled on the node
| version | full                | Full kernel version as reported by `/proc/sys/kernel/osrelease` (e.g. '4.5.6-7-g123abcde')
| <br>    | major               | First component of the kernel version (e.g. '4')
| <br>    | minor               | Second component of the kernel version (e.g. '5')
| <br>    | revision            | Third component of the kernel version (e.g. '6')

Kernel config file to use, and, the set of config options to be detected are
configurable.
See [configuration options](#configuration-options) for more information.

### Memory Features

| Feature | Attribute | Description                                            |
| ------- | --------- | ------------------------------------------------------ |
| numa    | <br>      | Multiple memory nodes i.e. NUMA architecture detected
| nv      | present   | NVDIMM device(s) are present

### Network Features

| Feature | Attribute  | Description                                           |
| ------- | ---------- | ----------------------------------------------------- |
| sriov   | capable    | [Single Root Input/Output Virtualization][sriov] (SR-IOV) enabled Network Interface Card(s) present
| <br>    | configured | SR-IOV virtual functions have been configured

### PCI Features

| Feature              | Attribute | Description                               |
| -------------------- | --------- | ----------------------------------------- |
| &lt;device label&gt; | present   | PCI device is detected

`<device label>` is composed of raw PCI IDs, separated by underscores.
The set of fields used in `<device label>` is configurable, valid fields being
`class`, `vendor`, `device`, `subsystem_vendor` and `subsystem_device`.
Defaults are `class` and `vendor`. An example label using the default
label fields:
```
feature.node.kubernetes.io/pci-1200_8086.present=true
```

Also  the set of PCI device classes that the feature source detects is
configurable. By default, device classes (0x)03, (0x)0b40 and (0x)12, i.e.
GPUs, co-processors and accelerator cards are detected.

See [configuration options](#configuration-options)
for more information on NFD config.

### Storage Features

| Feature name       | Description                                                                         |
| :--------------:   | :---------------------------------------------------------------------------------: |
| nonrotationaldisk  | Non-rotational disk, like SSD, is present in the node

### System Features

| Feature     | Attribute        | Description                                 |
| ----------- | ---------------- | --------------------------------------------|
| os_release  | ID               | Operating system identifier
| <br>        | VERSION_ID       | Operating system version identifier (e.g. '6.7')
| <br>        | VERSION_ID.major | First component of the OS version id (e.g. '6')
| <br>        | VERSION_ID.minor | Second component of the OS version id (e.g. '7')

### Feature Detector Hooks (User-specific Features)

NFD has a special feature source named *local* which is designed for getting the
labels from user-specific feature detector. It provides a mechanism for users to
implement custom feature sources in a pluggable way, without modifying nfd
source code or Docker images. The local feature source can be used to advertise
new user-specific features, and, for overriding labels created by the other
feature sources.

The *local* feature source gets its labels by two different ways:
* It tries to execute files found under `/etc/kubernetes/node-feature-discovery/source.d/`
directory. The hook files must be executable. When executed, the hooks are
supposed to print all discovered features in `stdout`, one per line.
* It reads files found under `/etc/kubernetes/node-feature-discovery/features.d/`
directory. The file content is expected to be similar to the hook output (described above).

These directories must be available inside the Docker image so Volumes and
VolumeMounts must be used if standard NFD images are used. The given template
files mount by default the `source.d` and the `features.d` directories
respectively from `/etc/kubernetes/node-feature-discovery/source.d/` and
`/etc/kubernetes/node-feature-discovery/features.d/` from the host. You should
update them to match your needs.

In both cases, the labels can be binary or non binary, using either `<name>` or
`<name>=<value>` format.

Unlike the other feature sources, the name of the file, instead of the name of
the feature source (that would be `local` in this case), is used as a prefix in
the label name, normally. However, if the `<name>` of the label starts with a
slash (`/`) it is used as the label name as is, without any additional prefix.
This makes it possible for the user to fully control the feature label names,
e.g. for overriding labels created by other feature sources.

You can also override the default namespace of your labels using this format:
`<namespace>/<name>[=<value>]`. You must whitelist your namespace using the
`--extra-label-ns` option on the master. In this case, the name of the
file will not be added to the label name. For example, if you want to add the
label `my.namespace.org/my-label=value`, your hook output or file must contains
`my.namespace.org/my-label=value` and you must add
`--extra-label-ns=my.namespace.org` on the master command line.

`stderr` output of the hooks is propagated to NFD log so it can be used for
debugging and logging.

**A hook example:**<br/>
User has a shell script
`/etc/kubernetes/node-feature-discovery/source.d/my-source` which has the
following `stdout` output:
```
MY_FEATURE_1
MY_FEATURE_2=myvalue
/override_source-OVERRIDE_BOOL
/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```
which, in turn, will translate into the following node labels:
```
feature.node.kubernetes.io/my-source-MY_FEATURE_1=true
feature.node.kubernetes.io/my-source-MY_FEATURE_2=myvalue
feature.node.kubernetes.io/override_source-OVERRIDE_BOOL=true
feature.node.kubernetes.io/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```

**A file example:**<br/>
User has a file
`/etc/kubernetes/node-feature-discovery/features.d/my-source` which contains the
following lines:
```
MY_FEATURE_1
MY_FEATURE_2=myvalue
/override_source-OVERRIDE_BOOL
/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```
which, in turn, will translate into the following node labels:
```
feature.node.kubernetes.io/my-source-MY_FEATURE_1=true
feature.node.kubernetes.io/my-source-MY_FEATURE_2=myvalue
feature.node.kubernetes.io/override_source-OVERRIDE_BOOL=true
feature.node.kubernetes.io/override_source-OVERRIDE_VALUE=123
override.namespace/value=456
```

NFD tries to run any regular files found from the hooks directory. Any
additional data files your hook might need (e.g. a configuration file) should
be placed in a separate directory in order to avoid NFD unnecessarily trying to
execute these. You can use a subdirectory under the hooks directory, for
example `/etc/kubernetes/node-feature-discovery/source.d/conf/`.

**NOTE!** NFD will blindly run any executables placed/mounted in the hooks
directory. It is the user's responsibility to review the hooks for e.g.
possible security implications.

## Getting started

For a stable version with ready-built images see the
[latest released version](https://github.com/kubernetes-sigs/node-feature-discovery/tree/v0.4.0) ([release notes](https://github.com/kubernetes-sigs/node-feature-discovery/releases/latest)).

If you want to use the latest development version (master branch) you need to
[build your own custom image](#building-from-source).

### System requirements

1. Linux (x86_64/Arm64)
1. [kubectl][kubectl-setup] (properly set up and configured to work with your
   Kubernetes cluster)
1. [Docker][docker-down] (only required to build and push docker images)

### Usage

#### nfd-master

Nfd-master runs as a DaemonSet, by default in the master node(s) only. You can
use the template spec provided to deploy nfd-master. If deploying a custom
image, you need to update the template to use the correct image (for the latest
released version, omit the `sed` part):
```
sed -E s',^(\s*)image:.+$,\1image: <YOUR_IMAGE_REPO>:<YOUR_IMAGE_TAG>,' nfd-master.yaml.template > nfd-master.yaml
kubectl create -f nfd-master.yaml
```
Nfd-master listens for connections from nfd-worker(s) and connects to the
Kubernetes API server to adds node labels advertised by them.

If you have RBAC authorization enabled (as is the default e.g. with clusters
initialized with kubeadm) you need to configure the appropriate ClusterRoles,
ClusterRoleBindings and a ServiceAccount in order for NFD to create node
labels. The provided template will configure these for you.


#### nfd-worker

Nfd-worker is preferably run as a Kubernetes DaemonSet. There is an
example spec that can be used as a template, or, as is when just trying out the
service. Similarly to nfd-master above, if using a custom-built image you need
to update the template with the correct image (for the latest release, omit the
`sed` part):
```
sed -E s',^(\s*)image:.+$,\1image: <YOUR_IMAGE_REPO>:<YOUR_IMAGE_TAG>,' nfd-worker-daemonset.yaml.template > nfd-worker-daemonset.yaml
kubectl create -f nfd-worker-daemonset.yaml
```

Nfd-worker connects to the nfd-master service to advertise hardware features.

When run as a daemonset, nodes are re-labeled at an interval specified using
the `--sleep-interval` option. In the [template](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-worker-daemonset.yaml.template#L26) the default interval is set to 60s
which is also the default when no `--sleep-interval` is specified.

Feature discovery can alternatively be configured as a one-shot job. There is
an example script in this repo that demonstrates how to deploy the job in the cluster.

```
./label-nodes.sh [<YOUR_IMAGE_REPO>:<YOUR_IMAGE_TAG>]
```

The label-nodes.sh script tries to launch as many jobs as there are Ready nodes.
Note that this approach does not guarantee running once on every node.
For example, if some node is tainted NoSchedule or fails to start a job for some other reason, then some other node will run extra job instance(s) to satisfy the request and the tainted/failed node does not get labeled.

#### nfd-master and nfd-worker in the same Pod

You can also run nfd-master and nfd-worker inside a single pod (skip the `sed`
part if running the latest released version):
```
sed -E s',^(\s*)image:.+$,\1image: <YOUR_IMAGE_REPO>:<YOUR_IMAGE_TAG>,' nfd-daemonset-combined.yaml.template > nfd-daemonset-combined.yaml
kubectl apply -f nfd-daemonset-combined.yaml
```
Similar to the nfd-worker setup above, this creates a DaemonSet that schedules
an NFD Pod an all worker nodes, with the difference that the Pod also also
contains an nfd-master instance. In this case no nfd-master service is run on
the master node(s), but, the worker nodes are able to label themselves.

This may be desirable e.g. in single-node setups.

#### TLS authentication

NFD supports mutual TLS authentication between the nfd-master and nfd-worker
instances.  That is, nfd-worker and nfd-master both verify that the other end
presents a valid certificate.

TLS authentication is enabled by specifying `--ca-file`, `--key-file` and
`--cert-file` args, on both the nfd-master and nfd-worker instances.
The template specs provided with NFD contain (commented out) example
configuration for enabling TLS authentication.

The Common Name (CN) of the nfd-master certificate must match the DNS name of
the nfd-master Service of the cluster. By default, nfd-master only check that
the nfd-worker has been signed by the specified root certificate (--ca-file).
Additional hardening can be enabled by specifying --verify-node-name in
nfd-master args, in which case nfd-master verifies that the NodeName presented
by nfd-worker matches the Common Name (CN) of its certificate. This means that
each nfd-worker requires a individual node-specific TLS certificate.


#### Usage demo

[![asciicast](https://asciinema.org/a/247316.svg)](https://asciinema.org/a/247316)

### Configuration options

Nfd-worker supports a configuration file. The default location is
`/etc/kubernetes/node-feature-discovery/nfd-worker.conf`, but,
this can be changed by specifying the`--config` command line flag. The file is
read inside the container, and thus, Volumes and VolumeMounts are needed to
make your configuration available for NFD. The preferred method is to use a
ConfigMap.
For example, create a config map using the example config as a template:
```
cp nfd-worker.conf.example nfd-worker.conf
vim nfd-worker.conf  # edit the configuration
kubectl create configmap nfd-worker-config --from-file=nfd-worker.conf
```
Then, configure Volumes and VolumeMounts in the Pod spec (just the relevant
snippets shown below):
```
...
  containers:
      volumeMounts:
        - name: nfd-worker-config
          mountPath: "/etc/kubernetes/node-feature-discovery/"
...
  volumes:
    - name: nfd-worker-config
      configMap:
        name: nfd-worker-config
...
```
You could also use other types of volumes, of course. That is, hostPath if
different config for different nodes would be required, for example.

The (empty-by-default)
[example config](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-worker.conf.example)
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
[CPU](#cpu-features), [PCI](#pci-features) and [Kernel](#kernel-features)
feature sources.

## Building from source

**Download the source code:**

```
git clone https://github.com/kubernetes-sigs/node-feature-discovery
```

**Build the container image:**<br>
See [customizing the build](#customizing-the-build) below for altering the
container image registry, for example.

```
cd <project-root>
make
```

**Push the container image:**<br>
Optional, this example with Docker.

```
docker push <image registry>/<image-name>:<version>
```

**Change the job spec to use your custom image (optional):**

To use your published image from the step above instead of the
`quay.io/kubernetes_incubator/node-feature-discovery` image, edit `image`
attribute in the spec template(s) to the new location
(`<quay-domain-name>/<registry-user>/<image-name>[:<version>]`).

### Customizing the Build
There are several Makefile variables that control the build process and the
name of the resulting container image.

| Variable        | Description                          | Default value
| --------------  | ------------------------------------ | ------------------- |
| IMAGE_BUILD_CMD | Command to build the image           | docker build
| IMAGE_REGISTRY  | Container image registry to use      | quay.io/kubernetes_incubator
| IMAGE_NAME      | Container image name                 | node-feature-discovery
| IMAGE_TAG_NAME  | Container image tag name             | &lt;nfd version&gt;
| IMAGE_REPO      | Container image repository to use    | &lt;IMAGE_REGISTRY&gt;/&lt;IMAGE_NAME&gt;
| IMAGE_TAG       | Full image:tag to tag the image with | &lt;IMAGE_REPO&gt;/&lt;IMAGE_NAME&gt;

For example, to use a custom registry:
```
make IMAGE_REGISTRY=<my custom registry uri>

```
Or to specify a build tool different from Docker:
```
make IMAGE_BUILD_CMD="buildah bud"
```

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
    feature.node.kubernetes.io/cpu-pstate.turbo: 'true'
```

For more details on targeting nodes, see [node selection][node-sel].

## References

Github issues

- [#28310](https://github.com/kubernetes/kubernetes/issues/28310)
- [#28311](https://github.com/kubernetes/kubernetes/issues/28311)
- [#28312](https://github.com/kubernetes/kubernetes/issues/28312)

[Design proposal](https://docs.google.com/document/d/1uulT2AjqXjc_pLtDu0Kw9WyvvXm-WAZZaSiUziKsr68/edit)

## Governance

This is a [SIG-node](https://github.com/kubernetes/community/blob/master/sig-node/README.md)
subproject, hosted under the
[Kubernetes SIGs](https://github.com/kubernetes-sigs) organization in
Github. The project was established in 2016 as a
[Kubernetes Incubator](https://github.com/kubernetes/community/blob/master/incubator.md)
project and migrated to Kubernetes SIGs in 2018.

## License

This is open source software released under the [Apache 2.0 License](LICENSE).

## Demo

A demo on the benefits of using node feature discovery can be found in [demo](demo/). 

<!-- Links -->
[cpuid]: http://man7.org/linux/man-pages/man4/cpuid.4.html
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
[intel-sst]: https://www.intel.com/content/www/us/en/architecture-and-technology/speed-select-technology-article.html
[sriov]: http://www.intel.com/content/www/us/en/pci-express/pci-sig-sr-iov-primer-sr-iov-technology-paper.html
[docker-down]: https://docs.docker.com/install
[golang-down]: https://golang.org/dl
[gcc-down]: https://gcc.gnu.org
[kubectl-setup]: https://kubernetes.io/docs/tasks/tools/install-kubectl
[node-sel]: http://kubernetes.io/docs/user-guide/node-selection
