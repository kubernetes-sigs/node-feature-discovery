---
title: "NFD-Topology-Updater"
layout: default
sort: 5
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

Because of the design and implementation of Kubernetes, only exclusively
allocated resources are accounted:
[CPU cores](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy-configuration)
(pods of the [Guaranteed Quality of Service](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed)
class requesting whole CPUs),
[memory](https://kubernetes.io/docs/tasks/administer-cluster/memory-manager/#policy-static)
(Guaranteed QoS pods) and
[devices](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
allocated by device plugins (for pods of any QoS class).

When run as a daemonset, nodes are re-examined for the allocated resources
(to determine the information of the allocatable resources on a per-zone basis
where a zone can be a NUMA node) at an interval specified using the
[`-sleep-interval`](../reference/topology-updater-commandline-reference.md#-sleep-interval)
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
[GetAllocatableResources][getallocatableresources] functionality is a
prerequisite for nfd-topology-updater to be able to run. It is enabled by
default on all Kubernetes versions supported by NFD (v1.24 and later), so no
kubelet feature gate needs to be set.

### NodeResourceTopology CRD

The NFD-Topology-Updater requires the `NodeResourceTopology` Custom Resource
Definition (CRD) to be installed in the cluster. Without this CRD the
topology-updater does not publish anything: the pods stay Running and Ready
while the updater retries (backoff from 5s up to 60s) and logs:

```plaintext
waiting for NodeResourceTopology CRD to be created. If using Helm, ensure 'topologyUpdater.createCRDs=true' is set
```

The updater continues automatically once the CRD exists.

When deploying with Helm, you **must** set `topologyUpdater.createCRDs=true`
along with `topologyUpdater.enable=true`:

```bash
helm install nfd --namespace node-feature-discovery --create-namespace \
  {{ site.helm_oci_repo }} --version {{ site.helm_chart_version }} \
  --set topologyUpdater.enable=true \
  --set topologyUpdater.createCRDs=true
```

If you manage CRDs separately (e.g., through a dedicated CRD management tool or
another Helm release), ensure the `NodeResourceTopology` CRD is installed before
deploying the topology-updater

## Topology-Updater Configuration

NFD-Topology-Updater supports configuration through a configuration file. The
default location is `/etc/kubernetes/node-feature-discovery/nfd-topology-updater.conf`,
but this can be changed by specifying the `-config` command line flag.

Topology-Updater configuration file is read inside the container,
and thus, Volumes and VolumeMounts are needed
to make your configuration available for NFD.
The preferred method is to use a ConfigMap
which provides easy deployment and re-configurability.

The provided deployment templates create an empty configmap
and mount it inside the nfd-topology-updater containers.

In Helm deployments,
[Topology Updater parameters](../deployment/helm.md#nfd-topology-updater)
`topologyUpdater.config` can be used to edit the respective configuration.

In Kustomize deployments, modify the `nfd-topology-updater-conf` ConfigMap
with a custom overlay.

See
[nfd-topology-updater configuration file reference](../reference/topology-updater-configuration-reference.md)
for more details.
The (empty-by-default)
[example config](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/topology-updater-config/nfd-topology-updater.conf.example)
contains all available configuration options and can be used as a reference
for creating a configuration. Quote the `*` key of `excludeList` (`'*'`): a
bare `*` is YAML alias syntax and fails to parse.

<!-- Links -->
[podresource-api]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#monitoring-device-plugin-resources
[getallocatableresources]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#grpc-endpoint-getallocatableresources
