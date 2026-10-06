# KEP-2602: NFD Operator in node-feature-discovery
<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [Repository layout](#repository-layout)
  - [Dependencies](#dependencies)
  - [Operand objects](#operand-objects)
  - [Image](#image)
  - [CRD ownership](#crd-ownership)
  - [Versioning](#versioning)
  - [OLM bundle](#olm-bundle)
  - [Pull request roadmap](#pull-request-roadmap)
  - [Test Plan](#test-plan)
  - [Graduation Criteria](#graduation-criteria)
- [Implementation History](#implementation-history)
- [Alternatives Considered](#alternatives-considered)
- [Open Questions](#open-questions)
<!-- /toc -->

## Summary

This proposal moves the NFD Operator from
[kubernetes-sigs/node-feature-discovery-operator](https://github.com/kubernetes-sigs/node-feature-discovery-operator)
into this repository. The operator then ships with NFD's version, image and release notes, deploys an
operand of its own version by default, and is tested by NFD's CI on every pull request. Existing
`NodeFeatureDiscovery` resources (`nfd.kubernetes.io/v1`) keep working without conversion.

Tracking issue: [#2602](https://github.com/kubernetes-sigs/node-feature-discovery/issues/2602).

## Motivation

The operator and NFD have drifted apart. The operator's last release is v0.6.0 (2023-03-21), and its
hand-built operand objects did not follow NFD's changes. With NFD v0.18 and later, the operand that
operator master deployed did not work: nfd-master was started with `--port=12000` while its probes
checked port 8080, so it restarted in a loop and no node was labelled; nfd-worker could not read its own
pod, and nfd-gc could not list nodes when installed outside the `default` namespace. Separately, the
operator pod itself stayed at 1/2 because its kube-rbac-proxy image was no longer published. Every NFD
change to flags, RBAC or CRDs has to be repeated in a second repository, by hand, and no CI job tests the
two together.

The operator was first brought up to date in its own repository (Go 1.26, k8s.io v0.35, controller-runtime
v0.23, operand NFD v0.19.0) in
[kubernetes-sigs/node-feature-discovery-operator#271](https://github.com/kubernetes-sigs/node-feature-discovery-operator/pull/271).
That removes the dependency churn from this move, so the pull requests here contain only the move and the
integration work.

In [kubernetes-sigs/node-feature-discovery-operator#251](https://github.com/kubernetes-sigs/node-feature-discovery-operator/issues/251),
@yevgeny-shnaidman proposed a definition of done for merging the two projects: the operand and operator
code live in one repository; that repository supports deploying either the operator or the operand; and
pull requests must pass a test that deploys NFD through the operator. The goals below include all three.

### Goals

- One repository and one release line for NFD and the NFD Operator.
- The operator deploys an operand of its own version by default, so upgrading the operator upgrades NFD.
- Existing `nfd.kubernetes.io/v1` `NodeFeatureDiscovery` resources keep working after the upgrade, with no
  conversion webhook, once `spec.operand.image` is unset or points at NFD v0.18.0 or later (resources made
  from the operator v0.6.0 sample pin v0.12.1; the migration guide covers the change). No field that a
  v0.6.0 resource may carry is rejected.
- A required presubmit that deploys NFD through the operator and runs on every pull request, next to the
  operand e2e tests, so an operand change that the operator does not follow fails CI.
- Installing NFD with Helm or kustomize works exactly as before.

### Non-Goals

- Publishing the operator on OperatorHub or through an OLM catalog in the first release (see
  [OLM bundle](#olm-bundle)).
- New `NodeFeatureDiscovery` fields or a redesign of the API.
- A conversion webhook or a new API version.
- Support for operand versions older than NFD v0.18.0.

## Proposal

The operator's code, its Helm chart and its kustomize and OLM manifests move into this repository in a
series of pull requests (see [Pull request roadmap](#pull-request-roadmap)). The first ones bring the code
in without shipping it; later ones add the binary to the NFD image with the operand image default, an
operator e2e test that runs on every pull request, the operator chart publishing and the documentation.
The operator keeps building its operand the way it does today (see [Operand objects](#operand-objects)).

### User Stories

- As a user who installs NFD with Helm or kustomize, nothing changes for me. The operand chart and the
  kustomize overlays stay as they are.
- As a user of the operator v0.6.0, I upgrade to the operator released with NFD. My
  `NodeFeatureDiscovery` resource keeps working once its operand image is NFD v0.18.0 or later, and a
  migration guide tells me how to upgrade without deleting the CRD (and with it my resource and the node
  labels).
- As a downstream distributor (for example the OpenShift NFD operator), I rebase on one repository whose
  operator version always matches the operand version.

### Risks and Mitigations

- **Node labels wiped during an upgrade.** Removing the old operator with its kustomize manifests
  (`make undeploy`), or deleting the CRD by hand, deletes the `NodeFeatureDiscovery` CRD, and with it the
  resource and the operand; with `prunerOnDelete` set, the node labels go too. (`helm uninstall` keeps
  CRDs that were installed from the chart's `crds/` directory.) Mitigation: the migration guide upgrades in
  place and keeps the CRD, and an upgrade e2e test from operator v0.6.0 asserts that node labels survive.
- **The operator and the operand chart both install the NFD CRDs.** Mitigation: see
  [CRD ownership](#crd-ownership).
- **Importing the history fails the CLA check.** The import keeps the operator's git history (Tide merges
  with merge commits in this repository). If EasyCLA rejects historical commits, the import falls back to
  a single squashed commit that links to the operator repository.
- **controller-runtime lags behind client-go.** The main module gains a dependency on
  controller-runtime, which is released after each Kubernetes minor and can hold back NFD's k8s.io bumps.
  Mitigation: if that happens, the operator code (`pkg/operator`, `cmd/nfd-operator`) moves under one
  directory with its own `go.mod`, which takes controller-runtime out of the main module.
- **The operator's operand objects drift from the operand chart.** The operator keeps its own copy of the
  operand's flags, probes and RBAC (see [Operand objects](#operand-objects)), so an operand change has to
  be repeated there. Mitigation: the operator e2e test runs on every pull request, next to the operand e2e
  tests, so an operand change that the operator does not follow fails CI. The test only covers what it
  runs: a flag used only on a path the test does not reach, or RBAC that grants more than the operand
  needs, does not fail it and is left to review.
- **Release timing.** If any of the pull requests that ship the operator (3 to 6 in the roadmap) is not
  merged before the v0.20 branch is cut, v0.20 ships only the code that is not user-visible yet, and the
  operator ships in v0.21.

## Design Details

### Repository layout

| Operator repository | This repository |
|---|---|
| `api/v1/` | `api/operator/v1/` |
| `internal/*` | `pkg/operator/*` |
| `main.go`, `main_test.go` | `cmd/nfd-operator/` |
| `deploy/helm/nfd-operator/` | `deployment/helm/nfd-operator/` |
| `config/` | `deployment/operator/config/` |
| `bundle/`, `bundle.Dockerfile`, `PROJECT` | `deployment/operator/` |

The operator repository's own build, CI and repository files (Makefile, go.mod, Dockerfiles, `.github`,
`scripts`, `OWNERS`, `README.md` and similar) are not imported; this repository's equivalents take over.
Its documentation is migrated into `docs/` by a later pull request.

Go code that imports the operator types changes its import path from
`sigs.k8s.io/node-feature-discovery-operator/api/v1` to `sigs.k8s.io/node-feature-discovery/api/operator/v1`.
The API group and version, `nfd.kubernetes.io/v1`, do not change, so existing `NodeFeatureDiscovery`
resources are not affected.

### Dependencies

The operator adds `sigs.k8s.io/controller-runtime` (v0.23.3, which builds with the k8s.io v0.35 modules
NFD already uses) and `go.uber.org/mock` (for its existing unit tests) to the main module.

The operator API is its own Go module, `sigs.k8s.io/node-feature-discovery/api/operator`, like `api/nfd`,
so that other projects can import the `NodeFeatureDiscovery` types without the main module's
dependencies. Its scheme registration moves from controller-runtime's `scheme.Builder` to
`runtime.NewSchemeBuilder` from `k8s.io/apimachinery`, the way `api/nfd` registers its types, so the module
needs only `k8s.io/api` and `k8s.io/apimachinery`.

### Operand objects

The operator keeps building the operand the way it does today. Its Go code creates the nfd-master and
nfd-gc Deployments, the nfd-worker and nfd-topology-updater DaemonSets, the nfd-worker ConfigMap and the
prune Job from the `NodeFeatureDiscovery` spec (`pkg/operator/deployment`, `daemonset`, `configmap` and
`job`), and the operand's ServiceAccounts and RBAC ship with the operator chart and its kustomize
manifests. When a pull request changes the operand's flags, probes or RBAC, the operator's copy changes in
the same pull request, and the operator e2e test (4) is the check that catches a missed one (see
[Risks and Mitigations](#risks-and-mitigations) for what it does not cover).

### Image

The `nfd-operator` binary is added to the existing NFD image, so no new image needs to be promoted. The
operator's default operand image is its own image, set by the operator chart through an environment
variable; `spec.operand.image` overrides it. The operator and the operand therefore have the same version
by default.

### CRD ownership

The operator chart owns the `NodeFeatureDiscovery` CRD. The NFD CRDs (`NodeFeature`, `NodeFeatureRule`,
`NodeFeatureGroup`) live in the operand chart's `crds/` directory. In operator mode they come from the
operator chart's `crds/` directory instead: `make generate` copies the same generated file there,
the way it already copies it into the operand chart, and a CI check added with the build wiring (3) fails
if the copies drift. The `NodeResourceTopology` CRD, which the operand chart renders from a template when
`topologyUpdater.enable` and `topologyUpdater.createCRDs` are both set, also ships in the operator chart's
`crds/` directory. The operator never applies a CRD and needs no permission to create or change one, and
every CRD has one owner.

Helm creates the CRDs in a chart's `crds/` directory when the chart is installed, and never creates or
changes them on `helm upgrade`. An upgrade of the operator chart therefore applies the CRDs with
`kubectl apply` first, as the operand chart's upgrade instructions already do (`docs/deployment/helm.md`).
From operator v0.6.0 the step is required: the v0.6.0 chart has no `NodeFeatureGroup` CRD, and its
`NodeFeatureDiscovery` CRD has no `spec.enableTaints`, so the API server rejects or drops that field until
the new CRD is applied. The migration guide (6) and the upgrade test from v0.6.0 include the step.

### Versioning

The operator's version jumps from its last release (v0.6.0 today) to the NFD version of the release that
first ships it. The
release notes say so, and link to the migration guide.

### OLM bundle

The bundle manifests move with the code. The build wiring (3) repairs `make bundle`, which fails on the
operator's master today (the ClusterServiceVersions carry the non-semver version `master`, and the bundle
holds a stale extra ClusterServiceVersion and a stale `NodeFeatureRule` CRD), and runs
`operator-sdk bundle validate` in CI. Publishing the bundle image and submitting it to community-operators
are deferred until after the first release, in coordination with the downstream distributors that use OLM.

### Pull request roadmap

| # | Pull request | Scope |
|---|---|---|
| 1 | This proposal | The design and the roadmap |
| 2 | Import the operator code | History and code in the paths above; builds and tests in NFD CI; not built into the image, not published, no docs |
| 3 | Build wiring and operand image | `nfd-operator` in the image and Makefile; `api/operator` as its own Go module; the operator's default operand image is its own image, set by the operator chart through an environment variable (`spec.operand.image` overrides it); code generation for the operator CRD, RBAC and mocks, with a CI check that fails when generated files or CRD copies drift; Helm lint for the operator chart; `make bundle` repaired and `operator-sdk bundle validate` in CI |
| 4 | Operator e2e | An operator mode for `test/e2e`: install the operator chart with the pull request's image, apply a `NodeFeatureDiscovery`, check the operand, node labels and a `NodeFeatureRule`; a required presubmit job that runs on every pull request, next to the operand e2e tests |
| 5 | Operator chart publishing | Chart documentation and values schema; publish the chart next to the operand chart |
| 6 | Docs and ownership | Operator deployment docs, a migration guide from operator v0.6.0, `OWNERS` for the operator code |

Two changes outside this repository go with them: a presubmit job in kubernetes/test-infra (with 4), and a
promoter entry in kubernetes/k8s.io for the operator chart (with 5); the operator binary needs no new
image entry because it ships in the NFD image.

After the operator ships, the operator repository gets a README that points here, its open issues and pull
requests are closed with pointers, and it is archived.

The operator ships once 1 to 6 are in. The end-to-end test (4) runs on every pull request from then on.

### Test Plan

- The operator's existing unit tests run in `make test` from the import on.
- An operator mode for the e2e tests that installs the operator from the pull request's image, applies a
  `NodeFeatureDiscovery`, waits for the operand and checks node labels and a `NodeFeatureRule`. It runs as a
  required presubmit on every pull request, next to the operand e2e tests.
- An upgrade test from operator v0.6.0: the CRDs are applied with `kubectl apply` before `helm upgrade`,
  as the migration guide does; afterwards all five CRDs carry the new schemas, the resource survives, the
  operand is reconciled and node labels are not removed. The v0.6.0 chart's kube-rbac-proxy image
  (`gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0`) is no longer published, so the test replaces it in the
  rendered manifests.

### Graduation Criteria

The operator ships in the first NFD release that contains pull requests 1 to 6, with the target of
v0.20. If any of them misses the v0.20 branch cut, v0.20 contains only the code that is not user-visible
yet, and the operator ships in v0.21.

## Implementation History

- 2026-09-29: the operator brought up to date in its own repository
  ([kubernetes-sigs/node-feature-discovery-operator#271](https://github.com/kubernetes-sigs/node-feature-discovery-operator/pull/271)).
- 2026-09-29: tracking issue [#2602](https://github.com/kubernetes-sigs/node-feature-discovery/issues/2602)
  and this proposal.
- 2026-10-02: after review, the operand image default and the operator e2e test come first, and rendering
  the operand from the chart becomes a later step.
- 2026-10-04: after review, rendering the operand from the chart is dropped from this proposal; the
  operator keeps building its operand in Go (see [Alternatives Considered](#alternatives-considered)).
- 2026-10-06: after review, the operator API becomes its own Go module, and the import path change for Go
  users is written down.
- 2026-10-06: after review, an upgrade applies the CRDs with `kubectl apply` before `helm upgrade`.

## Alternatives Considered

- **Keep two repositories.** Every change to the operand's flags, RBAC or CRDs would still have to be
  copied by hand, and nothing would test the two together. This is how the operator's operand stopped
  working with NFD v0.18.
- **Deprecate the operator and support Helm only.** Users and distributions that deploy through the
  operator or OLM would lose their install path, and the definition of done in
  kubernetes-sigs/node-feature-discovery-operator#251 asks for operator deployments to keep working.
- **Replace the operator with an operator-sdk Helm-based operator.** It would render the operand from the
  chart, but it drops the operator's Go reconcile logic (status conditions, the prune job on deletion),
  and its Helm release handling adds state the Go operator does not need.
- **Render the operand from the NFD chart in the operator.** The operator would embed
  `deployment/helm/node-feature-discovery`, map the `NodeFeatureDiscovery` spec to chart values and render
  the chart on every reconcile, leaving one source of operand manifests. It replaces the operator's Go code
  that builds the operand, which is the largest change to the operator and is not needed to ship it, and
  the operator e2e test on every pull request catches most of the drift it would remove. It can come back
  as its own proposal if keeping the operator's copy in step becomes a burden.
- **Render the chart at build time.** Pre-rendered manifests cannot follow the per-resource spec, so each
  `NodeFeatureDiscovery` would get the same objects.
- **Import the code as one squashed commit.** Simpler to review, but `git log` and `git blame` in this
  repository would start at the import. This remains the fallback if the CLA check rejects the history.

## Open Questions

- Who should review the operator code (`OWNERS` for `pkg/operator`)?
