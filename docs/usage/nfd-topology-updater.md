---
title: "NFD-Topology-Updater"
parent: "Usage"
layout: default
nav_order: 5
---

# NFD-Topology-Updater
{: .no_toc}

---

NFD-Topology-Updater is preferably run as a Kubernetes DaemonSet.
This assures re-examination on regular intervals
and/or per pod life-cycle events, capturing changes in the allocated
resources and hence the allocatable resources on a per-zone basis by updating
[NodeResourceTopology](custom-resources.md#noderesourcetopology) custom resources.
It makes sure that new NodeResourceTopology instances are created for each new
nodes that get added to the cluster.

Because of the design and implementation of Kubernetes, only resources exclusively
allocated to [Guaranteed Quality of Service](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed)
pods will be accounted.
This includes
[CPU cores](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy),
[memory](https://kubernetes.io/docs/tasks/administer-cluster/memory-manager/#policy-static)
and
[devices](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/).

When run as a daemonset, nodes are re-examined for the allocated resources
(to determine the information of the allocatable resources on a per-zone basis
where a zone can be a NUMA node) at an interval specified using the
[`-sleep-interval`](../reference/topology-updater-commandline-reference.html.md#-sleep-interval)
option. The default sleep interval is set to 60s
which is the value when no -sleep-interval is specified.
The re-examination can be disabled by setting the sleep-interval to 0.

Another option is to configure the updater to update
the allocated resources per pod life-cycle events.
The updater will monitor the checkpoint file stated in
[`-kubelet-state-dir`](../reference/topology-updater-commandline-reference.md#-kubelet-state-dir)
and triggers an update for every change occurs in the files.

In addition, it can avoid examining specific allocated resources
given a configuration of resources to exclude via [`-excludeList`](../reference/topology-updater-configuration-reference.md#excludelist)

## Deployment Notes

Kubelet [PodResource API][podresource-api] with the
[GetAllocatableResources][getallocatableresources] functionality enabled is a
prerequisite for nfd-topology-updater to be able to run (i.e. Kubernetes v1.21
or later is required).

Preceding Kubernetes v1.23, the `kubelet` must be started with
`--feature-gates=KubeletPodResourcesGetAllocatable=true`.

Starting from Kubernetes v1.23, the `KubeletPodResourcesGetAllocatable`
[feature gate][feature-gate].  is enabled by default

## Topology-Updater Configuration

NFD-Topology-Updater supports configuration through a configuration file. The
default location is `/etc/kubernetes/node-feature-discovery/topology-updater.conf`,
but, this can be changed by specifying the`-config` command line flag.

Topology-Updater configuration file is read inside the container,
and thus, Volumes and VolumeMounts are needed
to make your configuration available for NFD.
The preferred method is to use a ConfigMap
which provides easy deployment and re-configurability.

The provided deployment templates create an empty configmap
and mount it inside the nfd-topology-updater containers.

In Helm deployments,
[Topology Updater parameters](../deployment/helm.md#topology-updater-parameters)
`toplogyUpdater.config` can be used to edit the respective configuration.

In Kustomize deployments, modify the `nfd-worker-conf` ConfigMap with a custom
overlay.

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
[getallocatableresources]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#grpc-endpoint-getallocatableresources
