#!/usr/bin/env bash
# Re-upload course-20's two writer interactions with the student-trapping
# completion gate removed. See README.md in this directory for the diagnosis.
#
# Both files keep their EXISTING object keys, so the stored CourseDefinition is
# untouched — no definition-hash change, no session reset for students who are
# mid-course. That does mean the CDN may still hold the old copy: after this
# runs, refresh the path on the Aliyun CDN console (刷新预热 → 目录刷新):
#
#     https://mind-oss.uni-robot.cn/courses/course-20/interactions/html/
#
# Uploads go through the sanctioned authoring endpoint (the same one the course
# generator uses): POST /admin/courses/{slug}/asset-upload-url returns a
# presigned PUT at the deterministic key courses/<slug>/<relativePath>.
#
# This file contains NO secrets and is committed. The admin key is read at run
# time from the gitignored deploy env file (.deploy-local/env.prod).
#
# Usage:  bash deploy/course-20-interaction-gates/upload.sh
#         bash deploy/course-20-interaction-gates/upload.sh --rollback

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT/.deploy-local/env.prod"
API="${MIND_API:-https://mind-api.uni-robot.cn}"
SLUG="course-20"

DIR="$ROOT/deploy/course-20-interaction-gates"
if [[ "${1:-}" == "--rollback" ]]; then
  DIR="$DIR/original"
  echo "ROLLBACK: restoring the ORIGINAL interactions (the keyword gate comes back)."
fi

[[ -f "$ENV_FILE" ]] || { echo "missing $ENV_FILE" >&2; exit 1; }
# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a
[[ -n "${OSS_ADMIN_KEY:-}" ]] || { echo "OSS_ADMIN_KEY not set in $ENV_FILE" >&2; exit 1; }

upload_one() {
  local name="$1"
  local file="$DIR/$name"
  local rel="interactions/html/$name"
  local size
  size=$(wc -c < "$file" | tr -d ' ')

  local presign
  presign=$(curl -fsS -X POST \
    -H "Authorization: Bearer $OSS_ADMIN_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"relativePath\":\"$rel\",\"contentType\":\"text/html\",\"size\":$size}" \
    "$API/api/v1/admin/courses/$SLUG/asset-upload-url")

  local put_url object_key
  put_url=$(printf '%s' "$presign" | python3 -c 'import json,sys; print(json.load(sys.stdin)["putUrl"])')
  object_key=$(printf '%s' "$presign" | python3 -c 'import json,sys; print(json.load(sys.stdin)["objectKey"])')

  local code
  code=$(curl -fsS -o /dev/null -w '%{http_code}' -X PUT \
    -H "Content-Type: text/html" --data-binary "@$file" "$put_url")
  echo "  $object_key  <-  $name  ($size bytes, HTTP $code)"
}

echo "uploading to $API (course $SLUG):"
upload_one emotion-pause-writer.html
upload_one image-evidence-writer.html
echo
echo "done. Now refresh the CDN directory so students stop getting the cached copy:"
echo "  https://mind-oss.uni-robot.cn/courses/$SLUG/interactions/html/"
