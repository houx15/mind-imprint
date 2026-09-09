#!/usr/bin/env bash
# Walk the 分级阅读库 against a live API with a brand-new student.
#
# A fresh account, not the seeded one: 「她还没从库里读过任何东西」is a
# precondition that a shared account loses the first time anyone runs this.
#
# Usage: deploy/reading-library/smoke_prod.sh [https://mind-api.uni-robot.cn]
set -uo pipefail

API="${1:-https://mind-api.uni-robot.cn}"
JOIN_CODE="${E2E_JOIN_CODE:-DEMO-0001}"
JAR="${TMPDIR:-/tmp}/mind-library-smoke.cookies"
TAG="$(date +%s)$RANDOM"
EMAIL="library-smoke-$TAG@demo.mindimprint.local"
PASS="library-smoke-$TAG-pass"
rm -f "$JAR"

fail() { echo "SMOKE FAILED: $*" >&2; exit 1; }
step() { printf '\n=== %s ===\n' "$*"; }

step "signup + signin  ($EMAIL)"
curl -sS -c "$JAR" -X POST "$API/api/v1/auth/signup" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"display_name\":\"书架冒烟\",\"join_code\":\"$JOIN_CODE\"}" \
  -o /dev/null -w 'signup  HTTP %{http_code}\n'
curl -sS -c "$JAR" -b "$JAR" -X POST "$API/api/v1/auth/signin" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" -o /dev/null -w 'signin  HTTP %{http_code}\n'

step "GET /api/v1/library"
curl -sS -b "$JAR" "$API/api/v1/library" -o "${TMPDIR:-/tmp}/shelf.json" -w 'HTTP %{http_code}  %{size_download} bytes\n'
python3 - "${TMPDIR:-/tmp}/shelf.json" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
arts, recs = s["articles"], s["recommended"]
print("articles      :", len(arts))
print("default tier  :", s["tier"])
print("branches      :", ", ".join(f["zh"] for f in s["fields"]))
print("recommended   :", ", ".join(f'{r["slug"]}@{r["tier"]}' for r in recs))
missing = [a["slug"] for a in arts if not a["coverUrl"]]
print("covers missing:", missing or "none")
levels = {len(a["levels"]) for a in arts}
print("levels/article:", levels)
assert len(arts) == 20, f"expected 20 articles, got {len(arts)}"
assert len(recs) == 4, f"expected 4 recommendations, got {len(recs)}"
assert not missing, f"articles with no cover: {missing}"
assert levels == {5}, f"not every article has five levels: {levels}"
PY
[ "$?" = 0 ] || fail "shelf did not check out"

COVER="$(python3 -c 'import json,os;print(json.load(open(os.environ["TMPDIR"] if False else "'"${TMPDIR:-/tmp}"'/shelf.json"))["articles"][0]["coverUrl"])')"
step "cover image reads through the CDN"
curl -sS -o /dev/null -w 'cover  HTTP %{http_code}  %{size_download} bytes  %{content_type}\n' "$COVER"

step "start a reading at tier 3"
SLUG="$(python3 -c 'import json;print(json.load(open("'"${TMPDIR:-/tmp}"'/shelf.json"))["recommended"][0]["slug"])')"
RID="$(curl -sS -b "$JAR" -X POST "$API/api/v1/library/$SLUG/levels/3" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin).get("id",""))')"
[ -n "$RID" ] || fail "no reading id came back"
echo "reading id: $RID  (slug $SLUG)"

step "GET the article the room will render"
curl -sS -b "$JAR" "$API/api/v1/readings/$RID/source" -o "${TMPDIR:-/tmp}/source.json" -w 'HTTP %{http_code}  %{size_download} bytes\n'
python3 - "${TMPDIR:-/tmp}/source.json" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
ids = {b["id"] for b in s["blocks"]}
figs, heads = s.get("figures", []), s.get("headings", [])
print("title    :", s["title"])
print("blocks   :", len(s["blocks"]))
print("headings :", heads)
print("figures  :", [(f["after"] or "(lead)", f["width"], f["height"]) for f in figs])
assert s["blocks"], "no blocks"
assert figs, "the library article came back with no pictures"
for f in figs:
    assert f["url"].startswith("https://"), f"unsigned figure url: {f['url']!r}"
    assert f["caption"], "a figure with no caption"
    assert f["after"] == "" or f["after"] in ids, f"figure anchored to a missing block: {f['after']}"
for h in heads:
    assert h in ids, f"heading points at a missing block: {h}"
for b in s["blocks"]:
    assert not b["text"].startswith("##"), f"markdown heading leaked into the body: {b['text'][:40]}"
    assert not b["text"].startswith("!["), f"an image leaked into the body: {b['text'][:40]}"
PY
[ "$?" = 0 ] || fail "the article did not check out"

step "every picture in the article really loads"
python3 -c 'import json;[print(f["url"]) for f in json.load(open("'"${TMPDIR:-/tmp}"'/source.json"))["figures"]]' \
  | while read -r u; do
      curl -sS -o /dev/null -w "  HTTP %{http_code}  %{size_download} bytes  %{content_type}\n" "$u"
    done

step "the shelf now knows she opened it"
curl -sS -b "$JAR" "$API/api/v1/library" -o "${TMPDIR:-/tmp}/shelf2.json" -w 'HTTP %{http_code}\n'
python3 - "${TMPDIR:-/tmp}/shelf2.json" "$SLUG" <<'PY'
import json, sys
s = json.load(open(sys.argv[1])); slug = sys.argv[2]
row = next(a for a in s["articles"] if a["slug"] == slug)
print("readingId:", row.get("readingId"), "readTier:", row.get("readTier"))
assert row.get("readingId"), "the article she just opened is not marked as opened"
assert row.get("readTier") == 3, f"opened at tier 3, shelf says {row.get('readTier')}"
assert slug not in {r["slug"] for r in s["recommended"]}, "still recommending what she already opened"
PY
[ "$?" = 0 ] || fail "the shelf did not pick up what she opened"

rm -f "$JAR"
printf '\nSMOKE OK\n'
