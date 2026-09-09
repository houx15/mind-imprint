#!/usr/bin/env bash
# Sign in as one account and print what /auth/me says plus a couple of lite
# routes. Used to tell "the route is missing" apart from "this account's school
# is not on the lite edition", which both come back as a 404.
#
# Usage: deploy/reading-library/probe_routes.sh <email> <password> [api]
set -u
EMAIL="$1"; PASS="$2"; API="${3:-https://mind-api.uni-robot.cn}"
JAR="${TMPDIR:-/tmp}/mind-probe-routes.cookies"
rm -f "$JAR"

curl -sS -c "$JAR" -X POST "$API/api/v1/auth/signin" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" -o /dev/null -w 'signin  HTTP %{http_code}\n'
echo "--- /auth/me ---"
curl -sS -b "$JAR" "$API/api/v1/auth/me" | head -c 600; echo
echo "--- /readings (a known lite route) ---"
curl -sS -b "$JAR" "$API/api/v1/readings" -w '  <- HTTP %{http_code}\n' | head -c 200
echo "--- /library ---"
curl -sS -b "$JAR" "$API/api/v1/library" -w '  <- HTTP %{http_code}\n' | head -c 300
rm -f "$JAR"
