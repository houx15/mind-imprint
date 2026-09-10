---
name: uploading-reading-batches
description: Use when adding a new batch of Newsela-style leveled reading material (a folder of per-level .md exports plus images) to 思维印记's 分级阅读库 — including tagging articles with disciplines, uploading photographs to OSS, and rebuilding articles.json.
---

# Uploading a reading batch

## Overview

The library is a build artifact, not a hand-edited file. A batch goes in as
source markdown, gets registered, tagged, corrected, and built; the output is
`apps/api/internal/library/articles.json`, which Go `go:embed`s.

**Runbook with the commands, the file-by-file table, and the full failure
list: `deploy/reading-library/README.md`. Read it before starting.**

**Never hand-edit `articles.json`.** It is regenerated from scratch on every
build, so an edit there is gone at the next batch and silently absent from
the source of truth in between.

## The five inputs you actually change

| File | What you put in it |
|---|---|
| `sources.json` | one entry per batch — path only; layout is irrelevant |
| `tags.json` → `articles` | 中文标题, one-line reason, 2–3 disciplines from the closed table |
| `tags.json` → `holds` | slug → why this one is NOT going live |
| `corrections.json` → `replacements` | verbatim find/replace, each with a `count` |
| `corrections.json` → `figure_captions` | whole captions the export mangled past find/replace |

## Order of work

1. Register the batch in `sources.json`.
2. `parse.py > dist/parsed.json`, then run `audit_captions.py`, `audit_units.py`
   and `audit_names.py` on it. All three report; none of them fails a build.
3. **Inspect before tagging** (see below) — this is where the real defects are.
4. Write `tags.json` entries; hold back anything that cannot ship, with a reason.
5. `make_images.py` → `build.py` → `upload-reading-images.sh` → `go test` → deploy → `smoke_prod.sh`.

Steps 1–4 are cheap and reversible; steps 5+ push bytes to a CDN. Do not
start uploading until `build.py` exits 0.

## Inspect before tagging

Every batch so far arrived with defects that no test catches, because a
mangled caption is still a valid string. Check these four things directly:

- **Read all of `audit_captions.py`'s output, to the bottom.** The levels of one
  story are rewrites of the same reporting, so **the same photograph should
  carry the same caption in every level; where they disagree, one of them is
  usually corrupt** — PDF columns get interleaved, and a caption can go missing
  entirely. This is the highest-yield check, and the script only sorts: Newsela
  really does rewrite some captions per level, and the worst corruption can
  score as less similar than a legitimate rewrite. The script's own labels say
  which cases are certain; the rest are for you to judge.
- **Open the image.** When a caption is ambiguous, look at the picture rather
  than taking the majority spelling. One batch shipped two captions for one
  photograph; the majority caption described a photograph that was never
  exported, and would have gone live under an unrelated image.
- **Does anything markdown survive into the body?** Grep the parsed bodies for
  `^#`, `![`, `**`. A heading marker the parser does not know gets printed to
  the student verbatim.
- **Read `audit_names.py` hits in context.** Most are singulars and demonyms.
  A real-looking OCR slip can be a real name (`Hetal Patel`, not `Metal`).

## What each new batch has broken so far

Assume the export format drifted. Check, in this order: the directory shape,
the heading marker (`##` vs `###`), whether there is now a byline line under
the meta line, and which words introduce a credit (`Photo:`, `Photo credit:`,
`Image:`, `Art:`, `Drawing:`, `Photo from`, `(AP Photo/…)`). A credit marker
the parser misses does not error — the credit just stays glued to the end of
the caption.

## Two rules that are not negotiable

- **A story that cannot ship goes in `holds` with a reason, never quietly
  dropped.** A slug that is in neither `articles` nor `holds` fails the build,
  so forgetting one can never look like a decision.
- **Do not invent a value you cannot verify.** Where a number or a name is
  wrong and the right one is not findable, delete the wrong part rather than
  guessing — a student will quote it. `corrections.json` records why, per entry.

## Common mistakes

| Mistake | What happens |
|---|---|
| Hand-editing `articles.json` | Overwritten at the next build |
| Bumping a hardcoded article count to make a test pass | The test stops meaning anything; use a floor |
| Working in a git worktree | `docs/reference/` is gitignored and lives only in the main checkout — symlink it in or `parse.py` finds no corpus |
| Tagging with a discipline you invented | Build fails; the 42-entry closed table is `packages/contracts/disciplines/disciplines.json` |
| Trusting `finish` on the level count | One story arrived with 4 levels, not 5; the tier names assume 5 |
| Treating the audits as gates | All three only print. Skipping them builds green — nothing downstream reads their output |
| Assuming the article count proves nothing was lost | The count checks are floors. Losing ONE article out of a batch trips no gate anywhere; compare the shipped slugs against the batch yourself |
