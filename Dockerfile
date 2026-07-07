# syntax=docker/dockerfile:1

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

FROM debian:12-slim AS sfa-enduro-worker
RUN apt-get update && apt-get install -y --no-install-recommends \
	libxml2-utils \
	openjdk-17-jre-headless \
	&& rm -rf /var/lib/apt/lists/*

ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd --gid ${GROUP_ID} enduro && \
	useradd --uid ${USER_ID} --gid enduro --create-home enduro && \
	mkdir --parents /var/opt/verapdf/logs /home/enduro/shared && \
	chown -R enduro:enduro /var/opt/verapdf /home/enduro

USER enduro

COPY --from=ghcr.io/verapdf/cli:latest --link /opt/verapdf/ /opt/verapdf/
COPY --from=build-sfa-enduro-worker --link /out/sfa-enduro-worker /home/enduro/bin/sfa-enduro-worker

CMD ["/home/enduro/bin/sfa-enduro-worker"]
