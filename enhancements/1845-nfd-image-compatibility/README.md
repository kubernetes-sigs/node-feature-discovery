# KEP-1845: Image Compatibility with NFD

## Summary

Currently, there is no standard solution for describing container image requirements in relation to hardware or operating systems.
Cloud-native technologies are being adopted by high-demand industries where container compatibility is critical for service performance and cluster preparation.
This proposal introduces the concept of NFD image compatibility metadata.
NFD features via NodeFeatureRules CRs can be effectively added to images to specify requirements for a host or operating system.

The document has been prepared based on the experience and progress of the [OCI Image Compatibility working group](https://github.com/opencontainers/wg-image-compatibility/tree/main/docs/proposals).

## Motivation

Image compatibility metadata will help container image authors describe compatibility requirements in a standardized way.
This metadata will be uploaded with the image to the image registry.
As a result, container compatibility requirements will become discoverable and programmable, supporting various consumers and use cases where applications require a specific compatible environment.

### Goals

#### Phase 1

- Use existing NFD features via the NodeFeatureRule API to describe container image requirements.
- Create a new OCI artifact type for compatibility metadata.
- Allow verification of node compatibilitym including nodes that are not yet part of the k8s cluster.
- Add or extend the sources with missing features.

#### Phase 2

Phase 2 involves future prediction and shows the general direction.
After the completion of Phase 1, either this document should be updated, or a new proposal should be created that considers the following points:

- Update or generate pods with appropriate node selectors via a mutation webhook or a scheduler plugin.

### Non-Goals

- Make image compatibility a hard requirement for the NFD installation/usage.
- Cover applications ABI compatibility.

## Proposal

Build a new NFD client tool with the following initial scope:

- CRUD OCI artifact.
- Validate nodes based on provided metadata.
- Run directly on a host which is not part of the Kubernetes cluster, or run as a Kubernetes job on a Kubernetes node.

### Design Details

#### OCI Artifact

[An OCI artifact](https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidelines-for-artifact-usage) should be created to store image compatibility metadata on the image side.  
The artifact can be connected with an image over [the subject field](https://github.com/opencontainers/distribution-spec/blob/11b8e3fba7d2d7329513d0cff53058243c334858/spec.md#pushing-manifests-with-subject).

##### Manifest

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/vnd.k8s.nfd.image-compatibility.v1",
  "config": {
    "mediaType": "application/vnd.oci.empty.v1+json",
    "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
    "size": 2
  },
  "layers": [
    {
      "mediaType": "application/vnd.k8s.nfd.image-compatibility.spec.v1+yaml",
      "digest": "sha256:4a47f8ae4c713906618413cb9795824d09eeadf948729e213a1ba11a1e31d052",
      "size": 1710
    }
  ],
  "subject": {
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "digest": "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270",
    "size": 7682
  },
  "annotations": {
    "oci.opencontainers.image.created": "2024-03-27T08:08:08Z"
  }
}
```

##### Artifact Payload (Schema)

- **version** - *string*  
This REQUIRED property specifies the version of the API being used.

- **compatibilities** - *array of object*  
This REQUIRED property is a list of compatibility sets.

  - **rules** - *object*  
  This REQUIRED property is a reference to the spec of [NodeFeatureRule API](https://kubernetes-sigs.github.io/node-feature-discovery/v0.16/usage/custom-resources.html#nodefeaturerule).
  The spec makes it possible to describe image requirements using the discovered features from NFD sources.
  For further reading, please review [the documentation](https://kubernetes-sigs.github.io/node-feature-discovery/v0.16/usage/customization-guide.html#nodefeaturerule-custom-resource).

  - **weight** - *int*  
  This OPTIONAL property specify the [node affinity weight](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity-weight).

  - **tag** - *string*  
  This OPTIONAL property allows grouping or dividing of compatibility sets.

  - **description** - *string*  
  This OPTIONAL property is intended for a brief description of a compatibility set.

Example

```yaml
version: v1alpha1
compatibilities:
- tag: "prefered"
  weight: 10
  description: "Prefered node configuration"
  rules:
  - name: "kernel and cpu"
    matchFeatures:
    - feature: kernel.loadedmodule
      matchExpressions:
        vfio-pci: {op: Exists}
    - feature: cpu.model
      matchExpressions:
        vendor_id: {op: In, value: ["Intel", "Amd"]}
- tag: "fallback"
  weight: 1
  description: "Minimal required configuration"
  rules:
  - name: "cpu"
    matchFeatures:
    - feature: cpu.model
      matchExpressions:
        vendor_id: {op: In, value: ["Intel", "Amd"]}
```

##### Discovery

A compatibility artifact shall be associated with either an image index or a specific image via the subject field of the OCI Image Spec.
The Referrers API should be used to discover artifacts.
If an image has multiple artifacts, it is up to the client to choose the correct one.
By default, it is recommended to select the most recent artifact based on the 'created' timestamp.

#### NFD client

A new standalone command-line utility should be implemented for the NFD project that shares the same functionality as the [nfd kubectl plugin](https://nfd.sigs.k8s.io/usage/kubectl-plugin).
Both clients should implemented the following commands:

- `validate` -  validate a NodeFeatureRule object (implemented in kubectl plugin).
- `test` - test a NodeFeatureRule object against a node (implemented in kubectl plugin).
- `dryrun` - process a NodeFeatureRule file against a local NodeFeature file to dry run the rule against a node before applying it to a cluster (implemented in kubectl plugin).
- `compat` - compatibility command with the following subcommands:
  - `attach-spec` - create an artifact with image compatibility specification and attach to the image (initially users have to create the spec by hand).
  - `remove-spec` - remove an artifact with image compatibility specification from the image.
  - `validate-spec` - validate an artifact and image compatibility specification.
  - `validate-node` - validate image compatibility against a node.

### Test Plan

To ensure the proper functioning of the nfd client, the following test plan should be executed:

- **Unit Tests:** Write unit tests for the client.
- **Manual e2e Tests:** Run nfd client with sample data to CRUD artifact and validate a local host.
