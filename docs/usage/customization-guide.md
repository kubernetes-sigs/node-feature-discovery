---
title: "Customization guide"
layout: default
sort: 8
---

# Customization guide
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

## Overview

NFD provides multiple extension points for vendor and application specific
labeling:

- [`NodeFeature`](#nodefeature-custom-resource) objects can be
  used to communicate "raw" node features and node labeling requests to
  nfd-master.
- [`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects provide a way to
  deploy custom labeling rules via the Kubernetes API.
- [`local`](#local-feature-source) feature source of nfd-worker creates
  labels by reading text files.
- [`custom`](#custom-feature-source) feature source of nfd-worker creates
  labels based on user-specified rules.

## NodeFeature custom resource

NodeFeature objects provide a way for 3rd party extensions to advertise custom
features, both as "raw" features that serve as input to
[NodeFeatureRule](#nodefeaturerule-custom-resource) objects and as feature
labels directly.

Note that RBAC rules must be created for each extension for them to be able to
create and manipulate NodeFeature objects in their namespace.

### A NodeFeature example

Consider the following referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeature
metadata:
  labels:
    nfd.node.kubernetes.io/node-name: node-1
  name: vendor-features-for-node-1
spec:
  # Features for NodeFeatureRule matching
  features:
    flags:
      vendor.flags:
        elements:
          feature-x: {}
          feature-y: {}
    attributes:
      vendor.config:
        elements:
          setting-a: "auto"
          knob-b: "123"
    instances:
      vendor.devices:
        elements:
        - attributes:
            model: "dev-1000"
            vendor: "acme"
        - attributes:
            model: "dev-2000"
            vendor: "acme"
  # Labels to be created
  labels:
    vendor.io/feature.enabled: "true"
```

The object targets node named `node-1`. It lists two "flag type" features under
the `vendor.flags` domain, two "attribute type" features and under the
`vendor.config` domain and two "instance type" features under the
`vendor.devices` domain. These features will not be directly affecting the node
labels but they will be used as input when the
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects are evaluated.

In addition, the example requests directly the
`vendor.io/feature.enabled=true` node label to be created.

The `nfd.node.kubernetes.io/node-name=<node-name>` must be in place for each
NodeFeature object as NFD uses it to determine the node which it is targeting.

### Feature types

Features have three different types:

- **flag** features: a set of names without any associated values, e.g. CPUID
  flags or loaded kernel modules
- **attribute** features: a set of names each of which has a single value
  associated with it (essentially a map of key-value pairs), e.g. kernel config
  flags or os release information
- **instance** features: a list of instances, each of which has multiple
  attributes (key-value pairs of their own) associated with it, e.g. PCI or USB
  devices

## NodeFeatureRule custom resource

`NodeFeatureRule` objects provide an easy way to create vendor or application
specific labels and taints. It uses a flexible rule-based mechanism for creating
labels and optionally taints based on node features.

### A NodeFeatureRule example

Consider the following referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-sample-rule-object
spec:
  rules:
    - name: "my sample rule"
      labels:
        "feature.node.kubernetes.io/my-sample-feature": "true"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            dummy: {op: Exists}
        - feature: kernel.config
          matchExpressions:
            X86: {op: In, value: ["y"]}
```

It specifies one rule which creates node label
`feature.node.kubernetes.io/my-sample-feature=true` if both of the following
conditions are true (`matchFeatures` implements a logical AND over the
matchers):

- The `dummy` network driver module has been loaded
- X86 option in kernel config is set to `=y`

Create a `NodeFeatureRule` with a yaml file:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/examples/nodefeaturerule.yaml
```

Now, on X86 platforms the feature label appears after doing `modprobe dummy` on
a system and correspondingly the label is removed after `rmmod dummy`. Note a
re-labeling delay up to the sleep-interval of nfd-worker (1 minute by default).

See [Feature rule format](#feature-rule-format) for detailed description of
available fields and how to write labeling rules.

### Node tainting

In some circumstances, it is desirable to keep nodes with specialized hardware
away from running general workload and instead leave them for workloads that
need the specialized hardware. One way to achieve it is to taint the nodes with
the specialized hardware and add corresponding toleration to pods that require
the special hardware. NFD offers node tainting functionality which is disabled
by default. User can define one or more custom taints via the `taints` field of
the NodeFeatureRule CR. The same rule-based mechanism is applied here and the
NFD taints only rule matching nodes.

To enable the tainting feature, `--enable-taints` flag needs to be set to `true`.
If the flag `--enable-taints` is set to `false` (i.e. disabled), taints defined in
the NodeFeatureRule CR have no effect and will be ignored by the NFD master.

See documentation of the [taints field](#taints) for detailed description how
to specify taints in the NodeFeatureRule object.

> **NOTE:** Before enabling any taints, make sure to edit nfd-worker daemonset
> to tolerate the taints to be created. Otherwise, already running pods that do
> not tolerate the taint are evicted immediately from the node including the
> nfd-worker pod.

## NodeFeatureGroup custom resource

NodeFeatureGroup API is an alpha feature and disabled by default in NFD version
{{ site.version }}. Use the
[NodeFeatureAPI](../reference/feature-gates.md#nodefeaturegroupapi) feature
gate to enable it.

`NodeFeatureGroup` objects provide a way to create node groups that share the
same set of features. The `NodeFeatureGroup` object spec consists of a list of
`NodeFeatureRule` that follow the same format as the `NodeFeatureRule`,
but the difference in this case is that nodes that match any of the rules in the
`NodeFeatureGroup` will be listed in the `NodeFeatureGroup` status.

### A NodeFeatureGroup example

Consider the following referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureGroup
metadata:
  name: node-feature-group-example
spec:
  featureGroupRules:
    - name: "kernel version"
      matchFeatures:
        - feature: kernel.version
          matchExpressions:
            major: {op: In, value: ["6"]}
status:
  nodes:
    - name: node-1
    - name: node-2
    - name: node-3
```

The object specifies a group of nodes that share the same
`kernel.version.major` (Linux kernel v6.x).

Create a `NodeFeatureGroup` with a yaml file:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{ site.release }}/examples/nodefeaturegroup.yaml
```

See [Feature rule format](#feature-rule-format) for detailed description of
available fields and how to write group filtering rules.

## Local feature source

NFD-Worker has a special feature source named `local` which is an integration
point for external feature detectors. It provides a mechanism for pluggable
extensions, allowing the creation of new user-specific features and even
overriding built-in labels.

The `local` feature source uses feature files. The features discovered by the
`local` source can further be used in label rules specified in
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects and
the [`custom`](#custom-feature-source) feature source.

> **NOTE:** Be careful when creating and/or updating feature files
> while NFD is running. To avoid race conditions you should write
> into a temporary file, and atomically create/update the original file by
> doing a file rename operation. NFD ignores dot files,
> so temporary file can be written to the same directory and renamed
> (`.my.feature` -> `my.feature`) once file is complete. Both file names should
> (obviously) be unique for the given application.

### An example

Consider a plaintext file
`/etc/kubernetes/node-feature-discovery/features.d/my-features`
having the following contents:

```plaintext
feature.node.kubernetes.io/my-feature.1
feature.node.kubernetes.io/my-feature.2=myvalue
vendor.io/my-feature.3=456
```

This will translate into the following node labels:

```yaml
feature.node.kubernetes.io/my-feature.1: "true"
feature.node.kubernetes.io/my-feature.2: "myvalue"
vendor.io/my-feature.3: "456"
```

### Feature files

The `local` source reads files found in
`/etc/kubernetes/node-feature-discovery/features.d/`. File content is parsed
and translated into node labels, see the [input format below](#input-format).

### Input format

The feature files are expected to contain features in simple
key-value pairs, separated by newlines:

```plaintext
# This is a comment
<key>[=<value>]
```

The label value defaults to `true`, if not specified.

Label namespace must be specified with `<namespace>/<name>[=<value>]`.

> **NOTE:** The feature file size limit it 64kB. The feature file will be
> ignored if the size limit is exceeded.

Comment lines (starting with `#`) are ignored.

Adding following line anywhere to feature file defines date when
its content expires / is ignored:

```plaintext
# +expiry-time=2023-07-29T11:22:33Z
```

Also, the expiry-time value would stay the same during the processing of the
feature file until another expiry-time directive is encountered.
Considering the following file:

```plaintext
# +expiry-time=2012-07-28T11:22:33Z
vendor.io/feature1=featureValue

# +expiry-time=2080-07-28T11:22:33Z
vendor.io/feature2=featureValue2

# +expiry-time=2070-07-28T11:22:33Z
vendor.io/feature3=featureValue3

# +expiry-time=2002-07-28T11:22:33Z
vendor.io/feature4=featureValue4
```

After processing the above file, only `vendor.io/feature2` and
`vendor.io/feature3` would be included in the list of accepted features.

> **NOTE:** The time format supported is RFC3339. Also, the `expiry-time`
> tag is only evaluated in each re-discovery period, and the expiration of
> node labels is not tracked.

To exclude specific features from the `local.feature` Feature, you can use the
`# +no-feature` directive. The `# +no-label` directive causes the feature to
be excluded from the `local.label` Feature and a node label not to be generated.

Considering the following file:

```plaintext
# +no-feature
vendor.io/label-only=value

vendor.io/my-feature=value

vendor.io/foo=bar

# +no-label
foo=baz
```

Processing the above file would result in the following Features:

```yaml
local.features:
  foo: baz
  vendor.io/my-feature: value
local.labels:
  vendor.io/label-only: value
  vendor.io/my-feature: value
```

and the following labels added to the Node:

```plaintext
vendor.io/label-only=value
vendor.io/my-feature=value
```

> **NOTE:** use of unprefixed label names (like `foo=bar`) should not be used.
> In NFD {{ site.version }} unprefixed names will be automatically prefixed
> with `feature.node.kubernetes.io/` but this will change in a future version
> (see the [DisableAutoPrefix](../reference/feature-gates.md#disableautoprefix)
> feature gate). Unprefixed names for plain Features (tagged with `#+no-label`)
> can be used without restrictions, however.

### Mounts

The standard NFD deployments contain `hostPath` mounts for
`/etc/kubernetes/node-feature-discovery/features.d/`, making these directories
from the host available inside the nfd-worker container.

#### Injecting labels from other pods

One use case for the feature files is detecting features in other
Pods outside NFD, e.g. in Kubernetes device plugins. By using the same
`hostPath` mounts `/etc/kubernetes/node-feature-discovery/features.d/`
in the side-car (e.g. device plugin) creates a shared area for
deploying feature files to NFD.

## Custom feature source

The `custom` feature source in nfd-worker provides a rule-based mechanism for
label creation, similar to the
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects. The difference is
that the rules are specified in the worker configuration instead of a
Kubernetes API object.

See [worker configuration](nfd-worker.md#worker-configuration)
for instructions how to set-up and manage the worker configuration.

### An example custom feature source configuration

Consider the following referential configuration for nfd-worker:

```yaml
core:
  labelSources: ["custom"]
sources:
  custom:
    - name: "my sample rule"
      labels:
        "feature.node.kubenernetes.io/my-sample-feature": "true"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            dummy: {op: Exists}
        - feature: kernel.config
          matchExpressions:
            X86: {op: In, value: ["y"]}
```

It specifies one rule which creates node label
`feature.node.kubenernetes.io/my-sample-feature=true` if both of the following
conditions are true (`matchFeatures` implements a logical AND over the
matchers):

- The `dummy` network driver module has been loaded
- X86 option in kernel config is set to `=y`

In addition, the configuration only enables the `custom` source, disabling all
built-in labels.

Now, on X86 platforms the feature label appears after doing `modprobe dummy` on
a system and correspondingly the label is removed after `rmmod dummy`. Note a
re-labeling delay up to the sleep-interval of nfd-worker (1 minute by default).

### Additional configuration directory

In addition to the rules defined in the nfd-worker configuration file, the
`custom` feature source can read more configuration files located in the
`/etc/kubernetes/node-feature-discovery/custom.d/` directory. This makes more
dynamic and flexible configuration easier.

As an example, consider having file
`/etc/kubernetes/node-feature-discovery/custom.d/my-rule.yaml` with the
following content:

```yaml
- name: "my e1000 rule"
  labels:
    "feature.node.kubenernetes.io/e1000.present": "true"
  matchFeatures:
    - feature: kernel.loadedmodule
      matchExpressions:
        e1000: {op: Exists}
```

This simple rule will create `feature.node.kubenernetes.io/e1000.present=true`
label if the `e1000` kernel module has been loaded.

The
[`samples/custom-rules`](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/overlays/samples/custom-rules)
kustomize overlay sample contains an example for deploying a custom rule from a
ConfigMap.

## Node labels

Feature labels have the following format:

```plaintext
<namespace>/<name> = <value>
```

The namespace part (i.e. prefix) of the labels is controlled by nfd:

- All built-in labels use `feature.node.kubernetes.io`.
- Namespaces may be excluded with the
  [`-deny-label-ns`](../reference/master-commandline-reference.md#-deny-label-ns)
  command line flag of nfd-master
  - To allow specific namespaces that were denied, you can use
    [`-extra-label-ns`](../reference/master-commandline-reference.md#-extra-label-ns)
    command line flag of nfd-master.
    e.g: `nfd-master -deny-label-ns="*" -extra-label-ns=example.com`

## Feature rule format

This section describes the rule format used  in
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects and in the
configuration of the [`custom`](#custom-feature-source) feature source.

It is based on a generic feature matcher that covers all features discovered by
nfd-worker. The rules rely on a unified data model of the available features
and a generic expression-based format. Features that can be used in the rules
are described in detail in [available features](#available-features) below.

Take this rule as a referential example:

```yaml
    - name: "my feature rule"
      labels:
        "feature.node.kubernetes.io/my-special-feature": "my-value"
      matchFeatures:
        - feature: cpu.cpuid
          matchExpressions:
            AVX512F: {op: Exists}
        - feature: kernel.version
          matchExpressions:
            major: {op: In, value: ["5"]}
            minor: {op: Gt, value: ["1"]}
        - feature: pci.device
          matchExpressions:
            vendor: {op: In, value: ["8086"]}
            class: {op: In, value: ["0200"]}
```

This will yield `feature.node.kubernetes.io/my-special-feature=my-value` node
label if all of these are true (`matchFeatures` implements a logical AND over
the matchers):

- the CPU has AVX512F capability
- kernel version is 5.2 or later (must be v5.x)
- an Intel network controller is present

### Fields

#### name

The `.name` field is required and used as an identifier of the rule.

#### labels

The `.labels` is a map of the node labels to create if the rule matches.

Take this rule as a referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-sample-rule-object
spec:
  rules:
    - name: "my dynamic label value rule"
      labels:
        feature.node.kubernetes.io/linux-lsm-enabled: "@kernel.config.LSM"
        feature.node.kubernetes.io/custom-label: "customlabel"
```

Label `linux-lsm-enabled` uses the `@` notation for dynamic values.
The value of the label will be the value of the attribute `LSM`
of the feature `kernel.config`.

The `@<feature-name>.<element-name>` format can be used to inject values of
detected features to the label. See
[available features](#available-features) for possible values to use.

This will yield into the following node label:

```yaml
  labels:
    ...
    feature.node.kubernetes.io/linux-lsm-enabled: apparmor
    feature.node.kubernetes.io/custom-label: "customlabel"
```

#### labelsTemplate

The `.labelsTemplate` field specifies a text template for dynamically creating
labels based on the matched features. See [templating](#templating) for
details.

> **NOTE:** The `labels` field has priority over `labelsTemplate`, i.e.
> labels specified in the `labels` field will override anything
> originating from `labelsTemplate`.

#### annotations

The `.annotations` field is a list of features to be advertised as node
annotations.

Take this rule as a referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: feature-annotations-example
spec:
  rules:
    - name: "annotation-example"
      annotations:
        feature.node.kubernetes.io/defaul-ns-annotation: "foo"
        custom.vendor.io/feature: "baz"
      matchFeatures:
        - feature: kernel.version
          matchExpressions:
            major: {op: Exists}
```

This will yield into the following node annotations:

```yaml
  annotations:
    ...
    feature.node.kubernetes.io/defaul-ns-annotation: "foo"
    custom.vendor.io/feature: "baz"
    ...
```

NFD enforces some limitations to the namespace (or prefix)/ of the annotations:

- `kubernetes.io/` and its sub-namespaces (like `sub.ns.kubernetes.io/`) cannot
  generally be used
- the only exception is `feature.node.kubernetes.io/` and its sub-namespaces
  (like `sub.ns.feature.node.kubernetes.io`)
- if an unprefixed name (e.g., `my-annotation`) is used, NFD {{ site.version }}
  will automatically prefix it with `feature.node.kubernetes.io/` unless the
  `DisableAutoPrefix` feature gate is set to true, in which case no prefixing
  occurs.

> **NOTE:** The `annotations` field has will only advertise features via node
> annotations the features won't be advertised as node labels unless they are
> specified in the `labels` field.

#### taints

*taints* is a list of taint entries and each entry can have `key`, `value` and `effect`,
where the `value` is optional. Effect could be `NoSchedule`, `PreferNoSchedule`
or `NoExecute`. To learn more about the meaning of these effects, check out k8s [documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).

Example NodeFeatureRule with taints:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-sample-rule-object
spec:
  rules:
    - name: "my sample taint rule"
      taints:
        - effect: PreferNoSchedule
          key: "feature.node.kubernetes.io/special-node"
          value: "true"
        - effect: NoExecute
          key: "feature.node.kubernetes.io/dedicated-node"
      matchFeatures:
        - feature: kernel.loadedmodule
          matchExpressions:
            dummy: {op: Exists}
        - feature: kernel.config
          matchExpressions:
            X86: {op: In, value: ["y"]}
```

In this example, if the `my sample taint rule` rule is matched,
`feature.node.kubernetes.io/pci-0300_1d0f.present=true:NoExecute`
and `feature.node.kubernetes.io/cpu-cpuid.ADX:NoExecute` taints are set on the node.

There are some limitations to the namespace part (i.e. prefix/) of the taint
key:

- `kubernetes.io/` and its sub-namespaces (like `sub.ns.kubernetes.io/`) cannot
  generally be used
- the only exception is `feature.node.kubernetes.io/` and its sub-namespaces
  (like `sub.ns.feature.node.kubernetes.io`)
- unprefixed keys (like `foo`) keys are disallowed

> **NOTE:** taints field is not available for the custom rules of nfd-worker
> and only for NodeFeatureRule objects.

#### vars

The `.vars` field is a map of values (key-value pairs) to store for subsequent
rules to use. In other words, these are variables that are not advertised as
node labels. See [backreferences](#backreferences) for more details on the
usage of vars.

#### extendedResources

The `.extendedResources` field is a list of extended resources to advertise.
See [extended resources](#extended-resources) for more details.

Take this rule as a referential example:

```yaml
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: my-extended-resource-rule
spec:
  rules:
    - name: "my extended resource rule"
      extendedResources:
        vendor.io/dynamic: "@kernel.version.major"
        vendor.io/static: "123"
      matchFeatures:
        - feature: kernel.version
          matchExpressions:
            major: {op: Exists}
```

The extended resource `vendor.io/dynamic` is defined in the form `@feature.attribute`.
The value of the extended resource will be the value of the attribute `major`
of the feature `kernel.version`.

The `@<feature-name>.<element-name>` format can be used to inject values of
detected features to the extended resource. See
[available features](#available-features) for possible values to use. Note that
the value must be eligible as a
Kubernetes resource quantity.

This will yield into the following node status:

```yaml
  allocatable:
    ...
    vendor.io/dynamic: "5"
    vendor.io/static: "123"
    ...
  capacity:
    ...
    vendor.io/dynamic: "5"
    vendor.io/static: "123"
    ...
```

There are some limitations to the namespace part (i.e. prefix)/ of the Extended
Resources names:

- `kubernetes.io/` and its sub-namespaces (like `sub.ns.kubernetes.io/`) cannot
  generally be used
- the only exception is `feature.node.kubernetes.io/` and its sub-namespaces
  (like `sub.ns.feature.node.kubernetes.io`)
- if an unprefixed name (e.g., `my-er`) is used, NFD {{ site.version }}
  will automatically prefix it with `feature.node.kubernetes.io/` unless the
  `DisableAutoPrefix` feature gate is set to true, in which case no prefixing
  occurs.

> **NOTE:** `.extendedResources` is not supported by the
> [custom feature source](#custom-feature-source) -- it can only be used in
> NodeFeatureRule objects.

#### varsTemplate

The `.varsTemplate` field specifies a text template for dynamically creating
vars based on the matched features. See [templating](#templating) for details
on using templates and [backreferences](#backreferences) for more details on
the usage of vars.

> **NOTE:** The `vars` field has priority over `varsTemplate`, i.e.
> vars specified in the `vars` field will override anything originating from
> `varsTemplate`.

#### matchFeatures

The `.matchFeatures` field specifies a feature matcher, consisting of a list of
feature matcher terms. It implements a logical AND over the terms i.e. all
of them must match for the rule to trigger.

```yaml
      matchFeatures:
        - feature: <feature-name>
          matchExpressions:
            <key>:
              op: <op>
              value:
                - <value-1>
                - ...
          matchName:
            op: <op>
            value:
                - <value-1>
                - ...
```

The `.matchFeatures[].feature` field specifies the feature which to evaluate.

> **NOTE:**If both [`matchExpressions`](#matchexpressions) and
> [`matchName`](#matchname) are specified, they both must match.

##### matchExpressions

The `.matchFeatures[].matchExpressions` field is used to match against the
value(s) of a feature. The `matchExpressions` field consists of a set of
expressions, each of which is evaluated against all elements of the specified
feature.

```yaml
      matchExpressions:
        <key>:
          op: <op>
          value:
            - <value-1>
            - ...
          type: <type>
```

In each MatchExpression the `key` specifies the name of of the feature element
(*flag* and *attribute* features) or name of the attribute (*instance*
features) which to look for. The behavior of MatchExpression depends on the
[feature type](#feature-types):

- for *flag* and *attribute* features the MatchExpression operates on the
  feature element whose name matches the `<key>`
- for *instance* features all MatchExpressions are evaluated against the
  attributes of each instance separately

The `op` field specifies the operator to apply. Valid values are described
below.

| Operator        | Number of values | Matches when |
| --------------- | ---------------- | ----------- |
|  `In`           | 1 or greater | Input is equal to one of the values |
|  `NotIn`        | 1 or greater | Input is not equal to any of the values |
|  `InRegexp`     | 1 or greater | Values of the MatchExpression are treated as regexps and input matches one or more of them |
|  `Exists`       | 0            | The key exists |
|  `DoesNotExist` | 0            | The key does not exists |
|  `Gt`           | 1            | Input is greater than the value. Both the input and value must be integer numbers. |
|  `Ge`           | 1            | Input is greater than or equal to the value. Both the input and value must be integer numbers. |
|  `Lt`           | 1            | Input is less than the value. Both the input and value must be integer numbers. |
|  `Le`           | 1            | Input is less than or equal to the value. Both the input and value must be integer numbers. |
|  `GtLt`         | 2            | Input is between two values. Both the input and value must be integer numbers. |
|  `GeLe`         | 2            | Input falls within a range that includes the boundary values. Both the input and value must be integer numbers. |
|  `IsTrue`       | 0            | Input is equal to "true" |
|  `IsFalse`      | 0            | Input is equal "false" |

The `value` field of MatchExpression is a list of string arguments to the
operator.

Type optional `type` field specifies the type of the `value` field.
Valid types for specific operators are described below.

| Type      | Description | Supported Operators |
| --------- | ----------- | ------------------- |
| `version` | Input is recognized as a version in the following formats (major.minor.patch) `%d.%d.%d`, `%d.%d`, `%d` (e.g., "1.2.3", "1.2", "1") |`Gt`,`Ge`,`Lt`,`Le`,`GtLt`,`GeLe` |

##### matchName

The `.matchFeatures[].matchName` field is used to match against the
name(s) of a feature (whereas the [`matchExpressions`](#matchexpressions) field
matches against the value(s). The `matchName` field consists of a single
expression which is evaulated against the name of each element of the specified
feature.

```yaml
      matchName:
        op: <op>
        value:
          - <value-1>
          - ...
```

The behavior of `matchName` depends on the [feature type](#feature-types):

- for *flag* and *attribute* features the expression is evaluated against the
  name of each element
- for *instance* features the expression is evaluated against the name of
  each attribute, for each element (instance) separately (matches if the
  attributes of any of the elements satisfy the expression)

The `op` field specifies the operator to apply. Same operators as for
[`matchExpressions`](#matchexpressions) above are available.

| Operator        | Number of values | Matches |
| --------------- | ---------------- | ----------- |
|  `In`           | 1 or greater | All name is equal to one of the values |
|  `NotIn`        | 1 or greater | All name that is not equal to any of the values |
|  `InRegexp`     | 1 or greater | All name that matches any of the values (treated as regexps) |
|  `Exists`       | 0            | All elements |

Other operators are not practical with `matchName` (`DoesNotExist` never
matches; `Gt`,`Lt` and `GtLt` are only usable if feature names are integers;
`IsTrue` and `IsFalse` are only usable if the feature name is `true` or
`false`).

The `value` field is a list of string arguments to the operator.

An example:

```yaml
      matchFeatures:
        - feature: cpu.cpuid
          matchName: {op: InRegexp, value: ["^AVX"]}
```

The snippet above would match if any CPUID feature starting with AVX is present
(e.g. AVX1 or AVX2 or AVX512F etc).

#### matchAny

The `.matchAny` field is a list of of [`matchFeatures`](#matchfeatures)
matchers. A logical OR is applied over the matchers, i.e. at least one of them
must match for the rule to trigger.

Consider the following example:

```yaml
      matchAny:
        - matchFeatures:
            - feature: kernel.loadedmodule
              matchExpressions:
                kmod-1: {op: Exists}
            - feature: pci.device
              matchExpressions:
                vendor: {op: In, value: ["0eee"]}
                class: {op: In, value: ["0200"]}
        - matchFeatures:
            - feature: kernel.loadedmodule
              matchExpressions:
                kmod-2: {op: Exists}
            - feature: pci.device
              matchExpressions:
                vendor: {op: In, value: ["0fff"]}
                class: {op: In, value: ["0200"]}
```

This matches if kernel module kmod-1 is loaded and a network controller from
vendor 0eee is present, OR, if kernel module kmod-2 has been loaded and a
network controller from vendor 0fff is present (OR both of these conditions are
true).

### Available features

The following features are available for matching:

| Feature          | [Feature types](#feature-types) | Elements | Value type | Description |
| ---------------- | ------------ | -------- | ---------- | ----------- |
| **`cpu.cpuid`**  | flag         |          |            | Supported CPU capabilities |
|                  |              | **`<cpuid-flag>`** |  | CPUID flag is present |
|                  | attribute    |          |            | CPU capability attributes |
|                  |              | **AVX10_VERSION** | int | AVX10 vector ISA version (if supported) |
| **`cpu.cstate`** | attribute    |          |            | Status of cstates in the intel_idle cpuidle driver |
|                  |              | **`enabled`** | bool  | 'true' if cstates are set, otherwise 'false'. Does not exist of intel_idle driver is not active. |
| **`cpu.model`**  | attribute    |          |            | CPU model related attributes |
|                  |              | **`family`** | int    | CPU family |
|                  |              | **`vendor_id`** | string | CPU vendor ID |
|                  |              | **`id`** | int        | CPU model ID |
|                  |              | **`hypervisor`** | string | Hypervisor type information. On s390x read from `/proc/sysinfo`. On x86_64/arm64 detected via CPUID. Value 'none' on physical hardware. |
| **`cpu.pstate`** | attribute    |          |            | State of the Intel pstate driver. Does not exist if the driver is not enabled. |
|                  |              | **`status`** | string | Status of the driver, possible values are 'active' and 'passive' |
|                  |              | **`turbo`**  | bool   | 'true' if turbo frequencies are enabled, otherwise 'false' |
|                  |              | **`scaling`** | string | Active scaling_governor, possible values are 'powersave' or 'performance'. |
| **`cpu.rdt`**    | attribute    |          |            | Intel RDT capabilities supported by the system |
|                  |              | **`<rdt-flag>`** |    | RDT capability is supported, see [RDT flags](#intel-rdt-flags) for details |
|                  |              | **`RDTL3CA_NUM_CLOSID`** | int  | The number or available CLOSID (Class of service ID) for Intel L3 Cache Allocation Technology |
| **`cpu.security`** | attribute  |          |            | Features related to security and trusted execution environments |
|                  |              | **`sgx.enabled`** | bool | `true` if Intel SGX (Software Guard Extensions) has been enabled, otherwise does not exist |
|                  |              | **`sgx.epc`** | int | The total amount Intel SGX Encrypted Page Cache memory in bytes. It's only present if `sgx.enabled` is `true`. |
|                  |              | **`se.enabled`** | bool  | `true` if IBM Secure Execution for Linux is available and has been enabled, otherwise does not exist |
|                  |              | **`tdx.enabled`** | bool | `true` if Intel TDX (Trusted Domain Extensions) is available on the host and has been enabled, otherwise does not exist |
|                  |              | **`tdx.total_keys`** | int | The total amount of keys an Intel TDX (Trusted Domain Extensions) host can provide.  It's only present if `tdx.enabled` is `true`. |
|                  |              | **`tdx.protected`** | bool | `true` if a guest VM was started using Intel TDX (Trusted Domain Extensions), otherwise does not exist. |
|                  |              | **`sev.enabled`** | bool | `true` if AMD SEV (Secure Encrypted Virtualization) is available on the host and has been enabled, otherwise does not exist |
|                  |              | **`sev.es.enabled`** | bool | `true` if AMD SEV-ES (Encrypted State supported) is available on the host and has been enabled, otherwise does not exist |
|                  |              | **`sev.snp.enabled`** | bool | `true` if AMD SEV-SNP (Secure Nested Paging supported) is available on the host and has been enabled, otherwise does not exist |
|                  |              | **`sev.asids`** | int | The total amount of AMD SEV address-space identifiers (ASIDs), based on the `/sys/fs/cgroup/misc.capacity` information. |
|                  |              | **`sev.encrypted_state_ids`** | int | The total amount of AMD SEV-ES and SEV-SNP supported, based on the `/sys/fs/cgroup/misc.capacity` information. |
| **`cpu.sst`**    | attribute    |          |            | Intel SST (Speed Select Technology) capabilities |
|                  |              | **`bf.enabled`** | bool | `true` if Intel SST-BF (Intel Speed Select Technology - Base frequency) has been enabled, otherwise does not exist |
| **`cpu.topology`** | attribute  |          |            | CPU topology related features |
| | |          **`hardware_multithreading`** | bool       | Hardware multithreading, such as Intel HTT, is enabled |
| | |          **`socket_count`**            | int        | Number of CPU Sockets |
| **`cpu.coprocessor`** | attribute |        |            | CPU Coprocessor related features |
| | |          **`nx_gzip`**                 | bool       | Nest Accelerator GZIP support is enabled |
| **`kernel.config`** | attribute |          |            | Kernel configuration options |
|                  |              | **`<config-flag>`** | string | Value of the kconfig option |
| **`kernel.loadedmodule`** | flag |         |            | Kernel modules loaded on the node as reported by `/proc/modules` |
| **`kernel.enabledmodule`** | flag |        |            | Kernel modules loaded on the node and available as built-ins as reported by `modules.builtin` |
|                  |              | **`mod-name`** |      | Kernel module `<mod-name>` is loaded |
| **`kernel.selinux`** | attribute |         |            | Kernel SELinux related features |
|                  |              | **`enabled`** | bool  | `true` if SELinux has been enabled and is in enforcing mode, otherwise `false` |
| **`kernel.kvm`** | attribute |         |            | Kernel KVM related features |
|                  |              | **`enabled`** | bool  | `true` if KVM has been enabled, otherwise `false` |
| **`kernel.version`** | attribute |          |           | Kernel version information |
|                  |              | **`full`** | string   | Full kernel version (e.g. ‘4.5.6-7-g123abcde') |
|                  |              | **`major`** | int     | First component of the kernel version (e.g. ‘4') |
|                  |              | **`minor`** | int     | Second component of the kernel version (e.g. ‘5') |
|                  |              | **`revision`** | int  | Third component of the kernel version (e.g. ‘6') |
| **`local.label`** | attribute   |           |           | Labels from feature files, i.e. labels from the [*local* feature source](#local-feature-source) |
| **`local.feature`** | attribute   |           |         | Features from feature files, i.e. features from the [*local* feature source](#local-feature-source) |
|                  |              | **`<label-name>`** | string | Label `<label-name>` created by the local feature source, value equals the value of the label |
| **`memory.nv`**  | instance     |          |            | NVDIMM devices present in the system |
|                  |              | **`<sysfs-attribute>`** | string | Value of the sysfs device attribute, available attributes: `devtype`, `mode` |
| **`memory.numa`**  | attribute  |          |            | NUMA nodes |
|                  |              | **`is_numa`** | bool  | `true` if NUMA architecture, `false` otherwise |
|                  |              | **`node_count`** | int | Number of NUMA nodes |
| **`memory.swap`**  | attribute  |          |            | Swap enabled on node |
|                  |              | **`enabled`** | bool  | `true` if swap partition detected, `false` otherwise |
| **`memory.hugepages`**  | attribute  |          |       | Discovery of supported huge pages size on node |
|                  |              | **`enabled`** | bool  | `true` if total number of huge pages (of any page size) have been configured, otherwise `false` |
|                  |              | **`hugepages-<page-size>`** | string   | Total number of huge pages (e.g., `hugepages-1Gi=16`) |
| **`network.device`** | instance |          |            | Physical (non-virtual) network interfaces present in the system |
|                  |              | **`name`** | string   | Name of the network interface |
|                  |              | **`<sysfs-attribute>`** | string | Sysfs network interface attribute, available attributes: `operstate`, `speed`, `sriov_numvfs`, `sriov_totalvfs`, `mtu` |
| **`network.virtual`** | instance |          |            | Virtual network interfaces present in the system |
|                  |              | **`name`** | string   | Name of the network interface |
|                  |              | **`<sysfs-attribute>`** | string | Sysfs network interface attribute, available attributes: `operstate`, `speed`, `mtu` |
| **`pci.device`** | instance     |          |            | PCI devices present in the system |
|                  |              | **`<sysfs-attribute>`** | string | Value of the sysfs device attribute, available attributes: `class`, `vendor`, `device`, `subsystem_vendor`, `subsystem_device`, `sriov_totalvfs`, `iommu_group/type`, `iommu/intel-iommu/version` |
| **`storage.block`** | instance |          |             | Block storage devices present in the system |
|                  |              | **`name`** | string   | Name of the block device |
|                  |              | **`<sysfs-attribute>`** | string | Sysfs network interface attribute, available attributes: `dax`, `rotational`, `nr_zones`, `zoned` |
| **`system.osrelease`** | attribute |       |            | System identification data from `/etc/os-release` |
|                  |              | **`<parameter>`** | string | One parameter from `/etc/os-release` |
| **`system.dmiid`** | attribute |       |            | DMI identification data from `/sys/devices/virtual/dmi/id/` |
|                  |              | **`sys_vendor`** | string | Vendor name from `/sys/devices/virtual/dmi/id/sys_vendor` |
|                  |              | **`product_name`** | string | Product name from `/sys/devices/virtual/dmi/id/product_name` |
| **`system.name`** | attribute   |          |            | System name information |
|                  |              | **`nodename`** | string | Name of the kubernetes node object |
| **`usb.device`** | instance     |          |            | USB devices present in the system |
|                  |              | **`<sysfs-attribute>`** | string | Value of the sysfs device attribute, available attributes: `class`, `vendor`, `device`, `serial` |
| **`rule.matched`** | attribute  |          |            | Previously matched rules |
|                  |              | **`<label-or-var>`** | string | Label or var from a preceding rule that matched |

#### Intel RDT flags

| Flag      | Description                                                      |
| --------- | ---------------------------------------------------------------- |
| RDTMON    | Intel RDT Monitoring Technology                                   |
| RDTCMT    | Intel Cache Monitoring (CMT)                                      |
| RDTMBM    | Intel Memory Bandwidth Monitoring (MBM)                           |
| RDTL3CA   | Intel L3 Cache Allocation Technology                              |
| RDTl2CA   | Intel L2 Cache Allocation Technology                              |
| RDTMBA    | Intel Memory Bandwidth Allocation (MBA) Technology                |

### Templating

Rules support template-based creation of labels and vars with the
`.labelsTemplate` and `.varsTemplate` fields. These makes it possible to
dynamically generate labels and vars based on the features that matched.

The template must expand into a simple format with `<key>=<value>` pairs
separated by newline.

Consider the following example:
<!-- {% raw %} -->

```yaml
    labelsTemplate: |
      {{ range .pci.device }}vendor-{{ .class }}-{{ .device }}.present=true
      {{ end }}
    matchFeatures:
      - feature: pci.device
        matchExpressions:
          class: {op: InRegexp, value: ["^02"]}
          vendor: ["0fff"]
```

<!-- {% endraw %} -->
The rule above will create individual labels
`feature.node.kubernetes.io/vendor-<class-id>-<device-id>.present=true` for
each network controller device (device class starting with 02) from vendor
0fff.

All the matched features of each feature matcher term under `matchFeatures`
fields are available for the template engine. Matched features can be
referenced with `{%raw%}{{ .<feature-name> }}{%endraw%}` in the template, and
the available data could be described in yaml as follows:

```yaml
.
  <key-feature>:
    - Name: <matched-key>
    - ...

  <value-feature>:
    - Name: <matched-key>
      Value: <matched-value>
    - ...

  <instance-feature>:
    - <attribute-1-name>: <attribute-1-value>
      <attribute-2-name>: <attribute-2-value>
      ...
    - ...
```

That is, the per-feature data is a list of objects whose data fields depend on
the type of the feature:

- for *flag* features only 'Name' is available
- for *value* features 'Name' and 'Value' are available
- for *instance* features all attributes of the matched instance are available

A simple example of a template utilizing name and value from an *attribute*
feature:
<!-- {% raw %} -->

```yaml
    labelsTemplate: |
      {{ range .system.osrelease }}system-{{ .Name }}={{ .Value }}
      {{ end }}
    matchFeatures:
      - feature: system.osRelease
        matchExpressions:
          ID: {op: Exists}
          VERSION_ID.major: {op: Exists}
```

<!-- {% endraw %} -->
> **NOTE:**If both `matchExpressions` and `matchName` for a feature matcher
> term (see [`matchFeatures`](#matchfeatures)) is specified, the list of
> matched features (for the template engine) is the union from both of these.
<!-- note #2 -->
> **NOTE:** In case of matchAny is specified, the template is executed
> separately against each individual `matchFeatures` field and the final set of
> labels will be superset of all these separate template expansions. E.g.
> consider the following:

```yaml
  - name: <name>
    labelsTemplate: <template>
    matchFeatures: <matcher#1>
    matchAny:
      - matchFeatures: <matcher#2>
      - matchFeatures: <matcher#3>
```

In the example above (assuming the overall result is a match) the template
would be executed on matcher#1 as well as on matcher#2 and/or matcher#3
(depending on whether both or only one of them match). All the labels from
these separate expansions would be created, i.e. the end result would be a
union of all the individual expansions.

Rule templates use the Golang [text/template](https://pkg.go.dev/text/template)
package along with [Sprig functions](https://masterminds.github.io/sprig/)
and all their functionality (e.g. pipelines and functions) can
be used. An example template taking use of the built-in `len` function,
advertising the number of PCI network controllers from a specific vendor,
and using Sprig's `first`, `trim` and `substr` to advertise the first one's class:
<!-- {% raw %} -->

```yaml
    labelsTemplate: |
      num-intel-network-controllers={{ .pci.device | len }}
      first-intel-network-controllers={{ (.pci.device | first).class | trim | substr 0 63 }}
    matchFeatures:
      - feature: pci.device
        matchExpressions:
          vendor: {op: In, value: ["8086"]}
          class: {op: In, value: ["0200"]}

```

<!-- {% endraw %} -->

Imaginative template pipelines are possible, but care must be taken to
produce understandable and maintainable rule sets.

### Backreferences

Rules support referencing the output of preceding rules. This enables
sophisticated scenarios where multiple rules are combined together
to for more complex heuristics than a single rule can provide. The labels and
vars created by the execution of preceding rules are available as a special
`rule.matched` feature.

Consider the following configuration:

```yaml
  - name: "my kernel label rule"
    labels:
      kernel-feature: "true"
    matchFeatures:
      - feature: kernel.version
        matchExpressions:
          major: {op: Gt, value: ["4"]}

  - name: "my var rule"
    vars:
      nolabel-feature: "true"
    matchFeatures:
      - feature: cpu.cpuid
        matchExpressions:
          AVX512F: {op: Exists}
      - feature: pci.device
        matchExpressions:
          vendor: {op: In, value: ["0fff"]}
          device: {op: In, value: ["1234", "1235"]}

  - name: "my high level feature rule"
    labels:
      high-level-feature: "true"
    matchFeatures:
      - feature: rule.matched
        matchExpressions:
          kernel-feature: {op: IsTrue}
          nolabel-feature: {op: IsTrue}
```

The `feature.node.kubernetes.io/high-level-feature = true` label depends on the
two previous rules.

Note that when referencing rules across multiple
[`NodeFeatureRule`](#nodefeaturerule-custom-resource) objects attention must be
paid to the ordering. `NodeFeatureRule` objects are processed in alphabetical
order (based on their `.metadata.name`).

### Examples

Some more configuration examples below.

Match certain CPUID features:

```yaml
  - name: "example cpuid rule"
    labels:
      my-special-cpu-feature: "true"
    matchFeatures:
      - feature: cpu.cpuid
        matchExpressions:
          AESNI: {op: Exists}
          AVX: {op: Exists}
```

Require a certain loaded kernel module and OS version:

```yaml
  - name: "my multi-feature rule"
    labels:
      my-special-multi-feature: "true"
    matchFeatures:
      - feature: kernel.loadedmodule
        matchExpressions:
          e1000: {op: Exists}
      - feature: system.osrelease
        matchExpressions:
          NAME: {op: InRegexp, values: ["^openSUSE"]}
          VERSION_ID.major: {op: Gt, values: ["14"]}
```

Require a loaded  kernel module and two specific PCI devices (both of which
must be present):

```yaml
  - name: "my multi-device rule"
    labels:
      my-multi-device-feature: "true"
    matchFeatures:
      - feature: kernel.loadedmodule
        matchExpressions:
          my-driver-module: {op: Exists}
      - pci.device:
          vendor: "0fff"
          device: "1234"
      - pci.device:
          vendor: "0fff"
          device: "abcd"
```
