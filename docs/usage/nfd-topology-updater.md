---
title: "NFD-Topology-Updater"
layout: default
sort: 5
---

# NFD-Topology-Updater
{: .no_toc}

---

NFD-Topology-Updater is preferably run as a Kubernetes DaemonSet. This assures
re-examination on regular intervals, capturing changes in the allocated
resources and hence the allocatable resources on a per zone basis by updating
[NodeResourceTopology](custom-resources#noderesourcetopology) custom resources.
It makes sure that new NodeResourceTopology instances are created for each new
nodes that get added to the cluster.

When run as a daemonset, nodes are re-examined for the allocated resources
(to determine the information of the allocatable resources on a per zone basis
where a zone can be a NUMA node) at an interval specified using the
[`-sleep-interval`](../reference/topology-updater-commandline-reference.html#-sleep-interval)
option. The default sleep interval is set to 60s which is the value when no
-sleep-interval is specified.
In addition, it can avoid examining specific allocated resources
given a configuration of resources to exclude via [`-excludeList`](../reference/topology-updater-configuration-reference.md#excludelist)

## Deployment Notes

Kubelet [PodResource API][podresource-api] is a prerequisite for
nfd-topology-updater to be able to run.

Preceding Kubernetes v1.23, the `kubelet` must be started with
`--feature-gates=KubeletPodResourcesGetAllocatable=true`.

Starting from Kubernetes v1.23, the `KubeletPodResourcesGetAllocatable`
[feature gate][feature-gate].  is enabled by default

## Topology-Updater Configuration

NFD-Topology-Updater supports configuration through a configuration file. The
default location is `/etc/kubernetes/node-feature-discovery/topology-updater.conf`,
but, this can be changed by specifying the`-config` command line flag.
> NOTE: unlike nfd-worker,
> dynamic configuration updates are not currently supported.

Topology-Updater configuration file is read inside the container,
and thus, Volumes and VolumeMounts are needed
to make your configuration available for NFD.
The preferred method is to use a ConfigMap
which provides easy deployment and re-configurability.

The provided nfd-topology-updater deployment templates
create an empty configmap
and mount it inside the nfd-topology-updater containers.
In kustomize deployments, configuration can be edited with:

```bash
kubectl -n ${NFD_NS} edit configmap nfd-topology-updater-conf
```

In Helm deployments,
[Topology Updater parameters](../deployment/helm.md#topology-updater-parameters)
`toplogyUpdater.config` can be used to edit the respective configuration.

See
[nfd-topology-updater configuration file reference](../reference/topology-updater-configuration-reference.md)
for more details.
The (empty-by-default)
[example config](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/topology-updater-config/nfd-topology-updater.conf.example)
contains all available configuration options and can be used as a reference
for creating a configuration.

<!-- Links -->
[podresource-api]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#monitoring-device-plugin-resources
[feature-gate]: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates
