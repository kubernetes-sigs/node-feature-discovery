---
title: "Export"
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

## Export

If you are interested in exporting features or labels in a generic
context, the nfd client supports an export mode, where both can be
derived on the command line.

### Features

**Feature export is in the experimental version.**

This addresses use cases such as high performance computing (HPC) and
other environments with compute nodes that warrant assessment, but may
not have Kubernetes running, or may not be able to or want to run a
central daemon service for data. To export features, you can use `nfd
export features`:

```bash
nfd export features
```

By default, JSON structure with parsed key value pairs will appear in the
terminal. To save to a file path:

```bash
nfd export features --path features.json
```

### Labels

To export equivalent labels outside of a Kubernetes context,
you can use `nfd export labels`.

```bash
nfd export labels
```

Or export to an output file:

```bash
nfd export labels --path labels.json
```
