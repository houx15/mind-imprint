# Slice 3c · Anchored Card Frontend — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Render annotation-mode cards as an inline anchored "对话分支" (a flat list of AI guiding questions + student self-questions), highlight anchor quotes in the material pane, and carry anchors through submit — completing the keystone end-to-end on the web side. Also repair the web typecheck broken by Slice 3a's contract change.

**Architecture:** Fix all `CardInstance`/`CardSpec` literal sites for the new required `anchors`/`mode` fields; `api/cards.ts submitCard` forwards `anchors`; a new self-contained `AnnotationBranch` renders `activeCard.anchors` (flat list, `dimension` = chip label) with answer textareas, a "self-question" affordance (appends an `author:'student'` anchor), and submit; `WorkspaceView` branches on `activeSpec.mode==="annotation"` to render it inline (not `CardSheetHost`); `activeCard.anchors` are threaded to `MaterialPane` which highlights each anchor's `quote` within its block.

**Tech Stack:** React 18 + TS, Vitest + RTL + jest-dom. From `apps/web`: `npx vitest run <path>`, `npx tsc --noEmit`. Depends on 3a/3b (contracts `anchors`/`mode`, backend anchor generation) already on this branch.

## Global Constraints

- **Frontend only** (`apps/web`). Anchors already exist on the store's `CardInstance` (contract) and arrive from the server via `activateCard`.
- **Annotation cards do NOT use the form renderer / `CardBodyProps`** (that contract writes only `field_values`). `AnnotationBranch` uses a separate `anchors` prop contract.
- **Highlighting keys off the anchor's `quote`** located within the block (`block.text.indexOf(quote)`); numeric `start/end` are advisory (Go-byte vs JS-UTF16). Only `author` doesn't matter for highlight; highlight both ai + student anchors.
- **UI copy Chinese, verbatim from the design** (`TASKS: WORKSPACE` CRAAP branch + `material-annotation model`). Palette per the app.
- Every task ends green (`npx vitest run` for touched files) + `npx tsc --noEmit` clean, and is committed.

---

### Task 1: Restore the web typecheck (anchors/mode literals)

**Files (all under `apps/web/src`):**
- Modify: `cards/envelopeReducer.ts` (production `newEnvelope`)
- Modify the test/helper files that construct `CardInstance` or `CardSpec` literals: `agent/createConversation.test.ts`, `shell/directory/taskCardView.test.ts`, `shell/records/cardUsage.test.ts`, `store/createStore.messages-cards.test.ts`, `store/createStore.test.ts`, `workspace/cardSheetHost.customRenderer.test.tsx`, `workspace/cardSheetHost.note.test.tsx`, `workspace/processTree.test.ts`, `workspace/viewModel.test.ts`, `workspace/WorkspaceView.test.tsx` — plus any other file `tsc` flags.

**Interfaces:** none new — this only satisfies the already-required `CardInstance.anchors: Anchor[]` and `CardSpec.mode` from Slice 3a.

- [ ] **Step 1: See the full breakage**

Run: `cd apps/web && npx tsc --noEmit`
Expected: multiple `TS2741/TS2345/TS2322` errors — `Property 'anchors' is missing` on `CardInstance` literals and `Property 'mode' is missing` on `CardSpec` literals.

- [ ] **Step 2: Fix production `newEnvelope`**

In `apps/web/src/cards/envelopeReducer.ts`, add `anchors: [],` to the object `newEnvelope` returns (after `event_trace: [], rubric_tags: [],`):

```ts
  return {
    id: genId(),
    card_id, task_id, parent_node_id: null,
    status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
    anchors: [],
    created_at: now(), completed_at: null,
  };
```

- [ ] **Step 3: Fix every flagged literal**

For each `tsc` error: if it's a `CardInstance` literal missing `anchors`, add `anchors: [],`. If it's a `CardSpec` literal missing `mode`, add `mode: "form",`. Work through them by re-running `npx tsc --noEmit` until zero errors. These are all test fixtures except `newEnvelope`; the additions are mechanical (add the field with its default value). Do not change any other behavior. If a helper builds a `CardInstance` via a spread of a partial, add `anchors: []` to the base object so the spread satisfies the type.

