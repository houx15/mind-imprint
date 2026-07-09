# E2E Audit (PM lens) — 2026-07-08 · voice merge + card-flow

> Second live full-stack smoke, run from a product/project-manager perspective after the **voice (TTS/ASR) feature merged** onto `polish-e2e-fixes` and the earlier P0 fixes landed. Stack: web `:5174` → Go API `:8080` (**voice enabled**) → throwaway Postgres (migrated to `0015`, seeded) → **real DeepSeek** + **real Volcano Engine**. Driven through the browser as seeded student **Phoebe**. Scenario: "中国是否让地球变得更可持续？ / 卫星图显示中国让地球变绿".

Companion to `docs/2026-07-07-e2e-audit-and-polish-subtasks.md`. That run's **P0 (ST-1 rehydration)** is **re-verified FIXED** here. This run surfaces **three new material findings**, one of which silently guts the flagship loop.

---

## 1. Verdict

The platform is **still end-to-end functional and the AI quality is still genuinely high.** The previous release-blocker (task history not rehydrating) is **fixed and confirmed**. But driving the *annotation/keystone* card path and the *newly-merged voice* feature — neither of which the last audit exercised to completion — surfaced:

- **🔴 P1-A (flagship-critical, silent):** annotation-mode cards persist the student's answers in `anchors`, but **both** the chaperone **refeed** and the **evaluator** read only `field_values`. The flagship keystone card's actual thinking is dropped from "摘要回灌" and "过程即数据." The card looks used; it is hollow to the two systems that matter.
- **🔴 P1-B (cross-cutting):** the CORS allow-list is missing `PATCH` and `DELETE`, so **card-open, class-rename, remove-student, and unassign-teacher all fail from the browser.** Card-open failure is masked by optimistic UI; the teacher/admin ops just break.
- **🟠 P1-C (new feature non-functional live):** voice TTS is fully integrated and key-safe, but the **live Volcano call fails with `code 55000000`** regardless of speaker or format. It was only ever validated against a fake WS server.

The core is sound; these are the "challenging parts" the product still needs to close, plus two regressions the annotation/voice work introduced or unmasked.

---

## 2. What was re-verified (works) ✅

- **Auto-login** (`?trial=1`) as seeded Phoebe → directory greeting "下午好，Phoebe".
- **Courses**: grid renders the real course; **live AI teaching render** ("「随手一信」的陷阱", on-topic, in-template); persisted to `course_step_render`.
- **Workspace opening turn** is on-model: reframes 事实 vs 价值判断, distinguishes fact-claim from inference, **asks the student to paste the article** ("记得我打不开链接"), one question at a time. **AI restraint intact.**
- **Card summon**: AI summoned `sift_craap` with a contextual `nudge_text`; **confirm-to-open** respected (打开卡 / 暂不).
- **Schema-driven fill + submit**: annotation card rendered with 2 dimensions + material pane + the keystone affordance ("在右侧文章里划一句，自己向印记提问"); submit → **"已完成 · 已钉到过程树"**, workspace header **"已用 1 张工具卡"**.
- **Process tree** grows (任务根 → 工具卡使用 · SIFT×CRAAP · 已完成).
- **Milestone auto-eval** fired after the turn and rendered **"你的思维印记"**: SOLO framing, dimension families (🚀 生成式驾驭 / 🛡️ 批判式防护), honest process narrative.
- **🟢 ST-1 rehydration — FIXED (re-verified):** after full reload + re-entry, the conversation, the completed card, the process tree, and the evaluation **all survive**. The previous P0 is genuinely resolved.
- **Voice UI surfaces present**: 朗读本节 (course), 按住说话 mic + 朗读 play (workspace) all render with inline-SVG icons.
- **Key isolation** holds: the failing TTS call returns a clean `500` to the browser; the real Volcano error is logged **server-side only**, no secret leak.

Persisted state (Postgres): `messages` 3, `card_instances` 1 (completed), `evaluations` 1, `course_step_render` 1.

---

## 3. Findings (severity-ranked)

### 🔴 P1-A — Annotation/keystone card content never reaches refeed **or** evaluation
**Symptom:** after completing the `sift_craap` card with detailed SIFT + CRAAP answers, the AI's refeed replied **"你已经接受了这张卡，但看起来还没开始填写步骤"**, and the evaluation scored **"信息素养 0 项已评"** with a narrative that the student "思考仅停留在问题层面，没有进一步拆解、查证" — as if the card were empty.

**Root cause (confirmed at the data layer):**
- The completed card row has **`field_values = {}`** (empty). The student's two answers are persisted in **`anchors`** instead (each as an anchor with `dimension`, `question`, `answer`, `quote:""`, `author:"ai"`).
- `apps/api/internal/agent/refeed.go` — `SerializeCardForRefeed` reads only `FieldValues`.
- `apps/api/internal/agent/eval.go:187-198` — `EvalCards` builds eval input from `FieldValues` + `EventTraceLen` only; **never reads `anchors`**.

**Why it matters:** annotation mode is the flagship "material-anchored questioning" card family. Its content is persisted and shown in the UI, but is **invisible to the chaperone refeed and to the SOLO evaluator** — a direct violation of the two core invariants **摘要回灌** and **过程即数据**, precisely for the card type the product is built around. The card counts as "used" but teaches the downstream systems nothing.

**Fix direction:** make refeed + eval-input read annotation answers. Either (a) map annotation dimension answers into the `field_values` shape on submit, or (b) teach `SerializeCardForRefeed` and `EvalCards` to fold `anchors[].{dimension,question,answer}` into their payloads. Add a refeed golden + an eval-input test that asserts a completed *annotation* card contributes non-empty content.

