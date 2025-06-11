---
title: "Versions"
parent: "Reference"
layout: default
nav_order: 10
---

# Versions and deprecation
{: .no_toc}

---

## Supported versions

Node Feature Discovery follows [semantic versioning](https://semver.org/) where
the version number consists of three components, i.e. **MAJOR.MINOR.PATCH**.

The most recent two minor releases (or release branches) of Node Feature
Discovery are supported. That is, with X being the latest release, **X** and **X-1**
are supported and **X-1** reaches end-of-life when **X+1** is released.

## Deprecation policy

### Feature labels

Built-in [feature labels](../usage/features.md) and
[features](../usage/customization-guide.html#available-features) are supported
for 2 releases after being deprecated, at minimum. That is, if a feature label
is deprecated in version **X**, it will be supported in **X+1** and **X+2** and
may be dropped in **X+3**.

### Configuration options

Command-line flags and configuration file options are supported for 1 more
release after being deprecated, at minimum. That is, if option/flag is
deprecated in version **X**, it will be supported in **X+1** and may be removed
in **X+2**.

The same policy (support for 1 release after deprecation) also applies to Helm
chart parameters.

## Kubernetes compatibility

Node Feature Discovery is compatible with Kubernetes v1.24 and later.
