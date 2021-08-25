---
title: "Deployment and usage"
layout: default
sort: 3
---

# Deployment and usage

{: .no_toc }

## Table of contents

{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Requirements

1. Linux (x86_64/Arm64/Arm)
1. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl)
   (properly set up and configured to work with your Kubernetes cluster)

## Image variants

NFD currently offers two variants of the container image. The "full" variant is
currently deployed by default.

### Full

This image is based on
[debian:buster-slim](https://hub.docker.com/_/debian) and contains a full Linux
system for running shell-based nfd-worker hooks and doing live debugging and
diagnosis of the NFD images.

### Minimal

This is a minimal image based on
[gcr.io/distroless/base](https://github.com/GoogleContainerTools/distroless/blob/master/base/README.md)
and only supports running statically linked binaries.

The container image tag has suffix `-minimal`
(e.g. `{{ site.container_image }}-minimal`)

## Deployment options

### Operator

Deployment using the
[Node Feature Discovery Operator][nfd-operator]
is recommended to be done via
[operatorhub.io](https://operatorhub.io/operator/nfd-operator).

1. You need to have
   [OLM][OLM]
   installed. If you don't, take a look at the
   [latest release](https://github.com/operator-framework/operator-lifecycle-manager/releases/latest)
   for detailed instructions.
1. Install the operator:

    ```bash
    kubectl create -f https://operatorhub.io/install/nfd-operator.yaml
    ```

1. Create NodeFeatureDiscovery resource (in `nfd` namespace here):

    ```bash
    cat << EOF | kubectl apply -f -
    apiVersion: v1
    kind: Namespace
    metadata:
      name: nfd
    ---
    apiVersion: nfd.kubernetes.io/v1alpha1
    kind: NodeFeatureDiscovery
    metadata:
      name: my-nfd-deployment
      namespace: nfd
    EOF
    ```

In order to deploy the [minimal](#minimal) image you need to add

```yaml
  image: {{ site.container_image }}-minimal
```

to the metadata of NodeFeatureDiscovery object above.

### Deployment with kustomize

The kustomize overlays provided in the repo can be used directly:

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
```

This will required RBAC rules and deploy nfd-master (as a deployment) and
nfd-worker (as a daemonset) in the `node-feature-discovery` namespace.

Alternatively you can clone the repository and customize the deployment by
creating your own overlays. For example, to deploy the [minimal](#minimal)
image. See [kustomize][kustomize] for more information about managing
deployment configurations.

#### Default overlays

The NFD repository hosts a set of overlays for different usages and deployment
scenarios under
[`deployment/overlays`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays)

- [`default`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/default):
  default deployment of nfd-worker as a daemonset, descibed above
- [`default-combined`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/default-combined)
  see [Master-worker pod](#master-worker-pod) below
- [`default-job`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/default-job):
  see [Worker one-shot](#worker-one-shot) below
- [`prune`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/prune):
  clean up the cluster after uninstallation, see
  [Removing feature labels](#removing-feature-labels)
- [`samples/cert-manager`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/samples/cert-manager):
  an example for supplementing the default deployment with cert-manager for TLS
  authentication, see
  [Automated TLS certificate management using cert-manager](#automated-tls-certificate-management-using-cert-manager)
  for details
- [`samples/custom-rules`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/samples/custom-rules):
  an example for spicing up the default deployment with a separately managed
  configmap of custom labeling rules, see
  [Custom feature source](#features.md#custom) for more information about
  custom node labels

#### Master-worker pod

You can also run nfd-master and nfd-worker inside the same pod

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default-combined?ref={{ site.release }}

```

This creates a DaemonSet runs both nfd-worker and nfd-master in the same Pod.
In this case no nfd-master is run on the master node(s), but, the worker nodes
are able to label themselves which may be desirable e.g. in single-node setups.

#### Worker one-shot

Feature discovery can alternatively be configured as a one-shot job.
The `default-job` overlay may be used to achieve this:

```bash
NUM_NODES=$(kubectl get no -o jsonpath='{.items[*].metadata.name}' | wc -w)
kubectl kustomize https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default-job?ref={{ site.release }} | \
    sed s"/NUM_NODES/$NUM_NODES/" | \
    kubectl apply -f -
```

The example above launces as many jobs as there are non-master nodes. Note that
this approach does not guarantee running once on every node. For example,
tainted, non-ready nodes or some other reasons in Job scheduling may cause some
node(s) will run extra job instance(s) to satisfy the request.

### Deployment with Helm

Node Feature Discovery Helm chart allow to easily deploy and manage NFD.

#### Prerequisites

[Helm package manager](https://helm.sh/) should be installed.

#### Deployment

To install the latest stable version:

```bash
export NFD_NS=node-feature-discovery
helm repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
helm repo update
helm install nfd/node-feature-discovery --namespace $NFD_NS --create-namespace --generate-name
```

To install the latest development version you need to clone the NFD Git
repository and install from there.

```bash
git clone https://github.com/kubernetes-sigs/node-feature-discovery/
cd node-feature-discovery/deployment/helm
export NFD_NS=node-feature-discovery
helm install node-feature-discovery ./node-feature-discovery/ --namespace $NFD_NS --create-namespace
```

See the [configuration](#configuration) section below for instructions how to
alter the deployment parameters.

In order to deploy the [minimal](#minimal) image you need to override the image
tag:

```bash
helm install node-feature-discovery ./node-feature-discovery/ --set image.tag={{ site.release }}-minimal --namespace $NFD_NS --create-namespace
```

#### Configuration

You can override values from `values.yaml` and provide a file with custom values:

```bash
export NFD_NS=node-feature-discovery
helm install nfd/node-feature-discovery -f <path/to/custom/values.yaml> --namespace $NFD_NS --create-namespace
```

To specify each parameter separately you can provide them to helm install command:

```bash
export NFD_NS=node-feature-discovery
helm install nfd/node-feature-discovery --set nameOverride=NFDinstance --set master.replicaCount=2 --namespace $NFD_NS --create-namespace
```

#### Uninstalling the chart

To uninstall the `node-feature-discovery` deployment:

```bash
export NFD_NS=node-feature-discovery
helm uninstall node-feature-discovery --namespace $NFD_NS
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

#### Chart parameters

In order to tailor the deployment of the Node Feature Discovery to your cluster needs
We have introduced the following Chart parameters.

##### General parameters

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `image.repository` | string | `{{ site.container_image | split: ":" | first }}` | NFD image repository |
| `image.tag` | string | `{{ site.release }}` | NFD image tag |
| `image.pullPolicy` | string | `Always` | Image pull policy |
| `imagePullSecrets` | list | [] | ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec. If specified, these secrets will be passed to individual puller implementations for them to use. For example, in the case of docker, only DockerConfig type secrets are honored. [https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod](More info) |
| `serviceAccount.create` | bool | true | Specifies whether a service account should be created |
| `serviceAccount.annotations` | dict | {} | Annotations to add to the service account |
| `serviceAccount.name` | string |  | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| `rbac` | dict |  | RBAC [parameteres](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) |
| `nameOverride` | string |  | Override the name of the chart |
| `fullnameOverride` | string |  | Override a default fully qualified app name |

##### Master pod parameters

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `master.*` | dict |  | NFD master deployment configuration |
| `master.instance` | string |  |  Instance name. Used to separate annotation namespaces for multiple parallel deployments |
| `master.extraLabelNs` | array | [] | List of allowed extra label namespaces |
| `master.replicaCount` | integer | 1 | Number of desired pods. This is a pointer to distinguish between explicit zero and not specified |
| `master.podSecurityContext` | dict | {} | SecurityContext holds pod-level security attributes and common container settings |
| `master.service.type` | string | ClusterIP | NFD master service type |
| `master.service.port` | integer | port | NFD master service port |
| `master.resources` | dict | {} | NFD master pod [resources management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) |
| `master.nodeSelector` | dict | {} | NFD master pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) |
| `master.tolerations` | dict | _Scheduling to master node is disabled_ | NFD master pod [tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) |
| `master.annotations` | dict | {} | NFD master pod [metadata](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) |
| `master.affinity` | dict |  | NFD master pod required [node affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/) |

##### Worker pod parameters

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `worker.*` | dict |  | NFD master daemonset configuration |
| `worker.configmapName` | string | `nfd-worker-conf` | NFD worker pod ConfigMap name |
| `worker.config` | string | `` | NFD worker service configuration |
| `worker.podSecurityContext` | dict | {} | SecurityContext holds pod-level security attributes and common container settings |
| `worker.securityContext` | dict | {} | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) |
| `worker.resources` | dict | {} | NFD worker pod [resources management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) |
| `worker.nodeSelector` | dict | {} | NFD worker pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) |
| `worker.tolerations` | dict | {} | NFD worker pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) |
| `worker.annotations` | dict | {} | NFD worker pod [metadata](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) |

### Build your own

If you want to use the latest development version (master branch) you need to
build your own custom image.
See the [Developer Guide](../advanced/developer-guide) for instructions how to
build images and deploy them on your cluster.

## Usage

### NFD-Master

NFD-Master runs as a deployment (with a replica count of 1), by default
it prefers running on the cluster's master nodes but will run on worker
nodes if no master nodes are found.

For High Availability, you should simply increase the replica count of
the deployment object. You should also look into adding
[inter-pod](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity)
affinity to prevent masters from running on the same node.
However note that inter-pod affinity is costly and is not recommended
in bigger clusters.

NFD-Master listens for connections from nfd-worker(s) and connects to the
Kubernetes API server to add node labels advertised by them.

If you have RBAC authorization enabled (as is the default e.g. with clusters
initialized with kubeadm) you need to configure the appropriate ClusterRoles,
ClusterRoleBindings and a ServiceAccount in order for NFD to create node
labels. The provided template will configure these for you.

### NFD-Worker

NFD-Worker is preferably run as a Kubernetes DaemonSet. This assures
re-labeling on regular intervals capturing changes in the system configuration
and makes sure that new nodes are labeled as they are added to the cluster.
Worker connects to the nfd-master service to advertise hardware features.

When run as a daemonset, nodes are re-labeled at an default interval of 60s.
This can be changed by using the
[`core.sleepInterval`](../advanced/worker-configuration-reference.html#coresleepinterval)
config option (or
[`-sleep-interval`](../advanced/worker-commandline-reference.html#-sleep-interval)
command line flag).

The worker configuration file is watched and re-read on every change which
provides a simple mechanism of dynamic run-time reconfiguration. See
[worker configuration](#worker-configuration) for more details.

### Communication security with TLS

NFD supports mutual TLS authentication between the nfd-master and nfd-worker
instances.  That is, nfd-worker and nfd-master both verify that the other end
presents a valid certificate.

TLS authentication is enabled by specifying `-ca-file`, `-key-file` and
`-cert-file` args, on both the nfd-master and nfd-worker instances.
The template specs provided with NFD contain (commented out) example
configuration for enabling TLS authentication.

The Common Name (CN) of the nfd-master certificate must match the DNS name of
the nfd-master Service of the cluster. By default, nfd-master only check that
the nfd-worker has been signed by the specified root certificate (-ca-file).
Additional hardening can be enabled by specifying -verify-node-name in
nfd-master args, in which case nfd-master verifies that the NodeName presented
by nfd-worker matches the Common Name (CN) or a Subject Alternative Name (SAN)
of its certificate.

#### Automated TLS certificate management using cert-manager

[cert-manager](https://cert-manager.io/) can be used to automate certificate
management between nfd-master and the nfd-worker pods.

NFD source code repository contains an example kustomize overlay that can be
used to deploy NFD with cert-manager supplied certificates enabled. The
instructions below describe steps how to generate a self-signed CA certificate
and set up cert-manager's
[CA Issuer](https://cert-manager.io/docs/configuration/ca/) to sign
`Certificate` requests for NFD components in `node-feature-discovery`
namespace.

```bash
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.1/cert-manager.yaml
openssl genrsa -out deployment/overlays/samples/cert-manager/tls.key 2048
openssl req -x509 -new -nodes -key deployment/overlays/samples/cert-manager/tls.key -subj "/CN=nfd-ca" \
        -days 10000 -out deployment/overlays/samples/cert-manager/tls.crt
kubectl apply -k deployment/overlays/samples/cert-manager
```

## Worker configuration

NFD-Worker supports dynamic configuration through a configuration file. The
default location is `/etc/kubernetes/node-feature-discovery/nfd-worker.conf`,
but, this can be changed by specifying the`-config` command line flag.
Configuration file is re-read whenever it is modified which makes run-time
re-configuration of nfd-worker straightforward.

Worker configuration file is read inside the container, and thus, Volumes and
VolumeMounts are needed to make your configuration available for NFD. The
preferred method is to use a ConfigMap which provides easy deployment and
re-configurability.

The provided nfd-worker deployment templates create an empty configmap and
mount it inside the nfd-worker containers. Configuration can be edited with:

```bash
kubectl -n ${NFD_NS} edit configmap nfd-worker-conf
```

See
[nfd-worker configuration file reference](../advanced/worker-configuration-reference.md)
for more details.
The (empty-by-default)
[example config](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/nfd-worker.conf.example)
contains all available configuration options and can be used as a reference
for creating creating a configuration.

Configuration options can also be specified via the `-options` command line
flag, in which case no mounts need to be used. The same format as in the config
file must be used, i.e. JSON (or YAML). For example:

```bash
-options='{"sources": { "pci": { "deviceClassWhitelist": ["12"] } } }'
```

Configuration options specified from the command line will override those read
from the config file.

## Using node labels

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

For more details on targeting nodes, see
[node selection](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

## Uninstallation

### Operator was used for deployment

If you followed the deployment instructions above you can simply do:

```bash
kubectl -n nfd delete NodeFeatureDiscovery my-nfd-deployment
```

Optionally, you can also remove the namespace:

```bash
kubectl delete ns nfd
```

See the [node-feature-discovery-operator][nfd-operator] and [OLM][OLM] project
documentation for instructions for uninstalling the operator and operator
lifecycle manager, respectively.

### Manual

Simplest way is to invoke `kubectl delete` on the deployment files you used.
Beware that this will also delete the namespace that NFD is running in. For
example, in case the default deployment from the repo was used:

```bash

kubectl delete -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref={{ site.release }}
```

Alternatively you can delete create objects one-by-one, depending on the type
of deployment, for example:

```bash
NFD_NS=node-feature-discovery
kubectl -n $NFD_NS delete ds nfd-worker
kubectl -n $NFD_NS delete deploy nfd-master
kubectl -n $NFD_NS delete svc nfd-master
kubectl -n $NFD_NS delete sa nfd-master
kubectl delete clusterrole nfd-master
kubectl delete clusterrolebinding nfd-master
```

### Removing feature labels

NFD-Master has a special `-prune` command line flag for removing all
nfd-related node labels, annotations and extended resources from the cluster.

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/prune?ref={{ site.release }}
kubectl -n node-feature-discovery wait job.batch/nfd-prune --for=condition=complete && \
    kubectl delete -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/prune?ref={{ site.release }}
```

**NOTE:** You must run prune before removing the RBAC rules (serviceaccount,
clusterrole and clusterrolebinding).

<!-- Links -->
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[nfd-operator]: https://github.com/kubernetes-sigs/node-feature-discovery-operator
[OLM]: https://github.com/operator-framework/operator-lifecycle-manager
