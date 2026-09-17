#!/bin/sh
# Run one entry walk: sh e2e/readwalk/run-entry.sh text
# Each entry gets its own Playwright output dir so parallel runs don't clobber each other.
set -u
entry="$1"
cd "$(dirname "$0")/../.."
mkdir -p e2e/.readwalk
ENTRY="$entry" READ_BRAIN_MODEL="${READ_BRAIN_MODEL:-qwen3.7-plus}" \
  npx playwright test --config e2e/readwalk/playwright.config.ts entries \
  --output "e2e/.readwalk/pw-$entry" > "e2e/.readwalk/entry-$entry.log" 2>&1
echo "exit=$?" >> "e2e/.readwalk/entry-$entry.log"
