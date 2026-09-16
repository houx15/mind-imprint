#!/usr/bin/env bash
# Publish the current site-v2 working tree without resetting any checkout.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
release_id=$(date -u +%Y%m%dT%H%M%S)
release_stage=$(mktemp -d)
trap 'rm -rf "$release_stage"' EXIT
PUBLIC_V2_ASSET_BASE_URL=https://mind-assets.uni-robot.cn pnpm --filter site-v2 check
PUBLIC_V2_ASSET_BASE_URL=https://mind-assets.uni-robot.cn pnpm --filter site-v2 build
cp apps/site-v2/nginx.conf "$release_stage/nginx.conf"
cp -R apps/site-v2/dist "$release_stage/dist"
cat > "$release_stage/Dockerfile" <<'DOCKER'
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY dist /usr/share/nginx/html
DOCKER
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$release_stage/release.tar.gz" -C "$release_stage" Dockerfile nginx.conf dist
ssh -i .deploy-local/id_deploy -o BatchMode=yes deploy@47.93.151.131 'cat > /tmp/mind-site-v2-release.tar.gz' < "$release_stage/release.tar.gz"
ssh -i .deploy-local/id_deploy -o BatchMode=yes deploy@47.93.151.131 "bash -s -- $release_id" < deploy/remote-deploy-site-v2-artifact.sh
curl --fail --silent --show-error https://mind.uni-robot.cn/ -o "$release_stage/live.html"
rg -q 'https://mind-assets.uni-robot.cn/v2/learning-together-v3.webp' "$release_stage/live.html"
echo "Public V2 homepage verified: https://mind.uni-robot.cn/"
