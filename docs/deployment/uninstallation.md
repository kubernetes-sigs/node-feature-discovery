---
title: "Uninstallation"
layout: default
sort: 5
---

# Uninstallation
{: .no_toc}

---

Follow the uninstallation instructions of the deployment method used
([kustomize](kustomize.md#uninstallation),
[helm](helm.md#uninstalling-the-chart) or
[operator](operator.md#uninstallation)).

## Removing feature labels

> **NOTE:** This is unnecessary when using the Helm chart for deployment as it
> will clean up the nodes when NFD is uninstalled.

NFD-Master has a special `-prune` command line flag for removing all
nfd-related node labels, annotations, extended resources and taints from the
cluster.

```bash
kubectl apply -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/prune?ref={{ site.release }}"
kubectl -n node-feature-discovery wait job.batch/nfd-master --for=condition=complete && \
    kubectl delete -k "https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/prune?ref={{ site.release }}"
```

> **NOTE:** You must run prune before removing the RBAC rules (serviceaccount,
> clusterrole and clusterrolebinding).

## Removing stale NodeFeature objects

In case NFD-Worker is configured to set the owner reference to Node or Pod,
manually delete stale NodeFeature objects.

```bash
kubectl delete -n node-feature-discovery NodeFeature $(kubectl get -o json -n node-feature-discovery NodeFeature | jq ".items[].metadata.name")
```
