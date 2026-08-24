#!/usr/bin/env bash
# Ships the four COURSE-DATA fixes from the 2026-08-23 stuck-slice sweep.
# See README.md for the per-course diagnosis. The two RUNTIME fixes in the same
# sweep are code, not data, and ride the normal web deploy instead.
#
# Assets keep their EXISTING object keys, so the stored CourseDefinition is
# untouched for course-33 / course-05 / course-20 — no definition-hash change,
# no session reset. course-10 is the exception: its fix edits the definition
# itself, so its hash DOES change and in-progress course-10 sessions reset.
#
# After running, refresh the CDN directories (Aliyun console → 刷新预热 → 目录刷新):
#   https://mind-oss.uni-robot.cn/courses/course-33/interactions/html/
#   https://mind-oss.uni-robot.cn/courses/course-05/interactions/html/
#   https://mind-oss.uni-robot.cn/courses/course-20/interactions/html/
#
# No secrets here: the admin key is read at run time from the gitignored
# .deploy-local/env.prod.
#
# Usage:  bash deploy/course-gate-fixes-2026-08-23/upload.sh
#         bash deploy/course-gate-fixes-2026-08-23/upload.sh --rollback

set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/../.." && pwd)"
ENV_FILE="$ROOT/.deploy-local/env.prod"
API="${MIND_API:-https://mind-api.uni-robot.cn}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

ROLLBACK=0
[[ "${1:-}" == "--rollback" ]] && ROLLBACK=1

[[ -f "$ENV_FILE" ]] || { echo "missing $ENV_FILE" >&2; exit 1; }
# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a
[[ -n "${OSS_ADMIN_KEY:-}" ]] || { echo "OSS_ADMIN_KEY not set in $ENV_FILE" >&2; exit 1; }

# Uploads one file to courses/<slug>/<relativePath> via the sanctioned
# authoring endpoint (the same presigned-PUT path the course generator uses).
upload_one() {
  local slug="$1" name="$2" file="$3"
  local rel="interactions/html/$name" size presign put_url object_key code
  size=$(wc -c < "$file" | tr -d ' ')
  presign=$(curl -fsS -X POST \
    -H "Authorization: Bearer $OSS_ADMIN_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"relativePath\":\"$rel\",\"contentType\":\"text/html\",\"size\":$size}" \
    "$API/api/v1/admin/courses/$slug/asset-upload-url")
  put_url=$(printf '%s' "$presign" | python3 -c 'import json,sys; print(json.load(sys.stdin)["putUrl"])')
  object_key=$(printf '%s' "$presign" | python3 -c 'import json,sys; print(json.load(sys.stdin)["objectKey"])')
  code=$(curl -fsS -o "$WORK/put.out" -w '%{http_code}' -X PUT \
    -H "Content-Type: text/html" --data-binary "@$file" "$put_url")
  echo "  $object_key  <-  $(basename "$file")  ($size bytes, HTTP $code)"
}

echo "target: $API"
echo

# ---- course-33 · slice 5 — the guidance line that named the wrong blocker ----
echo "course-33 (source-chain-sort.html):"
if [[ $ROLLBACK -eq 1 ]]; then
  upload_one course-33 source-chain-sort.html "$DIR/original/source-chain-sort.html"
else
  upload_one course-33 source-chain-sort.html "$DIR/source-chain-sort.html"
fi
echo

# ---- course-05 · slice 3 — comprehension gating progression ----
# Only the PATCHED file is committed (it embeds 2.5 MB of imagery, so a second
# pristine copy would double that for a one-line change). Rollback regenerates
# the original from it instead: the patch is a single exact string swap, and
# `--reverse` on the committed file reproduces the live original byte for byte.
echo "course-05 (four-revisions-opcvl-lab.html):"
if [[ $ROLLBACK -eq 1 ]]; then
  python3 "$DIR/patch_opcvl_lab.py" --reverse "$DIR/four-revisions-opcvl-lab.html" "$WORK/c05.out.html"
  upload_one course-05 four-revisions-opcvl-lab.html "$WORK/c05.out.html"
else
  upload_one course-05 four-revisions-opcvl-lab.html "$DIR/four-revisions-opcvl-lab.html"
fi
echo

# ---- course-20 · slice 8 — the keyword-regex gate (fix written 2026-08-21) ----
echo "course-20 (delegating to the 2026-08-21 bundle):"
if [[ $ROLLBACK -eq 1 ]]; then
  bash "$ROOT/deploy/course-20-interaction-gates/upload.sh" --rollback
else
  bash "$ROOT/deploy/course-20-interaction-gates/upload.sh"
fi
echo

# ---- course-10 · slice 1 — a reference panel used as a completion gate ----
# This one edits the DEFINITION, so it changes the hash and resets in-progress
# course-10 sessions. Rollback is manual: re-PUT the previous workflow.
echo "course-10 (definition workflow):"
if [[ $ROLLBACK -eq 1 ]]; then
  echo "  SKIPPED — definition rollback is manual, see README.md"
else
  python3 "$DIR/fix_course10_slice1.py"
fi
echo

echo "done. Now refresh these CDN directories so students stop getting cached copies:"
echo "  https://mind-oss.uni-robot.cn/courses/course-33/interactions/html/"
echo "  https://mind-oss.uni-robot.cn/courses/course-05/interactions/html/"
echo "  https://mind-oss.uni-robot.cn/courses/course-20/interactions/html/"
