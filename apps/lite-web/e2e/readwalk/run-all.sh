#!/bin/sh
# The full online reading walk: every article type through the text entry,
# plus the Chinese DOCX upload entry, in parallel. Output per label under
# e2e/.readwalk/<prefix>-<label>. Usage:
#   sh e2e/readwalk/run-all.sh m
set -u
p="${1:-all}"
cd "$(dirname "$0")/../.."
rm -f e2e/.readwalk/"$p"-*.log
sh e2e/readwalk/run-genre.sh "$p-jingye" jingye-article.txt "敬业与乐业" &
sh e2e/readwalk/run-genre.sh "$p-argument" long-article.txt "停车场与公园" &
sh e2e/readwalk/run-genre.sh "$p-report" report-article.txt "暴雨之后的伊斯顿" &
sh e2e/readwalk/run-genre.sh "$p-narrative" narrative-article.txt "末班车" &
sh e2e/readwalk/run-genre.sh "$p-explain" explain-article.txt "说明文" &
(
  mkdir -p "e2e/.readwalk/$p-upload"
  ENTRY=upload READWALK_OUT="e2e/.readwalk/$p-upload" READ_BRAIN_MODEL="${READ_BRAIN_MODEL:-qwen3.7-plus}" \
    npx playwright test --config e2e/readwalk/playwright.config.ts entries \
    --output "e2e/.readwalk/pw-$p-upload" > "e2e/.readwalk/$p-upload.log" 2>&1
  echo "exit=$?" >> "e2e/.readwalk/$p-upload.log"
) &
wait
grep -H "exit=" e2e/.readwalk/"$p"-*.log
