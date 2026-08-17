# Course Runtime 2.0 — Release Gate & Browser Journey (Slice 10)

> **Status:** the **error-handling** half of Slice 10 (P2-10) is BUILT + shipped. The **real-browser 12-criteria journey** (P2-11 / the review's release-acceptance gate) is SPECIFIED here but **blocked on real fixture media** — see "Asset dependency" — so it is not yet a running green gate.

## What Slice 10 shipped (P2-10 — error handling)
- `CoursesContainer.PlayerRouter` now falls back to the legacy player **only on a genuine 404** (the authoritative "no 2.0 definition → legacy course" signal). A non-404 error (auth 401/403, network, 5xx, a malformed 422 definition) now shows a **retryable error state** instead of masking the real failure by mounting an unrelated legacy player. (`fix(web): legacy-player fallback only on 404` — `9f9ab90a`.)
- Per-block error surfaces already landed across Slices 2–7: scene-audio fallback (S2), video-interaction load error card + retry (S3), PDF load error + open/download fallback (S6), HTML load/validation error (S7). A single top-level course error boundary is a small future add on top of these.

## The release-acceptance gate (from the code review, §"Required release acceptance gate")
The student runtime is not "preview/publish ready" until ONE production-host browser journey proves all 12, with **real fixture assets**:
1. A valid CourseDefinition loads through the real API and matches the supported desktop one-screen layout.
2. All four layout presets render without uncontrolled page-scroll or clipped essential content.
3. Prepared narration plays or presents a recoverable user-start fallback.
4. A multi-page PDF loads, navigates, zooms, downloads, and survives URL renewal.
5. A video reaches a required cue, auto-pauses, shows the chosen (modal) presentation, saves the answer/result, resumes, and completes.
6. Interactive HTML completes only with a valid typed learning-result payload; invalid payloads are rejected.
7. HTML music follows the declared autoplay/fallback + audio-arbitration policy, and stops on hide/disable/navigation.
8. The HTML frame cannot make prohibited external network requests.
9. Previous/manual/automatic navigation and `student.continue` follow `NavigationDefinition`.
10. Reloading mid-course restores the current Slice, workflow step, attempts, cue state, interaction result, and media position.
11. The Closing uses real saved session evidence, is visible/audible, and only then completes the session and routes onward.
12. A transient session-save or signed-URL-refresh failure is visible/retried and does not erase learner progress.

### Coverage status per criterion (implemented at the unit/integration level; browser proof pending assets)
| # | Implemented in | Unit/integration proof | Browser proof |
|---|---|---|---|
| 1,2 | Slice 5 (course.css / one-screen) | structural (CSS source + classes) | **needs assets + viewport matrix** |
| 3 | Slice 2 (scene/narration audio + fallback) | ✅ renderer tests | needs a real audio asset |
| 4 | Slice 6 (native PDF viewer + URL-refresh) | ✅ renderer tests | **needs a real multi-page PDF** |
| 5 | Slice 3 (video cue modal + result + gate) | ✅ renderer tests | **needs a real MP4 + interaction JSON** |
| 6 | Slice 7 (typed HTML payload validation) | ✅ contract+renderer tests | **needs a real interactive HTML asset** |
| 7 | Slice 7 (audio arbiter + lifecycle) | ✅ renderer tests | needs a real audio-capable HTML asset |
| 8 | Slice 9 (publish self-containment gate) + sandbox | ✅ Go validator tests | runtime CSP header deferred (infra) |
| 9 | Slice 4 (NavigationDefinition) | ✅ renderer tests | needs a multi-slice real course |
| 10 | Slice 1 (persistence + resume) + S8 (revision reset) | ✅ runtime+renderer+host+Go tests | needs a real course to reload |
| 11 | Slice 2 (Closing lifecycle) | ✅ renderer+host tests | needs a real course |
| 12 | Slice 1 (durable saves) + S6 (URL-refresh) | ✅ host tests | needs a live session |

## Asset dependency (why this can't be a green gate yet)
The seeded golden course (`apps/api/internal/store/seed/courses/coverage-course.json`) references **placeholder asset paths with no stored OSS objects** (video/pdf/html/audio). A production-host browser journey over it dead-ends at the first real-media step (video/pdf/html), and Slice 9's asset gate would (correctly) refuse to publish it. Proving criteria 4/5/6/7/8 end-to-end therefore requires **real fixture media** — a real MP4 (H.264/AAC/faststart), a real multi-page PDF, a real self-contained interactive HTML (with the protocol handshake + a music asset), and narration audio (or TTS via the ship endpoint) — uploaded to `courses/<slug>/…` via the authoring `POST …/asset-upload-url` endpoint. This is a **content-provisioning task** (authoring real assets), not a code change.

## Playwright journey plan (ready to run once real assets exist)
Add `apps/web/e2e/journey-course-runtime-2.0.spec.ts` (mirroring the existing `journey-course-loop.spec.ts` legacy journey) that, against a seeded course with **real** assets:
1. logs in (the marketing `?trial=1` seeded-Phoebe path, or a seeded student), opens the 2.0 course → asserts the runtime player mounts (not legacy) and the shell does not page-scroll (criteria 1,2).
2. plays through narration (assert audio element / fallback), each layout preset, the PDF (page nav + download), the video to its required cue (assert modal, answer it, assert resume + gate), the interactive HTML (assert typed completion + rejected-invalid; assert no external network via request interception for criterion 8), navigation (previous/next/student.continue), reload mid-course (assert resume), and the Closing (assert visible before route change).
3. injects a transient save/URL-refresh failure and asserts the visible retry + no progress loss (criterion 12).

Until real assets land, run the existing unit/integration suites (course-contract / course-runtime / course-renderer / web / Go) as the standing regression gate — they cover every criterion at the component level.

## Remaining deferred work (post-Slice-10, needs assets/infra)
- Real fixture media assets (content) → then run the Playwright journey to actually close the 12-criteria gate.
- Slice 9 deep binary asset validation (video H.264/AAC/faststart, PDF page-tree, WebVTT) — reuse `internal/docextract` for PDF; needs real assets to be meaningful.
- Runtime CSP: a server-served `Content-Security-Policy` header on HTML assets (OSS object metadata / serving proxy) — infra.
- Full audio ducking (Slice 7 shipped basic pause-others); a top-level course error boundary; the Slice-4/5 a11y label polish.
