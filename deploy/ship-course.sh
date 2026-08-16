#!/usr/bin/env bash
# Publish a preview course: POST /admin/courses/<slug>/ship. Committed body,
# no secret here — drive it from the local wrapper
# .deploy-local/ship-course.sh, which injects API + ADMIN_KEY (same
# committed-body/secrets-injected-by-local-wrapper split as remote-deploy.sh).
#
# usage (on caller): API=… ADMIN_KEY=… bash ship-course.sh <slug> [cover]
set -euo pipefail

SLUG="${1:?usage: ship-course.sh <slug> [cover]}"
COVER="${2:-}"
: "${API:?set API}"
: "${ADMIN_KEY:?set ADMIN_KEY}"

curl -fsS -X POST "$API/api/v1/admin/courses/$SLUG/ship" \
  -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" \
  -d "{\"cover\":\"$COVER\"}"
echo