- [ ] **Step 4: Verify — tsc clean + full suite green**

Run: `cd apps/web && npx tsc --noEmit && npx vitest run`
Expected: tsc clean; the entire web suite passes (the added fields are inert defaults, so no test assertion changes).

- [ ] **Step 5: Commit**

```bash
git add -A apps/web/src
git commit -m "fix(web): add anchors/mode defaults to CardInstance/CardSpec literals (3a contract fallout)"
```

(Use `git add -A apps/web/src` here because the fix touches ~11 files; confirm `git status` shows only the intended `apps/web/src` files staged and nothing outside it.)

---

### Task 2: `submitCard` forwards anchors

**Files:**
- Modify: `apps/web/src/api/cards.ts`
- Test: `apps/web/src/api/cards.test.ts` (create if absent)

**Interfaces:**
- Produces: `submitCard` PUT body now includes `anchors: env.anchors`.

- [ ] **Step 1: Write the failing test**

Create (or add to) `apps/web/src/api/cards.test.ts`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { CardInstance } from "@mind-imprint/contracts";

vi.mock("./client", async (orig) => {
  const real = await orig<typeof import("./client")>();
  return { ...real, apiFetch: vi.fn(async () => ({ card: {} })) };
});

import { apiFetch } from "./client";
import { submitCard } from "./cards";

const env: CardInstance = {
  id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null,
  status: "completed", field_values: {}, event_trace: [], rubric_tags: [],
  anchors: [{ id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 3, quote: "美航局", dimension: "权威性", author: "student", question: "可信吗", answer: "存疑" }],
  created_at: "1", completed_at: "2",
};

