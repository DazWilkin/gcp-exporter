#!/bin/bash

# Replaced with GitHub Actions

REPO="ghcr.io/dazwilkin/gcp-exporter"
VERSION=$(uname --kernel-release)
COMMIT=$(git rev-parse HEAD)

docker build \
--rm \
--file=Dockerfile \
--build-arg=VERSION="${VERSION}" \
--build-arg=COMMIT="${COMMIT}" \
--tag=${REPO}:${COMMIT} \
.

docker push ${REPO}:${COMMIT}

sed --in-place "s|${REPO}:[0-9a-f]\{40\}|${REPO}:${COMMIT}|g" ./docker-compose.yml
sed --in-place "s|${REPO}:[0-9a-f]\{40\}|${REPO}:${COMMIT}|g" ./README.md
