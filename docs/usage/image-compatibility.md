---
title: "Image Compatibility Artifact"
layout: default
sort: 11
---

# Image Compatibility Artifact
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

## Image Compatibility

**Image Compatibility is in the experimental `v1alpha1` version.**

Image compatibility metadata enables container image authors to define their
image requirements using a spec similar to
[Node Feature Groups](./custom-resources.md#nodefeaturegroup).
This complementary solution allows features discovered on nodes to be matched
directly from images. As a result, container requirements become discoverable
and programmable, supporting various consumers and use cases where applications
need a specific compatible environment.

### Compatibility Specification

The compatibility specification is a list of compatibility objects that contain
rules similar to [Node Feature Groups](./custom-resources.md#nodefeaturegroup),
along with additional fields to control the execution of validation between the
image and the host.

### Schema

- **version** - *string*  
  This REQUIRED property specifies the version of the API in use.

- **compatibilities** - *array of object*  
  This REQUIRED property is a list of compatibility sets.

  - **rules** - *object*  
    This REQUIRED property is a reference to the spec of the [NodeFeatureGroup API](./custom-resources.md#nodefeaturegroup).
    The spec allows image requirements to be described using the features
    discovered from NFD sources. For more details, please refer to [the documentation](./custom-resources.md#nodefeaturegroup).

  - **weight** - *int*  
    This OPTIONAL property specifies the [node affinity weight](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity-weight).

  - **tag** - *string*  
    This OPTIONAL property allows for the grouping or separation of
    compatibility sets.

  - **description** - *string*  
    This OPTIONAL property provides a brief description of a compatibility set.

#### Example

```yaml
version: v1alpha1
compatibilities:
- description: "My image requirements"
  rules:
  - name: "kernel and cpu"
    matchFeatures:
    - feature: kernel.loadedmodule
      matchExpressions:
        vfio-pci: {op: Exists}
    - feature: cpu.model
      matchExpressions:
        vendor_id: {op: In, value: ["Intel", "AMD"]}
  - name: "one of available nics"
    matchAny:
    - matchFeatures:
      - feature: pci.device
        matchExpressions:
          vendor: {op: In, value: ["0eee"]}
          class: {op: In, value: ["0200"]}
    - matchFeatures:
      - feature: pci.device
        matchExpressions:
          vendor: {op: In, value: ["0fff"]}
          class: {op: In, value: ["0200"]}
```

### OCI Artifact

An [OCI artifact](https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidelines-for-artifact-usage)
is used to store image compatibility metadata.
The artifact can be associated with a specific image through [the subject field](https://github.com/opencontainers/distribution-spec/blob/11b8e3fba7d2d7329513d0cff53058243c334858/spec.md#pushing-manifests-with-subject)
and pushed to the registry along with the image.

Example manifest:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/vnd.nfd.image-compatibility.v1alpha1",
  "config": {
    "mediaType": "application/vnd.oci.empty.v1+json",
    "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
    "size": 2
  },
  "layers": [
    {
      "mediaType": "application/vnd.nfd.image-compatibility.spec.v1alpha1+yaml",
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

#### Attach the artifact to the image

Create an image compatibility specification for the image, then install the
[ORAS](https://github.com/oras-project/oras/) tool and execute `oras attach`
command.

Example:

```sh
oras attach --artifact-type application/vnd.nfd.image-compatibility.v1alpha1 \
<image-url> <path-to-spec>.yaml:application/vnd.nfd.image-compatibility.spec.v1alpha1+yaml
```

**Note**: The attach command is planned to be integrated into the `nfd` client
tool. This will streamline the process, allowing you to perform the operation
directly within the tool rather than using a separate command.

### Validate the host against the image compatibility specification

1. Build `nfd` client: `make build`
1. Run `./bin/nfd compat validate-node --image <image-url>`

For more information about the available commands and flags, refer to
[the client documentation](../reference/node-feature-client-reference.md).

### Validate the k8s cluster node with the validate-image Job

**Note**: This does not require installation of NFD master and workers.
Additionally, public registry certificates must be included in the job.
In the example below, this is done using hostPath,
but it can be done using any Kubernetes-supported method.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: validate-image
spec:
  backoffLimit: 1
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: image-compatibility
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
        image: <image-with-nfd-client>
        command: ["nfd", "compat", "validate-node", "--image", "<image-to-be-validated>"]
        volumeMounts:
          - mountPath: /host-boot
            name: host-boot  
            readOnly: true
          - mountPath: /host-etc/os-release
            name: host-os-release
            readOnly: true
          - mountPath: /host-sys
            name: host-sys
            readOnly: true
          - mountPath: /host-usr/lib
            name: host-usr-lib
            readOnly: true
          - mountPath: /host-lib
            name: host-lib
            readOnly: true
          - mountPath: /host-proc
            name: host-proc
            readOnly: true
      volumes:
      - hostPath:
          path: /boot
          type: ""
        name: host-boot
      - hostPath:
          path: /etc/os-release
          type: ""
        name: host-os-release
      - hostPath:
          path: /sys
          type: ""
        name: host-sys
      - hostPath:
          path: /usr/lib
          type: ""
        name: host-usr-lib
      - hostPath:
          path: /lib
          type: ""
        name: host-lib
      - hostPath:
          path: /proc
          type: ""
        name: host-proc
      - hostPath:
          path: "<path-to-registry-public-certs>"
          type: ""
        name: certs
```
