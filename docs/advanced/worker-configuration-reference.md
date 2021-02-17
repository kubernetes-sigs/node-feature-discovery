---
title: "Worker Config Reference"
layout: default
sort: 4
---

# NFD-Worker Configuration File Reference
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

***WORK IN PROGRESS***

1. TOC
{:toc}

---

See the
[sample configuration file](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{ site.release }}/nfd-worker.conf.example)
for a full example configuration.

## core

The `core` section contains common configuration settings that are not specific
to any particular feature source.

### core.sleepInterval

`core.sleepInterval` specifies the interval between consecutive passes of
feature (re-)detection, and thus also the interval between node re-labeling. A
non-positive value implies infinite sleep interval, i.e. no re-detection or
re-labeling is done.

Note: Overridden by the deprecated `--sleep-interval` command line flag (if
specified).

Default: `60s`

Example:

```yaml
core:
  sleepInterval: 60s
```

### core.sources

`core.sources` specifies the list of enabled feature sources. A special value
`all` enables all feature sources.

Note: Overridden by the deprecated `--sources` command line flag (if
specified).

Default: `[all]`

Example:

```yaml
core:
  sources:
    - system
    - custom
```

### core.labelWhiteList

`core.labelWhiteList` specifies a regular expression for filtering feature
labels based on the label name. Non-matching labels are not published.

Note: The regular expression is only matches against the "basename" part of the
label, i.e. to the part of the name after '/'. The label prefix (or namespace)
is omitted.

Note: Overridden by the deprecated `--label-whitelist` command line flag (if
specified).

Default: `null`

Example:

```yaml
core:
  labelWhiteList: '^cpu-cpuid'
```

### core.noPublish

Setting `core.noPublish` to `true` disables all communication with the
nfd-master. It is effectively a "dry-run" flag: nfd-worker runs feature
detection normally, but no labeling requests are sent to nfd-master.

Note: Overridden by the `--no-publish` command line flag (if specified).

Default: `false`

Example:

```yaml
core:
  noPublish: true
```

## sources

The `sources` section contains feature source specific configuration parameters.

### sources.cpu

### sources.kernel

### soures.pci

### sources.usb

### sources.custom
