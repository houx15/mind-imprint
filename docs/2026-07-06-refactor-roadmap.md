# Mind Imprint · Refactor Roadmap (Whole-View)

> 2026-07-06 · Status: decomposition agreed with product, implemented slice by slice.
> Relationship: this is the **execution breakdown**, downstream of `docs/2026-07-06-spec.md` (v0.2 product-refactor spec) and the binding design `思维印记 工作区.dc.html` (Claude Design project `0772a92d-1e24-43b6-9237-5f93fb01f920`).
> Each slice runs its own **spec → plan → TDD build → review → merge**; the next slice starts only after the previous one merges.

---

## 0. Two non-negotiable premises

1. **The backend is Go + Postgres** (`apps/api`); P1–P3 are already merged. The "Node + Express / SQLite" stack line in v0.2 spec §8 is **superseded** — for anything backend / DB / API, the `docs/architecture/*` documents govern.
2. **The client never calls the model directly.** All LLM calls go through the Go gateway; the card-spec single source of truth is the `packages/contracts` registry; the standard envelope is the data spine. (v0.1 foundation, carried forward wholesale.)

---

## 1. Core mechanic (the keystone of the whole refactor)

**Material-anchored guided questioning — card-agnostic:**

- **The agent picks the card.** Reuse the existing `summon_card` decision layer to choose a suitable card at the right moment.
- **The agent generates anchored questions.** Instead of rendering the card's dimensions as a blank form, it **reads the student's real material and generates guiding questions from the card's dimensions, anchored to specific spans** (original text highlighted + side notes).
- **Invite the student to ask their own questions.** After the student answers, **sometimes** invite them to pose their own question on the same dimension — because one dimension can be understood from several angles.
- **Reveal the framework only at close.** The card's abstract framework appears afterward, as a transferable **takeaway**, not as an entry form.

CRAAP / SIFT / Concession / etc. **all run through this one engine** — that is the cost-control point. Engineering builds **one material-anchored runtime**, driving both the working-portal tool cards and the course challenges.

---

## 2. The five slices (agreed decomposition and order)

| # | Slice | Delivers | Touches | Why here |
|---|-------|----------|---------|----------|
| **1** | **Nav / IA shell** | 4-tab left rail ("课程" · "批判思维" · "我的评估" · "设置"), tab renames, "项目" (project) vocabulary, directory reskin, and the real Courses card grid rendered from a **mock course fixture** (cards inert on click; the player is Slice 4) | Frontend only | Thin, safe, fast to merge; gives every later slice a home, with no backend risk |
| **2** | **Material substrate** | `material` entity (text + span index) into contracts + Go + API; right sidebar flips between Material / Process-tree; article rendering with highlightable spans; scratch note; material tabs (article / PDF / draft) | Frontend + Go | The card mechanic must anchor to material with spans — build the substrate before the mechanic |
| **3** | **Material-anchored card engine** ⭐ | `anchors` added to the envelope; agent generates anchored guiding questions per card dimension (backend prompt/gateway); cards render inline as conversation "branches", **generically driven by dimensions**; invite-student-question step; submit-and-pin-to-tree; "how to use" methodology modal; close-out abstract framework | Frontend + Go | **The keystone** — the real new value; slices 1–2 exist to serve it |
| **4** | **Courses pillar** | Course player (teaching scenes + challenge scenes that **reuse the slice-3 runtime**); course report; course content-pack format + backend course data (**replaces the slice-1 mock fixture**); stage progress; voice (TTS/STT) as its own sub-step | Frontend + Go | Depends on the slice-3 runtime; voice is the one genuinely new tech and is isolatable |
| **5** | **My-Evaluation + Settings polish** | Reskin the existing Records surface into the "我的评估" design; Settings screen | Frontend only | Purely cosmetic; safest to land last |

- Slices **2+3** = the "P0 core" the spec calls out; **1** is the foundation beneath them; **4** = P1; **5** = polish.
- **Product-design portal (the second working portal) = P2, not in this round** (slice 1 does not even stub it — the binding design omits it).
- **No teacher-facing surface.** Evaluation data still runs; it is shown only to the student.

---

## 3. Invariants carried forward (v0.1 / v0.2 still hold)

Four design laws: AI stays restrained and never concludes for the student · no manipulation (no addictive mechanics, no forced modals) · one question at a time · process is data.
Not building: document editor / authoring / submission (the process tree is read-only); no addictive gamification.

---

## 4. Progress tracker

- [x] Slice 1 · Nav / IA shell — branch `slice-1-nav-ia-shell` (kept, unmerged)
- [x] Slice 2 · Material substrate — branch `slice-2-material-substrate` (stacked, kept)
- [x] Slice 3 · Material-anchored card engine ⭐ — branch `slice-3-anchored-cards` (stacked, kept)
- [x] Slice 4 · Courses pillar (hybrid AI-generated; voice deferred to 4-voice) — branch `slice-4-courses` (stacked, kept)
- [x] Slice 5 · My-Evaluation + Settings polish — branch `slice-5-eval-settings-polish` (stacked, kept)

> ✅ **All 5 slices complete** (2026-07-07). Each built TDD via subagent-driven development with per-task + whole-branch reviews; branches stacked and kept unmerged per the user. Deferred follow-ups: 3d polish (methodology modal, close-out framework, click-sync), 4-voice (TTS/STT), 4c-ask (course ask-panel), course-level SOLO eval + note-export, CoursePlayer render-error state.
