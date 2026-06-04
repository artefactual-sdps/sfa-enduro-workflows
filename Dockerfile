# syntax = docker/dockerfile:1.4

ARG TARGET=sfa-enduro-worker
ARG GO_VERSION

FROM golang:${GO_VERSION}-alpine AS build-go
WORKDIR /src
ENV CGO_ENABLED=0
COPY --link go.* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY --link . .

FROM build-go AS build-sfa-enduro-worker
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
	-o /out/sfa-enduro-worker \
	./cmd/worker

FROM build-go AS build-sfa-dips
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
	-o /out/sfa-dips \
	./cmd/sfa-dips

FROM debian:12-slim AS base
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd --gid ${GROUP_ID} enduro && \
	useradd --uid ${USER_ID} --gid enduro --create-home enduro
USER enduro

FROM base AS sfa-enduro-worker
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
	libxml2-utils \
	openjdk-17-jre-headless \
	&& rm -rf /var/lib/apt/lists/* && \
	mkdir --parents /var/opt/verapdf/logs /home/enduro/shared && \
	chown -R enduro:enduro /var/opt/verapdf /home/enduro
USER enduro
COPY --link --chown=enduro:enduro --from=ghcr.io/verapdf/cli:latest /opt/verapdf/ /opt/verapdf/
COPY --link --chown=enduro:enduro --from=build-sfa-enduro-worker /out/sfa-enduro-worker /home/enduro/bin/sfa-enduro-worker
CMD ["/home/enduro/bin/sfa-enduro-worker"]

FROM base AS sfa-dips
COPY --link --chown=enduro:enduro --from=build-sfa-dips /out/sfa-dips /home/enduro/bin/sfa-dips
CMD ["/home/enduro/bin/sfa-dips"]

FROM ${TARGET}
