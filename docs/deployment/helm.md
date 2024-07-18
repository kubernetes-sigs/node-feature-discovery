---
title: "Helm"
layout: default
sort: 3
---

# Deployment with Helm
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

Node Feature Discovery provides a Helm chart to manage its deployment.

> **NOTE:** NFD is not ideal for other Helm charts to depend on as that may
> result in multiple parallel NFD deployments in the same cluster which is not
> fully supported by the NFD Helm chart.

## Prerequisites

[Helm package manager](https://helm.sh/) should be installed.

## Deployment

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

## Configuration

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

## Uninstalling the chart

To uninstall the `node-feature-discovery` deployment:

```bash
export NFD_NS=node-feature-discovery
helm uninstall node-feature-discovery --namespace $NFD_NS
```

The command removes all the Kubernetes components associated with the chart and
deletes the release. It also runs a post-delete hook that cleans up the nodes
of all labels, annotations, taints and extended resources that were created by
NFD.

## Chart parameters

To tailor the deployment of the Node Feature Discovery to your needs following
Chart parameters are available.

### General parameters

| Name                                                | Type   | Default                                             | Description                                                                                                                                                                                                  |
| --------------------------------------------------- | ------ | --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `image.repository`                                  | string | `{{ site.container_image \| split: ":" \| first }}` | NFD image repository                                                                                                                                                                                         |
| `image.tag`                                         | string | `{{ site.release }}`                                | NFD image tag                                                                                                                                                                                                |
| `image.pullPolicy`                                  | string | `Always`                                            | Image pull policy                                                                                                                                                                                            |
| `imagePullSecrets`                                  | array  | []                                                  | ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec. If specified, these secrets will be passed to individual puller implementations for them to use. For example, in the case of docker, only DockerConfig type secrets are honored. [More info](https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod). |
| `nameOverride`                                      | string |                                                     | Override the name of the chart                                                                                                                                                                               |
| `fullnameOverride`                                  | string |                                                     | Override a default fully qualified app name                                                                                                                                                                  |
| `tls.enable`                                        | bool   | false                                               | Specifies whether to use TLS for communications between components. **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release.                          |
| `tls.certManager`                                   | bool   | false                                               | If enabled, requires [cert-manager](https://cert-manager.io/docs/) to be installed and will automatically create the required TLS certificates. **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release |
| `tls.certManager.certManagerCertificate.issuerName` | string |                                                     | If specified, it will use a pre-existing issuer instead for the required TLS certificates. **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release.   |
| `tls.certManager.certManagerCertificate.issuerKind` | string |                                                     | Specifies on what kind of issuer is used, can be either ClusterIssuer or Issuer (default).  Requires `tls.certManager.certManagerCertificate.issuerName` to be set.  **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release |
| `featureGates.NodeFeatureAPI`                       | bool   | true                                                | Enable the [NodeFeature](../usage/custom-resources.md#nodefeature) CRD API for communicating node features. This will automatically disable the gRPC communication.                                          |
| `featureGates.NodeFeatureGroupAPI`                  | bool   | false                                               | Enable the [NodeFeatureGroup](../usage/custom-resources.md#nodefeaturegroup) CRD API.                                                                                                                        |
| `featureGates.DisableAutoPrefix`                    | bool   | false                                               | Enable [DisableAutoPrefix](../reference/feature-gates.md#disableautoprefix) feature gate. Disables automatic prefixing of unprefixed labels, annotations and extended resources.                             |
| `prometheus.enable`                                 | bool   | false                                               | Specifies whether to expose metrics using prometheus operator                                                                                                                                                |
| `prometheus.labels`                                 | dict   | {}                                                  | Specifies labels for use with the prometheus operator to control how it is selected                                                                                                                          |
| `prometheus.scrapeInterval`                         | string | 10s                                                 | Specifies the interval by which metrics are scraped                                                                                                                                                          |
| `priorityClassName`                                 | string |                                                     | The name of the PriorityClass to be used for the NFD pods.                                                                                                                                                   |

Metrics are configured to be exposed using prometheus operator API's by
default. If you want to expose metrics using the prometheus operator
API's you need to install the prometheus operator in your cluster.

### Master pod parameters

| Name                                | Type    | Default                          | Description                                                                                                                                                                                            |
| ----------------------------------- | ------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `master.*`                          | dict    |                                  | NFD master deployment configuration                                                                                                                                                                    |
| `master.enable`                     | bool    | true                             | Specifies whether nfd-master should be deployed                                                                                                                                                        |
| `master.port`                       | integer |                                  | Specifies the TCP port that nfd-master listens for incoming requests. **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release                   |
| `master.metricsPort`                | integer | 8081                             | Port on which to expose metrics from components to prometheus operator                                                                                                                                 |
| `master.instance`                   | string  |                                  | Instance name. Used to separate annotation namespaces for multiple parallel deployments                                                                                                                |
| `master.resyncPeriod`               | string  |                                  | NFD API controller resync period.                                                                                                                                                                      |
| `master.extraLabelNs`               | array   | []                               | List of allowed extra label namespaces                                                                                                                                                                 |
| `master.resourceLabels`             | array   | []                               | List of labels to be registered as extended resources                                                                                                                                                  |
| `master.enableTaints`               | bool    | false                            | Specifies whether to enable or disable node tainting                                                                                                                                                   |
| `master.crdController`              | bool    | null                             | Specifies whether the NFD CRD API controller is enabled. If not set, controller will be enabled if `master.instance` is empty.                                                                         |
| `master.featureRulesController`     | bool    | null                             | DEPRECATED: use `master.crdController` instead                                                                                                                                                         |
| `master.replicaCount`               | integer | 1                                | Number of desired pods. This is a pointer to distinguish between explicit zero and not specified                                                                                                       |
| `master.podSecurityContext`         | dict    | {}                               | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container settings  |
| `master.securityContext`            | dict    | {}                               | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                                     |
| `master.serviceAccount.create`      | bool    | true                             | Specifies whether a service account should be created                                                                                                                                                  |
| `master.serviceAccount.annotations` | dict    | {}                               | Annotations to add to the service account                                                                                                                                                              |
| `master.serviceAccount.name`        | string  |                                  | The name of the service account to use. If not set and create is true, a name is generated using the fullname template                                                                                 |
| `master.rbac.create`                | bool    | true                             | Specifies whether to create [RBAC][rbac] configuration for nfd-master                                                                                                                                  |
| `master.service.type`               | string  | ClusterIP                        | NFD master service type. **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release                                                                |
| `master.service.port`               | integer | 8080                             | NFD master service port. **NOTE**: this parameter is related to the deprecated gRPC API and will be removed with it in a future release                                                                |
| `master.resources.limits`           | dict    | {memory: 4Gi}                    | NFD master pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                                  |
| `master.resources.requests`         | dict    | {cpu: 100m, memory: 128Mi}       | NFD master pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits). You may want to use the same value for `requests.memory` and `limits.memory`. The “requests” value affects scheduling to accommodate pods on nodes. If there is a large difference between “requests” and “limits” and nodes experience memory pressure, the kernel may invoke the OOM Killer, even if the memory does not exceed the “limits” threshold. This can cause unexpected pod evictions. Memory cannot be compressed and once allocated to a pod, it can only be reclaimed by killing the pod.  [Natan Yellin 22/09/2022](https://home.robusta.dev/blog/kubernetes-memory-limit) that discusses this issue. |
| `master.tolerations`                | dict    | _Schedule to control-plane node_ | NFD master pod [tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                                            |
| `master.annotations`                | dict    | {}                               | NFD master pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                           |
| `master.affinity`                   | dict    |                                  | NFD master pod required [node affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)                                                               |
| `master.deploymentAnnotations`      | dict    | {}                               | NFD master deployment [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                    |
| `master.nfdApiParallelism`          | integer | 10                               | Specifies the maximum number of concurrent node updates.                                                                                                                                               |
| `master.config`                     | dict    |                                  | NFD master [configuration](../reference/master-configuration-reference)                                                                                                                                |
| `master.args`                       | array   | []                               | Additional [command line arguments](../reference/master-commandline-reference.md) to pass to nfd-master                                                                                                |
| `master.revisionHistoryLimit`       | integer |                                  | Specify how many old ReplicaSets for this Deployment you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit)          |

### Worker pod parameters

| Name                                | Type   | Default                 | Description                                                                                                                                                                                          |
| ----------------------------------- | ------ | ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `worker.*`                          | dict   |                         | NFD worker daemonset configuration                                                                                                                                                                   |
| `worker.enable`                     | bool   | true                    | Specifies whether nfd-worker should be deployed                                                                                                                                                      |
| `worker.metricsPort*`               | int    | 8081                    | Port on which to expose metrics from components to prometheus operator                                                                                                                               |
| `worker.config`                     | dict   |                         | NFD worker [configuration](../reference/worker-configuration-reference)                                                                                                                              |
| `worker.podSecurityContext`         | dict   | {}                      | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container settins |
| `worker.securityContext`            | dict   | {}                      | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                                   |
| `worker.serviceAccount.create`      | bool   | true                    | Specifies whether a service account for nfd-worker should be created                                                                                                                                 |
| `worker.serviceAccount.annotations` | dict   | {}                      | Annotations to add to the service account for nfd-worker                                                                                                                                             |
| `worker.serviceAccount.name`        | string |                         | The name of the service account to use for nfd-worker. If not set and create is true, a name is generated using the fullname template (suffixed with `-worker`)                                      |
| `worker.rbac.create`                | bool   | true                    | Specifies whether to create [RBAC][rbac] configuration for nfd-worker                                                                                                                                |
| `worker.mountUsrSrc`                | bool   | false                   | Specifies whether to allow users to mount the hostpath /user/src. Does not work on systems without /usr/src AND a read-only /usr                                                                     |
| `worker.resources.limits`           | dict   | {memory: 512Mi}         | NFD worker pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                                |
| `worker.resources.requests`         | dict   | {cpu: 5m, memory: 64Mi} | NFD worker pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                              |
| `worker.nodeSelector`               | dict   | {}                      | NFD worker pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)                                                                                |
| `worker.tolerations`                | dict   | {}                      | NFD worker pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                                     |
| `worker.priorityClassName`          | string |                         | NFD worker pod [priority class](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/)                                                                                    |
| `worker.annotations`                | dict   | {}                      | NFD worker pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                         |
| `worker.daemonsetAnnotations`       | dict   | {}                      | NFD worker daemonset [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                   |
| `worker.args`                       | array  | []                      | Additional [command line arguments](../reference/worker-commandline-reference.md) to pass to nfd-worker                                                                                              |
| `worker.revisionHistoryLimit`       | integer |                        | Specify how many old ControllerRevisions for this DaemonSet you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec)          |

### Topology updater parameters

| Name                                          | Type    | Default                  | Description                                                                                                                                                                                      |
| --------------------------------------------- | ------- | ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `topologyUpdater.*`                           | dict    |                          | NFD Topology Updater configuration                                                                                                                                                               |
| `topologyUpdater.enable`                      | bool    | false                    | Specifies whether the NFD Topology Updater should be created                                                                                                                                     |
| `topologyUpdater.createCRDs`                  | bool    | false                    | Specifies whether the NFD Topology Updater CRDs should be created                                                                                                                                |
| `topologyUpdater.serviceAccount.create`       | bool    | true                     | Specifies whether the service account for topology updater should be created                                                                                                                     |
| `topologyUpdater.serviceAccount.annotations`  | dict    | {}                       | Annotations to add to the service account for topology updater                                                                                                                                   |
| `topologyUpdater.serviceAccount.name`         | string  |                          | The name of the service account for topology updater to use. If not set and create is true, a name is generated using the fullname template and `-topology-updater` suffix                       |
| `topologyUpdater.rbac.create`                 | bool    | true                     | Specifies whether to create [RBAC][rbac] configuration for topology updater                                                                                                                      |
| `topologyUpdater.metricsPort`                 | integer | 8081                     | Port on which to expose prometheus metrics                                                                                                                                                       |
| `topologyUpdater.kubeletConfigPath`           | string  | ""                       | Specifies the kubelet config host path                                                                                                                                                           |
| `topologyUpdater.kubeletPodResourcesSockPath` | string  | ""                       | Specifies the kubelet sock path to read pod resources                                                                                                                                            |
| `topologyUpdater.updateInterval`              | string  | 60s                      | Time to sleep between CR updates. Non-positive value implies no CR update.                                                                                                                       |
| `topologyUpdater.watchNamespace`              | string  | `*`                      | Namespace to watch pods, `*` for all namespaces                                                                                                                                                  |
| `topologyUpdater.podSecurityContext`          | dict    | {}                       | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container sett|
| `topologyUpdater.securityContext`             | dict    | {}                       | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                               |
| `topologyUpdater.resources.limits`            | dict    | {memory: 60Mi}           | NFD Topology Updater pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                  |
| `topologyUpdater.resources.requests`          | dict    | {cpu: 50m, memory: 40Mi} | NFD Topology Updater pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                |
| `topologyUpdater.nodeSelector`                | dict    | {}                       | Topology updater pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)                                                                      |
| `topologyUpdater.tolerations`                 | dict    | {}                       | Topology updater pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                           |
| `topologyUpdater.annotations`                 | dict    | {}                       | Topology updater pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                               |
| `topologyUpdater.daemonsetAnnotations`        | dict    | {}                       | Topology updater daemonset [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                         |
| `topologyUpdater.affinity`                    | dict    | {}                       | Topology updater pod [affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)                                                                 |
| `topologyUpdater.config`                      | dict    |                          | [configuration](../reference/topology-updater-configuration-reference)                                                                                                                           |
| `topologyUpdater.podSetFingerprint`           | bool    | true                     | Enables compute and report of pod fingerprint in NRT objects.                                                                                                                                    |
| `topologyUpdater.kubeletStateDir`             | string  | /var/lib/kubelet         | Specifies kubelet state directory path for watching state and checkpoint files. Empty value disables kubelet state tracking.                                                                     |
| `topologyUpdater.args`                        | array   | []                       | Additional [command line arguments](../reference/topology-updater-commandline-reference.md) to pass to nfd-topology-updater                                                                      |
| `topologyUpdater.revisionHistoryLimit`       | integer |                           | Specify how many old ControllerRevisions for this DaemonSet you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec)          |

### Garbage collector parameters

| Name                                  | Type    | Default                   | Description                                                                                                                                                                                             |
| ------------------------------------- | ------- | ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gc.*`                                | dict    |                           | NFD Garbage Collector configuration                                                                                                                                                                     |
| `gc.enable`                           | bool    | true                      | Specifies whether the NFD Garbage Collector should be created                                                                                                                                           |
| `gc.serviceAccount.create`            | bool    | true                      | Specifies whether the service account for garbage collector should be created                                                                                                                           |
| `gc.serviceAccount.annotations`       | dict    | {}                        | Annotations to add to the service account for garbage collector                                                                                                                                         |
| `gc.serviceAccount.name`              | string  |                           | The name of the service account for garbage collector to use. If not set and create is true, a name is generated using the fullname template and `-gc` suffix                                           |
| `gc.rbac.create`                      | bool    | true                      | Specifies whether to create [RBAC][rbac] configuration for garbage collector                                                                                                                            |
| `gc.interval`                         | string  | 1h                        | Time between periodic garbage collector runs                                                                                                                                                            |
| `gc.podSecurityContext`               | dict    | {}                        | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container settings   |
| `gc.resources.limits`                 | dict    | {memory: 1Gi}             | NFD Garbage Collector pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                        |
| `gc.resources.requests`               | dict    | {cpu: 10m, memory: 128Mi} | NFD Garbage Collector pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                      |
| `gc.metricsPort`                      | integer | 8081                      | Port on which to serve Prometheus metrics                                                                                                                                                               |
| `gc.nodeSelector`                     | dict    | {}                        | Garbage collector pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)                                                                            |
| `gc.tolerations`                      | dict    | {}                        | Garbage collector pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                                 |
| `gc.annotations`                      | dict    | {}                        | Garbage collector pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                     |
| `gc.deploymentAnnotations`            | dict    | {}                        | Garbage collector deployment [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                              |
| `gc.affinity`                         | dict    | {}                        | Garbage collector pod [affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)                                                                       |
| `gc.args`                             | array   | []                        | Additional [command line arguments](../reference/gc-commandline-reference.md) to pass to nfd-gc                                                                                                         |
| `gc.revisionHistoryLimit`             | integer |                           | Specify how many old ReplicaSets for this Deployment you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit)           |

<!-- Links -->
[rbac]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
