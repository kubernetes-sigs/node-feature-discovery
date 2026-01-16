---
title: "NFD API Reference"
layout: default
sort: 8
---

# NFD API Reference
{: .no_toc}

## Table of Contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

This document provides the API reference for Node Feature Discovery Custom Resources.

## Packages
- [nfd.k8s-sigs.io/v1alpha1](#nfdk8s-sigsiov1alpha1)


## nfd.k8s-sigs.io/v1alpha1

Package v1alpha1 is the v1alpha1 version of the nfd API.

### Resource Types
- [NodeFeature](#nodefeature)
- [NodeFeatureGroup](#nodefeaturegroup)
- [NodeFeatureGroupList](#nodefeaturegrouplist)
- [NodeFeatureList](#nodefeaturelist)
- [NodeFeatureRule](#nodefeaturerule)
- [NodeFeatureRuleList](#nodefeaturerulelist)



#### AttributeFeatureSet



AttributeFeatureSet is a set of features having string value.



_Appears in:_
- [Features](#features)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `elements` _object (keys:string, values:string)_ | Individual features of the feature set. |  |  |


#### FeatureGroupNode







_Appears in:_
- [NodeFeatureGroupStatus](#nodefeaturegroupstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the node. |  |  |


#### FeatureMatcher

_Underlying type:_ _[FeatureMatcherTerm](#featurematcherterm)_

FeatureMatcher specifies a set of feature matcher terms (i.e. per-feature
matchers), all of which must match.



_Appears in:_
- [GroupRule](#grouprule)
- [MatchAnyElem](#matchanyelem)
- [Rule](#rule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `feature` _string_ | Feature is the name of the feature set to match against. |  |  |
| `matchExpressions` _[MatchExpressionSet](#matchexpressionset)_ | MatchExpressions is the set of per-element expressions evaluated. These<br />match against the value of the specified elements. |  |  |
| `matchName` _[MatchExpression](#matchexpression)_ | MatchName in an expression that is matched against the name of each<br />element in the feature set. |  |  |


#### FeatureMatcherTerm



FeatureMatcherTerm defines requirements against one feature set. All
requirements (specified as MatchExpressions) are evaluated against each
element in the feature set.



_Appears in:_
- [FeatureMatcher](#featurematcher)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `feature` _string_ | Feature is the name of the feature set to match against. |  |  |
| `matchExpressions` _[MatchExpressionSet](#matchexpressionset)_ | MatchExpressions is the set of per-element expressions evaluated. These<br />match against the value of the specified elements. |  |  |
| `matchName` _[MatchExpression](#matchexpression)_ | MatchName in an expression that is matched against the name of each<br />element in the feature set. |  |  |


#### Features



Features is the collection of all discovered features.



_Appears in:_
- [NodeFeatureSpec](#nodefeaturespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `flags` _object (keys:string, values:[FlagFeatureSet](#flagfeatureset))_ | Flags contains all the flag-type features of the node. |  |  |
| `attributes` _object (keys:string, values:[AttributeFeatureSet](#attributefeatureset))_ | Attributes contains all the attribute-type features of the node. |  |  |
| `instances` _object (keys:string, values:[InstanceFeatureSet](#instancefeatureset))_ | Instances contains all the instance-type features of the node. |  |  |


#### FlagFeatureSet



FlagFeatureSet is a set of simple features only containing names without values.



_Appears in:_
- [Features](#features)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `elements` _object (keys:string, values:[Nil](#nil))_ | Individual features of the feature set. |  |  |


#### GroupRule



GroupRule defines a rule for nodegroup filtering.



_Appears in:_
- [NodeFeatureGroupSpec](#nodefeaturegroupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the rule. |  |  |
| `vars` _object (keys:string, values:string)_ | Vars is the variables to store if the rule matches. Variables can be<br />referenced from other rules enabling more complex rule hierarchies. |  |  |
| `varsTemplate` _string_ | VarsTemplate specifies a template to expand for dynamically generating<br />multiple variables. Data (after template expansion) must be keys with an<br />optional value (<key>[=<value>]) separated by newlines. |  |  |
| `matchFeatures` _[FeatureMatcher](#featurematcher)_ | MatchFeatures specifies a set of matcher terms all of which must match. |  |  |
| `matchAny` _[MatchAnyElem](#matchanyelem) array_ | MatchAny specifies a list of matchers one of which must match. |  |  |


#### InstanceFeature



InstanceFeature represents one instance of a complex features, e.g. a device.



_Appears in:_
- [InstanceFeatureSet](#instancefeatureset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `attributes` _object (keys:string, values:string)_ | Attributes of the instance feature. |  |  |


#### InstanceFeatureSet



InstanceFeatureSet is a set of features each of which is an instance having multiple attributes.



_Appears in:_
- [Features](#features)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `elements` _[InstanceFeature](#instancefeature) array_ | Individual features of the feature set. |  |  |


#### MatchAnyElem



MatchAnyElem specifies one sub-matcher of MatchAny.



_Appears in:_
- [GroupRule](#grouprule)
- [Rule](#rule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchFeatures` _[FeatureMatcher](#featurematcher)_ | MatchFeatures specifies a set of matcher terms all of which must match. |  |  |


#### MatchExpression

_Underlying type:_ _[struct{Op MatchOp "json:\"op\""; Value MatchValue "json:\"value,omitempty\""; Type ValueType "json:\"type,omitempty\""}](#struct{op-matchop-"json:\"op\"";-value-matchvalue-"json:\"value,omitempty\"";-type-valuetype-"json:\"type,omitempty\""})_

MatchExpression specifies an expression to evaluate against a set of input
values. It contains an operator that is applied when matching the input and
an array of values that the operator evaluates the input against.



_Appears in:_
- [FeatureMatcherTerm](#featurematcherterm)



#### MatchExpressionSet

_Underlying type:_ _[map[string]*MatchExpression](#map[string]*matchexpression)_

MatchExpressionSet contains a set of MatchExpressions, each of which is
evaluated against a set of input values.



_Appears in:_
- [FeatureMatcherTerm](#featurematcherterm)







#### Nil



Nil is a dummy empty struct for protobuf compatibility.
NOTE: protobuf definitions have been removed but this is kept for API compatibility.



_Appears in:_
- [FlagFeatureSet](#flagfeatureset)



#### NodeFeature



NodeFeature resource holds the features discovered for one node in the
cluster.



_Appears in:_
- [NodeFeatureList](#nodefeaturelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeature` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NodeFeatureSpec](#nodefeaturespec)_ | Specification of the NodeFeature, containing features discovered for a node. |  |  |


#### NodeFeatureGroup



NodeFeatureGroup resource holds Node pools by featureGroup



_Appears in:_
- [NodeFeatureGroupList](#nodefeaturegrouplist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureGroup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NodeFeatureGroupSpec](#nodefeaturegroupspec)_ | Spec defines the rules to be evaluated. |  |  |
| `status` _[NodeFeatureGroupStatus](#nodefeaturegroupstatus)_ | Status of the NodeFeatureGroup after the most recent evaluation of the<br />specification. |  |  |


#### NodeFeatureGroupList



NodeFeatureGroupList contains a list of NodeFeatureGroup objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureGroupList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[NodeFeatureGroup](#nodefeaturegroup) array_ | List of NodeFeatureGroups. |  |  |


#### NodeFeatureGroupSpec



NodeFeatureGroupSpec describes a NodeFeatureGroup object.



_Appears in:_
- [NodeFeatureGroup](#nodefeaturegroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `featureGroupRules` _[GroupRule](#grouprule) array_ | List of rules to evaluate to determine nodes that belong in this group. |  |  |


#### NodeFeatureGroupStatus







_Appears in:_
- [NodeFeatureGroup](#nodefeaturegroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodes` _[FeatureGroupNode](#featuregroupnode) array_ | Nodes is a list of FeatureGroupNode in the cluster that match the featureGroupRules |  |  |


#### NodeFeatureList



NodeFeatureList contains a list of NodeFeature objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[NodeFeature](#nodefeature) array_ | List of NodeFeatures. |  |  |


#### NodeFeatureRule



NodeFeatureRule resource specifies a configuration for feature-based
customization of node objects, such as node labeling.



_Appears in:_
- [NodeFeatureRuleList](#nodefeaturerulelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureRule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NodeFeatureRuleSpec](#nodefeaturerulespec)_ | Spec defines the rules to be evaluated. |  |  |


#### NodeFeatureRuleList



NodeFeatureRuleList contains a list of NodeFeatureRule objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureRuleList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[NodeFeatureRule](#nodefeaturerule) array_ | List of NodeFeatureRules. |  |  |


#### NodeFeatureRuleSpec



NodeFeatureRuleSpec describes a NodeFeatureRule.



_Appears in:_
- [NodeFeatureRule](#nodefeaturerule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rules` _[Rule](#rule) array_ | Rules is a list of node customization rules. |  |  |


#### NodeFeatureSpec



NodeFeatureSpec describes a NodeFeature object.



_Appears in:_
- [NodeFeature](#nodefeature)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `features` _[Features](#features)_ | Features is the full "raw" features data that has been discovered. |  |  |
| `labels` _object (keys:string, values:string)_ | Labels is the set of node labels that are requested to be created. |  |  |


#### Rule



Rule defines a rule for node customization such as labeling.



_Appears in:_
- [NodeFeatureRuleSpec](#nodefeaturerulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the rule. |  |  |
| `labels` _object (keys:string, values:string)_ | Labels to create if the rule matches. |  |  |
| `labelsTemplate` _string_ | LabelsTemplate specifies a template to expand for dynamically generating<br />multiple labels. Data (after template expansion) must be keys with an<br />optional value (<key>[=<value>]) separated by newlines. |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations to create if the rule matches. |  |  |
| `vars` _object (keys:string, values:string)_ | Vars is the variables to store if the rule matches. Variables do not<br />directly inflict any changes in the node object. However, they can be<br />referenced from other rules enabling more complex rule hierarchies,<br />without exposing intermediary output values as labels. |  |  |
| `varsTemplate` _string_ | VarsTemplate specifies a template to expand for dynamically generating<br />multiple variables. Data (after template expansion) must be keys with an<br />optional value (<key>[=<value>]) separated by newlines. |  |  |
| `taints` _[Taint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#taint-v1-core) array_ | Taints to create if the rule matches. |  |  |
| `extendedResources` _object (keys:string, values:string)_ | ExtendedResources to create if the rule matches. |  |  |
| `matchFeatures` _[FeatureMatcher](#featurematcher)_ | MatchFeatures specifies a set of matcher terms all of which must match. |  |  |
| `matchAny` _[MatchAnyElem](#matchanyelem) array_ | MatchAny specifies a list of matchers one of which must match. |  |  |




