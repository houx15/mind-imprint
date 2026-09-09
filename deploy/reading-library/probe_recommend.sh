#!/usr/bin/env bash
# Prove, against the live API, that the shelf really is ranked by her tree.
#
# A brand-new account has an empty tree, so its shelf is the cold-start filler
# and tells us nothing about the ranking. This registers a throwaway student,
# writes ONE keyword and ONE keyword→discipline edge straight into the
# production database for that account, asks for the shelf again, and then
# deletes the account. The unit tests cover the scoring itself
# (internal/library/library_test.go); what this checks is that the wiring
# between the tree and the shelf exists in the deployed build.
#
# Usage: deploy/reading-library/probe_recommend.sh
set -uo pipefail

API="${MIND_API:-https://mind-api.uni-robot.cn}"
SSH_KEY="${MIND_SSH_KEY:-/Users/houyuxin/08Coding/mind-imprint/.deploy-local/id_deploy}"
SSH_HOST="${MIND_SSH_HOST:-deploy@47.93.151.131}"
JOIN_CODE="${E2E_JOIN_CODE:-G624-UXFE}"   # 轻量版体验班 (the lite school)
DISCIPLINE="${1:-astronomy}"

TAG="$(date +%s)$RANDOM"
EMAIL="rec-probe-$TAG@demo.mindimprint.local"
PASS="rec-probe-$TAG-pass"
JAR="${TMPDIR:-/tmp}/mind-rec-probe.cookies"
rm -f "$JAR"

psql_prod() {
  ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "$SSH_HOST" \
    "cd ~/mind-imprint/deploy && docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T db psql -U mindimprint -d mindimprint -qAt -c \"$1\""
}

curl -sS -c "$JAR" -X POST "$API/api/v1/auth/signup" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"display_name\":\"推荐探针\",\"join_code\":\"$JOIN_CODE\"}" \
  -o /dev/null -w 'signup  HTTP %{http_code}\n'
curl -sS -c "$JAR" -b "$JAR" -X POST "$API/api/v1/auth/signin" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" -o /dev/null -w 'signin  HTTP %{http_code}\n'

echo "--- shelf with an EMPTY tree ---"
curl -sS -b "$JAR" "$API/api/v1/library" -o "${TMPDIR:-/tmp}/rec-cold.json"
python3 - "${TMPDIR:-/tmp}/rec-cold.json" <<'PY'
import json, sys
for r in json.load(open(sys.argv[1]))["recommended"]:
    print(f'  {r["slug"]}@{r["tier"]}  why={r["why"]}')
PY

UID_="$(psql_prod "SELECT id FROM users WHERE email = '$EMAIL';" | tr -d '\r')"
[ -n "$UID_" ] || { echo "could not find the account we just made" >&2; exit 1; }
echo "--- planting one keyword on $UID_ routed to $DISCIPLINE ---"
psql_prod "INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, strength, note) VALUES ('$UID_', '小行星', 'asteroid', 'asteroid', 'science', 5, '探针');" >/dev/null
KID="$(psql_prod "SELECT id FROM interest_keyword WHERE user_id = '$UID_' LIMIT 1;" | tr -d '\r')"
psql_prod "INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale) VALUES ('$KID', '$DISCIPLINE', 0.9, 'llm', '探针');" >/dev/null

echo "--- shelf with ONE keyword on $DISCIPLINE ---"
curl -sS -b "$JAR" "$API/api/v1/library" -o "${TMPDIR:-/tmp}/rec-shelf.json"
python3 - "${TMPDIR:-/tmp}/rec-shelf.json" "$DISCIPLINE" <<'PY'
import json, sys
s = json.load(open(sys.argv[1])); want = sys.argv[2]
by_slug = {a["slug"]: a for a in s["articles"]}
for r in s["recommended"]:
    print(f'  {r["slug"]}@{r["tier"]}  why={r["why"]}')
tagged = [r for r in s["recommended"] if want in by_slug[r["slug"]]["tags"] and False]
hits = [r for r in s["recommended"] if any(t["id"] == want for t in by_slug[r["slug"]]["tags"])]
assert hits, f"nothing tagged {want} made the shelf — the tree is not reaching the ranking"
assert all(r["why"] for r in hits), "an article matched by discipline came back with no reason"
print(f"  -> {len(hits)}/{len(s['recommended'])} recommendations carry {want}")
PY
rc=$?

echo "--- cleaning up the throwaway account ---"
psql_prod "DELETE FROM users WHERE id = '$UID_';" >/dev/null
rm -f "$JAR"
[ "$rc" = 0 ] && echo "PROBE OK" || echo "PROBE FAILED"
exit "$rc"
