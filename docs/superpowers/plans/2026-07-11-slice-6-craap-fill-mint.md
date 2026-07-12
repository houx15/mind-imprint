# Slice 6 — CRAAP fill→mint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Light up the CRAAP fill→mint bridge so a real student can complete a source-evaluation card end-to-end — AI anchors per-dimension questions on source spans, the student answers each + writes 作用与风险, locks, and the answered anchors mint an evidence node and flip the `every_source_evaluated` gate.

**Architecture:** Port the legacy "AI-anchors-questions → student-answers" pedagogy into the Studio path. Backend: fix `AnchorGenerator` to key anchors off the card's `params.tags` (so `dimension` == the completion tag), and wire anchor generation into the Studio surface seam. Frontend: a new `StudioAnnotateCard` renders anchors as answerable questions + a risk-note field + a lock button, submitting filled anchors through 5c-2's existing submit→mint endpoint. The submit→mint→gate→refeed backend path is untouched 5c-2 code.

**Tech Stack:** Go (`net/http`, sqlc, testcontainers), TypeScript + React + vitest, Zod contracts.

## Global Constraints

- **The client never calls the model directly.** Anchor generation runs server-side (`apps/api`); the frontend only renders anchors it is handed and posts filled anchors back. (AGENTS.md hard constraint.)
- **`dimension` on a generated anchor MUST be the completion tag** (`currency` / `relevance` / `authority` / `accuracy` / `purpose`), never a step title. This is what makes `EvaluateCompletion`'s `every_tag_present` satisfiable. (The load-bearing correctness fact — spec §"generator↔completion reconciliation".)
- **The answer renderer is authorship-agnostic.** `StudioAnnotateCard` renders whatever anchors it is given and never assumes the AI authored them — this keeps guidance levels L2/L3 (student finds spans / elicits questions) as future config, not a rewrite. Add a short comment saying so at the generation seam and in the component.
- **Binding design copy is verbatim.** The CRAAP coach-rail card matches `docs/design/思维印记_工作区.dc.html` `s3CoachCard` (~L1282–1320). Chinese UI copy is copied exactly; fix the test, never the copy.
- **Icons are inline SVG**, not `lucide-react`.
- **Card JSON is single-source-of-truth; do not add or restructure a card renderer per card.** No changes to `craap.json` in this slice (its `steps` block is the deferred consolidation framework).
- **Go tests run serialized** — `CGO_ENABLED=0 go test -p 1 ./...` (parallel hangs on Docker contention on this machine). `make sqlc` = `CGO_ENABLED=0 go tool sqlc generate` in `apps/api`.
- **No `/dev/null` redirects, out-of-repo paths, or bare `eval`** (project-boundary hook blocks them).

---

## File Structure

**Backend (`apps/api/internal`):**
- `agent/anchors.go` (modify) — dimension-keyed generation branch (`fallbackAnchors` + `buildAnchorPrompt`).
- `agent/anchors_test.go` (modify/add) — assert generated `dimension` == completion tags for a `params.tags` card.
- `api/studioturn.go` (modify) — expand `streamAction` into a method that, for a surfaced `annotate` card, generates + persists + emits anchors; update both call sites.
- `api/studioturn_test.go` (modify) or a new test — surface craap in the Studio path → persisted anchors carry tag dimensions.
- `api/projectcards_test.go` (modify/add) — the non-vacuous end-to-end: surface → fill answers + risk_note → submit → new evidence node minted; negative: missing dimension → active, no mint.

**Frontend (`apps/web/src`):**
- `studio/StudioAnnotateCard.tsx` (new) + `.test.tsx` — the annotate answer-mode host.
- `studio/conversation.ts` (modify) — carry `anchors` on `CardState`.
- `studio/CoachRail.tsx` (modify) — fork `annotate` → `StudioAnnotateCard`; `LiveCard` gains `anchors`.
- `studio/state.ts` / `StudioContainer.tsx` / `StudioShell.tsx` (modify) — thread `anchors` + primitive through to the rail.
- `workspace/material/SourceDossier.tsx` (modify, thin) — highlight live anchor spans in the open source.

