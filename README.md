#CPU Feature Discovery for Kubernetes
This software enables Intel Architecture (IA) feature discovery for Kubernetes. It detects the CPU features, such as Intel [Resource Director Technology][intel-rdt] (RDT), available in each node of the Kubernetes cluster and labels them appropriately. 

 - [Getting Started](#getting-started)
   * [System Requirements](#system-requirements)
   * [Installation](#installation)
   * [Usage](#usage)
 - [Documentation](#documentation)
 - [License](#license)

##Getting Started
### System Requirements
At a minimum, you will need:
 1. [Docker] [docker-down] (only required to build and push docker images)
 2. [GCC] [gcc-down] (only required to build software to detect Intel RDT feature set)
 3. Linux (x86_64)
 4. [kubectl] [kubectl-setup] (properly setup and configured to work with a Kubernetes cluster)

###Installation
####Downloading the Source Code
```
git clone https://github.com/intelsdi-x/dbi-iafeature-discovery <repo-name>
``` 

####Building from Source
The build steps described below are optional. The default docker image in Docker hub at `intelsdi/nodelabels` can be used to decorate the Kubernetes node with IA features. Skip to usage instructions if you do not need to build your own docker image. 
 
Build the Intel RDT Detection Software Using `make` (Optional):
```
cd <rep-name>/rdt-discovery
make 
```

Build the Docker Image (Optional): 
```
cd <repo-name>
docker build -t <user-name>/<image-name> .
```

Push the Docker Image (Optional)
```
docker push
```

Change the Job Spec to Use the Appropriate Image (Optional): 
Change line #40 in `featurelabeling-job.json.template` to the appropriate image name in the `<user-name>/<image-name>` format used for building the image above or use the default `intelsdi/nodelabels` image.

###Usage
Deploying Kubernetes Job for Node Feature Labeling:
```
./label-nodes.sh
```
The above command will label each node in the cluster with the features from cpuid. The labeling format used is 
```
<key, val> = <"node.alpha.intel.com/<feature-name>", "true">
```
Note that only features that are available in that node are labelled. 

##Documentation
This software determines the Intel Architecture (IA) features associated with a processor using `cpuid`. The following set of features are discovered and labelled by this software. 

###Intel Resource Director Technology (RDT) Features
|   Label        |  Feature(s) Represented                                                             | 
| :------------: | :---------------------------------------------------------------------------------: | 
| RDTMON         | Intel Cache Monitoring Technology (CMT) and Intel Memory Bandwidth Monitoring (MBM) |
| RDTL3CA        | Intel L3 Cache Allocation Technology |
| RDTL2CA        | Intel L2 Cache Allocation Technology |

###Other Features (Partial List)
|   Label        |  Feature(s) Represented                                                             | 
| :------------: | :---------------------------------------------------------------------------------: | 
| ADX            | Multi-Precision Add-Carry Instruction Extensions (ADX) |  
| AESNI            | Advanced Encryption Standard (AES) New Instructions (AES-NI) |  
| AVX            | Advanced Vector Extensions (AVX)|  
| AVX2            | Advanced Vector Extensions 2 (AVX2)|  
| BMI1            | Bit Manipulation Instruction Set 1 (BMI)|  
| BMI2            | Bit Manipulation Instruction Set 2 (BMI2)|  
| SSE4.1           | Streaming SIMD Extensions 4.1 (SSE4.1)|  
| SSE4.2           | Streaming SIMD Extensions 4.2 (SSE4.2)|  
| SGX            | Software Guard Extensions (SGX) |  

##License
This is an Open Source software released under the Apache 2.0 [License](LICENSE).

[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[docker-down]: https://docs.docker.com/engine/installation/
[golang-down]: https://golang.org/dl/
[gcc-down]: https://gcc.gnu.org/
[kubectl-setup]: https://coreos.com/kubernetes/docs/latest/configure-kubectl.html
[balaji-github]: https://github.com/balajismaniam
