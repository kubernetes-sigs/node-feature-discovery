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

**EXPERIMENTAL**
Controller for [NodeFeature](custom-resources#nodefeature-custom-resource)
objects can be enabled with the
[`-enable-nodefeature-api`](../reference/master-commandline-reference#-enable-nodefeature-api)
command line flag. When enabled, features from NodeFeature objects are used as
the input for the [NodeFeatureRule](custom-resources#nodefeaturerule)
processing pipeline. In addition, any labels listed in the NodeFeature object
are created on the node (note the allowed
[label namespaces](customization-guide#node-labels) are controlled).

> NOTE: NodeFeature API must also be enabled in nfd-worker with
> its [`-enable-nodefeature-api`](../reference/worker-commandline-reference#-enable-nodefeature-api)
> flag.

## NodeFeatureRule controller

NFD-Master acts as the controller for
[NodeFeatureRule](custom-resources#nodefeaturerule) objects.
It applies the rules specified in NodeFeatureRule objects on raw feature data
and creates node labels accordingly. The feature data used as the input can be
received from nfd-worker instances through the gRPC interface or from
[NodeFeature](custom-resources#nodefeature-custom-resource) objects. The latter
requires that the [NodeFeaure controller](#nodefeature-controller) has been
enabled.

> NOTE: when gRPC is used for communicating the features (the default
> mechanism), (re-)labelling only happens when a request is received from
> nfd-worker. That is, in practice rules are evaluated and labels for each node
> are created on intervals specified by the
> [`core.sleepInterval`](../reference/worker-configuration-reference#coresleepinterval)
> configuration option of nfd-worker instances. This means that modification or
> creation of NodeFeatureRule objects does not instantly cause the node
> labels to be updated.  Instead, the changes only come visible in node labels
> as nfd-worker instances send their labelling requests. This limitation is not
> present when gRPC interface is disabled
> and [NodeFeature](custom-resources#nodefeature-custom-resource) API is used.

## Deployment notes

NFD-Master runs as a deployment, by default
it prefers running on the cluster's master nodes but will run on worker
nodes if no master nodes are found.

For High Availability, you should simply increase the replica count of
the deployment object. You should also look into adding
[inter-pod](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity)
affinity to prevent masters from running on the same node.
However note that inter-pod affinity is costly and is not recommended
in bigger clusters.

> NOTE: If the [NodeFeature controller](#nodefeature-controller) is enabled the
> replica count should be 1.

If you have RBAC authorization enabled (as is the default e.g. with clusters
initialized with kubeadm) you need to configure the appropriate ClusterRoles,
ClusterRoleBindings and a ServiceAccount in order for NFD to create node
labels. The provided template will configure these for you.
