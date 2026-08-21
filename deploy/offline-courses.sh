#!/usr/bin/env bash
# 下线 courses: flip published -> preview so students stop seeing them.
#
# NOT a delete. The course row, every student's session/progress, and every
# recorded event stay exactly where they are — the course simply leaves the
# catalog (ListCourseRows returns 'published' only for a student) and 404s for
# a non-admin session. Re-publishing is one ship call away:
#
#   curl -X POST -H "Authorization: Bearer $OSS_ADMIN_KEY" \
#     https://mind-api.uni-robot.cn/api/v1/admin/courses/<slug>/ship -d '{}'
#
# Idempotent: a slug that is already 'preview' reports changed:false, not an error.
#
# This file contains NO secrets and is committed. The admin key is read at run
# time from the gitignored deploy env file (.deploy-local/env.prod).
#
# Usage:
#   bash deploy/offline-courses.sh                 # the 2026-08-22 retirement batch below
#   bash deploy/offline-courses.sh slug-a slug-b   # or any slugs you name

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API="${MIND_API:-https://mind-api.uni-robot.cn}"

# .deploy-local is gitignored, so it exists only in the MAIN checkout — running
# this from a git worktree must still find it. `--git-common-dir` resolves to
# the main checkout's .git from anywhere in the repo.
ENV_FILE="${MIND_ENV_FILE:-$ROOT/.deploy-local/env.prod}"
if [[ ! -f "$ENV_FILE" ]]; then
  common_git="$(git -C "$ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
  [[ -n "$common_git" ]] && ENV_FILE="$(dirname "$common_git")/.deploy-local/env.prod"
fi

# The 2026-08-22 batch: two mid-migration duplicates (a-mid/b-mid), a
# single-slice fixture (evidence-comparability), an end-to-end cover test
# artifact, and the teacher-simulation copy of Follow the Money.
DEFAULT_SLUGS=(
  a-mid
  b-mid
  evidence-comparability
  course-authoring-v14-cover-e2e-20260820
  follow-the-money-teacher-sim-20260818
)

SLUGS=("$@")
if [[ ${#SLUGS[@]} -eq 0 ]]; then
  SLUGS=("${DEFAULT_SLUGS[@]}")
fi

[[ -f "$ENV_FILE" ]] || { echo "missing $ENV_FILE" >&2; exit 1; }
# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a
[[ -n "${OSS_ADMIN_KEY:-}" ]] || { echo "OSS_ADMIN_KEY not set in $ENV_FILE" >&2; exit 1; }

echo "下线 ${#SLUGS[@]} course(s) on $API:"
failed=0
for slug in "${SLUGS[@]}"; do
  if body=$(curl -fsS -X POST -H "Authorization: Bearer $OSS_ADMIN_KEY" \
      "$API/api/v1/admin/courses/$slug/unpublish"); then
    echo "  $slug  ->  $body"
  else
    echo "  $slug  ->  FAILED" >&2
    failed=1
  fi
done

echo
echo "verify (a student's catalog must not list them):"
echo "  curl -sH \"Authorization: Bearer \$OSS_ADMIN_KEY\" $API/api/v1/admin/courses | python3 -m json.tool"
exit "$failed"
