---
title: "Crypto Source"
layout: default
sort: 8
---

# Crypto Source

---

## Overview

The crypto source detects IBM Crypto Express (CEX) cryptographic
cards on s390x systems. These hardware security modules (HSMs)
provide cryptographic acceleration and secure key management
capabilities for IBM Z and LinuxONE platforms.

CEX cards are specialized cryptographic coprocessors that offload
cryptographic operations from the main CPU, providing:

- Hardware-accelerated encryption and decryption
- Secure key storage and management
- Cryptographic operations in FIPS-compliant environments
- Support for various cryptographic algorithms (RSA, AES, SHA, etc.)

## Platform Support

**Architecture:** s390x only

The crypto source is specifically designed for IBM Z and LinuxONE
systems and will not detect any features on other architectures.

## Supported Card Types

The crypto source detects the following IBM Crypto Express card generations:

- **CEX4** - Fourth generation
- **CEX5** - Fifth generation
- **CEX6** - Sixth generation
- **CEX7** - Seventh generation
- **CEX8** - Eighth generation (latest)

Each generation may be available in different configurations:

- **CEX-A (Accelerator)** - Provides cryptographic acceleration
  for clear key operations
- **CEX-C (CCA Coprocessor)** - Supports IBM Common Cryptographic
  Architecture (CCA) for secure key operations
- **CEX-P (EP11 Coprocessor)** - Supports Enterprise PKCS#11
  (EP11) for secure key operations

## Hardware Requirements

### Bare Metal Systems

On bare metal IBM Z or LinuxONE systems, CEX cards are
automatically detected if:

- The system has CEX cards installed
- The AP (Adjunct Processor) bus is enabled in the system
  configuration
- The Linux kernel has AP bus support enabled
- The cards are online and accessible

### Virtual Machines

For virtual machines (z/VM, KVM, etc.), CEX card detection requires:

- **Hardware passthrough** - The CEX card must be passed through
  to the VM
- **AP bus configuration** - The VM must have access to the AP bus
- **Proper virtualization setup** - The hypervisor must support
  cryptographic passthrough

**Important:** CEX cards are **not** detected in VMs without
hardware passthrough. The cards must be explicitly assigned to
the VM at the virtualization layer.

## Detection Mechanism

The crypto source scans the sysfs AP bus directory
(`/sys/bus/ap/devices/`) to discover CEX cards. For each detected
card, it reads:

- Card type and generation (e.g., CEX8C)
- Operational mode (Accelerator, CCA, EP11) — derived from the type suffix
- Online/offline status
- Configuration status (whether the card is configured at HMC level)
- Hardware type identifier
- Queue depth
- Installed AP function facilities (ap_functions)
- Associated cryptographic queues

## Labels Generated

The crypto source generates the following node labels:

### `crypto-cex.present`

**Type:** Boolean (true)

Indicates that at least one CEX card is present and detected on the node.

```yaml
feature.node.kubernetes.io/crypto-cex.present: "true"
```

### `crypto-cex.count`

**Type:** String (numeric)

The total number of CEX cards detected on the node.

```yaml
feature.node.kubernetes.io/crypto-cex.count: "2"
```

### `crypto-cex.type-<TYPE>`

**Type:** Boolean (true)

A label is created for each unique card type detected. The
`<TYPE>` placeholder is replaced with the actual card type
(e.g., `CEX8C`, `CEX7P`, `CEX6A`).

```yaml
feature.node.kubernetes.io/crypto-cex.type-CEX8C: "true"
feature.node.kubernetes.io/crypto-cex.type-CEX7P: "true"
```

### `crypto-cex.mode-<MODE>`

**Type:** Boolean (true)

A label is created for each unique operational mode present. Possible values:

- `accelerator` — Clear key acceleration (CEX*A cards)
- `cca` — IBM Common Cryptographic Architecture coprocessor (CEX*C cards)
- `ep11` — Enterprise PKCS#11 coprocessor (CEX*P cards)