---

### 🔴 P1-B — CORS allow-list omits `PATCH` and `DELETE` → four browser operations broken
**Symptom:** opening a tool card logs a CORS error — `No 'Access-Control-Allow-Origin' header ... /tasks/{id}/cards/{cid}`. The card still *appears* to open (optimistic UI), but the server never records the `active` transition.

**Root cause (confirmed):** `apps/api/internal/httpx/middleware.go:97`
```go
AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
// missing: http.MethodPatch, http.MethodDelete
```
Verified empirically: the `PATCH` preflight returns `204` with **no** `Access-Control-Allow-Origin`; the `PUT` preflight returns the header. Any browser blocks the request.

**Blast radius (client actually issues these):**
| Client call | Method | Effect when blocked |
|---|---|---|
| `cards.ts` `activateCard` (card open) | PATCH | "active" transition never recorded; open-without-submit leaves **no** server trace |
| `classes.ts` rename / settings | PATCH | teacher cannot rename a class from the browser |
| `classes.ts` `removeEnrollment` | DELETE | teacher cannot remove a student |
| `admin.ts` remove class teacher | DELETE | admin cannot unassign a teacher |

Card **submit** (PUT) and **skip** (POST) are unaffected, which is why the last audit's card lifecycle looked fine — it never depended on the PATCH.

**Fix (one line):** add `http.MethodPatch, http.MethodDelete` to `AllowedMethods`. Add a preflight test asserting `Allow-Origin` is echoed for PATCH/DELETE.

---

### 🟠 P1-C — Voice TTS fails against the live Volcano account (`code 55000000`)
**Symptom:** clicking 朗读本节 → `POST /api/v1/voice/tts` returns `500`; server log: `voice: tts error from server (code 55000000)`.

**Diagnosis (3 variables tested, all `55000000`):**
- Speaker id: tried the default guess **and** the reference repo's known-good `zh_female_tianmeixiaoyuan_moon_bigtts` → no change.
- Audio format: tried `mp3` (our code) **and** `pcm` (the reference's value) → no change.

So it is **neither the speaker nor the format** — `55000000` is a generic Volcano "invalid params/auth". The likely causes are (a) this Volcano app (`.env.volcano`) isn't provisioned for the `seed-tts-2.0` TTS resource, or (b) a protocol-framing/handshake mismatch vs. the live v3 endpoint. The feature was only ever validated against the **fake WS server** in `tts_test.go`, which cannot catch this.

**Positive:** the whole pipeline is wired (button → client → API → Volcano WS → error path), and key isolation is clean (browser sees a bare `500`; the Volcano code is logged server-side only, no secret leak). This is a **config/live-probe** gap, not a broken build.

**Fix direction:** confirm the Volcano app's enabled TTS resource id in the console (may not be `seed-tts-2.0`); align `VOICE_TTS_RESOURCE_ID` + speaker; if still failing, diff our v3 binary framing against a raw capture of the reference client. ASR (the spoken-input half) was not reachable to test once TTS was down.

---

### 🟡 P2 — Directory card-count badge shows "0 张卡" (workspace shows "1")
The project card in the directory list reads **"0 张卡"** while the workspace header for the same task correctly reads **"已用 1 张工具卡"**. The ST-1 fix repaired the workspace/history count path but the directory list badge still reads from a source that undercounts. (Carried from last audit's P2; now pinpointed to the directory list, not the workspace.)

---

## 4. Observations (not bugs, but product-flow risks)

- **Keystone never triggered in the positive case.** The AI summoned the material-anchoring card *before* the student pasted the material, so the AnchorGenerator had no material to anchor to — the persisted anchors have `quote:""`, `block_id:""`, and the card fell back to generic schema prompts ("从「SIFT」这个角度看这份材料…"). The flagship "AI asks a question pinned to a sentence you're reading" was **not observed**. Consider gating the material-anchoring summon on material-present, or re-anchoring when material arrives.
- **Refeed round-trip cost.** Submitting the card produced an extra AI turn that re-asked for the article — partly a symptom of P1-A (it couldn't see the answers). Fixing P1-A should also fix this UX.
- **`?trial=1` auto-eval.** A milestone evaluation auto-fired without the student pressing 生成思维印记 (directory then labels the task 已评估). Confirm this is intended (it reads well, but it front-runs the student's own "生成" action).

---

## 5. Recommended order

1. **P1-A** (flagship data-flow) — highest product value; the keystone is inert without it.
2. **P1-B** (CORS one-liner) — cheap, unblocks card-open telemetry + all teacher/admin PATCH/DELETE.
3. **P1-C** (voice live-probe) — needs Volcano console access; isolate resource-id/provisioning before shipping voice.
4. **P2** (directory badge) — small.

Findings 1–2 are code fixes in this repo; 3 is a config/provisioning probe. None block the *existing* (form-card) lifecycle, which remains solid.

## 6. How to reproduce
```
docker run -d --name mindimprint-e2e-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mindimprint -p 55432:5432 postgres:16
cd apps/api && DATABASE_URL="postgres://postgres:postgres@localhost:55432/mindimprint?sslmode=disable" go run ./cmd/api -migrate-up
# API with voice: map .env.volcano APP_ID/ACCESS_TOKEN → VOICE_APP_ID/VOICE_ACCESS_KEY, set VOICE_TTS_VOICE, CORS_ORIGINS=http://localhost:5174
pnpm --filter web dev --port 5174 --strictPort
# open http://localhost:5174/?trial=1
```
(Web ran on `:5174` because `:5173` was occupied by the peraspera marketing dev server.)
