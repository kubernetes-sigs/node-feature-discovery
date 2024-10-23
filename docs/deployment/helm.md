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

## Upgrading the chart

To upgrade the `node-feature-discovery` deployment to {{ site.release }} via Helm.

### From v0.7 and older

Please see
the [uninstallation guide](https://kubernetes-sigs.github.io/node-feature-discovery/v0.7/get-started/deployment-and-usage.html#uninstallation).
And then follow the standard [deployment instructions](#deployment).

### From v0.8 - v0.11

Helm deployment of NFD was introduced in v0.8.0.

```bash
export NFD_NS=node-feature-discovery
# Uninstall the old NFD deployment
helm uninstall node-feature-discovery --namespace $NFD_NS
# Update Helm repository
helm repo update
# Install the new NFD deployment
helm upgrade --install node-feature-discovery nfd/node-feature-discovery --namespace $NFD_NS --set master.enable=false
# Wait for NFD Worker to be ready
kubectl wait --timeout=-1s --for=condition=ready pod -l app.kubernetes.io/name=node-feature-discovery --namespace $NFD_NS
# Enable the NFD Master
helm upgrade --install node-feature-discovery nfd/node-feature-discovery --namespace $NFD_NS --set master.enable=true
```

### From v0.12 - v0.13

In v0.12 the `NodeFeature` CRD was introduced as experimental.
The API was not enabled by default.

```bash
export NFD_NS=node-feature-discovery
# Update Helm repository
helm repo update
# Install and upgrade CRD's
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/master/deployment/base/nfd-crds/nfd-api-crds.yaml
# Install the new NFD deployment
helm upgrade node-feature-discovery nfd/node-feature-discovery --namespace $NFD_NS --set master.enable=false
# Wait for NFD Worker to be ready
kubectl wait --timeout=-1s --for=condition=ready pod -l app.kubernetes.io/name=node-feature-discovery --namespace $NFD_NS
# Enable the NFD Master
helm upgrade node-feature-discovery nfd/node-feature-discovery --namespace $NFD_NS --set master.enable=true
```

### From v0.14+

As of version v0.14 the Helm chart is the primary deployment method for NFD,
and the CRD `NodeFeature` is enabled by default.

```bash
export NFD_NS=node-feature-discovery
# Update Helm repository
helm repo update
# Install and upgrade CRD's
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/deployment/base/nfd-crds/nfd-api-crds.yaml
# Install the new NFD deployment
helm upgrade node-feature-discovery nfd/node-feature-discovery --namespace $NFD_NS
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

| Name                                                | Type   | Default                                             | Description                                                                                                                                                                                                                                                                         |
|-----------------------------------------------------|--------|-----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `image.repository`                                  | string | `{{ site.container_image \| split: ":" \| first }}` | NFD image repository                                                                                                                                                                                                                                                                |
| `image.tag`                                         | string | `{{ site.release }}`                                | NFD image tag                                                                                                                                                                                                                                                                       |
| `image.pullPolicy`                                  | string | `Always`                                            | Image pull policy                                                                                                                                                                                                                                                                   |
| `imagePullSecrets`                                  | array  | []                                                  | ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec. [More info](https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod).                             |
| `nameOverride`                                      | string |                                                     | Override the name of the chart                                                                                                                                                                                                                                                      |
| `fullnameOverride`                                  | string |                                                     | Override a default fully qualified app name                                                                                                                                                                                                                                         |
| `featureGates.NodeFeatureGroupAPI`                  | bool   | false                                               | Enable the [NodeFeatureGroup](../usage/custom-resources.md#nodefeaturegroup) CRD API.                                                                                                                                                                                               |
| `featureGates.DisableAutoPrefix`                    | bool   | false                                               | Enable [DisableAutoPrefix](../reference/feature-gates.md#disableautoprefix) feature gate. Disables automatic prefixing of unprefixed labels, annotations and extended resources.                                                                                                    |
| `prometheus.enable`                                 | bool   | false                                               | Specifies whether to expose metrics using prometheus operator                                                                                                                                                                                                                       |
| `prometheus.labels`                                 | dict   | {}                                                  | Specifies labels for use with the prometheus operator to control how it is selected                                                                                                                                                                                                 |
| `prometheus.scrapeInterval`                         | string | 10s                                                 | Specifies the interval by which metrics are scraped                                                                                                                                                                                                                                 |
| `priorityClassName`                                 | string |                                                     | The name of the PriorityClass to be used for the NFD pods.                                                                                                                                                                                                                          |

Metrics are configured to be exposed using prometheus operator API's by
default. If you want to expose metrics using the prometheus operator
API's you need to install the prometheus operator in your cluster.

### Master pod parameters

| Name                                        | Type    | Default                          | Description                                                                                                                                                                                           |
|---------------------------------------------|---------|----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `master.*`                                  | dict    |                                  | NFD master deployment configuration                                                                                                                                                                   |
| `master.enable`                             | bool    | true                             | Specifies whether nfd-master should be deployed                                                                                                                                                       |
| `master.hostNetwork`                        | bool    | false                            | Specifies whether to enable or disable running the container in the host's network namespace                                                                                                          |
| `master.metricsPort`                        | integer | 8081                             | Port on which to expose metrics from components to prometheus operator. **DEPRECATED**: will be replaced by `master.port` in NFD v0.18.                                                               |
| `master.healthPort`                         | integer | 8082                             | Port on which to expose the grpc health endpoint, will be also used for the probes. **DEPRECATED**: will be replaced by `master.port` in NFD v0.18.                                                   |
| `master.instance`                           | string  |                                  | Instance name. Used to separate annotation namespaces for multiple parallel deployments                                                                                                               |
| `master.resyncPeriod`                       | string  |                                  | NFD API controller resync period.                                                                                                                                                                     |
| `master.extraLabelNs`                       | array   | []                               | List of allowed extra label namespaces                                                                                                                                                                |
| `master.enableTaints`                       | bool    | false                            | Specifies whether to enable or disable node tainting                                                                                                                                                  |
| `master.replicaCount`                       | integer | 1                                | Number of desired pods. This is a pointer to distinguish between explicit zero and not specified                                                                                                      |
| `master.podSecurityContext`                 | dict    | {}                               | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container settings |
| `master.securityContext`                    | dict    | {}                               | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                                    |
| `master.serviceAccount.create`              | bool    | true                             | Specifies whether a service account should be created                                                                                                                                                 |
| `master.serviceAccount.annotations`         | dict    | {}                               | Annotations to add to the service account                                                                                                                                                             |
| `master.serviceAccount.name`                | string  |                                  | The name of the service account to use. If not set and create is true, a name is generated using the fullname template                                                                                |
| `master.rbac.create`                        | bool    | true                             | Specifies whether to create [RBAC][rbac] configuration for nfd-master                                                                                                                                 |
| `master.resources.limits`                   | dict    | {memory: 4Gi}                    | NFD master pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                                 |
| `master.resources.requests`                 | dict    | {cpu: 100m, memory: 128Mi}       | NFD master pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits). See `[0]` for more info                                      |
| `master.tolerations`                        | dict    | _Schedule to control-plane node_ | NFD master pod [tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                                           |
| `master.annotations`                        | dict    | {}                               | NFD master pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                          |
| `master.affinity`                           | dict    |                                  | NFD master pod required [node affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)                                                              |
| `master.deploymentAnnotations`              | dict    | {}                               | NFD master deployment [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                   |
| `master.nfdApiParallelism`                  | integer | 10                               | Specifies the maximum number of concurrent node updates.                                                                                                                                              |
| `master.config`                             | dict    |                                  | NFD master [configuration](../reference/master-configuration-reference)                                                                                                                               |
| `master.extraArgs`                          | array   | []                               | Additional [command line arguments](../reference/master-commandline-reference.md) to pass to nfd-master                                                                                               |
| `master.extraEnvs`                          | array   | []                               | Additional environment variables to pass to nfd-master                                                                                                                                                |
| `master.revisionHistoryLimit`               | integer |                                  | Specify how many old ReplicaSets for this Deployment you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit)         |
| `master.startupProbe.initialDelaySecond s`  | integer | 0 (by Kubernetes)                | Specifies the number of seconds after the container has started before startup probes are initiated.                                                                                                  |
| `master.startupProbe.failureThreshold`      | integer | 30                               | Specifies the number of consecutive failures of startup probes before considering the pod as not ready.                                                                                               |
| `master.startupProbe.periodSeconds`         | integer | 10 (by Kubernetes)               | Specifies how often (in seconds) to perform the startup probe.                                                                                                                                        |
| `master.startupProbe.timeoutSeconds`        | integer | 1 (by Kubernetes)                | Specifies the number of seconds after which the probe times out.                                                                                                                                      |
| `master.livenessProbe.initialDelaySeconds`  | integer | 0 (by Kubernetes)                | Specifies the number of seconds after the container has started before liveness probes are initiated.                                                                                                 |
| `master.livenessProbe.failureThreshold`     | integer | 3 (by Kubernetes)                | Specifies the number of consecutive failures of liveness probes before considering the pod as not ready.                                                                                              |
| `master.livenessProbe.periodSeconds`        | integer | 10 (by Kubernetes)               | Specifies how often (in seconds) to perform the liveness probe.                                                                                                                                       |
| `master.livenessProbe.timeoutSeconds`       | integer | 1 (by Kubernetes)                | Specifies the number of seconds after which the probe times out.                                                                                                                                      |
| `master.readinessProbe.initialDelaySeconds` | integer | 0 (by Kubernetes)                | Specifies the number of seconds after the container has started before readiness probes are initiated.                                                                                                |
| `master.readinessProbe.failureThreshold`    | integer | 10                               | Specifies the number of consecutive failures of readiness probes before considering the pod as not ready.                                                                                             |
| `master.readinessProbe.periodSeconds`       | integer | 10 (by Kubernetes)               | Specifies how often (in seconds) to perform the readiness probe.                                                                                                                                      |
| `master.readinessProbe.timeoutSeconds`      | integer | 1 (by Kubernetes)                | Specifies the number of seconds after which the probe times out.                                                                                                                                      |
| `master.readinessProbe.successThreshold`    | integer | 1 (by Kubernetes)                | Specifies the number of consecutive successes of readiness probes before considering the pod as ready.                                                                                                |
| `master.dnsPolicy`                          | array   | ClusterFirstWithHostNet          | NFD master pod [dnsPolicy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy)                                                                                 |

> `[0]` Additional info for `master.resources.requests`: \
> You may want to use the same value for `requests.memory` and `limits.memory`.
> The “requests” value affects scheduling to accommodate pods on nodes.
> If there is a large difference between “requests” and “limits” and nodes
> experience memory pressure, the kernel may invoke the OOM Killer, even if
> the memory does not exceed the “limits” threshold.
> This can cause unexpected pod evictions. Memory cannot be compressed and
> once allocated to a pod, it can only be reclaimed by killing the pod.
> [Natan Yellin 22/09/2022](https://home.robusta.dev/blog/kubernetes-memory-limit)
> that discusses this issue.

### Worker pod parameters

| Name                                        | Type    | Default                 | Description                                                                                                                                                                                                  |
|---------------------------------------------|---------|-------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `worker.*`                                  | dict    |                         | NFD worker daemonset configuration                                                                                                                                                                           |
| `worker.enable`                             | bool    | true                    | Specifies whether nfd-worker should be deployed                                                                                                                                                              |
| `worker.hostNetwork`                        | bool    | false                   | Specifies whether to enable or disable running the container in the host's network namespace                                                                                                                 |
| `worker.port`                               | int     | 8080                    | Port on which to serve http for metrics and healthz endpoints.                                                                                                                                               |
| `worker.config`                             | dict    |                         | NFD worker [configuration](../reference/worker-configuration-reference)                                                                                                                                      |
| `worker.podSecurityContext`                 | dict    | {}                      | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container settins         |
| `worker.securityContext`                    | dict    | {}                      | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                                           |
| `worker.serviceAccount.create`              | bool    | true                    | Specifies whether a service account for nfd-worker should be created                                                                                                                                         |
| `worker.serviceAccount.annotations`         | dict    | {}                      | Annotations to add to the service account for nfd-worker                                                                                                                                                     |
| `worker.serviceAccount.name`                | string  |                         | The name of the service account to use for nfd-worker. If not set and create is true, a name is generated using the fullname template (suffixed with `-worker`)                                              |
| `worker.rbac.create`                        | bool    | true                    | Specifies whether to create [RBAC][rbac] configuration for nfd-worker                                                                                                                                        |
| `worker.mountUsrSrc`                        | bool    | false                   | Specifies whether to allow users to mount the hostpath /user/src. Does not work on systems without /usr/src AND a read-only /usr                                                                             |
| `worker.resources.limits`                   | dict    | {memory: 512Mi}         | NFD worker pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                                        |
| `worker.resources.requests`                 | dict    | {cpu: 5m, memory: 64Mi} | NFD worker pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                                      |
| `worker.nodeSelector`                       | dict    | {}                      | NFD worker pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)                                                                                        |
| `worker.tolerations`                        | dict    | {}                      | NFD worker pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                                             |
| `worker.priorityClassName`                  | string  |                         | NFD worker pod [priority class](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/)                                                                                            |
| `worker.annotations`                        | dict    | {}                      | NFD worker pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                                 |
| `worker.daemonsetAnnotations`               | dict    | {}                      | NFD worker daemonset [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                           |
| `worker.extraArgs`                          | array   | []                      | Additional [command line arguments](../reference/worker-commandline-reference.md) to pass to nfd-worker                                                                                                      |
| `worker.extraEnvs`                          | array   | []                      | Additional environment variables to pass to nfd-worker                                                                                                                                                       |
| `worker.revisionHistoryLimit`               | integer |                         | Specify how many old ControllerRevisions for this DaemonSet you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/ #DaemonSetSpec) |
| `worker.livenessProbe.initialDelaySeconds`  | integer | 10                      | Specifies the number of seconds after the container has started before liveness probes are initiated.                                                                                                        |
| `worker.livenessProbe.failureThreshold`     | integer | 3 (by Kubernetes)       | Specifies the number of consecutive failures of liveness probes before considering the pod as not ready.                                                                                                     |
| `worker.livenessProbe.periodSeconds`        | integer | 10 (by Kubernetes)      | Specifies how often (in seconds) to perform the liveness probe.                                                                                                                                              |
| `worker.livenessProbe.timeoutSeconds`       | integer | 1 (by Kubernetes)       | Specifies the number of seconds after which the probe times out.                                                                                                                                             |
| `worker.readinessProbe.initialDelaySeconds` | integer | 5                       | Specifies the number of seconds after the container has started before readiness probes are initiated.                                                                                                       |
| `worker.readinessProbe.failureThreshold`    | integer | 10                      | Specifies the number of consecutive failures of readiness probes before considering the pod as not ready.                                                                                                    |
| `worker.readinessProbe.periodSeconds`       | integer | 10 (by Kubernetes)      | Specifies how often (in seconds) to perform the readiness probe.                                                                                                                                             |
| `worker.readinessProbe.timeoutSeconds`      | integer | 1 (by Kubernetes)       | Specifies the number of seconds after which the probe times out.                                                                                                                                             |
| `worker.readinessProbe.successThreshold`    | integer | 1 (by Kubernetes)       | Specifies the number of consecutive successes of readiness probes before considering the pod as ready.                                                                                                       |
| `worker.dnsPolicy`                          | array   | ClusterFirstWithHostNet | NFD worker pod [dnsPolicy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy)                                                                                        |

### Topology updater parameters

| Name                                                 | Type    | Default                  | Description                                                                                                                                                                                                 |
|------------------------------------------------------|---------|--------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `topologyUpdater.*`                                  | dict    |                          | NFD Topology Updater configuration                                                                                                                                                                          |
| `topologyUpdater.enable`                             | bool    | false                    | Specifies whether the NFD Topology Updater should be created                                                                                                                                                |
| `topologyUpdater.hostNetwork`                        | bool    | false                    | Specifies whether to enable or disable running the container in the host's network namespace                                                                                                                |
| `topologyUpdater.createCRDs`                         | bool    | false                    | Specifies whether the NFD Topology Updater CRDs should be created                                                                                                                                           |
| `topologyUpdater.serviceAccount.create`              | bool    | true                     | Specifies whether the service account for topology updater should be created                                                                                                                                |
| `topologyUpdater.serviceAccount.annotations`         | dict    | {}                       | Annotations to add to the service account for topology updater                                                                                                                                              |
| `topologyUpdater.serviceAccount.name`                | string  |                          | The name of the service account for topology updater to use. If not set and create is true, a name is generated using the fullname template and `-topology-updater` suffix                                  |
| `topologyUpdater.rbac.create`                        | bool    | true                     | Specifies whether to create [RBAC][rbac] configuration for topology updater                                                                                                                                 |
| `topologyUpdater.metricsPort`                        | integer | 8081                     | Port on which to expose prometheus metrics. **DEPRECATED**: will be replaced by `topologyUpdater.port` in NFD v0.18.                                                                                        |
| `topologyUpdater.healthPort`                         | integer | 8082                     | Port on which to expose the grpc health endpoint, will be also used for the probes. **DEPRECATED**: will be replaced by `topologyUpdater.port` in NFD v0.18.                                                |
| `topologyUpdater.kubeletConfigPath`                  | string  | ""                       | Specifies the kubelet config host path                                                                                                                                                                      |
| `topologyUpdater.kubeletPodResourcesSockPath`        | string  | ""                       | Specifies the kubelet sock path to read pod resources                                                                                                                                                       |
| `topologyUpdater.updateInterval`                     | string  | 60s                      | Time to sleep between CR updates. Non-positive value implies no CR update.                                                                                                                                  |
| `topologyUpdater.watchNamespace`                     | string  | `*`                      | Namespace to watch pods, `*` for all namespaces                                                                                                                                                             |
| `topologyUpdater.podSecurityContext`                 | dict    | {}                       | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container sett           |
| `topologyUpdater.securityContext`                    | dict    | {}                       | Container [security settings](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                                          |
| `topologyUpdater.resources.limits`                   | dict    | {memory: 60Mi}           | NFD Topology Updater pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                             |
| `topologyUpdater.resources.requests`                 | dict    | {cpu: 50m, memory: 40Mi} | NFD Topology Updater pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                           |
| `topologyUpdater.nodeSelector`                       | dict    | {}                       | Topology updater pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)                                                                                 |
| `topologyUpdater.tolerations`                        | dict    | {}                       | Topology updater pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                                      |
| `topologyUpdater.annotations`                        | dict    | {}                       | Topology updater pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                          |
| `topologyUpdater.daemonsetAnnotations`               | dict    | {}                       | Topology updater daemonset [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                    |
| `topologyUpdater.affinity`                           | dict    | {}                       | Topology updater pod [affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)                                                                            |
| `topologyUpdater.config`                             | dict    |                          | [configuration](../reference/topology-updater-configuration-reference)                                                                                                                                      |
| `topologyUpdater.podSetFingerprint`                  | bool    | true                     | Enables compute and report of pod fingerprint in NRT objects.                                                                                                                                               |
| `topologyUpdater.kubeletStateDir`                    | string  | /var/lib/kubelet         | Specifies kubelet state directory path for watching state and checkpoint files. Empty value disables kubelet state tracking.                                                                                |
| `topologyUpdater.extraArgs`                          | array   | []                       | Additional [command line arguments](../reference/topology-updater-commandline-reference.md) to pass to nfd-topology-updater                                                                                 |
| `topologyUpdater.extraEnvs`                          | array   | []                       | Additional environment variables to pass to nfd-topology-updater                                                                                                                                            |
| `topologyUpdater.revisionHistoryLimit`               | integer |                          | Specify how many old ControllerRevisions for this DaemonSet you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec) |
| `topologyUpdater.livenessProbe.initialDelaySeconds`  | integer | 10                       | Specifies the number of seconds after the container has started before liveness probes are initiated.                                                                                                       |
| `topologyUpdater.livenessProbe.failureThreshold`     | integer | 3 (by Kubernetes)        | Specifies the number of consecutive failures of liveness probes before considering the pod as not ready.                                                                                                    |
| `topologyUpdater.livenessProbe.periodSeconds`        | integer | 10 (by Kubernetes)       | Specifies how often (in seconds) to perform the liveness probe.                                                                                                                                             |
| `topologyUpdater.livenessProbe.timeoutSeconds`       | integer | 1 (by Kubernetes)        | Specifies the number of seconds after which the probe times out.                                                                                                                                            |
| `topologyUpdater.readinessProbe.initialDelaySeconds` | integer | 5                        | Specifies the number of seconds after the container has started before readiness probes are initiated.                                                                                                      |
| `topologyUpdater.readinessProbe.failureThreshold`    | integer | 10                       | Specifies the number of consecutive failures of readiness probes before considering the pod as not ready.                                                                                                   |
| `topologyUpdater.readinessProbe.periodSeconds`       | integer | 10 (by Kubernetes)       | Specifies how often (in seconds) to perform the readiness probe.                                                                                                                                            |
| `topologyUpdater.readinessProbe.timeoutSeconds`      | integer | 1 (by Kubernetes)        | Specifies the number of seconds after which the probe times out.                                                                                                                                            |
| `topologyUpdater.readinessProbe.successThreshold`    | integer | 1 (by Kubernetes)        | Specifies the number of consecutive successes of readiness probes before considering the pod as ready.                                                                                                      |
| `topologyUpdater.dnsPolicy`                          | array   | ClusterFirstWithHostNet  | Topology updater pod [dnsPolicy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy)                                                                                 |

### Garbage collector parameters

| Name                            | Type    | Default                   | Description                                                                                                                                                                                           |
|---------------------------------|---------|---------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gc.*`                          | dict    |                           | NFD Garbage Collector configuration                                                                                                                                                                   |
| `gc.enable`                     | bool    | true                      | Specifies whether the NFD Garbage Collector should be created                                                                                                                                         |
| `gc.hostNetwork`                | bool    | false                     | Specifies whether to enable or disable running the container in the host's network namespace                                                                                                          |
| `gc.serviceAccount.create`      | bool    | true                      | Specifies whether the service account for garbage collector should be created                                                                                                                         |
| `gc.serviceAccount.annotations` | dict    | {}                        | Annotations to add to the service account for garbage collector                                                                                                                                       |
| `gc.serviceAccount.name`        | string  |                           | The name of the service account for garbage collector to use. If not set and create is true, a name is generated using the fullname template and `-gc` suffix                                         |
| `gc.rbac.create`                | bool    | true                      | Specifies whether to create [RBAC][rbac] configuration for garbage collector                                                                                                                          |
| `gc.interval`                   | string  | 1h                        | Time between periodic garbage collector runs                                                                                                                                                          |
| `gc.podSecurityContext`         | dict    | {}                        | [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod) holds pod-level security attributes and common container settings |
| `gc.resources.limits`           | dict    | {memory: 1Gi}             | NFD Garbage Collector pod [resources limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                      |
| `gc.resources.requests`         | dict    | {cpu: 10m, memory: 128Mi} | NFD Garbage Collector pod [resources requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                    |
| `gc.metricsPort`                | integer | 8081                      | Port on which to serve Prometheus metrics. **DEPRECATED**: will be replaced by `gc.port` in NFD v0.18.                                                                                                |
| `gc.nodeSelector`               | dict    | {}                        | Garbage collector pod [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)                                                                          |
| `gc.tolerations`                | dict    | {}                        | Garbage collector pod [node tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)                                                                               |
| `gc.annotations`                | dict    | {}                        | Garbage collector pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                                   |
| `gc.deploymentAnnotations`      | dict    | {}                        | Garbage collector deployment [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)                                                                            |
| `gc.affinity`                   | dict    | {}                        | Garbage collector pod [affinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)                                                                     |
| `gc.extraArgs`                  | array   | []                        | Additional [command line arguments](../reference/gc-commandline-reference.md) to pass to nfd-gc                                                                                                       |
| `gc.extraEnvs`                  | array   | []                        | Additional environment variables to pass to nfd-gc                                                                                                                                                    |
| `gc.revisionHistoryLimit`       | integer |                           | Specify how many old ReplicaSets for this Deployment you want to retain. [revisionHistoryLimit](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit)         |
| `gc.dnsPolicy`                  | array   | ClusterFirstWithHostNet   | Garbage collector pod [dnsPolicy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy)                                                                          |

<!-- Links -->

[rbac]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
