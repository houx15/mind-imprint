#!/bin/sh
# Run one writing entry walk: sh e2e/writewalk/run-entry.sh direct-en
# Each entry gets its own Playwright output dir and log, so several can run at once.
# Local by default when E2E_BASE_URL is set; otherwise the config's online default.
set -u
entry="$1"
cd "$(dirname "$0")/../.."
mkdir -p e2e/.writewalk
ENTRY="$entry" WRITE_BRAIN_MODEL="${WRITE_BRAIN_MODEL:-qwen3.7-plus}" \
  npx playwright test --config e2e/writewalk/playwright.config.ts entries \
  --output "e2e/.writewalk/pw-$entry" > "e2e/.writewalk/entry-$entry.log" 2>&1
echo "exit=$?" >> "e2e/.writewalk/entry-$entry.log"
