# course-12 · slice 4 「跟着录屏，动手核查这条推文」 · 视频到 0:17 就不动了

Reported: *"SIFT 信息横向调查 — slice 4, step 2 - 停, the video pauses at 0:17."*

## What was actually wrong

Slice 4 (`piece-sift-practice`) is a single `interactiveHtml` block,
`interactions/html/sift-check.html`. Its rail has 7 steps; **step 2 is 「停」**
(`data-step="stop"`), the S of SIFT.

Each step plays only a **slice of a longer screen recording**:

| step | file | segment |
|---|---|---|
| 停 `stop` | v1 (42.1s) | 0:00 – **0:17.4** |
| 找报道 `find` | v1 (same file) | 0:17.4 – 0:42.0 |
| 追出处 `trace` | v2 | 0:00 – 0:25.2 |
| 到原文 `origin` | v3 | 0:00 – 0:13.3 |

A `timeupdate` clamp pauses at `seg.end` and rewinds to `end - 0.05`. So the
停 step pausing at 0:17 **is the authored cut, and it is correct** — the file
was probed at 42.1s and the segment map matches it exactly.

What was wrong is that watch mode *also* handed the learner the browser's
**native `<video>` controls**, which advertise the whole 42-second file: a
scrubber sitting at `0:17 / 0:42`, and a play button. Both are lies.

Verified live on prod, after the clamp fired:

```
press native play      -> { currentTime: 17.35, paused: true }   (nothing happens)
drag scrubber to 0:30  -> { currentTime: 17.35, paused: true }   (snaps straight back)
```

The learner sees a video two-fifths through, frozen, with a play button that
does nothing and a timeline that refuses to move. It reads as broken. It isn't
a dead end — 「看完了，动手做 →」 still advances — but nothing says so.

## The fix — stop lying about the timeline, and make the ending legible

`patch.py` makes 9 exact-match edits to the interaction HTML (it refuses to run
unless every needle matches exactly once):

1. **Never attach native controls.** They expose a timeline the code refuses to
   honour. The interaction's own chips become the only transport.
2. **A 播放/暂停 chip**, plus click-the-frame to pause/resume in watch mode —
   pausing mid-segment was the one thing the native controls were good for.
3. **The clamp announces itself**: the hint becomes
   「本段到这里结束——这一步只截取了录屏的其中一节。想再看一遍，点「重看本段」。」
   and the chip becomes 重看本段. A deliberate ending, not a freeze.
4. **A slim progress bar inside the frame, scaled to the segment** (not the
   file), so "how much of this clip is left" is visible at all.

The **CourseDefinition is untouched** — same object key, so no definition-hash
change and no in-progress session reset.

## Verified (patched file, driven headlessly before upload)

| check | result |
|---|---|
| 停 segment plays 0 → 17.35 then parks | ✅ chip 重看本段, hint switched, bar 99.7% |
| native controls present | ✅ never (`ctrls: false` throughout) |
| 重看本段 replays the segment | ✅ back to t≈2.5, chip 暂停, hint restored |
| click frame pauses / resumes | ✅ |
| `find` (17.4→42.0, **same file**) after 停 ended | ✅ plays from 20.4 → 24.4 |
| `trace` (different file) then back to 停 | ✅ restarts at 0, `segEnded` cleared |
| zoom overlay open / toggle / close | ✅ label tracks `segEnded` |
| 看完了，动手做 → do mode, question renders | ✅ completion path untouched |
| page errors / console errors | ✅ none |

## Running it

```sh
python3 fetch_original.py     # pull the live asset into original/
python3 patch.py              # -> patched/course-12__sift-check.html
python3 upload.py --apply     # PUT back to the SAME object key
```

## ⚠️ Uploading is not deploying

`mind-oss.uni-robot.cn` answers with `X-Swift-CacheTime: 2592000` (**30 days**).
After `upload.py --apply` returned `200`, the browser still received the old
bytes. Students keep getting the stale copy until this directory is refreshed
in the Aliyun console (刷新预热 → 目录刷新) — the deploy RAM key cannot purge:

```
https://mind-oss.uni-robot.cn/courses/course-12/interactions/html/
```

And note edges **disagree with each other**: a `curl` that returns the new
bytes does not prove students see them. Verify in a real browser.

## Is it systemic? No.

`sweep_native_controls.py` Range-fetches only the tail of every published
course's `interactiveHtml` assets (the script sits after the base64 blobs, so
this reads ~400KB instead of ~100MB) and greps for the same shape.

**99 interactiveHtml assets across 39 courses — `course-12`'s `sift-check.html`
is the only one that attaches native `<video>` controls.** Four legacy/e2e
fixture slugs (`evidence-comparability`, `academic-writing-sustainability`,
`course-authoring-v14-cover-e2e-20260820`, `follow-the-money-teacher-sim-20260818`)
returned no signable assets and were not scanned; none is a catalog course.
