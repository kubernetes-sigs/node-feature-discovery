---
title: "Developer guide"
layout: default
sort: 5
---

# Developer guide
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

## Building from source

### Download the source code

```bash
git clone https://github.com/kubernetes-sigs/node-feature-discovery
cd node-feature-discovery
```

### Docker build

#### Build the container image

See [customizing the build](#customizing-the-build) below for altering the
container image registry, for example.

```bash
make
```

#### Push the container image

Optional, this example with Docker.

```bash
docker push <IMAGE_TAG>
```

### Docker multi-arch builds with buildx

The default set of architectures enabled for mulit-arch builds are `linux/amd64`
and `linux/arm64`. If more architectures are needed one can override the
`IMAGE_ALL_PLATFORMS` variable with a comma separated list of `OS/ARCH` tuples.

#### Build the manifest-list with a container image per arch

```bash
make image-all
```

Currently `docker` does not support loading of manifest-lists meaning the images
are not shown when executing `docker images`, see:
[buildx issue #59](https://github.com/docker/buildx/issues/59).

#### Push the manifest-list with container image per arch

```bash
make push-all
```

The resulting container image can be used in the same way on each arch by pulling
e.g. `node-feature-discovery:{{ site.release }}` without specifying the
architecture. The manifest-list will take care of providing the right
architecture image.

#### Change the job spec to use your custom image (optional)

To use your published image from the step above instead of the
`registry.k8s.io/nfd/node-feature-discovery` image, edit `image`
attribute in the spec template(s) to the new location
(`<registry-name>/<image-name>[:<version>]`).

### Deployment

The `yamls` makefile generates a `kustomization.yaml` matching your locally
built image and using the `deploy/overlays/default` deployment. See
[build customization](#customizing-the-build) below for configurability, e.g.
changing the deployment namespace.

```bash
K8S_NAMESPACE=my-ns make yamls
kubectl apply -k .
```

You can use alternative deployment methods by modifying the auto-generated
kustomization file. For example, deploying worker and master in the same pod by
pointing to `deployment/overlays/default-combined`.

### Building locally

You can also build the binaries locally

```bash
make build
```

This will compile binaries under `bin/`

### Customizing the build

There are several Makefile variables that control the build process and the
name of the resulting container image. The following are targeted targeted for
build customization and they can be specified via environment variables or
makefile overrides.

| Variable                   | Description                                                       | Default value
| -------------------------- | ----------------------------------------------------------------- | ----------- |
| HOSTMOUNT_PREFIX           | Prefix of system directories for feature discovery (local builds) | / (*local builds*) /host- (*container builds*)
| IMAGE_BUILD_CMD            | Command to build the image                                        | docker build
| IMAGE_BUILD_EXTRA_OPTS     | Extra options to pass to build command                            | *empty*
| IMAGE_BUILDX_CMD           | Command to build and push multi-arch images with buildx           | DOCKER_CLI_EXPERIMENTAL=enabled docker buildx build --platform=${IMAGE_ALL_PLATFORMS} --progress=auto --pull
| IMAGE_ALL_PLATFORMS        | Comma separated list of OS/ARCH tuples for mulit-arch builds    | linux/amd64,linux/arm64
| IMAGE_PUSH_CMD             | Command to push the image to remote registry                      | docker push
| IMAGE_REGISTRY             | Container image registry to use                                   | registry.k8s.io/nfd
| IMAGE_TAG_NAME             | Container image tag name                                          | &lt;nfd version&gt;
| IMAGE_EXTRA_TAG_NAMES      | Additional container image tag(s) to create when building image   | *empty*
| K8S_NAMESPACE              | nfd-master and nfd-worker namespace                               | node-feature-discovery
| KUBECONFIG                 | Kubeconfig for running e2e-tests                                  | *empty*
| E2E_TEST_CONFIG            | Parameterization file of e2e-tests (see [example][e2e-config-sample]) | *empty*
| E2E_PULL_IF_NOT_PRESENT    | True-ish value makes the image pull policy IfNotPresent (to be used only in e2e tests) | false
| OPENSHIFT                  | Non-empty value enables OpenShift specific support (currently only effective in e2e tests) | *empty*

For example, to use a custom registry:

```bash
make IMAGE_REGISTRY=<my custom registry uri>
```

Or to specify a build tool different from Docker, It can be done in 2 ways:

1. via environment

    ```bash
    IMAGE_BUILD_CMD="buildah bud" make
    ```

1. by overriding the variable value

    ```bash
    make  IMAGE_BUILD_CMD="buildah bud"
    ```

### Testing

Unit tests are automatically run as part of the container image build. You can
also run them manually in the source code tree by simply running:

```bash
make test
```

End-to-end tests are built on top of the e2e test framework of Kubernetes, and,
they required a cluster to run them on. For running the tests on your test
cluster you need to specify the kubeconfig to be used:

```bash
make e2e-test KUBECONFIG=$HOME/.kube/config
```

## Running locally

You can run NFD locally, either directly on your host OS or in containers for
testing and development purposes. This may be useful e.g. for checking
features-detection.

### NFD-Master

When running as a standalone container labeling is expected to fail because
Kubernetes API is not available. Thus, it is recommended to use `-no-publish`
command line flag. E.g.

```bash
$ export NFD_CONTAINER_IMAGE={{ site.container_image }}
$ docker run --rm --name=nfd-test ${NFD_CONTAINER_IMAGE} nfd-master -no-publish
2019/02/01 14:48:21 Node Feature Discovery Master <NFD_VERSION>
2019/02/01 14:48:21 gRPC server serving on port: 8080
```

### NFD-Worker

In order to run nfd-worker as a "stand-alone" container against your
standalone nfd-master you need to run them in the same network namespace:

```bash
$ docker run --rm --network=container:nfd-test ${NFD_CONTAINER_IMAGE} nfd-worker
2019/02/01 14:48:56 Node Feature Discovery Worker <NFD_VERSION>
...
```

If you just want to try out feature discovery without connecting to nfd-master,
pass the `-no-publish` flag to nfd-worker.

**NOTE** Some feature sources need certain directories and/or files from the
host mounted inside the NFD container. Thus, you need to provide Docker with the
correct `--volume` options in order for them to work correctly when run
stand-alone directly with `docker run`. See the
[default deployment](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/common/worker-mounts.yaml)
for up-to-date information about the required volume mounts.

### NFD-Topology-Updater

In order to run nfd-topology-updater as a "stand-alone" container against your
standalone nfd-master you need to run them in the same network namespace:

```bash
$ docker run --rm --network=container:nfd-test ${NFD_CONTAINER_IMAGE} nfd-topology-updater
2019/02/01 14:48:56 Node Feature Discovery Topology Updater <NFD_VERSION>
...
```

If you just want to try out feature discovery without connecting to nfd-master,
pass the `-no-publish` flag to nfd-topology-updater.

NOTE:

NFD topology updater needs certain directories and/or files from the
host mounted inside the NFD container. Thus, you need to provide Docker with the
correct `--volume` options in order for them to work correctly when run
stand-alone directly with `docker run`. See the
[template spec](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/deployment/components/topology-updater/topologyupdater-mounts.yaml)
for up-to-date information about the required volume mounts.

[PodResource API][podresource-api] is a prerequisite for nfd-topology-updater.
Preceding Kubernetes v1.23, the `kubelet` must be started with the following flag:
`--feature-gates=KubeletPodResourcesGetAllocatable=true`.
Starting Kubernetes v1.23, the `GetAllocatableResources` is enabled by default
through `KubeletPodResourcesGetAllocatable` [feature gate][feature-gate].

## Running with Tilt

Another option for building NFD locally is via Tilt tool, which can build container
images, push them to a local registry and reload your Kubernetes pods automatically.
When using Tilt, you don't have to build container images and re-deploy your pods
manually but instead let the Tilt take care of it. Tiltfile is a configuration file
for the Tilt and is located at the root directory. To develop NFD with Tilt, follow
the steps below.

### Prerequisites

1. Install [Docker](https://docs.docker.com/engine/install/)
1. Setup Docker as a non-root user.
1. Install [kubectl](https://kubernetes.io/docs/tasks/tools/)
1. Install [kustomize](https://github.com/kubernetes-sigs/kustomize)
1. Install [tilt](https://docs.tilt.dev/install.html)
1. Create a local Kubernetes cluster

To start up your Tilt development environment, just run

```shell
tilt up
```

at the root of your local NFD codebase. Tilt will start a web interface in the
localhost and port 10350. From the web interface, you are able to see how NFD worker
and master are progressing,  watch their build and runtime logs. Once your code changes
are saved locally, Tilt will notice it and re-build the container image from the
current code, push the image to the registry and re-deploy NFD pods with the latest
container image.

### Environment variables

To override environment variables used in the Tiltfile during image build,
export them in your current terminal before starting Tilt.

```shell
export IMAGE_TAG_NAME="v1"
tilt up
```

This will override the default value(`master`) of `IMAGE_TAG_NAME` variable defined
in the Tiltfile.

## Documentation

All documentation resides under the
[docs](https://github.com/kubernetes-sigs/node-feature-discovery/tree/{{site.release}}/docs)
directory in the source tree. It is designed to be served as a html site by
[GitHub Pages](https://pages.github.com/).

Building the documentation is containerized in order to fix the build
environment. The recommended way for developing documentation is to run:

```bash
make site-serve
```

This will build the documentation in a container and serve it under
[localhost:4000/](http://localhost:4000/) making it easy to verify the results.
Any changes made to the `docs/` will automatically re-trigger a rebuild and are
reflected in the served content and can be inspected with a simple browser
refresh.

In order to just build the html documentation run:

```bash
make site-build
```

This will generate html documentation under `docs/_site/`.

<!-- Links -->
[e2e-config-sample]: https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{site.release}}/test/e2e/e2e-test-config.exapmle.yaml
[podresource-api]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/#monitoring-device-plugin-resources
[feature-gate]: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates
