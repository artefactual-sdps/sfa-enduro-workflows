# syntax = docker/dockerfile:1.4

ARG TARGET=preprocessing-worker
ARG GO_VERSION

FROM golang:${GO_VERSION}-alpine AS build-go
WORKDIR /src
ENV CGO_ENABLED=0
COPY --link go.* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY --link . .

FROM build-go AS build-preprocessing-worker
ARG VERSION_PATH
ARG VERSION_LONG
ARG VERSION_SHORT
ARG VERSION_GIT_HASH
ARG STRIP=1
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	ldflags="-X '${VERSION_PATH}.Long=${VERSION_LONG}' -X '${VERSION_PATH}.Short=${VERSION_SHORT}' -X '${VERSION_PATH}.GitCommit=${VERSION_GIT_HASH}'" && \
	if [ "$STRIP" = "1" ]; then ldflags="-s $ldflags"; fi && \
	go build \
	-trimpath \
	-ldflags="$ldflags" \
	-o /out/preprocessing-worker \
	./cmd/worker

FROM build-go AS build-sfa-dips-worker
ARG VERSION_PATH
ARG VERSION_LONG
ARG VERSION_SHORT
ARG VERSION_GIT_HASH
ARG STRIP=1
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	ldflags="-X '${VERSION_PATH}.Long=${VERSION_LONG}' -X '${VERSION_PATH}.Short=${VERSION_SHORT}' -X '${VERSION_PATH}.GitCommit=${VERSION_GIT_HASH}'" && \
	if [ "$STRIP" = "1" ]; then ldflags="-s $ldflags"; fi && \
	go build \
	-trimpath \
	-ldflags="$ldflags" \
	-o /out/sfa-dips-worker \
	./cmd/dips-worker

FROM debian:12-slim AS base
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd --gid ${GROUP_ID} preprocessing && \
	useradd --uid ${USER_ID} --gid preprocessing --create-home preprocessing
USER preprocessing

FROM base AS preprocessing-worker
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
	libxml2-utils \
	openjdk-17-jre-headless \
	&& rm -rf /var/lib/apt/lists/* && \
	mkdir --parents /var/opt/verapdf/logs /home/preprocessing/shared && \
	chown -R preprocessing:preprocessing /var/opt/verapdf /home/preprocessing
USER preprocessing
COPY --link --chown=preprocessing:preprocessing --from=ghcr.io/verapdf/cli:latest /opt/verapdf/ /opt/verapdf/
COPY --link --chown=preprocessing:preprocessing --from=build-preprocessing-worker /out/preprocessing-worker /home/preprocessing/bin/preprocessing-worker
CMD ["/home/preprocessing/bin/preprocessing-worker"]

FROM base AS sfa-dips-worker
COPY --link --chown=preprocessing:preprocessing --from=build-sfa-dips-worker /out/sfa-dips-worker /home/preprocessing/bin/sfa-dips-worker
CMD ["/home/preprocessing/bin/sfa-dips-worker"]

FROM ${TARGET}