describe("submitCard", () => {
  beforeEach(() => { vi.clearAllMocks(); });
  it("forwards anchors in the PUT body", async () => {
    await submitCard("t1", "c1", env);
    const [, init] = (apiFetch as any).mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body).toMatchObject({ status: "completed" });
    expect(body.anchors).toHaveLength(1);
    expect(body.anchors[0]).toMatchObject({ author: "student", answer: "存疑" });
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/api/cards.test.ts`
Expected: FAIL — `body.anchors` is undefined (submitCard doesn't send it).

- [ ] **Step 3: Add anchors to the body**

In `apps/web/src/api/cards.ts`, in `submitCard`, change the PUT body to include anchors:

```ts
    body: JSON.stringify({ status: "completed", field_values: env.field_values, event_trace: env.event_trace, anchors: env.anchors }),
```

- [ ] **Step 4: Run — expect PASS + tsc**

Run: `cd apps/web && npx vitest run src/api/cards.test.ts && npx tsc --noEmit`
Expected: PASS; tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/cards.ts apps/web/src/api/cards.test.ts
git commit -m "feat(web): submitCard forwards anchors to the API"
```

---

### Task 3: `AnnotationBranch` component

**Files:**
- Create: `apps/web/src/workspace/AnnotationBranch.tsx`
- Test: `apps/web/src/workspace/AnnotationBranch.test.tsx`

**Interfaces:**
- Produces: `AnnotationBranch({ card, spec, onSubmit, onClose }: { card: CardInstance; spec: CardSpec; onSubmit: (id: string, final: CardInstance) => void; onClose: (id: string) => void })` — renders `card.anchors` as a flat list of dimension-chipped questions with answer textareas, a "self-question" affordance appending an `author:'student'` anchor, and submit/close.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/workspace/AnnotationBranch.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CardInstance, CardSpec } from "@mind-imprint/contracts";
import { AnnotationBranch } from "./AnnotationBranch";

const spec = { id: "sift_craap", name: "CRAAP", category: "信息素养", mode: "annotation", steps: [], rubric_tags: [], purpose: "", trigger_condition: "" } as unknown as CardSpec;

function cardWith(anchors: CardInstance["anchors"]): CardInstance {
  return { id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null, status: "active", field_values: {}, event_trace: [], rubric_tags: [], anchors, created_at: "1", completed_at: null };
}

const aiAnchor = { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 5, quote: "美航局发现", dimension: "权威性 · Authority", author: "ai" as const, question: "这处「美航局发现」——转载者是权威吗？", answer: "" };

describe("AnnotationBranch", () => {
  it("renders the card name and each anchor's dimension + question", () => {
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText("CRAAP · 在真实材料上核查")).toBeInTheDocument();
    expect(screen.getByText("权威性 · Authority")).toBeInTheDocument();
    expect(screen.getByText("这处「美航局发现」——转载者是权威吗？")).toBeInTheDocument();
  });

  it("submits with the edited anchor answers and completed status", () => {
    const onSubmit = vi.fn();
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={onSubmit} onClose={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText(/写下你的判断/), { target: { value: "只是转载，存疑" } });
    fireEvent.click(screen.getByText("提交并钉到过程树"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
    const [, final] = onSubmit.mock.calls[0];
    expect(final.status).toBe("completed");
    expect(final.anchors[0].answer).toBe("只是转载，存疑");
  });

  it("adds a student self-question anchor", () => {
    const onSubmit = vi.fn();
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={onSubmit} onClose={vi.fn()} />);
    fireEvent.click(screen.getByText(/自己向印记提问/));
    fireEvent.change(screen.getByPlaceholderText(/写下你自己的问题/), { target: { value: "这个数据是哪年的？" } });
    fireEvent.click(screen.getByText("提交并钉到过程树"));
    const [, final] = onSubmit.mock.calls[0];
    const student = final.anchors.find((a: any) => a.author === "student");
    expect(student?.question).toBe("这个数据是哪年的？");
  });

  it("close calls onClose", () => {
    const onClose = vi.fn();
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={vi.fn()} onClose={onClose} />);
    fireEvent.click(screen.getByText("收起"));
    expect(onClose).toHaveBeenCalledWith("c1");
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/workspace/AnnotationBranch.test.tsx`
Expected: FAIL — `Failed to resolve import "./AnnotationBranch"`.

- [ ] **Step 3: Implement `AnnotationBranch.tsx`**

Create `apps/web/src/workspace/AnnotationBranch.tsx`:

```tsx
import { useState } from "react";
import type { Anchor, CardInstance, CardSpec } from "@mind-imprint/contracts";
import { envelopeReducer } from "../cards/envelopeReducer";

export function AnnotationBranch({
  card, spec, onSubmit, onClose,
}: {
  card: CardInstance;
  spec: CardSpec;
  onSubmit: (id: string, final: CardInstance) => void;
  onClose: (id: string) => void;
}) {
  const [anchors, setAnchors] = useState<Anchor[]>(card.anchors);
  const [asking, setAsking] = useState(false);
  const [ownQ, setOwnQ] = useState("");

  function setAnswer(i: number, answer: string) {
    setAnchors((prev) => prev.map((a, idx) => (idx === i ? { ...a, answer } : a)));
  }

  function addOwnQuestion() {
    const q = ownQ.trim();
    if (!q) return;
    const first = anchors[0];
    setAnchors((prev) => [
      ...prev,
      {
        id: `student_${prev.length}`,
        material_id: first?.material_id ?? "",
        block_id: "", start: 0, end: 0, quote: "",
        dimension: "我的提问", author: "student", question: q, answer: "",
      },
    ]);
    setOwnQ("");
    setAsking(false);
  }

  function submit() {
    const completed = envelopeReducer(card, { type: "submit" });
    onSubmit(card.id, { ...completed, anchors });
  }

  const answered = anchors.filter((a) => a.author === "ai" && a.answer.trim()).length;
  const aiCount = anchors.filter((a) => a.author === "ai").length;

  return (
    <div style={{ background: "#fff", border: "1px solid #F0DACF", borderRadius: 14, overflow: "hidden", boxShadow: "0 4px 16px rgba(217,130,99,.10)", margin: "0 0 12px" }}>
      <div style={{ height: 4, background: "#D98263" }} />
      <div style={{ padding: "15px 18px 12px", borderBottom: "1px solid #F5F0ED" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 5 }}>
          <span style={{ fontSize: 11, fontWeight: 700, color: "#D98263", letterSpacing: ".05em" }}>工具卡 · 对话分支</span>
          <span style={{ fontSize: 11, fontWeight: 600, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 9px", borderRadius: 999 }}>{spec.category}</span>
        </div>
        <div style={{ fontSize: 16, fontWeight: 800, color: "#1C2333" }}>{spec.name} · 在真实材料上核查</div>
        <div style={{ fontSize: 12.5, color: "#8A92A3", marginTop: 3 }}>文章已在右侧打开 · 点亮的句子是我圈的</div>
      </div>

      <div style={{ padding: "14px 18px 6px", background: "#FAFBFC" }}>
        {anchors.map((a, i) => (
          <div key={a.id} style={{ border: "1px solid #ECEEF3", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#fff" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11, fontWeight: 700, padding: "3px 10px", borderRadius: 999, color: "#fff", background: a.author === "student" ? "#2A3B7A" : "#7C6BB5" }}>{a.dimension}</span>
              {a.answer.trim() && (
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              )}
            </div>
            <div style={{ fontSize: 14, lineHeight: 1.7, color: "#2B3346", fontWeight: 500 }}>{a.question}</div>
            <textarea
              value={a.answer}
              onChange={(e) => setAnswer(i, e.target.value)}
              rows={2}
              placeholder="写下你的判断……"
              style={{ width: "100%", marginTop: 9, border: "1px solid #E1E4ED", borderRadius: 9, padding: "9px 11px", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }}
            />
          </div>
        ))}

        {asking ? (
          <div style={{ border: "1px dashed #2A3B7A", borderRadius: 11, padding: "11px 13px", marginBottom: 12 }}>
            <textarea
              value={ownQ}
              onChange={(e) => setOwnQ(e.target.value)}
              rows={2}
              placeholder="写下你自己的问题——从印记没覆盖的角度……"
              style={{ width: "100%", border: "none", outline: "none", resize: "vertical", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "transparent" }}
            />
            <button type="button" onClick={addOwnQuestion} style={{ marginTop: 6, background: "#2A3B7A", color: "#fff", border: "none", padding: "7px 14px", borderRadius: 9, fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>加上我的问题</button>
          </div>
        ) : (
          <div onClick={() => setAsking(true)} style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 12px", padding: "10px 13px", border: "1px dashed #D3D8E4", borderRadius: 11, color: "#8A92A3", fontSize: 12.5, cursor: "pointer" }}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 20h9" /><path d="M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4z" /></svg>
            在右侧文章里划一句，自己向印记提问
          </div>
        )}
      </div>

      <div style={{ padding: "12px 18px", borderTop: "1px solid #F0F1F5", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
        <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 600 }}>{answered} / {aiCount} 维已回应</span>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <button type="button" onClick={() => onClose(card.id)} style={{ background: "none", border: "none", color: "#9AA1B0", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: "8px 6px" }}>收起</button>
          <button type="button" onClick={submit} style={{ display: "inline-flex", alignItems: "center", gap: 7, background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 10, fontSize: 13.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            提交并钉到过程树
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run — expect PASS + tsc**

Run: `cd apps/web && npx vitest run src/workspace/AnnotationBranch.test.tsx && npx tsc --noEmit`
Expected: PASS (4 tests); tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/AnnotationBranch.tsx apps/web/src/workspace/AnnotationBranch.test.tsx
git commit -m "feat(web): AnnotationBranch — inline anchored questions + self-question + submit"
```

---

### Task 4: Wire into workspace + material highlighting

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceView.tsx` (render `AnnotationBranch` for annotation cards; pass anchors to `RightPanel`)
- Modify: `apps/web/src/workspace/RightPanel.tsx` (thread `anchors` prop → `MaterialPane`)
- Modify: `apps/web/src/workspace/MaterialPane.tsx` (highlight anchor quotes in blocks)
- Test: `apps/web/src/workspace/materialHighlight.test.tsx` (new) + update `WorkspaceView.test.tsx` if needed

**Interfaces:**
- Consumes: `AnnotationBranch` (Task 3).
- Produces: `WorkspaceView` renders `AnnotationBranch` inline (in the chat column) when `activeSpec.mode==="annotation"` instead of `CardSheetHost`; `RightPanel`/`MaterialPane` gain an optional `anchors?: Anchor[]` prop; `MaterialPane` highlights each anchor's `quote` within its `block_id` block.

- [ ] **Step 1: Write the failing highlight test**

Create `apps/web/src/workspace/materialHighlight.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import type { Anchor, Material } from "@mind-imprint/contracts";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return { ...real, api: { ...real.api, listMaterials: vi.fn(), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() } };
});

import { api } from "../api";
import { MaterialPane } from "./MaterialPane";

const material: Material = {
  id: "m1", task_id: "t1", kind: "article", source: "fetched", title: "文章",
  source_url: "https://x", blocks: [{ id: "b0", text: "最近某科技博主综合整理的文章刷屏了。" }], scratch: "", created_at: "1",
};
const anchors: Anchor[] = [
  { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "某科技博主综合整理", dimension: "权威性", author: "ai", question: "?", answer: "" },
];

describe("MaterialPane highlighting", () => {
  beforeEach(() => vi.clearAllMocks());
  it("wraps the anchor quote in a highlight mark", async () => {
    (api.listMaterials as any).mockResolvedValue([material]);
    render(<MaterialPane taskId="t1" seedUrl="https://x" anchors={anchors} />);
    const mark = await screen.findByText("某科技博主综合整理");
    expect(mark.tagName.toLowerCase()).toBe("mark");
    // surrounding plain text still present
    expect(screen.getByText(/最近/)).toBeInTheDocument();
    expect(screen.getByText(/的文章刷屏了/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/workspace/materialHighlight.test.tsx`
Expected: FAIL — `MaterialPane` has no `anchors` prop / renders the block as one text node (no `<mark>`).

- [ ] **Step 3: Add highlighting to `MaterialPane`**

3a. In `apps/web/src/workspace/MaterialPane.tsx`, add `Anchor` to the type import and an `anchors` prop:

```ts
import type { Material, Anchor } from "@mind-imprint/contracts";
```
Change the signature to `export function MaterialPane({ taskId, seedUrl, anchors = [] }: { taskId: string; seedUrl: string | null; anchors?: Anchor[] })` and pass `anchors` to the active `MaterialBody`: `<MaterialBody key={active.id} taskId={taskId} material={active} anchors={anchors} />`.

3b. Change `MaterialBody`'s signature to accept `anchors` and render each block with highlights. Replace the `material.blocks.map(...)` `<p>` with a call to a `renderBlock` helper:

```tsx
function renderBlock(text: string, quotes: string[]): React.ReactNode {
  if (quotes.length === 0) return text;
  // Highlight the first occurrence of each distinct quote.
  const marks = Array.from(new Set(quotes.filter(Boolean)));
  type Seg = { text: string; hl: boolean };
  let segs: Seg[] = [{ text, hl: false }];
  for (const q of marks) {
    const next: Seg[] = [];
    for (const s of segs) {
      if (s.hl) { next.push(s); continue; }
      const idx = s.text.indexOf(q);
      if (idx < 0) { next.push(s); continue; }
      if (idx > 0) next.push({ text: s.text.slice(0, idx), hl: false });
      next.push({ text: q, hl: true });
      const rest = s.text.slice(idx + q.length);
      if (rest) next.push({ text: rest, hl: false });
    }
    segs = next;
  }
  return segs.map((s, i) =>
    s.hl ? (
      <mark key={i} style={{ background: "#F0ECF8", color: "#1C2333", borderBottom: "2px solid #7C6BB5", borderRadius: 3, padding: "1px 2px" }}>{s.text}</mark>
    ) : (
      <span key={i}>{s.text}</span>
    ),
  );
}
```

and in `MaterialBody`, render blocks as:

```tsx
        {material.blocks.map((b) => {
          const quotes = anchors.filter((a) => a.block_id === b.id).map((a) => a.quote);
          return (
            <p key={b.id} style={{ fontSize: 15, lineHeight: 2.1, color: "#2B3346", margin: "0 0 14px" }}>
              {renderBlock(b.text, quotes)}
            </p>
          );
        })}
```

(Add the `anchors: Anchor[]` param to `MaterialBody`'s props and import `React` types as needed; `renderBlock` returns `React.ReactNode`.)

- [ ] **Step 4: Thread `anchors` through `RightPanel` + wire `WorkspaceView`**

4a. In `apps/web/src/workspace/RightPanel.tsx`: add `anchors?: Anchor[]` (import the type) to the props and pass it to `<MaterialPane taskId={taskId} seedUrl={seedUrl} anchors={anchors} />`. Optionally default the tab to `"material"` when `anchors && anchors.length > 0` (so highlights are visible when an annotation card is active) — set the initial `useState<Tab>(anchors && anchors.length ? "material" : "tree")`.

4b. In `apps/web/src/workspace/WorkspaceView.tsx`:
- Pass anchors to the sidebar: `<RightPanel nodes={treeNodes} taskId={taskId} seedUrl={task?.seed ?? null} anchors={activeCard?.anchors} />`.
- Branch the active-card rendering (the `phase==="card_active" && activeCard && activeSpec` block that renders `CardSheetHost`): when `activeSpec.mode === "annotation"`, render `<AnnotationBranch card={activeCard} spec={activeSpec} onSubmit={(id, final) => void conversation.submitCard(id, final)} onClose={(id) => conversation.closeCard(id)} />` inside the chat column (not the absolute overlay); otherwise render the existing `CardSheetHost` as today. Import `AnnotationBranch`.

Concretely, place the `AnnotationBranch` right after the `<Composer .../>` inside the chat column `<div>` (so it appears inline under the conversation), guarded by `phase === "card_active" && activeCard && activeSpec?.mode === "annotation"`; and guard the existing `CardSheetHost` block additionally with `activeSpec?.mode !== "annotation"`.

- [ ] **Step 5: Run — expect PASS (highlight + workspace) + full suite + tsc**

Run: `cd apps/web && npx vitest run src/workspace/materialHighlight.test.tsx src/workspace/WorkspaceView.test.tsx && npx tsc --noEmit`
Expected: PASS both; tsc clean. Then run the full suite: `cd apps/web && npx vitest run` → all green.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/workspace/WorkspaceView.tsx apps/web/src/workspace/RightPanel.tsx apps/web/src/workspace/MaterialPane.tsx apps/web/src/workspace/materialHighlight.test.tsx apps/web/src/workspace/WorkspaceView.test.tsx
git commit -m "feat(web): render annotation cards as inline branch + highlight anchor quotes"
```

---

## Self-Review

**Spec coverage:** compile-fix (T1) ✅; submit carries anchors (T2) ✅; inline anchored branch with AI questions + student self-question + submit-to-tree (T3) ✅; wire on `mode==="annotation"` + material highlighting keyed off `quote` (T4) ✅. Methodology "怎么用" modal + close-out abstract framework are **deferred to a Slice 3d polish** (noted below) to bound this slice.

**Placeholder scan:** none — complete code / exact edits. T1 is mechanical-but-broad (add default fields until tsc is clean).

**Type consistency:** `AnnotationBranch({card, spec, onSubmit, onClose})` (T3) matches the `WorkspaceView` call site (T4) and `conversation.submitCard/closeCard`. `MaterialPane`/`RightPanel` `anchors?: Anchor[]` (T4) threaded from `activeCard?.anchors`. `submitCard` body anchors (T2) consumed by the 3a backend `putCard`. Highlight keys off `quote` per the global constraint.

**Ordering:** T1 (compile) → T2 (submit) → T3 (branch) → T4 (wire, consumes T3). T1 must be first — nothing else compiles until it lands.

## Deferred to Slice 3d (polish, noted for the roadmap)
- Methodology "怎么用" modal (工具说明书 — the design's detailed why/how/help, sourced from `CARD_REGISTRY[id].steps[].methodology`).
- Close-out abstract framework (the card's dimensions shown as a transferable takeaway after submit).
- Click-sync between a branch question and its highlighted span.
- `processTree.cardSub` fallback to `anchors[0].answer` for a nicer node subtitle.

## Note for executor
- T1: after fixing, run `git status` before committing to ensure only `apps/web/src` files are staged (the pre-existing untracked repo files must not be swept in). Use explicit paths if `-A` is risky.
