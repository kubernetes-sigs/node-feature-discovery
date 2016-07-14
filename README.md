# CPU Feature Discovery for Kubernetes

- [Overview](#overview)
- [Getting Started](#getting-started)
  * [System Requirements](#system-requirements)
  * [Usage](#usage)
- [Building from source](#building-from-source)
- [Targeting Nodes with Specific Features](#targeting-nodes-with-specific-features)
- [License](#license)

## Overview

This software enables Intel Architecture (IA) feature discovery for Kubernetes.
It detects CPU features available on each node in a Kubernetes cluster, such as
Intel [Resource Director Technology][intel-rdt] and advertises those
features using node labels.

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

- A "namespace" to denote vendor-specific information
  (`node.alpha.intel.com`).
- The version of this discovery code (e.g. `v0.1.0`) that wrote the
  label.
- The relevant hardware component each label describes (e.g. `cpu`).
- The name of the discovered feature as it appears in the underlying
  source, mostly `cpuid` (e.g. `AESNI`).

_Note: only features that are available on a given node are labeled, so the
only label value published is the string `"true"`. This feature discovery code
will not add a label with the value `"false"` for features that are not
present._

```
"node.alpha.intel.com/v0.1.0-cpu-<feature-name>": "true"
```

### System Requirements

At a minimum, you will need:

1. Linux (x86_64)
1. [kubectl] [kubectl-setup] (properly set up and configured to work with your Kubernetes cluster)
1. [GCC] [gcc-down] (only required to build software to detect Intel RDT feature set)
1. [Docker] [docker-down] (only required to build and push docker images)

### Usage

Feature discovery is done as a one-shot job. There is an example script in this
repo that demonstrates how to deploy the job to unlabeled nodes.

```
./label-nodes.sh
```

The discovery script will launch a job on each each unlabeled node in the
cluster. When the job runs, it contacts the Kubernetes API server to add
labels to the node to advertise hardware features (initially, from `cpuid` and
RDT).

## Building from source

Download the source code.

```
git clone https://github.com/intelsdi-x/dbi-iafeature-discovery
```

The build steps described below are optional. The default docker image in
Dockerhub at `intelsdi/nodelabels` can be used to decorate the Kubernetes node
with CPU features. Skip to usage instructions if you do not need to build your
own docker image.

**Build the Intel RDT Detection Software Using `make` (Optional):**

```
cd <project-root>/rdt-discovery
make
```

**Build the Docker image (optional):**

```
cd <project-root>
docker build -t <user-name>/<image-name> .
```

Push the Docker Image (optional)

```
docker push
```

**Change the job spec to use your new image (optional):**

To use your published image from the step above instead of the
`intelsdi/nodelabels` image, edit line 40 in the file
`[dbi-iafeature-discovery-job.json.template](dbi-iafeature-discoverz-job.json.template)` to
the new location (`<user>/<image-name>`).

## Targeting Nodes with Specific Features

Nodes with specific features can be targeted using the `nodeSelector` field. The following example shows
how to target the Intel RDT L3 cache allocation (RDTL3CA) feature.

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
                "node.alpha.intel.com/v0.1.0-cpu-RDTL3CA": "true"
        }
    }
}
```

For more details on targeting nodes, see [node selection][node-sel].


## License

This is open source software released under the [Apache 2.0 License](LICENSE).

<!-- Links -->
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[docker-down]: https://docs.docker.com/engine/installation/
[golang-down]: https://golang.org/dl/
[gcc-down]: https://gcc.gnu.org/
[kubectl-setup]: https://coreos.com/kubernetes/docs/latest/configure-kubectl.html
[balaji-github]: https://github.com/balajismaniam
[node-sel]: http://kubernetes.io/docs/user-guide/node-selection/ 
