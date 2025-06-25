---
title: "Feature Export"
layout: default
sort: 12
---

# Feature Export
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

## Feature Export

**Feature export is in the experimental `v1alpha1` version.**

If you are interested in exporting features in a generic context, the nfd client supports an export mode, where features can be derived without requiring a Kubernetes context.
This addresses use cases such as high performance computing (HPC) and other environments with compute nodes that warrant assessment, but may not have Kubernetes running, or may not be able to or want to run a central daemon service for data. 
To use export, you can use `nfd export features`:

```bash
nfd export features
```

By default, JSON structure with parsed key value pairs will appear in the terminal.
To save to a file path:

```bash
nfd export features --path features.json
```