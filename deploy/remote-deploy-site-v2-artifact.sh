#!/usr/bin/env bash
# Deploy the already-built V2 artifact; only the marketing compose service changes.
set -euo pipefail
release_id="${1:?release id required}"
[[ "$release_id" =~ ^[0-9T-]+$ ]] || exit 2
release="$HOME/mind-imprint-site-releases/$release_id"
mkdir -p "$release"
tar -xzf /tmp/mind-site-v2-release.tar.gz -C "$release"
old_container=$(docker ps -q --filter 'name=^/mindimprint-site-site-1$')
[ -n "$old_container" ] || { echo 'Existing site container missing'; exit 1; }
old_image=$(docker inspect --format '{{.Image}}' "$old_container")
rollback_image="mindimprint-site:rollback-$release_id"
new_image="mindimprint-site:v2-$release_id"
docker tag "$old_image" "$rollback_image"
printf '%s\n' "$rollback_image" > "$release/rollback-image.txt"
docker build --pull=false -t "$new_image" "$release"
candidate="mind-site-v2-check-$release_id"
cleanup() { docker rm -f "$candidate" >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker run -d --name "$candidate" -p 127.0.0.1::80 "$new_image"
docker exec "$candidate" nginx -t < /dev/null
candidate_port=$(docker port "$candidate" 80/tcp | cut -d: -f2)
check_site() {
  local origin="$1" route code
  for route in / /zh/ /about/ /zh/about/ /product/reading/ /product/explore/ /product/courses/ /research-studio/ /schools/ /courses/ai-ethics/ /courses/ai-use/ /courses/source-evaluation/ /courses/historical-evidence/ /courses/multimedia-reading/ /courses/data-literacy/ /courses/self-exploration/; do
    code=$(curl --retry 8 --retry-all-errors --retry-connrefused --retry-delay 1 --max-time 10 -s -o /dev/null -w '%{http_code}' "$origin$route")
    [ "$code" = 200 ] || { echo "$route -> $code"; return 1; }
  done
  for route in /en/ /algorithm /product/teacher /editions/; do
    [ "$(curl --max-time 10 -s -o /dev/null -w '%{http_code}' "$origin$route")" = 301 ] || return 1
  done
  [ "$(curl --max-time 10 -s -o /dev/null -w '%{http_code}' "$origin/definitely-not-a-page")" = 404 ] || return 1
  curl -fsS "$origin/" -o "$release/verified-home.html"
  grep -q 'https://mind-assets.uni-robot.cn/v2/learning-together-v3.webp' "$release/verified-home.html"
}
check_site "http://127.0.0.1:$candidate_port"
cat > "$release/compose.yml" <<YAML
name: mindimprint-site
services:
  site:
    image: $new_image
    restart: unless-stopped
    ports:
      - "127.0.0.1:8092:80"
YAML
sed "s|$new_image|$rollback_image|" "$release/compose.yml" > "$release/rollback.yml"
if ! docker compose -f "$release/compose.yml" up -d site || ! check_site http://127.0.0.1:8092; then
  echo 'V2 verification failed; restoring previous site'
  docker compose -f "$release/rollback.yml" up -d site
  exit 1
fi
ln -sfn "$release" "$HOME/mind-imprint-site-releases/current"
echo "DEPLOYED $new_image"
# The user requested no retained old site image. The rollback tag is temporary
# during the switch and removed after the new site passes verification.
# Remove only marketing-site tags for the superseded image.
while IFS= read -r previous_tag; do
  case "$previous_tag" in
    mindimprint-site:*) docker image rm "$previous_tag" ;;
  esac
done < <(docker image inspect "$old_image" --format '{{range .RepoTags}}{{println .}}{{end}}')
rm -f "$release/rollback.yml" "$release/rollback-image.txt"
echo "Previous site image removed after successful verification"
docker ps --filter 'name=mindimprint-site' --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}'
