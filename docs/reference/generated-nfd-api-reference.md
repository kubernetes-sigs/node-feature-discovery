---
title: "NFD API Reference"
layout: default
sort: 8
---

# API Reference

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
| `elements` _object (keys:string, values:string)_ | Individual features of the feature set. |  | Required: \{\} <br /> |


#### FeatureGroupNode



FeatureGroupNode is a node that matches the rules of a NodeFeatureGroup.



_Appears in:_
- [NodeFeatureGroupStatus](#nodefeaturegroupstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the node. |  | Required: \{\} <br /> |


#### FeatureMatcher

_Underlying type:_ _[FeatureMatcherTerm](#featurematcherterm) array_

FeatureMatcher is a list (array) of FeatureMatcherTerm (i.e. per-feature
matchers), all of which must match.



_Appears in:_
- [GroupRule](#grouprule)
- [MatchAnyElem](#matchanyelem)
- [Rule](#rule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `feature` _string_ | Feature is the name of the feature set to match against. |  | Required: \{\} <br /> |
| `matchExpressions` _[MatchExpressionSet](#matchexpressionset)_ | MatchExpressions is the set of per-element expressions evaluated. These<br />match against the value of the specified elements. |  | Optional: \{\} <br /> |
| `matchName` _[MatchExpression](#matchexpression)_ | MatchName in an expression that is matched against the name of each<br />element in the feature set. |  | Optional: \{\} <br /> |


#### FeatureMatcherTerm



FeatureMatcherTerm defines requirements against one feature set. All
requirements (specified as MatchExpressions) are evaluated against each
element in the feature set.



_Appears in:_
- [FeatureMatcher](#featurematcher)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `feature` _string_ | Feature is the name of the feature set to match against. |  | Required: \{\} <br /> |
| `matchExpressions` _[MatchExpressionSet](#matchexpressionset)_ | MatchExpressions is the set of per-element expressions evaluated. These<br />match against the value of the specified elements. |  | Optional: \{\} <br /> |
| `matchName` _[MatchExpression](#matchexpression)_ | MatchName in an expression that is matched against the name of each<br />element in the feature set. |  | Optional: \{\} <br /> |


#### Features



Features is the collection of all discovered features.



_Appears in:_
- [NodeFeatureSpec](#nodefeaturespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `flags` _object (keys:string, values:[FlagFeatureSet](#flagfeatureset))_ | Flags contains all the flag-type features of the node. |  | Optional: \{\} <br /> |
| `attributes` _object (keys:string, values:[AttributeFeatureSet](#attributefeatureset))_ | Attributes contains all the attribute-type features of the node. |  | Optional: \{\} <br /> |
| `instances` _object (keys:string, values:[InstanceFeatureSet](#instancefeatureset))_ | Instances contains all the instance-type features of the node. |  | Optional: \{\} <br /> |


#### FlagFeatureSet



FlagFeatureSet is a set of simple features only containing names without values.



_Appears in:_
- [Features](#features)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `elements` _object (keys:string, values:[Nil](#nil))_ | Individual features of the feature set. |  | Required: \{\} <br /> |


#### GroupRule



GroupRule defines a rule for nodegroup filtering.



_Appears in:_
- [NodeFeatureGroupSpec](#nodefeaturegroupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the rule. |  | Required: \{\} <br /> |
| `vars` _object (keys:string, values:string)_ | Vars is the variables to store if the rule matches. Variables can be<br />referenced from other rules enabling more complex rule hierarchies. |  | Optional: \{\} <br /> |
| `varsTemplate` _string_ | VarsTemplate specifies a template to expand for dynamically generating<br />multiple variables. Data (after template expansion) must be keys with an<br />optional value (<key>[=<value>]) separated by newlines. |  | Optional: \{\} <br /> |
| `matchFeatures` _[FeatureMatcher](#featurematcher)_ | MatchFeatures specifies a set of matcher terms all of which must match. |  | Optional: \{\} <br /> |
| `matchAny` _[MatchAnyElem](#matchanyelem) array_ | MatchAny specifies a list of matchers one of which must match. |  | Optional: \{\} <br /> |


#### InstanceFeature



InstanceFeature represents one instance of a complex features, e.g. a device.



_Appears in:_
- [InstanceFeatureSet](#instancefeatureset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `attributes` _object (keys:string, values:string)_ | Attributes of the instance feature. |  | Required: \{\} <br /> |


#### InstanceFeatureSet



InstanceFeatureSet is a set of features each of which is an instance having multiple attributes.



_Appears in:_
- [Features](#features)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `elements` _[InstanceFeature](#instancefeature) array_ | Individual features of the feature set. |  | Required: \{\} <br /> |


#### MatchAnyElem



MatchAnyElem specifies one sub-matcher of MatchAny.



_Appears in:_
- [GroupRule](#grouprule)
- [Rule](#rule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchFeatures` _[FeatureMatcher](#featurematcher)_ | MatchFeatures specifies a set of matcher terms all of which must match. |  | Required: \{\} <br /> |


#### MatchExpression



MatchExpression specifies an expression to evaluate against a set of input
values. It contains an operator that is applied when matching the input and
an array of values that the operator evaluates the input against.



_Appears in:_
- [FeatureMatcherTerm](#featurematcherterm)
- [MatchExpressionSet](#matchexpressionset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `op` _[MatchOp](#matchop)_ | Op is the operator to be applied. |  | Enum: [In NotIn InRegexp Exists DoesNotExist Gt Ge Lt Le GtLt GeLe IsTrue IsFalse] <br />Required: \{\} <br /> |
| `value` _[MatchValue](#matchvalue)_ | Value is the list of values that the operand evaluates the input<br />against. Value should be empty if the operator is Exists, DoesNotExist,<br />IsTrue or IsFalse. Value should contain exactly one element if the<br />operator is Gt or Lt and exactly two elements if the operator is GtLt.<br />In other cases Value should contain at least one element. |  | Optional: \{\} <br /> |
| `type` _[ValueType](#valuetype)_ | Type defines the value type for specific operators.<br />The currently supported type is 'version' for Gt,Ge,Lt,Le,GtLt,GeLe operators. |  | Optional: \{\} <br /> |


#### MatchExpressionSet

_Underlying type:_ _object (keys:string, values:[MatchExpression](#matchexpression))_

MatchExpressionSet contains a set of MatchExpressions, each of which is
evaluated against a set of input values.



_Appears in:_
- [FeatureMatcherTerm](#featurematcherterm)



#### MatchOp

_Underlying type:_ _string_

MatchOp is the match operator that is applied on values when evaluating a
MatchExpression.

_Validation:_
- Enum: [In NotIn InRegexp Exists DoesNotExist Gt Ge Lt Le GtLt GeLe IsTrue IsFalse]

_Appears in:_
- [MatchExpression](#matchexpression)

| Field | Description |
| --- | --- |
| `In` | MatchIn returns true if any of the values stored in the expression is<br />equal to the input.<br /> |
| `NotIn` | MatchNotIn returns true if none of the values in the expression are<br />equal to the input.<br /> |
| `InRegexp` | MatchInRegexp treats values of the expression as regular expressions and<br />returns true if any of them matches the input.<br /> |
| `Exists` | MatchExists returns true if the input is valid. The expression must not<br />have any values.<br /> |
| `DoesNotExist` | MatchDoesNotExist returns true if the input is not valid. The expression<br />must not have any values.<br /> |
| `Gt` | MatchGt returns true if the input is greater than the value of the<br />expression (number of values in the expression must be exactly one).<br />Both the input and value must be integer numbers, otherwise an error is<br />returned.<br /> |
| `Ge` | MatchGe returns true if the input is greater than or equal to the value of the<br />expression (number of values in the expression must be exactly one).<br />Both the input and value must be integer numbers, otherwise an error is<br />returned.<br /> |
| `Lt` | MatchLt returns true if the input is less  than the value of the<br />expression (number of values in the expression must be exactly one).<br />Both the input and value must be integer numbers, otherwise an error is<br />returned.<br /> |
| `Le` | MatchLe returns true if the input is less than or equal to the value of the<br />expression (number of values in the expression must be exactly one).<br />Both the input and value must be integer numbers, otherwise an error is<br />returned.<br /> |
| `GtLt` | MatchGtLt returns true if the input is between two values, i.e. greater<br />than the first value and less than the second value of the expression<br />(number of values in the expression must be exactly two). Both the input<br />and values must be integer numbers, otherwise an error is returned.<br /> |
| `GeLe` | MatchGeLe returns true if the input is between two values including the boundary values,<br />i.e. greater than or equal to the first value and less than or equal to the second value<br />of the expression (number of values in the expression must be exactly two). Both the input<br />and values must be integer numbers, otherwise an error is returned.<br /> |
| `IsTrue` | MatchIsTrue returns true if the input holds the value "true". The<br />expression must not have any values.<br /> |
| `IsFalse` | MatchIsFalse returns true if the input holds the value "false". The<br />expression must not have any values.<br /> |


#### MatchValue

_Underlying type:_ _string array_

MatchValue is the list of values associated with a MatchExpression.



_Appears in:_
- [MatchExpression](#matchexpression)



#### Nil



Nil is a dummy empty struct for protobuf compatibility.
NOTE: protobuf definitions have been removed but this is kept for API compatibility.



_Appears in:_
- [FlagFeatureSet](#flagfeatureset)



#### NodeFeature



NodeFeature resource holds the features discovered for one node in the
cluster.
NodeFeature is namespaced.



_Appears in:_
- [NodeFeatureList](#nodefeaturelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeature` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NodeFeatureSpec](#nodefeaturespec)_ | Specification of the NodeFeature, containing features discovered for a node. |  | Required: \{\} <br /> |


#### NodeFeatureGroup



NodeFeatureGroup resource holds Node pools by featureGroup.
NodeFeatureGroup is namespaced (short name `nfg`) and has a status subresource.
Only objects in the namespace of nfd-master are processed.



_Appears in:_
- [NodeFeatureGroupList](#nodefeaturegrouplist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureGroup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NodeFeatureGroupSpec](#nodefeaturegroupspec)_ | Spec defines the rules to be evaluated. |  | Required: \{\} <br /> |
| `status` _[NodeFeatureGroupStatus](#nodefeaturegroupstatus)_ | Status of the NodeFeatureGroup after the most recent evaluation of the<br />specification. |  | Optional: \{\} <br /> |


#### NodeFeatureGroupList



NodeFeatureGroupList contains a list of NodeFeatureGroup objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureGroupList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[NodeFeatureGroup](#nodefeaturegroup) array_ | List of NodeFeatureGroups. |  |  |


#### NodeFeatureGroupSpec



NodeFeatureGroupSpec describes a NodeFeatureGroup object.



_Appears in:_
- [NodeFeatureGroup](#nodefeaturegroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `featureGroupRules` _[GroupRule](#grouprule) array_ | List of rules to evaluate to determine nodes that belong in this group. |  | Required: \{\} <br /> |


#### NodeFeatureGroupStatus



NodeFeatureGroupStatus is the status of a NodeFeatureGroup, i.e. the result
of the most recent evaluation of its rules.



_Appears in:_
- [NodeFeatureGroup](#nodefeaturegroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodes` _[FeatureGroupNode](#featuregroupnode) array_ | Nodes is a list of FeatureGroupNode in the cluster that match the featureGroupRules |  | Optional: \{\} <br /> |


#### NodeFeatureList



NodeFeatureList contains a list of NodeFeature objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[NodeFeature](#nodefeature) array_ | List of NodeFeatures. |  |  |


#### NodeFeatureRule



NodeFeatureRule resource specifies a configuration for feature-based
customization of node objects, such as node labeling.
NodeFeatureRule is cluster-scoped (short name `nfr`).



_Appears in:_
- [NodeFeatureRuleList](#nodefeaturerulelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureRule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NodeFeatureRuleSpec](#nodefeaturerulespec)_ | Spec defines the rules to be evaluated. |  | Required: \{\} <br /> |


#### NodeFeatureRuleList



NodeFeatureRuleList contains a list of NodeFeatureRule objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nfd.k8s-sigs.io/v1alpha1` | | |
| `kind` _string_ | `NodeFeatureRuleList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[NodeFeatureRule](#nodefeaturerule) array_ | List of NodeFeatureRules. |  |  |


#### NodeFeatureRuleSpec



NodeFeatureRuleSpec describes a NodeFeatureRule.



_Appears in:_
- [NodeFeatureRule](#nodefeaturerule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rules` _[Rule](#rule) array_ | Rules is a list of node customization rules. |  | Required: \{\} <br /> |


#### NodeFeatureSpec



NodeFeatureSpec describes a NodeFeature object.



_Appears in:_
- [NodeFeature](#nodefeature)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `features` _[Features](#features)_ | Features is the full "raw" features data that has been discovered. |  | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels is the set of node labels that are requested to be created. |  | Optional: \{\} <br /> |


#### Rule



Rule defines a rule for node customization such as labeling.



_Appears in:_
- [NodeFeatureRuleSpec](#nodefeaturerulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the rule. |  | Required: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels to create if the rule matches. |  | Optional: \{\} <br /> |
| `labelsTemplate` _string_ | LabelsTemplate specifies a template to expand for dynamically generating<br />multiple labels. Data (after template expansion) must be keys with an<br />optional value (<key>[=<value>]) separated by newlines. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations to create if the rule matches. |  | Optional: \{\} <br /> |
| `vars` _object (keys:string, values:string)_ | Vars is the variables to store if the rule matches. Variables do not<br />directly inflict any changes in the node object. However, they can be<br />referenced from other rules enabling more complex rule hierarchies,<br />without exposing intermediary output values as labels. |  | Optional: \{\} <br /> |
| `varsTemplate` _string_ | VarsTemplate specifies a template to expand for dynamically generating<br />multiple variables. Data (after template expansion) must be keys with an<br />optional value (<key>[=<value>]) separated by newlines. |  | Optional: \{\} <br /> |
| `taints` _[Taint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#taint-v1-core) array_ | Taints to create if the rule matches. |  | Optional: \{\} <br /> |
| `extendedResources` _object (keys:string, values:string)_ | ExtendedResources to create if the rule matches. |  | Optional: \{\} <br /> |
| `matchFeatures` _[FeatureMatcher](#featurematcher)_ | MatchFeatures specifies a set of matcher terms all of which must match. |  | Optional: \{\} <br /> |
| `matchAny` _[MatchAnyElem](#matchanyelem) array_ | MatchAny specifies a list of matchers one of which must match. |  | Optional: \{\} <br /> |


#### ValueType

_Underlying type:_ _string_

ValueType represents the type of value in the expression.



_Appears in:_
- [MatchExpression](#matchexpression)

| Field | Description |
| --- | --- |
| `version` | TypeVersion represents a version with the following supported formats (major.minor.patch):<br />%d.%d.%d (e.g., 1.2.3),<br />%d.%d (e.g., 1.2),<br />%d (e.g., 1)<br /> |


