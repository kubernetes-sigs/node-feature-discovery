---
title: "Deployment"
layout: default
sort: 2
---

# Deployment

Node Feature Discovery can be deployed on any recent version of Kubernetes
(v1.21+).

See [Image variants](image-variants.md) for description of the different NFD
container images available.

[Using Kustomize](kustomize.md) provides straightforward deployment with
`kubectl` integration and declarative customization.

[Using Helm](helm.md) provides easy management of NFD deployments with nice
configuration management and easy upgrades.

[Using Operator](operator.md) provides deployment and configuration management via
CRDs.
