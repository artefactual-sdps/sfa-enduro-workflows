#!/usr/bin/env sh

set -eu

TARGET=${1:-}
if [ -z "$TARGET" ]; then
	echo "Service name must be supplied, e.g.:"
	echo "\t $ $(basename "$0") preprocessing-worker"
	exit 1
fi
case "$TARGET" in
	"preprocessing-worker")
		IMAGE_NAME="preprocessing-sfa-worker"
		TARGET="preprocessing-worker"
		FOLDER="."
		;;
	"sfa-dips-worker")
		IMAGE_NAME="sfa-dips-worker"
		TARGET="sfa-dips-worker"
		FOLDER="."
		;;
	*)
		echo "Accepted values: preprocessing-worker, sfa-dips-worker."
		exit 1
		;;
esac

eval $(./hack/build_dist.sh shellvars)

DEFAULT_IMAGE_NAME="${IMAGE_NAME}:${2:-${VERSION_SHORT}}"
TILT_EXPECTED_REF=${EXPECTED_REF:-}
IMAGE_NAME="${TILT_EXPECTED_REF:-$DEFAULT_IMAGE_NAME}"
BUILD_OPTS="${BUILD_OPTS:-}"
STRIP="${STRIP:-1}"

GO_VERSION=$(grep "^go " go.mod | awk '{print $2}')
if [ -z "$GO_VERSION" ]; then
	echo "Error: Go version not found in go.mod."
	exit 1
fi

env DOCKER_BUILDKIT=1 docker build \
	-t "$IMAGE_NAME" \
	-f "$FOLDER/Dockerfile" \
	--build-arg="TARGET=$TARGET" \
	--build-arg="GO_VERSION=$GO_VERSION" \
	--build-arg="VERSION_PATH=$VERSION_PATH" \
	--build-arg="VERSION_LONG=$VERSION_LONG" \
	--build-arg="VERSION_SHORT=$VERSION_SHORT" \
	--build-arg="VERSION_GIT_HASH=$VERSION_GIT_HASH" \
	--build-arg="STRIP=$STRIP" \
	$BUILD_OPTS \
	$FOLDER
