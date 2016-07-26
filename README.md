# Node feature discovery for [Kubernetes](https://kubernetes.io)

- [Overview](#overview)
- [License](#license)

## Overview

This software enables node feature discovery for Kubernetes. It detects
hardware features available on each node in a Kubernetes cluster, and advertises
those features using node labels.

### Feature sources

- [CPUID][cpuid] for x86 CPU details
- [Intel Resource Director Technology][intel-rdt]
- [Intel P-State driver][intel-pstate]

### Feature labels

The published node labels encode a few pieces of information:

- A "namespace" to denote the vendor/provider (e.g. `node.alpha.intel.com`).
- The version of this discovery code that wrote the label, according to
  `git describe --tags --dirty --always`.
- The source for each label (e.g. `cpuid`).
- The name of the discovered feature as it appears in the underlying
  source, (e.g. `AESNI` from cpuid).

_Note: only features that are available on a given node are labeled, so
the only label value published for features is the string `"true"`._

```json
{
  "node.alpha.intel.com/dbi-iafeature-discovery.version": "v0.1.0",
  "node.alpha.intel.com/v0.1.0-cpuid-<feature-name>": "true",
  "node.alpha.intel.com/v0.1.0-rdt-<feature-name>": "true",
  "node.alpha.intel.com/v0.1.0-pstate-<feature-name>": "true"
}
```

## License

This is open source software released under the [Apache 2.0 License](LICENSE).

<!-- Links -->
[cpuid]: http://man7.org/linux/man-pages/man4/cpuid.4.html
[intel-rdt]: http://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html
[intel-pstate]: https://www.kernel.org/doc/Documentation/cpu-freq/intel-pstate.txt
