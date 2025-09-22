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

### -list-size

The pagination size to use when calling api-server to list nodefeatures.
Pagination is useful for controlling the load on api-server/etcd as the
nodefeature resources can be large.
A value of 0 will disable pagination.

Default: 200

Example:

```bash
nfd-gc -list-size=100
```

### -gc-interval

The `-gc-interval` specifies the interval between periodic garbage collector runs.

Default: 1h

Example:

```bash
nfd-gc -gc-interval=1h
```

### -port

The `-port` flag specifies the port on which metrics are served on.

Default: 8080

Example:

```bash
nfd-gc -port=12345
```
