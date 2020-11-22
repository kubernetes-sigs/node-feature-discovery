---
title: "Developer Guide"
layout: default
sort: 1
---

# Developer Guide
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Building from Source

### Download the source code

```bash
git clone https://github.com/kubernetes-sigs/node-feature-discovery
cd node-feature-discovery
```

### Docker Build

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

#### Change the job spec to use your custom image (optional)

To use your published image from the step above instead of the
`k8s.gcr.io/nfd/node-feature-discovery` image, edit `image`
attribute in the spec template(s) to the new location
(`<registry-name>/<image-name>[:<version>]`).

### Deployment

The `yamls` makefile generates deployment specs matching your locally built
image. See [build customization](#customizing-the-build) below for
configurability, e.g. changing the deployment namespace.

```bash
K8S_NAMESPACE=my-ns make yamls
kubectl apply -f nfd-master.yaml
kubectl apply -f nfd-worker-daemonset.yaml
```

Alternatively, deploying worker and master in the same pod:

```bash
K8S_NAMESPACE=my-ns make yamls
kubectl apply -f nfd-master.yaml
kubectl apply -f nfd-daemonset-combined.yaml
```

Or worker as a one-shot job:

```bash
K8S_NAMESPACE=my-ns make yamls
kubectl apply -f nfd-master.yaml
NUM_NODES=$(kubectl get no -o jsonpath='{.items[*].metadata.name}' | wc -w)
sed s"/NUM_NODES/$NUM_NODES/" nfd-worker-job.yaml | kubectl apply -f -
```

### Building Locally

You can also build the binaries locally

```bash
make build
```

This will compile binaries under `bin/`

### Customizing the Build

There are several Makefile variables that control the build process and the
name of the resulting container image. The following are targeted targeted for
build customization and they can be specified via environment variables or
makefile overrides.

| Variable                   | Description                                                       | Default value
| -------------------------- | ----------------------------------------------------------------- | ----------- |
| HOSTMOUNT_PREFIX           | Prefix of system directories for feature discovery (local builds) | / (*local builds*) /host- (*container builds*)
| IMAGE_BUILD_CMD            | Command to build the image                                        | docker build
| IMAGE_BUILD_EXTRA_OPTS     | Extra options to pass to build command                            | *empty*
| IMAGE_PUSH_CMD             | Command to push the image to remote registry                      | docker push
| IMAGE_REGISTRY             | Container image registry to use                                   | k8s.gcr.io/nfd
| IMAGE_TAG_NAME             | Container image tag name                                          | &lt;nfd version&gt;
| IMAGE_EXTRA_TAG_NAMES      | Additional container image tag(s) to create when building image   | *empty*
| K8S_NAMESPACE              | nfd-master and nfd-worker namespace                               | kube-system
| KUBECONFIG                 | Kubeconfig for running e2e-tests                                  | *empty*
| E2E_TEST_CONFIG            | Parameterization file of e2e-tests (see [example][e2e-config-sample]) | *empty*

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

## Running Locally

You can run NFD locally, either directly on your host OS or in containers for
testing and development purposes. This may be useful e.g. for checking
features-detection.

### NFD-Master

When running as a standalone container labeling is expected to fail because
Kubernetes API is not available. Thus, it is recommended to use `--no-publish`
command line flag. E.g.

```bash
$ export NFD_CONTAINER_IMAGE={{ site.container_image }}
$ docker run --rm --name=nfd-test ${NFD_CONTAINER_IMAGE} nfd-master --no-publish
2019/02/01 14:48:21 Node Feature Discovery Master <NFD_VERSION>
2019/02/01 14:48:21 gRPC server serving on port: 8080
```

Command line flags of nfd-master:

```bash
$ docker run --rm ${NFD_CONTAINER_IMAGE} nfd-master --help
...
Usage:
  nfd-master [--prune] [--no-publish] [--label-whitelist=<pattern>] [--port=<port>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
     [--verify-node-name] [--extra-label-ns=<list>] [--resource-labels=<list>]
     [--kubeconfig=<path>]
  nfd-master -h | --help
  nfd-master --version

  Options:
  -h --help                       Show this screen.
  --version                       Output version and exit.
  --prune                         Prune all NFD related attributes from all nodes
                                  of the cluster and exit.
  --kubeconfig=<path>             Kubeconfig to use [Default: ]
  --port=<port>                   Port on which to listen for connections.
                                  [Default: 8080]
  --ca-file=<path>                Root certificate for verifying connections
                                  [Default: ]
  --cert-file=<path>              Certificate used for authenticating connections
                                  [Default: ]
  --key-file=<path>               Private key matching --cert-file
                                  [Default: ]
  --verify-node-name              Verify worker node name against CN from the TLS
                                  certificate. Only has effect when TLS authentication
                                  has been enabled.
  --no-publish                    Do not publish feature labels
  --label-whitelist=<pattern>     Regular expression to filter label names to
                                  publish to the Kubernetes API server.
                                  NB: the label namespace is omitted i.e. the filter
                                  is only applied to the name part after '/'.
                                  [Default: ]
  --extra-label-ns=<list>         Comma separated list of allowed extra label namespaces
                                  [Default: ]
  --resource-labels=<list>        Comma separated list of labels to be exposed as extended resources.
                                  [Default: ]
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
pass the `--no-publish` flag to nfd-worker.

Command line flags of nfd-worker:

```bash
$ docker run --rm ${NFD_CONTAINER_IMAGE} nfd-worker --help
...
  Usage:
  nfd-worker [--no-publish] [--sources=<sources>] [--label-whitelist=<pattern>]
     [--oneshot | --sleep-interval=<seconds>] [--config=<path>]
     [--options=<config>] [--server=<server>] [--server-name-override=<name>]
     [--ca-file=<path>] [--cert-file=<path>] [--key-file=<path>]
  nfd-worker -h | --help
  nfd-worker --version

  Options:
  -h --help                   Show this screen.
  --version                   Output version and exit.
  --config=<path>             Config file to use.
                              [Default: /etc/kubernetes/node-feature-discovery/nfd-worker.conf]
  --options=<config>          Specify config options from command line. Config
                              options are specified in the same format as in the
                              config file (i.e. json or yaml). These options
                              will override settings read from the config file.
                              [Default: ]
  --ca-file=<path>            Root certificate for verifying connections
                              [Default: ]
  --cert-file=<path>          Certificate used for authenticating connections
                              [Default: ]
  --key-file=<path>           Private key matching --cert-file
                              [Default: ]
  --server=<server>           NFD server address to connecto to.
                              [Default: localhost:8080]
  --server-name-override=<name> Name (CN) expect from server certificate, useful
                              in testing
                              [Default: ]
  --sources=<sources>         Comma separated list of feature sources. Special
                              value 'all' enables all feature sources.
                              [Default: all]
  --no-publish                Do not publish discovered features to the
                              cluster-local Kubernetes API server.
  --label-whitelist=<pattern> Regular expression to filter label names to
                              publish to the Kubernetes API server.
                              NB: the label namespace is omitted i.e. the filter
                              is only applied to the name part after '/'.
                              [Default: ]
  --oneshot                   Label once and exit.
  --sleep-interval=<seconds>  Time to sleep between re-labeling. Non-positive
                              value implies no re-labeling (i.e. infinite
                              sleep). [Default: 60s]

```

**NOTE** Some feature sources need certain directories and/or files from the
host mounted inside the NFD container. Thus, you need to provide Docker with the
correct `--volume` options in order for them to work correctly when run
stand-alone directly with `docker run`. See the
[template spec](https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{ site.release }}/nfd-worker-daemonset.yaml.template)
for up-to-date information about the required volume mounts.

## Documentation

All documentation resides under the
[docs](https://github.com/kubernetes-sigs/node-feature-discovery/tree/{{ site.release }}/docs)
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
[e2e-config-sample]: https://github.com/kubernetes-sigs/node-feature-discovery/blob/{{ site.release }}/test/e2e/e2e-test-config.exapmle.yaml
