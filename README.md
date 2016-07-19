# CPU Feature Discovery for Kubernetes

[![Build Status](https://travis-ci.com/intelsdi-x/dbi-iafeature-discovery.svg?token=ajyZ5osyX5HNjsUu5muj&branch=master)](https://travis-ci.com/intelsdi-x/dbi-iafeature-discovery)

- [Overview](#overview)
- [Getting Started](#getting-started)
  * [System Requirements](#system-requirements)
  * [Usage](#usage)
- [Building from source](#building-from-source)
- [Targeting Nodes with Specific Features](#targeting-nodes-with-specific-features)
- [License](#license)

## Overview

This software enables node feature discovery for Kubernetes. It detects
hardware features available on each node in a Kubernetes cluster, and advertises
those features using node labels.

### Feature sources

The current set of feature sources are the following:

- [CPUID](http://man7.org/linux/man-pages/man4/cpuid.4.html) for x86 CPU details
- [Intel Resource Director Technology][intel-rdt]
- [Intel P-State driver][intel-pstate]

The `--sources` flag controls which sources to use for discovery.

### Command line interface

```
dbi-iafeature-discovery.

	Usage:
	dbi-iafeature-discovery [--no-publish --sources=<sources>]
	dbi-iafeature-discovery -h | --help
	dbi-iafeature-discovery --version

	Options:
	-h --help           Show this screen.
	--version           Output version and exit.
	--sources=<sources> Comma separated list of feature sources. [Default: cpuid,rdt,pstate]
	--no-publish        Do not publish discovered features to the cluster-local Kubernetes API server.
```

### Intel Resource Director Technology (RDT) Features

| Feature name   | Description                                                                         |
| :------------: | :---------------------------------------------------------------------------------: |
| RDTMON         | Intel Cache Monitoring Technology (CMT) and Intel Memory Bandwidth Monitoring (MBM)
| RDTL3CA        | Intel L3 Cache Allocation Technology
| RDTL2CA        | Intel L2 Cache Allocation Technology

### Other Features (Partial List)

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

The published node labels encode a few pieces of information:

- A "namespace" to denote vendor-specific information (`node.alpha.intel.com`).
- The version of this discovery code that wrote the label, for example
  `"node.alpha.intel.com/dbi-iafeature-discovery.version": "v0.1.0"`.
  The value of this label corresponds to the output from
  `git describe --tags --dirty --always`.
- The relevant information source for each label (e.g. `cpuid`).
- The name of the discovered feature as it appears in the underlying
  source, mostly `cpuid` (e.g. `AESNI`).

_Note: only features that are available on a given node are labeled, so
the only label value published for features is the string `"true"`. This
feature discovery code will not add a label with the value `"false"` for
features that are not present._

```json
{
  "node.alpha.intel.com/v0.1.0-cpuid-<feature-name>": "true",
  "node.alpha.intel.com/v0.1.0-rdt-<feature-name>": "true",
  "node.alpha.intel.com/v0.1.0-pstate-<feature-name>": "true"
}
```

### System Requirements

At a minimum, you will need:

1. Linux (x86_64)
1. [kubectl] [kubectl-setup] (properly set up and configured to work with your
   Kubernetes cluster)
1. [Docker] [docker-down] (only required to build and push docker images)

### Usage

Feature discovery is done as a one-shot job. There is an example script in this
repo that demonstrates how to deploy the job to unlabeled nodes.

```
./label-nodes.sh
```

The discovery script will launch a job on each each unlabeled node in the
cluster. When the job runs, it contacts the Kubernetes API server to add labels
to the node to advertise hardware features (initially, from `cpuid` and RDT).

## Building from source

Download the source code.

```
git clone https://github.com/intelsdi-x/dbi-iafeature-discovery
```

**Build the Docker image:**

```
cd <project-root>
make
```

**NOTE: To override the `DOCKER_REGISTRY_USER` use the `-e` option as follows:
`DOCKER_REGISTRY_USER=<my-username> make docker -e`**

Push the Docker Image (optional)

```
docker push <registry-user>/<image-name>:<version>
```

**Change the job spec to use your custom image (optional):**

To use your published image from the step above instead of the
`intelsdi/nodelabels` image, edit line 40 in the file
[dbi-iafeature-discovery-job.json.template](dbi-iafeature-discovery-job.json.template)
to the new location (`<registry-user>/<image-name>`).

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
                "node.alpha.intel.com/v0.1.0-pstate-turbo": "true"
        }
    }
}
```

For more details on targeting nodes, see [node selection][node-sel].


## License

This is open source software released under the [Apache 2.0 License](LICENSE).

<!-- Links -->
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
[docker-down]: https://docs.docker.com/engine/installation
[golang-down]: https://golang.org/dl
[gcc-down]: https://gcc.gnu.org
[kubectl-setup]: https://coreos.com/kubernetes/docs/latest/configure-kubectl.html
[balaji-github]: https://github.com/balajismaniam
[node-sel]: http://kubernetes.io/docs/user-guide/node-selection