**Interfaces produced (referenced across tasks):**
- Go `Anchor` (`agent/anchors.go`): `{ ID, MaterialID, BlockID string; Start, End int; Quote, Dimension, Author, Question, Answer string }`.
- TS `Anchor` (`@mind-imprint/contracts`, `anchor.ts`): `{ id, material_id, block_id: string; start, end: number; quote, dimension, author: "ai"|"student", question, answer: string }`.
- TS `CardInstance` (`envelope.ts`): carries `field_values`, `event_trace`, and `anchors: Anchor[]` (default `[]`).
- `submitProjectCard` (5c-2, unchanged): `POST /projects/{id}/cards/{cid}/submit` body `{ field_values, event_trace, anchors }` (SSE).

---

## Task 1: Dimension-keyed anchor generation (the load-bearing fix)

**Files:**
- Modify: `apps/api/internal/agent/anchors.go` (`fallbackAnchors`, `buildAnchorPrompt`)
- Test: `apps/api/internal/agent/anchors_test.go`

**Interfaces:**
- Consumes: `cards.Spec.Params.Tags []string` + `cards.Spec.Params.TagPrompts map[string]string` (the C2 annotate dimensions + per-dimension prompts).
- Produces: for a `params.tags` card, `fallbackAnchors`/generated anchors carry `Dimension == tag` (e.g. `"currency"`).

**Context:** Today `fallbackAnchors` iterates `spec.Steps` and sets `Dimension: st.Title` (e.g. `"C · Currency 时效性"`). `EvaluateCompletion`'s `every_tag_present` checks `Dimension == "currency"`. They never match → no live CRAAP fill can ever complete. The fix: when `spec.Params.Tags` is non-empty, key off tags; else keep the legacy step-title path (legacy cards without a C2 block are unaffected).

- [ ] **Step 1: Write the failing test**

Add to `anchors_test.go` (load the real craap spec via `cards.ByID("craap")`):

```go
func TestFallbackAnchorsKeysOffParamsTags(t *testing.T) {
	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("craap spec not found")
	}
	mats := []Material{{ID: "m1", Blocks: []Block{{ID: "b0", Text: "some source text"}}}}
	got := fallbackAnchors(spec, mats)

	// One anchor per completion tag, dimension == the tag (NOT the step title).
	wantDims := map[string]bool{"currency": false, "relevance": false, "authority": false, "accuracy": false, "purpose": false}
	for _, a := range got {
		if _, isTag := wantDims[a.Dimension]; !isTag {
			t.Fatalf("anchor dimension %q is not a completion tag (regression: step-title keying)", a.Dimension)
		}
		if a.Author != "ai" || a.Answer != "" {
			t.Fatalf("generated anchor must be ai-authored with empty answer, got author=%q answer=%q", a.Author, a.Answer)
		}
		if a.Question == "" {
			t.Fatalf("anchor for %q has empty question", a.Dimension)
		}
		wantDims[a.Dimension] = true
	}
	for tag, seen := range wantDims {
		if !seen {
			t.Fatalf("no generated anchor for completion tag %q", tag)
		}
	}
}
```

