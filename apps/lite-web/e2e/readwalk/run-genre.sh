#!/bin/sh
# Walk one article type through the text entry, isolated output per label:
#   sh e2e/readwalk/run-genre.sh report report-article.txt "暴雨之后的伊斯顿"
# Several labels can run in parallel — each has its own output dir and log.
set -u
label="$1"
fixture="$2"
title="$3"
cd "$(dirname "$0")/../.."
mkdir -p "e2e/.readwalk/$label"
ENTRY=text READWALK_OUT="e2e/.readwalk/$label" READWALK_FIXTURE="$fixture" READWALK_TITLE="$title" \
  READ_BRAIN_MODEL="${READ_BRAIN_MODEL:-qwen3.7-plus}" \
  npx playwright test --config e2e/readwalk/playwright.config.ts entries \
  --output "e2e/.readwalk/pw-$label" > "e2e/.readwalk/$label.log" 2>&1
echo "exit=$?" >> "e2e/.readwalk/$label.log"
