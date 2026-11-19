# Development image for dsk-eel: the Go toolchain, the one C library the
# build links against, and the lint/benchmark tools dev/check uses. Nothing
# here needs to be installed on the host.
#
# Use it through dev/run rather than calling docker directly: dev/run builds
# the image when this file changes, mounts the checkout at /src and keeps the
# Go caches in a named volume.

# go.mod's `go` line stays the minimum supported version; this is only the
# toolchain used for development. It is pinned to a patch release so that
# benchmark numbers do not move when the image is rebuilt; bump it on purpose.
# Override with GO_VERSION=1.23 dev/run ... to check that the minimum still
# builds.
ARG GO_VERSION=1.27.4
FROM golang:${GO_VERSION}-bookworm

# go-hid compiles its bundled hidapi with cgo; on Linux that is the hidraw
# backend, which links against libudev. gcc and libc headers are already in
# the base image (the race detector needs them too).
RUN apt-get update \
    && apt-get install -y --no-install-recommends libudev-dev \
    && rm -rf /var/lib/apt/lists/*

# Pinned so a rebuild does not silently change lint results. These need a
# recent Go to build, so let the go command fetch one if GO_VERSION is older;
# the downloaded toolchain is dropped again with the module cache.
ARG STATICCHECK_VERSION=2026.2.2
ARG BENCHSTAT_VERSION=v0.0.0-20261117185222-4febdb082b2c
RUN export GOBIN=/usr/local/bin GOTOOLCHAIN=auto \
    && go install honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION} \
    && go install golang.org/x/perf/cmd/benchstat@${BENCHSTAT_VERSION} \
    && go clean -cache -modcache

# dev/run starts the container as the host user so files written to the
# checkout are not root-owned, so the cache volume must be writable by
# whichever uid uses it first. A new named volume inherits this directory's
# mode. One volume is not meant to be shared between different uids.
RUN mkdir -m 1777 /cache
ENV GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/mod \
    GOFLAGS=-buildvcs=false

WORKDIR /src
CMD ["bash"]
