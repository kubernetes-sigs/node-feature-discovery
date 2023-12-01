---
title: "NFD-Master"
layout: default
sort: 3
---

# NFD-Master
{: .no_toc}

---

NFD-Master is responsible for connecting to the Kubernetes API server and
updating node objects. More specifically, it modifies node labels, taints and
extended resources based on requests from nfd-workers and 3rd party extensions.

## NodeFeature controller

The NodeFeature Controller uses NodeFeature objects as
the input for the [NodeFeatureRule](custom-resources.md#nodefeaturerule)
processing pipeline. In addition, any labels listed in the NodeFeature object
are created on the node (note the allowed
[label namespaces](customization-guide.md#node-labels) are controlled).

## NodeFeatureRule controller

NFD-Master acts as the controller for
[NodeFeatureRule](custom-resources.md#nodefeaturerule) objects.
It applies the rules specified in NodeFeatureRule objects on raw feature data
and creates node labels accordingly. The feature data used as the input is
received from nfd-worker instances through
[NodeFeature](custom-resources.md#nodefeature-custom-resource) objects.

> **NOTE**: when gRPC (**DEPRECATED**)  is used for communicating
> the features (by setting the flag `-enable-nodefeature-api=false` on both
> nfd-master and nfd-worker, or via Helm values.enableNodeFeatureApi=false),
> (re-)labelling only happens when a request is received from nfd-worker.
> That is, in practice rules are evaluated and labels for each node are created
> on intervals specified by the
> [`core.sleepInterval`](../reference/worker-configuration-reference.md#coresleepinterval)
> configuration option of nfd-worker instances. This means that modification or
> creation of NodeFeatureRule objects does not instantly cause the node
> labels to be updated.  Instead, the changes only come visible in node labels
> as nfd-worker instances send their labelling requests. This limitation is not
> present when gRPC interface is disabled
> and [NodeFeature](custom-resources.md#nodefeature-custom-resource) API is used.

## Master configuration

NFD-Master supports dynamic configuration through a configuration file. The
default location is `/etc/kubernetes/node-feature-discovery/nfd-master.conf`,
but, this can be changed by specifying the`-config` command line flag.
Configuration file is re-read whenever it is modified which makes run-time
re-configuration of nfd-master straightforward.

Master configuration file is read inside the container, and thus, Volumes and
VolumeMounts are needed to make your configuration available for NFD. The
preferred method is to use a ConfigMap which provides easy deployment and
re-configurability.

The provided nfd-master deployment templates create an empty configmap and
mount it inside the nfd-master containers. In kustomize deployments,
configuration can be edited with:

```bash
kubectl -n ${NFD_NS} edit configmap nfd-master-conf
```

In Helm deployments,
[Master pod parameter](../deployment/helm.md#master-pod-parameters)
`master.config` can be used to edit the respective configuration.

See
[nfd-master configuration file reference](../reference/master-configuration-reference.md)
for more details.
The (empty-by-default)
[example config](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/master-config/nfd-master.conf.example)
contains all available configuration options and can be used as a reference
for creating a configuration.

## Deployment notes

NFD-Master runs as a deployment, by default
it prefers running on the cluster's master nodes but will run on worker
nodes if no master nodes are found.

For High Availability, you should increase the replica count of
the deployment object. You should also look into adding
[inter-pod](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity)
affinity to prevent masters from running on the same node.
However note that inter-pod affinity is costly and is not recommended
in bigger clusters.

> **Note:** When NFD-Master is intended to run with more than one replica,
> it is advised to use `-enable-leader-election` flag. This flag turns on
> leader election for NFD-Master and let only one replica to act on changes
> in NodeFeature and NodeFeatureRule objects.

If you have RBAC authorization enabled (as is the default e.g. with clusters
initialized with kubeadm) you need to configure the appropriate ClusterRoles,
ClusterRoleBindings and a ServiceAccount for NFD to create node
labels. The provided template will configure these for you.
