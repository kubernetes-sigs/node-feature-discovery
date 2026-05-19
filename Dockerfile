# BUILDER_IMAGE MUST be Debian-family (xx-apt-get requires apt-get).
# Project's Makefile pins it to golang:1.26-trixie (matches go.mod
# minimum Go version) which satisfies this. Override with a non-Debian
# image only if you also swap xx-apt-get for the matching package manager.
#
# tonistiigi/xx is pinned by sha256 digest below. Minimum acceptable
# version is 1.5.0 (arm/v7 + $TARGETVARIANT handling). To update:
#   docker pull tonistiigi/xx:1.x.y
#   docker inspect --format='{{index .RepoDigests 0}}' tonistiigi/xx:1.x.y
# Track in Renovate / Dependabot alongside the runtime base-image pins.
ARG BUILDER_IMAGE
ARG BASE_IMAGE_FULL
ARG BASE_IMAGE_MINIMAL
ARG XX_IMAGE=tonistiigi/xx@sha256:010d4b66aed389848b0694f91c7aaee9df59a6f20be7f5d12e53663a37bd14e2

# Cross-build helper (host arch; never shipped)
FROM --platform=$BUILDPLATFORM ${XX_IMAGE} AS xx

# Build node-feature-discovery on the build host, cross-compile to TARGET.
# Running here on $BUILDPLATFORM avoids the QEMU translation-cache instability
# that crashed go mod download under emulation (see PR commit body for the
# crash signatures).
FROM --platform=$BUILDPLATFORM ${BUILDER_IMAGE:-golang} AS builder
COPY --from=xx / /

ARG TARGETPLATFORM
ARG TARGETARCH
# Cross-toolchain packages (gcc, libc6-dev) are intentionally unpinned —
# Debian point releases shift these frequently and the build-stage is
# discarded; pinning would create frequent CI churn for no security gain.
# hadolint ignore=DL3008
RUN xx-apt-get update && \
    xx-apt-get install -y --no-install-recommends gcc libc6-dev && \
    rm -rf /var/lib/apt/lists/*

# Module cache fetched on $BUILDPLATFORM. Modules are arch-independent;
# no QEMU involvement here.
COPY go.mod go.sum /go/node-feature-discovery/
COPY api/nfd/go.mod api/nfd/go.sum /go/node-feature-discovery/api/nfd/
WORKDIR /go/node-feature-discovery

RUN --mount=type=cache,target=/go/pkg/mod/ \
    go mod download

# Force CGO on. xx-go --wrap defaults CGO_ENABLED=0 for cross-compile
# targets, which would build-tag-filter out source/cpu/cpuid_linux_*.go
# (which use #include <sys/auxv.h>) and silently fall back to the
# cpuid_stub.go no-op variant.
ENV CGO_ENABLED=1

# Cross-compile via xx-go --wrap. After --wrap, /usr/local/go/bin/go is a
# shim that sets CC, GOOS, GOARCH, GOARM from $TARGETPLATFORM for every
# subsequent invocation in this stage. Use explicit per-binary go build
# -o to bypass go install's $GOPATH/bin/$GOOS_$GOARCH/<name> redirect for
# cross-targets (Go refuses GOBIN override on cross-compile).
ARG VERSION
ARG HOSTMOUNT_PREFIX

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=src=.,target=. \
    xx-go --wrap && \
    # Silent-CGO-disable guard: source/cpu/cpuid_linux_{arm,arm64,ppc64le,s390x}.go
    # each `import "C"` and wrap glibc's getauxval() via a small inline C
    # function called gethwcap(). If xx-apt-get install silently failed
    # or xx-go --wrap unset CGO_ENABLED, those files get build-tag-filtered
    # out and the binary would ship with empty CPU feature labels at
    # runtime with no build error.
    #
    # We probe by building an UNSTRIPPED nfd-worker (the production build
    # below uses -s -w which destroys the symbol table) and grepping for
    # the CGO-emitted wrapper symbol that can only exist if the linux
    # cpuid file was compiled+linked. The Go package object is then
    # cached, so the production -s -w link is cheap.
    #
    # amd64 is excluded: source/cpu/cpuid_amd64.go uses pure-Go cpuid via
    # github.com/klauspost/cpuid/v2 — no CGO required on that arch.
    case "$TARGETARCH" in \
      arm64|arm|ppc64le|s390x) \
        go build -v -tags osusergo,netgo -o /tmp/nfd-worker-guard ./cmd/nfd-worker && \
        go tool nm /tmp/nfd-worker-guard | grep -q 'source/cpu\._Cfunc_gethwcap' \
          || { echo "FAIL: source/cpu._Cfunc_gethwcap missing from nfd-worker — CGO likely disabled for $TARGETPLATFORM"; exit 1; } && \
        rm -f /tmp/nfd-worker-guard \
        ;; \
    esac && \
    for bin in nfd-master nfd-worker nfd-topology-updater nfd-gc kubectl-nfd nfd; do \
      go build -v -tags osusergo,netgo \
        -ldflags "-s -w -extldflags=-static -X sigs.k8s.io/node-feature-discovery/pkg/version.version=${VERSION} -X sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath.pathPrefix=${HOSTMOUNT_PREFIX}" \
        -o /go/bin/$bin ./cmd/$bin || exit 1; \
    done && \
    xx-verify /go/bin/nfd-master

# Runtime stages run at $TARGETPLATFORM (implicit). They only COPY, set USER,
# and set ENV — trivial work under QEMU.

FROM ${BASE_IMAGE_FULL:-debian:stable-slim} AS full

USER 65534:65534
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY deployment/components/worker-config/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
# Enumerate explicitly — matches Makefile BUILD_BINARIES. Defensive against
# any future shim-leakage from xx-go.
COPY --from=builder /go/bin/nfd-master /go/bin/nfd-worker /go/bin/nfd-topology-updater /go/bin/nfd-gc /go/bin/kubectl-nfd /go/bin/nfd /usr/bin/

FROM ${BASE_IMAGE_MINIMAL:-scratch} AS minimal

USER 65534:65534
ENV GRPC_GO_LOG_SEVERITY_LEVEL="INFO"

COPY deployment/components/worker-config/nfd-worker.conf.example /etc/kubernetes/node-feature-discovery/nfd-worker.conf
COPY --from=builder /go/bin/nfd-master /go/bin/nfd-worker /go/bin/nfd-topology-updater /go/bin/nfd-gc /go/bin/kubectl-nfd /go/bin/nfd /usr/bin/
