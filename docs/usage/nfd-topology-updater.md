---
title: "NFD-Topology-Updater"
layout: default
sort: 5
---

# NFD-Topology-Updater
{: .no_toc}

---

NFD-Topology-Updater is preferably run as a Kubernetes DaemonSet. This assures
re-examination (and CR updates) on regular intervals capturing changes in
the allocated resources and hence the allocatable resources on a per zone
basis. It makes sure that more CR instances are created as new nodes get
added to the cluster. Topology-Updater connects to the nfd-master service
to create CR instances corresponding to nodes.

When run as a daemonset, nodes are re-examined for the allocated resources
(to determine the information of the allocatable resources on a per zone basis
where a zone can be a NUMA node) at an interval specified using the
[`-sleep-interval`](../reference/topology-updater-commandline-reference.html#-sleep-interval)
option. The default sleep interval is set to 60s which is the the value when no
-sleep-interval is specified.
