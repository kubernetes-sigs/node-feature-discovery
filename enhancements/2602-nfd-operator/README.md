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
  - [Rendering the operand from the NFD chart](#rendering-the-operand-from-the-nfd-chart)
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
into this repository. The operator then ships with NFD's version, image and release notes, deploys its
operand by rendering the NFD Helm chart instead of building the objects by hand, and is tested by NFD's
CI on every pull request. Existing `NodeFeatureDiscovery` resources (`nfd.kubernetes.io/v1`) keep working
without conversion.

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
- One source of operand manifests: the operator renders the NFD Helm chart
  (`deployment/helm/node-feature-discovery`), so every chart change also applies to operator installs.
- Existing `nfd.kubernetes.io/v1` `NodeFeatureDiscovery` resources keep working after the upgrade, with no
  conversion webhook, once `spec.operand.image` is unset or points at NFD v0.18.0 or later (resources made
  from the operator v0.6.0 sample pin v0.12.1; the migration guide covers the change). No field that a
  v0.6.0 resource may carry is rejected.
- A required presubmit that deploys NFD through the operator.
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
in without shipping it; later ones add the binary to the NFD image, an operator e2e mode, the chart-driven
rendering, the operator chart publishing and the documentation.

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
  Mitigation: if that happens, the operator code (`api/operator`, `pkg/operator`,
  `cmd/nfd-operator`) moves under one directory with its own `go.mod`, which takes controller-runtime out of
  the main module.
- **Release timing.** If the chart-driven rendering or the operator chart publishing is not merged before
  the v0.20 branch is cut, v0.20 ships only the code that is not user-visible yet, and the operator ships in
  v0.21.

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

### Dependencies

The operator adds `sigs.k8s.io/controller-runtime` (v0.23.3, which builds with the k8s.io v0.35 modules
NFD already uses) and `go.uber.org/mock` (for its existing unit tests) to the main module.
`helm.sh/helm/v3` is already in the module graph.

### Rendering the operand from the NFD chart

The operator embeds the `deployment/helm/node-feature-discovery` chart with `go:embed`. On every
reconcile it maps the `NodeFeatureDiscovery` spec to chart values, renders the chart with Helm's template
engine (`helm.sh/helm/v3/pkg/engine`; rendering only, no Helm releases or release storage), and applies
the result with server-side apply (field manager `nfd-operator`), owner references to the resource, and
pruning by label.

Rendering at reconcile time, not at build time, keeps per-resource configuration working. Every spec field
is either mapped to a chart value, deprecated with a status condition, or rejected by validation; a
mapping table in the documentation lists each field. Golden-file tests render each sample resource.

### Image

The `nfd-operator` binary is added to the existing NFD image, so no new image needs to be promoted. The
operator's default operand image is its own image, set by the operator chart through an environment
variable; `spec.operand.image` overrides it. The operator and the operand therefore have the same version
by default.

### CRD ownership

The operator chart owns the `NodeFeatureDiscovery` CRD. The NFD CRDs (`NodeFeature`, `NodeFeatureRule`,
`NodeFeatureGroup`) live in the operand chart's `crds/` directory, which Helm's template engine does not
render, so rendering the operand chart never produces them, whatever the values. In operator mode they come
from the operator chart's `crds/` directory instead: `make generate` copies the same generated file there,
the way it already copies it into the operand chart, and a CI check added with the build wiring (3) fails
if the copies drift. The `NodeResourceTopology` CRD, which the operand chart renders from a template when
`topologyUpdater.createCRDs` is set, also ships in the operator chart's `crds/` directory, and the operator
always renders the operand chart with `topologyUpdater.createCRDs=false`. The operator therefore never
applies a CRD and never needs permission to create or change one, and every CRD has one owner.

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
| 3 | Build wiring | `nfd-operator` in the image and Makefile; code generation for the operator CRD, RBAC and mocks, with a CI check that fails when generated files or CRD copies drift; Helm lint for the operator chart; `make bundle` repaired and `operator-sdk bundle validate` in CI |
| 4 | Operator e2e | An operator mode for `test/e2e`: install the operator chart with the pull request's image, apply a `NodeFeatureDiscovery`, check the operand, node labels and a `NodeFeatureRule`; a presubmit job |
| 5 | Chart-driven rendering | Render the operand from the embedded NFD chart; mapping table; golden tests; remove the hand-built objects |
| 6 | Operator chart publishing | Chart documentation and values schema; publish the chart next to the operand chart; the operator's default operand image |
| 7 | Docs and ownership | Operator deployment docs, a migration guide from operator v0.6.0, `OWNERS` for the operator code |

Two changes outside this repository go with them: a presubmit job in kubernetes/test-infra (with 4), and a
promoter entry in kubernetes/k8s.io for the operator chart (with 6); the operator binary needs no new
image entry because it ships in the NFD image.

After the operator ships, the operator repository gets a README that points here, its open issues and pull
requests are closed with pointers, and it is archived.

The end-to-end test (4) lands before the rendering change (5), so the largest behaviour change is covered
by a test that checks what the operator actually deploys.

### Test Plan

- The operator's existing unit tests run in `make test` from the import on.
- Golden-file tests for the chart rendering, one per sample `NodeFeatureDiscovery`.
- An operator mode for the e2e tests that installs the operator from the pull request's image, applies a
  `NodeFeatureDiscovery`, waits for the operand and checks node labels and a `NodeFeatureRule`.
- An upgrade test from operator v0.6.0: the resource survives, the operand is reconciled and node labels
  are not removed. The v0.6.0 chart's kube-rbac-proxy image (`gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0`)
  is no longer published, so the test replaces it in the rendered manifests.

### Graduation Criteria

The operator ships in the first NFD release that contains pull requests 1 to 7, with the target of
v0.20. If the chart-driven rendering (5) or the chart publishing (6) misses the v0.20 branch cut, v0.20
contains only the code that is not user-visible yet, and the operator ships in v0.21.

## Implementation History

- 2026-09-29: the operator brought up to date in its own repository
  ([kubernetes-sigs/node-feature-discovery-operator#271](https://github.com/kubernetes-sigs/node-feature-discovery-operator/pull/271)).
- 2026-09-29: tracking issue [#2602](https://github.com/kubernetes-sigs/node-feature-discovery/issues/2602)
  and this proposal.

## Alternatives Considered

- **Keep two repositories.** Every change to the operand's flags, RBAC or CRDs would still have to be
  copied by hand, and nothing would test the two together. This is how the operator's operand stopped
  working with NFD v0.18.
- **Deprecate the operator and support Helm only.** Users and distributions that deploy through the
  operator or OLM would lose their install path, and the definition of done in
  kubernetes-sigs/node-feature-discovery-operator#251 asks for operator deployments to keep working.
- **Replace the operator with an operator-sdk Helm-based operator.** It would render the chart as well,
  but it drops the operator's Go reconcile logic (status conditions, the prune job on deletion), and its
  Helm release handling adds state the Go operator does not need.
- **Render the chart at build time.** Pre-rendered manifests cannot follow the per-resource spec, so each
  `NodeFeatureDiscovery` would get the same objects.
- **Import the code as one squashed commit.** Simpler to review, but `git log` and `git blame` in this
  repository would start at the import. This remains the fallback if the CLA check rejects the history.

## Open Questions

- Should the operator API (`api/operator/v1`) live in the main module, or in its own Go module like
  `api/nfd`, so that downstream projects can import the types without the main module's dependencies?
- Who should review the operator code (`OWNERS` for `pkg/operator`)?
