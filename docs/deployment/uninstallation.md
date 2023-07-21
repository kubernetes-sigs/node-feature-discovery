---
title: "Uninstallation"
layout: default
sort: 6
---

# Uninstallation
{: .no_toc}

---

Follow the uninstallation instructions of the deployment method used
([kustomize](kustomize.md#uninstallation),
[helm](helm.md#uninstalling-the-chart) or
[operator](operator.md#uninstallation)).

## Removing feature labels

NFD-Master has a special `-prune` command line flag for removing all
nfd-related node labels, annotations, extended resources and taints from the
cluster.

```bash
kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/prune?ref={{ site.release }}
kubectl -n node-feature-discovery wait job.batch/nfd-master --for=condition=complete && \
    kubectl delete -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/prune?ref={{ site.release }}
```

**NOTE:** You must run prune before removing the RBAC rules (serviceaccount,
clusterrole and clusterrolebinding).