```yaml
feature.node.kubernetes.io/crypto-cex.mode-cca: "true"
feature.node.kubernetes.io/crypto-cex.mode-ep11: "true"
```

## Instance Features

In addition to labels, the crypto source exposes detailed
information about each card through instance features under the
`crypto.cex-card` feature set.

### Available Attributes

| Attribute      | Description                                           | Example Value    |
|----------------|-------------------------------------------------------|------------------|
| `name`         | Card device name                                      | `card00`         |
| `type`         | Card type and generation                              | `CEX8C`          |
| `mode`         | Operational mode (accelerator, cca, ep11)             | `cca`            |
| `online`       | Online status (1=online, 0=offline)                   | `1`              |
| `config`       | HMC configuration status (1=configured, 0=deconfig)  | `1`              |
| `hwtype`       | Hardware type identifier                              | `14`             |
| `depth`        | Queue depth                                           | `8`              |
| `ap_functions` | Installed AP function facilities (hex bitmask)        | `0x93800000`     |
| `queue_count`  | Number of queues associated with the card             | `2`              |
| `queues`       | Comma-separated list of queue identifiers             | `00.0014,00.0015`|

### Example Instance Features

```yaml
crypto:
  cex-card:
    - name: "card00"
      type: "CEX8C"
      mode: "cca"
      online: "1"
      config: "1"
      hwtype: "14"
      depth: "8"
      ap_functions: "0x93800000"
      queue_count: "2"
      queues: "00.0014,00.0015"
    - name: "card01"
      type: "CEX8A"
      mode: "accelerator"
      online: "1"
      config: "1"
      hwtype: "14"
      depth: "8"
      ap_functions: "0x93800000"
      queue_count: "2"
      queues: "01.0016,01.0017"
```

## Usage with NodeFeatureRule

You can use NodeFeatureRule to create custom labels and extended
resources based on detected CEX cards.

### Example: Label Nodes with Specific Card Types

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: cex-card-types
spec:
  rules:
    - name: "cex8-coprocessor"
      labels:
        "crypto/cex8-coprocessor": "true"
      matchFeatures:
        - feature: crypto.cex-card
          matchExpressions:
            type:
              op: In
              value:
                - "CEX8C"
                - "CEX8P"
```

## Verification

### Check if CEX Cards are Detected

On an s390x system with CEX cards, verify detection:

```bash
# Check node labels
kubectl get nodes -o json | jq '.items[].metadata.labels' | grep crypto

# Expected output:
# "feature.node.kubernetes.io/crypto-cex.count": "2",
# "feature.node.kubernetes.io/crypto-cex.present": "true",
# "feature.node.kubernetes.io/crypto-cex.type-CEX8C": "true"
```

### Check Instance Features

```bash
# Get NodeFeature object
kubectl get nodefeature -n node-feature-discovery <node-name> -o yaml

# Look for crypto.cex-card section
```

### Verify AP Bus on the Host

```bash
# Check if AP bus exists
ls -la /sys/bus/ap/devices/

# Expected output shows card* entries:
# card00
# card01
# 00.0014
# 00.0015
# ...

# Check card details
cat /sys/bus/ap/devices/card00/type
# Expected output: CEX8C

cat /sys/bus/ap/devices/card00/online
# Expected output: 1
```

## References
[ibm-cex-concepts]: https://www.ibm.com/docs/en/linux-on-systems?topic=linuxone-concepts-crypto-hw
[ibm-z-crypto]: https://www.ibm.com/docs/en/systems-hardware/zsystems/9175-ME1?topic=more-crypto-adapters
[ibm-zcrypt]: https://www.ibm.com/docs/en/linux-on-systems?topic=tasks-loading-zcrypt-device-driver
[kernel-ap]: https://docs.kernel.org/arch/s390/vfio-ap.html