(If `Block`/`Material` field names differ, read `agent/prompt.go` / `agent/anchors.go` for the exact struct — `Material{ID, Blocks}`, `Block{ID, Text}` — and match them.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestFallbackAnchorsKeysOffParamsTags -v`
Expected: FAIL — current `fallbackAnchors` yields step-title dimensions.

- [ ] **Step 3: Implement the branch**

In `fallbackAnchors`, branch on `params.tags`:

```go
func fallbackAnchors(spec cards.Spec, materials []Material) []Anchor {
	matID := ""
	if len(materials) > 0 {
		matID = materials[0].ID
	}
	// C2 annotate cards (params.tags present) key anchors off the completion
	// tags so EvaluateCompletion's every_tag_present is satisfiable. Guidance
	// level L1: the AI authors the question here; higher levels populate
	// anchors from student action upstream without touching this path.
	if len(spec.Params.Tags) > 0 {
		out := make([]Anchor, 0, len(spec.Params.Tags))
		for i, tag := range spec.Params.Tags {
			q := spec.Params.TagPrompts[tag]
			if q == "" {
				q = "从「" + tag + "」这个角度看这份材料，你注意到什么？"
			}
			out = append(out, Anchor{
				ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
				Dimension: tag, Author: "ai", Question: q, Answer: "",
			})
		}
		return out
	}
	// Legacy step-title path (cards without a C2 params block).
	out := make([]Anchor, 0, len(spec.Steps))
	for i, st := range spec.Steps {
		out = append(out, Anchor{
			ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
			Dimension: st.Title, Author: "ai",
			Question: "从「" + st.Title + "」这个角度看这份材料，你注意到什么？",
			Answer:   "",
		})
	}
	return out
}
```

Then update `buildAnchorPrompt` so the LLM path also produces tag-keyed dimensions when `params.tags` is present: list the tags + their `tag_prompts` and require the model's `dimension` field to be exactly one of the tags. Keep the legacy step-title prompt when `params.tags` is empty. (Mirror the existing prompt's JSON-array output contract; only the dimension vocabulary changes.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestFallbackAnchorsKeysOffParamsTags -v`
Expected: PASS. Also run the whole agent package to catch legacy-path regressions: `CGO_ENABLED=0 go test -p 1 ./internal/agent/`.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/anchors.go apps/api/internal/agent/anchors_test.go
git commit -m "fix(refactor2): key annotate anchor generation off params.tags so completion is satisfiable"
```

---

## Task 2: Wire anchor generation into the Studio surface seam

**Files:**
- Modify: `apps/api/internal/api/studioturn.go` (`streamAction` → method; both call sites)
- Test: `apps/api/internal/api/studioturn_test.go` (or a new `projectcards`-style testcontainers test)

**Interfaces:**
- Consumes: `Task 1`'s generator; `a.d.Provider`, `a.d.ChatResolver`, `a.d.Queries`; `agent.NewAnchorGenerator`, `agent.NewSqlcAgentStore`, `store.SetCardInstanceAnchors`, `ListMaterialsByProject`, `cards.ByID`.
- Produces: on a surfaced `annotate` card, `card_instance.anchors` is persisted and the `card` SSE frame carries the same JSON (no longer `[]byte("[]")`).

**Context:** `streamAction(em, action)` is a package function with no context/store/deps. Anchor generation needs all of them. Expand it to a method and pass what it needs. Both call sites — `postProjectTurn` (~L176) and `submitProjectCard` (~L254) — already have `r.Context()`, `store`, and `projectID` in scope.

- [ ] **Step 1: Write the failing test**

A testcontainers test (mirror the existing `projectcards`/`studioturn` testcontainers setup — seed a project with a source `material` via `CreateProjectMaterial`, force a `surface_card` for craap). After driving the surface path, assert the persisted `card_instance.anchors` is non-empty and every anchor's `dimension` is a completion tag:

```go
// after the surface step produces a craap card_instance `cid`:
row, err := q.GetCardInstance(ctx, cid)
// unmarshal row.Anchors (jsonb) into []agent.Anchor
if len(anchors) == 0 {
	t.Fatal("expected generated anchors on annotate-card surface, got none")
}
tags := map[string]bool{"currency": true, "relevance": true, "authority": true, "accuracy": true, "purpose": true}
for _, a := range anchors {
	if !tags[a.Dimension] {
		t.Fatalf("persisted anchor dimension %q is not a completion tag", a.Dimension)
	}
}
```

(Read the existing Studio testcontainers test for the exact harness — DB bootstrap, `API` construction with a stub/echo provider so the generator's resolver path falls back deterministically, and how a `surface_card` is forced.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run <NewTestName> -v`
Expected: FAIL — surface currently emits `[]byte("[]")` and persists nothing.

- [ ] **Step 3: Implement**

Convert `streamAction` to a method and generate on annotate surface:

```go
func (a *API) streamAction(ctx context.Context, em *studioEmitter, action *agent.Action, projectID uuid.UUID, store agent.AgentStore) {
	switch {
	case action == nil:
	case action.Kind == "surface_card":
		spec, _ := cards.ByID(action.CardID)
		anchors := []byte("[]")
		if spec.Primitive == "annotate" {
			if raw, ok := a.surfaceAnchors(ctx, store, projectID, spec, action.CardInstanceID); ok {
				anchors = raw
			}
		}
		_ = em.Card(action.CardInstanceID, action.CardID, spec.Name, anchors)
	case action.Kind == "intervention":
		_ = em.Intervention(action.InterventionID, action.Output.Body, "", action.Output.Criterion, "")
	case action.Kind == "check_gate" && action.GateReport != nil:
		gr := action.GateReport
		_ = em.Gate(gr.Contract, gr.Status, 0, 0, gr.Missing)
	}
}

// surfaceAnchors generates the L1 AI anchors for an annotate card, persists
// them on the card_instance, and returns the JSON to carry on the SSE frame.
// Degrades to (nil,false) on any error so the card still surfaces (the
// student simply sees no pre-anchored questions) rather than failing the turn.
func (a *API) surfaceAnchors(ctx context.Context, store agent.AgentStore, projectID uuid.UUID, spec cards.Spec, cardInstanceID string) ([]byte, bool) {
	cid, err := uuid.Parse(cardInstanceID)
	if err != nil {
		return nil, false
	}
	materials, err := a.projectMaterials(ctx, projectID) // []agent.Material with ID + Blocks
	if err != nil {
		return nil, false
	}
	gen := agent.NewAnchorGenerator(a.d.Provider, a.d.ChatResolver)
	anchors, err := gen.Generate(ctx, spec, materials)
	if err != nil || len(anchors) == 0 {
		return nil, false
	}
	raw, err := json.Marshal(anchors)
	if err != nil {
		return nil, false
	}
	if err := store.SetCardInstanceAnchors(ctx, projectID, cid, raw); err != nil {
		slog.Warn("surface anchors: persist failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
		return nil, false
	}
	return raw, true
}
```

Add `projectMaterials(ctx, projectID) ([]agent.Material, error)` — `ListMaterialsByProject` → `[]agent.Material` with `ID` **and `Blocks`** (unmarshal each row's `blocks` JSONB into `[]agent.Block`; the fallback needs only `ID`, but the real LLM path needs `Blocks` for quote→offset). If a shared converter already exists (grep `agent.Material{` / how `turn.go` builds the materials it passes to `AnchorGen`), reuse it.

Update both call sites: `postProjectTurn` → `a.streamAction(r.Context(), em, action, projectID, store)`; `submitProjectCard` → same. Add imports (`context`, `encoding/json`, `github.com/google/uuid`) as needed.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run <NewTestName> -v`
Then the package: `CGO_ENABLED=0 go test -p 1 ./internal/api/`.
Expected: PASS; no 5c-2 studio test regressed.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/studioturn.go apps/api/internal/api/studioturn_test.go
git commit -m "feat(refactor2): generate + persist + emit AI anchors on annotate-card surface (Studio path)"
```

---

## Task 3: End-to-end mint test (non-vacuous keystone)

**Files:**
- Test: `apps/api/internal/api/projectcards_test.go`

**Interfaces:**
- Consumes: Task 2's surface (generates anchors), the unchanged 5c-2 `submitProjectCard` (persist → CompleteCard → mint → gate).
- Produces: proof that the real surface→fill→submit path mints — the regression the 5c-2 whole-branch review demanded (no hand-built anchors bypassing the generator).

**Context:** 5c-2's mint test hand-built satisfying anchors, which was vacuous. This test drives the **generated** anchors: surface craap → read the generated anchors → fill each `answer` + append a student `risk_note` anchor → POST submit → assert a NEW evidence node minted.

- [ ] **Step 1: Write the failing test**

Mirror the 5c-2 submit testcontainers test. After surfacing craap (Task 2) and reading the persisted generated anchors:

```go
// fill every generated anchor's answer; append the risk_note anchor
for i := range anchors {
	anchors[i].Answer = "学生的判断与理由，足够长以通过校验"
}
anchors = append(anchors, agent.Anchor{
	ID: "risk_note", MaterialID: anchors[0].MaterialID, BlockID: "",
	Dimension: "risk_note", Author: "student",
	Question: "这条来源在你的论证里起什么作用？有什么风险？",
	Answer:   "它支撑我的核心数据，但只有单一来源，需交叉验证。",
})
body := submitBody{Anchors: mustJSON(anchors), FieldValues: json.RawMessage(`{}`), EventTrace: json.RawMessage(`[]`)}
// POST /projects/{id}/cards/{cid}/submit, drain SSE

nodesBefore := countEvidenceNodes(ctx, q, projectID)
// ... submit ...
nodesAfter := countEvidenceNodes(ctx, q, projectID)
if nodesAfter != nodesBefore+1 {
	t.Fatalf("expected exactly one new evidence node minted, before=%d after=%d", nodesBefore, nodesAfter)
}
// card_instance.status == "completed"
```

Add a **negative case**: submit with one dimension's answer left empty → status stays `active`, no new evidence node (the 5c-2 gating regression). Prove load-bearing (removing the risk_note anchor → test fails).

- [ ] **Step 2: Run test to verify it fails (or errors)**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run <MintE2ETest> -v`
Expected: FAIL until Tasks 1+2 are in (they are) — if it passes immediately, verify it is not vacuous by breaking the answer-fill loop and confirming failure.

- [ ] **Step 3: (no new production code)** This task is the assertion that Tasks 1+2 + 5c-2 connect. If it reveals a gap, fix in the relevant task's file.

- [ ] **Step 4: Run + prove non-vacuous**

Break→fail→revert→pass: empty one answer → the positive assertion fails; restore → passes.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/projectcards_test.go
git commit -m "test(refactor2): non-vacuous CRAAP surface→fill→submit→mint E2E (drives generated anchors)"
```

---

## Task 4: `StudioAnnotateCard` component

**Files:**
- Create: `apps/web/src/studio/StudioAnnotateCard.tsx`
- Test: `apps/web/src/studio/StudioAnnotateCard.test.tsx`

**Interfaces:**
- Consumes: `{ spec: CardSpec; anchors: Anchor[]; onSubmit: (env: CardInstance) => void; onSkip: (eventTrace: TraceEvent[]) => void }`.
- Produces (on lock): a `CardInstance` with `field_values: {}`, `event_trace: [...]`, and `anchors` = the input anchors with each `answer` filled + a `risk_note` anchor (`author: "student"`).

**Context:** Design `s3CoachCard` (~L1282–1320): header (工具卡 · CRAAP + category + 评估这条来源); a hint line; one row per anchor (dimension chip + question + answer `textarea` + ✓ when answered); a **作用与风险（自己写）** `textarea`; a lock button. Copy verbatim from the design. Authorship-agnostic: render whatever anchors are passed.

- [ ] **Step 1: Write the failing test**

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { StudioAnnotateCard } from "./StudioAnnotateCard";

const anchors = [
  { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "", dimension: "currency", author: "ai" as const, question: "数据是哪一年的？", answer: "" },
  { id: "a1", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "", dimension: "authority", author: "ai" as const, question: "谁站在这条主张背后？", answer: "" },
];
const spec = { id: "craap", name: "信源辨识卡 CRAAP / CRRAAB", category: "信息素养", purpose: "x", primitive: "annotate" } as any;

test("renders one answerable row per anchor and locks with filled anchors + risk_note", () => {
  const onSubmit = vi.fn();
  render(<StudioAnnotateCard spec={spec} anchors={anchors} onSubmit={onSubmit} onSkip={() => {}} />);
  const boxes = screen.getAllByRole("textbox");
  // one per anchor + one risk-note field
  expect(boxes).toHaveLength(anchors.length + 1);
  fireEvent.change(boxes[0], { target: { value: "数据是 2019 年的，偏旧" } });
  fireEvent.change(boxes[1], { target: { value: "只是博主，无机构背景" } });
  fireEvent.change(boxes[anchors.length], { target: { value: "支撑核心数据，但单一来源有风险" } });
  fireEvent.click(screen.getByRole("button", { name: /锁定|评估完成|完成/ }));
  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];
  expect(env.anchors.find((a: any) => a.dimension === "currency").answer).toBe("数据是 2019 年的，偏旧");
  const risk = env.anchors.find((a: any) => a.dimension === "risk_note");
  expect(risk).toBeTruthy();
  expect(risk.author).toBe("student");
  expect(risk.answer).toBe("支撑核心数据，但单一来源有风险");
});
```

(Match the lock-button label to the design's `openLockLabel`; read ~L1314–1317 and use that copy.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/studio/StudioAnnotateCard.test.tsx`
Expected: FAIL — component does not exist.

- [ ] **Step 3: Implement**

Create `StudioAnnotateCard.tsx`. Local state: `answers: Record<string, string>` keyed by anchor id, and `riskNote: string`. Render per the design. On lock, build the `CardInstance`:

```tsx
import { useState } from "react";
import type { Anchor, CardInstance, CardSpec, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

export type StudioAnnotateCardProps = {
  spec: CardSpec;
  anchors: Anchor[];
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

const RISK_NOTE_QUESTION = "这条来源在你的论证里起什么作用？有什么风险？";

// Authorship-agnostic annotate answer host (design s3CoachCard, ~L1282-1320).
// Renders whatever anchors it is given; guidance levels L2/L3 change only where
// the anchors come from, never this renderer.
export function StudioAnnotateCard({ spec, anchors, onSubmit, onSkip }: StudioAnnotateCardProps) {
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [riskNote, setRiskNote] = useState("");

  function handleLock() {
    const filled: Anchor[] = anchors.map((a) => ({ ...a, answer: answers[a.id] ?? a.answer }));
    const materialId = anchors[0]?.material_id ?? "";
    filled.push({
      id: "risk_note", material_id: materialId, block_id: "", start: 0, end: 0, quote: "",
      dimension: "risk_note", author: "student", question: RISK_NOTE_QUESTION, answer: riskNote.trim(),
    });
    const env: CardInstance = { ...newEnvelope(spec.id, ""), anchors: filled };
    onSubmit(env);
  }
  // ... render header + per-anchor rows (chip = dimension, question, textarea, ✓ when answers[a.id]?.trim())
  //     + 作用与风险 textarea + lock button + 跳过 button (onSkip(env.event_trace)).
}
```

Match the design's colors, chip styles, hint line ("左边点亮的句子对应下面每个维度——挑一个作答，理由自己写：" / L1294), risk-note label ("作用与风险（自己写）" / L1312), and lock label. Use `newEnvelope` for the `field_values: {}` / `event_trace: []` scaffold, then override `anchors`. (Confirm `CardInstance` has an `anchors` field — envelope.ts — and `newEnvelope` defaults it to `[]`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/studio/StudioAnnotateCard.test.tsx`
Expected: PASS. Prove the risk-note assertion is load-bearing: temporarily drop the risk-note push → test fails → restore.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/StudioAnnotateCard.tsx apps/web/src/studio/StudioAnnotateCard.test.tsx
git commit -m "feat(refactor2): StudioAnnotateCard — CRAAP answer-mode producing filled anchors + risk_note"
```

---

## Task 5: Carry `anchors` on the conversation card state

**Files:**
- Modify: `apps/web/src/studio/conversation.ts`
- Test: `apps/web/src/studio/conversation.test.ts` (existing)

**Interfaces:**
- Consumes: the `card` `StudioTurnEvent` (already carries `anchors: Anchor[]`).
- Produces: `CardState` / the snapshot's `card` gains `anchors: Anchor[]`, populated from the event, threaded to the rail.

**Context:** `applyEvent`'s `card` branch (L49–51) reads `cardInstanceId`/`cardId` but drops `e.anchors`. The card renderer needs them.

- [ ] **Step 1: Write the failing test**

Add to `conversation.test.ts`: feed a `card` event with two anchors through a fake `studioTurn`, then assert `getSnapshot().card.anchors` has length 2 with the right dimensions.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/studio/conversation.test.ts`
Expected: FAIL — `card` has no `anchors`.

- [ ] **Step 3: Implement**

Add `anchors: Anchor[]` to the `CardState` type (import `Anchor` — already imported at L1). In the `card` branch:

```ts
} else if (e.type === "card") {
  const spec = CARD_REGISTRY[e.cardId];
  if (spec) set({ card: { cardInstanceId: e.cardInstanceId, cardId: e.cardId, spec, status: "proposed", anchors: e.anchors } });
}
```

`submitCard` already forwards `{ field_values, event_trace, anchors }` and clears the card on `done` guarded by `submittedId` — leave that intact.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/studio/conversation.test.ts`
Expected: PASS; no other conversation test regressed.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/conversation.ts apps/web/src/studio/conversation.test.ts
git commit -m "feat(refactor2): carry anchors on the studio card state"
```

---

## Task 6: Fork the active-card slot — annotate → StudioAnnotateCard

**Files:**
- Modify: `apps/web/src/studio/CoachRail.tsx`
- Modify (thread props): `apps/web/src/studio/state.ts`, `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/studio/StudioShell.tsx`
- Test: `apps/web/src/studio/CoachRail.test.tsx` (existing) or a new test

**Interfaces:**
- Consumes: `card.anchors` (Task 5), `card.spec.primitive`.
- Produces: an active `annotate` card renders `StudioAnnotateCard` (Task 4); other active cards keep rendering `StudioCardSheet`.

**Context:** `CoachRail`'s active-card branch (L266–271) always renders `StudioCardSheet` (schema field renderer — the 5c-2 disjunction). Fork on primitive. `LiveCard` (L9–14) gains `anchors: Anchor[]`. The static `CraapPlaceholder` (L90) for the 素材 view is superseded whenever a live card exists; keep it only for the no-card 素材 idle state, or remove if unused after wiring — confirm by grep.

First confirm the `CardSpec` Zod contract exposes `primitive` (craap.json has it). If it does not, add `primitive: z.string().optional()` (or the existing PrimitiveKind enum) to the `CardSpec` schema and its type — mirror the Go `Spec.Primitive`.

- [ ] **Step 1: Write the failing test**

In `CoachRail.test.tsx`: render with an active card whose `spec.primitive === "annotate"` and two anchors → assert `StudioAnnotateCard`'s hallmark (e.g. the risk-note label "作用与风险（自己写）") is present and `StudioCardSheet`'s "提交并钉到过程树" button is absent. A second case: active card with a non-annotate primitive → `StudioCardSheet` renders.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/studio/CoachRail.test.tsx`
Expected: FAIL — annotate card currently renders `StudioCardSheet`.

- [ ] **Step 3: Implement**

Add `anchors: Anchor[]` to `LiveCard`. Import `StudioAnnotateCard`. Replace the active branch:

```tsx
) : card && card.status === "active" ? (
  card.spec.primitive === "annotate" ? (
    <StudioAnnotateCard
      spec={card.spec}
      anchors={card.anchors}
      onSubmit={(env) => onSubmitCard?.(env)}
      onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
    />
  ) : (
    <StudioCardSheet spec={card.spec} onSubmit={(env) => onSubmitCard?.(env)} onSkip={(eventTrace) => onSkipCard?.(eventTrace)} />
  )
) : activeView === "素材" ? (
```

Thread `card.anchors` from `StudioContainer` (the snapshot's `card`) into the `CoachRail card` prop (extend the mapping that builds `LiveCard`). `state.ts`'s `StudioCallbacks.onSubmitCard` already takes `CardInstance` — unchanged. Verify `StudioContainer`'s `onSubmitCard` maps the `CardInstance` (which now carries filled `anchors`) into `conversation.submitCard({ field_values, event_trace, anchors })` — since `CardInstance` has all three, pass them straight through.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/studio/CoachRail.test.tsx && npx vitest run src/studio`
Expected: PASS across the studio suite.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/
git commit -m "feat(refactor2): fork active annotate cards to StudioAnnotateCard in the coach rail"
```

---

## Task 7: Highlight live anchor spans in the open source (thin include)

**Files:**
- Modify: `apps/web/src/workspace/material/SourceDossier.tsx`
- Test: `apps/web/src/workspace/material/SourceDossier.test.tsx`

**Interfaces:**
- Consumes: the live card's `anchors` for the open source (map `Anchor` → `AnnotateSpan`: `{ id, block_ref: block_id, range: {start,end}, tag: dimension, note: answer||question, author }`).
- Produces: the open article pane highlights the AI-circled spans (design pairing left-highlight ↔ right-question).

**Context:** The mint does **not** depend on this — it is the design's paired presentation, reusing the existing `Annotate` renderer. Keep it small; if it balloons, cut it to a follow-up and note in the ledger.

- [ ] **Step 1: Write the failing test**

Extend `SourceDossier.test.tsx`: pass anchors for the open source → assert the mapped spans render (a `mark` element containing the quote / the dimension styling appears).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/workspace/material/SourceDossier.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

Accept an optional `anchors?: Anchor[]` prop (or thread from the studio state), map them to `AnnotateSpan[]` for the open source's `material_id`, and pass the merged `AnnotateState` to `Annotate`. Only spans whose `material_id`/`block_id` match the open source render. Author styling already exists in `Annotate` (`AUTHOR_MARK_STYLE`).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/workspace/material/SourceDossier.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/material/SourceDossier.tsx apps/web/src/workspace/material/SourceDossier.test.tsx
git commit -m "feat(refactor2): highlight live CRAAP anchor spans in the open source"
```

---

## Task 8: Full-suite green + roadmap/ledger

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md`

- [ ] **Step 1: Run the whole backend suite**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...`
Expected: all green. If `make sqlc` was needed (no schema changes expected this slice — none of these tasks add migrations), regenerate and rerun.

- [ ] **Step 2: Run the whole web suite + typecheck**

Run: `cd apps/web && npx vitest run && npx tsc --noEmit` (and `cd packages/contracts && npx vitest run` if the `CardSpec.primitive` contract changed).
Expected: all green.

- [ ] **Step 3: Update the roadmap**

Mark Slice 6 complete in `docs/2026-07-11-whole-product-refactor-roadmap.md`: CRAAP fill→mint live (L1 guidance; author-seam kept open for L2/L3); record deferred follow-ups (S2 source-log → 6b, SIFT lateral → 6c, student free span-creation, R-9 framework reveal). Note Slice 5d (routing cutover) is next.

- [ ] **Step 4: Commit**

```bash
git add docs/2026-07-11-whole-product-refactor-roadmap.md
git commit -m "docs(refactor2): Slice 6 complete — CRAAP fill→mint live (L1); 6b/6c + 5d follow-ups"
```

---

## Self-Review (author checklist, run before dispatch)

- **Spec coverage:** generator fix (T1), surface wiring (T2), non-vacuous E2E (T3), answer UI (T4), state carry (T5), rail fork (T6), span highlight (T7), suite+docs (T8). All spec acceptance criteria map to a task.
- **Types consistent:** Go `Anchor.Dimension` (string) ↔ TS `Anchor.dimension`; `CardInstance.anchors` (envelope.ts) is the submit vehicle; `CardSpec.primitive` gates the fork (add to contract if absent — T6).
- **No placeholders:** each code step carries real code or a precise "mirror X at path:line" reference for existing-pattern reuse.
- **Guardrail:** the authorship-agnostic renderer + the "L2/L3 is future config" comments appear in T1 (generation seam) and T4 (component).
