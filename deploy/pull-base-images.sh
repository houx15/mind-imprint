#!/usr/bin/env bash
# China registry workaround. The Aliyun ECS's configured docker mirrors return
# "not found" for several official images (they carry some tags but not all), and
# gcr.io (the API runtime base) is unreachable from mainland China. Pull each
# base image through the DaoCloud public proxy and retag to its canonical name,
# so `docker compose build` / `up` use the local copies without any change to the
# shared host's docker daemon config.
#
# Run once on the server before the first build (and after bumping any base image
# tag in a Dockerfile / compose):
#   bash deploy/pull-base-images.sh
#
# Outside China this is unnecessary — plain `docker compose build` pulls directly.
set -uo pipefail
PROXY="${DAOCLOUD_PROXY:-m.daocloud.io}"
pull_tag() {
  echo "--- $2 ---"
  docker pull "$1" && docker tag "$1" "$2" && echo "OK $2" || echo "FAIL $2"
}
pull_tag "$PROXY/docker.io/library/postgres:16-alpine"    postgres:16-alpine
pull_tag "$PROXY/gcr.io/distroless/static:nonroot"        gcr.io/distroless/static:nonroot
pull_tag "$PROXY/docker.io/library/node:22-bookworm-slim" node:22-bookworm-slim
pull_tag "$PROXY/docker.io/library/nginx:1.27-alpine"     nginx:1.27-alpine
pull_tag "$PROXY/docker.io/library/golang:1.26"           golang:1.26
echo "=== local base images ==="
docker images | grep -E "postgres|distroless|node |nginx|golang" || true
