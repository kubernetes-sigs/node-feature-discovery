---
title: "Garbage Collector Cmdline Reference"
layout: default
sort: 7
---

# NFD-GC Commandline Flags
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

To quickly view available command line flags execute `nfd-gc -help`.
In a docker container:

```bash
docker run {{ site.container_image }} \
nfd-gc -help
```

### -h, -help

Print usage and exit.

### -version

Print version and exit.

### -gc-interval

The `-gc-interval` specifies the interval between periodic garbage collector runs.

Default: 1h

Example:

```bash
nfd-gc -gc-interval=1h
```
