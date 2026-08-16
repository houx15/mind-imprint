# Slice 6 — PDF Viewer + Signed-URL Refresh Safety

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** Replace the broken PDF scaffold with a real, slot-contained, browser-native PDF viewer (multi-page + zoom + download via the browser's own viewer, `initialPage`, loading/error/unsupported states); and make signed-URL refresh safe for active media (PDF re-reads the renewed URL; video/iframe don't lose state / never 403). Fixes review **P1-07, P1-11**.

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` findings P1-07 (PDF scaffold) + P1-11 (URL refresh breaks active media). Program: `docs/superpowers/specs/2026-08-16-course-runtime-hardening-program-design.md`.

**Product decision (revises D2):** use the review's sanctioned **native-browser PDF viewer** path (contract reduced to browser-native pages/zoom/download/print) rather than bundling `pdfjs-dist` + a worker — far lower integration/CSP risk. Full pdf.js is a documented future enhancement. Be honest in the report that page-count/programmatic-nav is delegated to the browser viewer.

**Depends on:** Slice 5 (slot containment CSS), Slice 3 (asset-url refresh machinery in RuntimeCoursePlayer).

## Global Constraints
- Determinism preserved (no Date.now/Math.random in pure packages).
- Don't break the green suites (renderer 164 / web green / runtime 38 / contract 56).
- The renderer stays pure/injectable; the browser viewer is a plain element (no network in the package beyond the element loading its `src`).

---

### Task 1 — Browser-native PDF viewer (course-renderer)

**Files:** `packages/course-renderer/src/media/pdfEngine.ts`, `packages/course-renderer/src/blocks/media/PdfRenderer.tsx`, `packages/course-renderer/src/styles/course.css` (viewer sizing); tests under `packages/course-renderer/test/media/`.

- [ ] **Slot-contained viewer (P1-07):** render the signed PDF URL in a properly-SIZED container (fills its slot, real height from the Slice-5 stylesheet — no more zero-height viewport) using the browser's native PDF viewer (`<iframe>` or `<embed>` — pick the one that reliably shows the browser toolbar with pages/zoom/download/print; `<iframe>` is usually best). Media containment (Slice 5) keeps it inside the slot.
- [ ] **initialPage:** honor the block's `initialPage` via the URL fragment (`#page=N`).
- [ ] **Loading / error / unsupported states:** show a loading state until the element loads, an error state if it fails, and an "在新标签打开 / 下载" fallback link for a browser that can't inline-render the PDF. A download affordance (the browser viewer has one; also provide an explicit link).
- [ ] **Remove the broken bespoke controls:** the current custom page-nav (`totalPages()===1`, Next always disabled) and non-working zoom are misleading — remove them (the browser viewer provides real nav/zoom), OR keep a minimal honest chrome. Simplify `pdfEngine.ts` accordingly (its `totalPages`-driven Next was the bug); if the `PdfEngine` seam is no longer meaningfully used, reduce it to what the native viewer needs (loaded/error) rather than faking a page count.
- [ ] **Survives URL renewal:** the viewer must pick up a renewed signed URL (coordinate with Task 2) — do not capture the URL in a mount-only effect that then holds an expired URL (the current P1-11 PDF bug).
- [ ] Tests: the viewer renders the resolved URL in a sized container; `initialPage` adds `#page=N`; a load error shows the error + download fallback; no always-disabled fake page controls remain. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

### Task 2 — Signed-URL refresh safety for active media (course-renderer + host)

**Files:** `packages/course-renderer/src/blocks/media/{VideoRenderer,PdfRenderer}.tsx` + `blocks/html/HtmlInteractionRenderer.tsx` + `blocks/ImagesRenderer.tsx` (as needed), `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx` (refresh policy); tests.

**Requirements source:** review P1-11 — `RuntimeCoursePlayer` replaces the WHOLE signed-URL map + forces a re-render, so video/iframe/image `src` change mid-play (reload / lost state) while PDF captures its URL mount-only (keeps the expired one).

- [ ] **Define per-block renewal (P1-11):** do NOT blindly swap an ACTIVE media element's `src` on refresh. Options per block, pick the correct one:
  - **Video:** on a URL refresh, preserve current playback time + cue state (or refresh lazily — only re-resolve the src on a load error / when paused), so a refresh doesn't reset a playing video.
  - **Iframe (interactiveHtml):** do NOT reload the iframe on refresh (that destroys interaction state) — keep the current src; only re-resolve on an explicit reload/error.
  - **Image:** swapping is cheap/harmless — fine to update.
  - **PDF:** the opposite bug — it must actually PICK UP the renewed URL (fix the mount-only capture) so it doesn't hold an expired URL; do it without a jarring reload if possible (or reload is acceptable for a static PDF).
- [ ] **Host refresh policy:** `RuntimeCoursePlayer` should make the refreshed map available WITHOUT forcing a hard re-render that reloads every active media element. The resolver already reads `assetUrlsRef.current` live; ensure a refresh updates the ref and lets each block decide (via the per-block policy above) whether/when to re-resolve — rather than a global `setRefreshTick` that reloads everything mid-use.
- [ ] Tests: a URL refresh during active video does not reset playback/cue state (assert the video element's src/currentTime is preserved or only re-resolved on error); an iframe is not reloaded on refresh; a PDF picks up the renewed URL (no expired URL retained); an image may update. `pnpm --filter @mind-imprint/course-renderer test` (+ `web` if host changed) + `typecheck`.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green (known pre-existing contracts typecheck error excepted).
- [ ] Report notes: PDF page-count/programmatic-nav is delegated to the browser's native viewer (D2 revised); a forced URL refresh during active video/PDF/iframe/image loses no state and produces no 403 (P1-11) — full proof is Slice 10's browser journey.
