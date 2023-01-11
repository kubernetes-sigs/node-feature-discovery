---
title: "NFD-Topology-Garbage-Collector"
layout: default
sort: 6
---

# NFD-Topology-Garbage-Collector
{: .no_toc}

---

NFD-Topology-Garbage-Collector is preferably run as a Kubernetes deployment
with one replica. It makes sure that all
[NodeResourceTopology](custom-resources#noderesourcetopology)
have corresponding worker nodes and removes stale objects for worker nodes
which are no longer part of Kubernetes cluster.

This service watches for Node deletion events and removes NodeResourceTopology
objects upon them. It is also running periodically to make sure no event was
missed or NodeResourceTopology object was created without corresponding worker
node. The default garbage collector interval is set to 1h which is the value
when no -gc-interval is specified.

## Topology-Garbage-Collector Configuration

In Helm deployments,
(see [Topology Garbage Collector](../deployment/helm.md#topology-garbage-collector-parameters)
for parameters). NFD-Topology-Garbage-Collector will only be deployed when
topologyUpdater.enable is set to true.
