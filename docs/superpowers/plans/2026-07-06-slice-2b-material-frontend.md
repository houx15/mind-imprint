# Slice 2b · Material Substrate — Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Surface materials in the workspace: an API client for the Slice-2a material endpoints, and a **材料 / 过程树** tabbed, draggable right sidebar whose 材料 pane fetches the task's seed (falling back to paste) and renders material blocks + a 随手记 scratch note.

**Architecture:** New `src/api/materials.ts` (+ wired into the `api` aggregate) with a typed `MaterialFetchError`. `TreeBody` is extracted from `TreePanel` so both the panel and the new `RightPanel` render the same node list. `MaterialPane` is self-contained (owns its material state via the API client — no store change). `RightPanel` is the sidebar shell (segmented tabs + drag + collapse) composing `TreeBody` and `MaterialPane`; `WorkspaceView` renders it in place of the bare `TreePanel`.

**Tech Stack:** React 18 + TS, Vitest + @testing-library/react + jest-dom. Web tests run from `apps/web` with `npx vitest run <path>`. Depends on Slice 2a's backend + the `Material` contract (already on this branch).

## Global Constraints

- **Frontend only** (`apps/web`). No backend/contract/store-schema changes. `MaterialPane` holds material state locally (no store entanglement).
- **UI copy is Chinese, verbatim from the binding design** (`TASKS: WORKSPACE` right sidebar + `material-annotation model`). Code identifiers English.
- **`fetchMaterialFromSeed` maps a `material_fetch_failed` ApiError → `MaterialFetchError(reason)`**; the pane treats that (and any load failure) as "show the paste fallback", never an error toast.
- **Blocks render as plain `<p>` paragraphs — no highlighting** (Slice 3 adds anchors).
- Material `kind` is `"article" | "draft"`. Palette tokens as elsewhere (indigo `#2A3B7A`, hairline `#EAECF2`, muted `#8A92A3`/`#9AA1B0`, bg `#F3F4F8`).
- Every task ends green (`npx vitest run <files>`) + `npx tsc --noEmit` clean, and is committed.

---

### Task 1: Material API client

**Files:**
- Create: `apps/web/src/api/materials.ts`
- Modify: `apps/web/src/api/index.ts`
- Test: `apps/web/src/api/materials.test.ts`

**Interfaces:**
- Produces: `MaterialFetchError extends Error { reason: string }`; `listMaterials(taskId): Promise<Material[]>`; `createMaterial(taskId, {kind,title,text}): Promise<Material>`; `fetchMaterialFromSeed(taskId): Promise<Material>` (throws `MaterialFetchError` on `material_fetch_failed`); `saveScratch(taskId, materialId, scratch): Promise<Material>`. All added to the `ApiClient` interface + `api` object; `MaterialFetchError` re-exported from `../api`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/api/materials.test.ts`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "./client";

vi.mock("./client", async (orig) => {
  const real = await orig<typeof import("./client")>();
  return { ...real, apiFetch: vi.fn() };
});

import { apiFetch } from "./client";
import { fetchMaterialFromSeed, listMaterials, MaterialFetchError } from "./materials";

const material = {
  id: "m1", task_id: "t1", kind: "article", source: "fetched", title: "T",
  source_url: "https://x", blocks: [{ id: "b0", text: "一段" }], scratch: "", created_at: "1",
};

describe("materials api", () => {
  beforeEach(() => vi.clearAllMocks());

  it("listMaterials unwraps the materials array", async () => {
    (apiFetch as any).mockResolvedValue({ materials: [material] });
    expect(await listMaterials("t1")).toEqual([material]);
  });

  it("fetchMaterialFromSeed returns the material on success", async () => {
    (apiFetch as any).mockResolvedValue({ material });
    expect(await fetchMaterialFromSeed("t1")).toEqual(material);
  });

  it("fetchMaterialFromSeed maps material_fetch_failed → MaterialFetchError(reason)", async () => {
    (apiFetch as any).mockRejectedValue(new ApiError("material_fetch_failed", "x", 422, { reason: "blocked" }));
    await expect(fetchMaterialFromSeed("t1")).rejects.toMatchObject({ name: "MaterialFetchError", reason: "blocked" });
  });

  it("fetchMaterialFromSeed rethrows other ApiErrors unchanged", async () => {
    (apiFetch as any).mockRejectedValue(new ApiError("not_found", "x", 404));
    await expect(fetchMaterialFromSeed("t1")).rejects.toMatchObject({ code: "not_found" });
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/api/materials.test.ts`
Expected: FAIL — `Failed to resolve import "./materials"`.

- [ ] **Step 3: Create the client module**

Create `apps/web/src/api/materials.ts`:

```ts
import type { Material } from "@mind-imprint/contracts";
import { apiFetch, ApiError } from "./client";

// Thrown when the server could not fetch/extract the seed URL. The pane treats
// this as "fall back to paste", not an error to surface.
export class MaterialFetchError extends Error {
  constructor(public readonly reason: string) {
    super(`material_fetch_failed: ${reason}`);
    this.name = "MaterialFetchError";
  }
}

export async function listMaterials(taskId: string): Promise<Material[]> {
  const r = await apiFetch<{ materials: Material[] }>(`/api/v1/tasks/${taskId}/materials`);
  return r.materials;
}

export async function createMaterial(
  taskId: string,
  input: { kind: "article" | "draft"; title: string; text: string },
): Promise<Material> {
  const r = await apiFetch<{ material: Material }>(`/api/v1/tasks/${taskId}/materials`, {
    method: "POST",
    body: JSON.stringify(input),
  });
  return r.material;
}

export async function fetchMaterialFromSeed(taskId: string): Promise<Material> {
  try {
    const r = await apiFetch<{ material: Material }>(`/api/v1/tasks/${taskId}/materials/from-seed`, {
      method: "POST",
    });
    return r.material;
  } catch (e) {
    if (e instanceof ApiError && e.code === "material_fetch_failed") {
      const reason = (e.details as { reason?: string } | undefined)?.reason ?? "unreachable";
      throw new MaterialFetchError(reason);
    }
    throw e;
  }
}

export async function saveScratch(taskId: string, materialId: string, scratch: string): Promise<Material> {
  const r = await apiFetch<{ material: Material }>(
    `/api/v1/tasks/${taskId}/materials/${materialId}/scratch`,
    { method: "PUT", body: JSON.stringify({ scratch }) },
  );
  return r.material;
}
```

- [ ] **Step 4: Wire into the api aggregate**

In `apps/web/src/api/index.ts`:

4a. Add `Material` to the contracts type import and import the module:

```ts
import type { Task, CardInstance, Evaluation, TraceEvent, Material } from "@mind-imprint/contracts";
```
```ts
import { listMaterials, createMaterial, fetchMaterialFromSeed, saveScratch, MaterialFetchError } from "./materials";
```

4b. Re-export the error class (next to `export { ApiError } from "./client";`):

```ts
export { MaterialFetchError } from "./materials";
```

4c. Add to the `ApiClient` interface (after the card methods):

```ts
  listMaterials(taskId: string): Promise<Material[]>;
  createMaterial(taskId: string, input: { kind: "article" | "draft"; title: string; text: string }): Promise<Material>;
  fetchMaterialFromSeed(taskId: string): Promise<Material>;
  saveScratch(taskId: string, materialId: string, scratch: string): Promise<Material>;
```

4d. Add to the `api` object literal:

```ts
  listMaterials, createMaterial, fetchMaterialFromSeed, saveScratch,
```

- [ ] **Step 5: Run — expect PASS + typecheck**

Run: `cd apps/web && npx vitest run src/api/materials.test.ts && npx tsc --noEmit`
Expected: PASS (4 tests); tsc clean.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/materials.ts apps/web/src/api/materials.test.ts apps/web/src/api/index.ts
git commit -m "feat(web): material API client (list/create/from-seed/scratch)"
```

---

### Task 2: Extract `TreeBody` from `TreePanel`

**Files:**
- Create: `apps/web/src/workspace/TreeBody.tsx`
- Modify: `apps/web/src/workspace/TreePanel.tsx`
- Test: `apps/web/src/workspace/TreeBody.test.tsx`

**Interfaces:**
- Produces: `TreeBody({ nodes }: { nodes?: ProcessNode[] })` — the scrollable node-list body (the padded container + node rows + the "边做边长 · 随评估归并枝节" footer), extracted verbatim from `TreePanel`'s expanded body so both `TreePanel` and the later `RightPanel` render it.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/workspace/TreeBody.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TreeBody } from "./TreeBody";
import type { ProcessNode } from "./processTree";

const nodes: ProcessNode[] = [
  { id: "root", kind: "task_root", title: "任务根", sub: undefined } as ProcessNode,
  { id: "n1", kind: "card_use", title: "用了 CRAAP", sub: "信息素养" } as ProcessNode,
];

describe("TreeBody", () => {
  it("renders node titles and the growth footer", () => {
    render(<TreeBody nodes={nodes} />);
    expect(screen.getByText("用了 CRAAP")).toBeInTheDocument();
    expect(screen.getByText("边做边长 · 随评估归并枝节")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/workspace/TreeBody.test.tsx`
Expected: FAIL — `Failed to resolve import "./TreeBody"`.

- [ ] **Step 3: Create `TreeBody.tsx`**

Create `apps/web/src/workspace/TreeBody.tsx` (the body copied verbatim from `TreePanel.tsx` lines ~94–170):

```tsx
import type { ProcessNode } from "./processTree";
import { nodeView } from "./nodeView";

export function TreeBody({ nodes = [] }: { nodes?: ProcessNode[] }) {
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "18px 20px 28px" }}>
      {nodes.length > 1
        ? nodes.map((n) => {
            const v = nodeView(n);
            return (
              <div key={n.id} style={v.rowStyle}>
                <div style={{ flex: "none", display: "flex", flexDirection: "column", alignItems: "center", paddingTop: "3px" }}>
                  <div style={v.markerStyle} />
                  <div style={{ width: "2px", flex: 1, background: "#EDEEF3", marginTop: "4px", minHeight: "8px" }} />
                </div>
                <div style={{ flex: 1, paddingBottom: "10px" }}>
                  <span style={v.tagStyle}>{v.tag}</span>
                  <div style={{ fontSize: "13.5px", fontWeight: 600, color: "#2B3346", lineHeight: 1.5, marginTop: "6px" }}>
                    {n.title}
                  </div>
                  {n.sub && (
                    <div style={{ fontSize: "12px", color: "#9AA1B0", marginTop: "3px" }}>{n.sub}</div>
                  )}
                </div>
              </div>
            );
          })
        : null}
      <div style={{ textAlign: "center", fontSize: "11.5px", color: "#C2C8D6", marginTop: "10px", fontWeight: 500 }}>
        边做边长 · 随评估归并枝节
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Make `TreePanel` use `TreeBody`**

In `apps/web/src/workspace/TreePanel.tsx`: add the import `import { TreeBody } from "./TreeBody";`, and replace the entire expanded-state body `<div>` (the one with `padding: "18px 20px 28px"` containing the `nodes.length > 1 ? …` block and the footer) with:

```tsx
        <TreeBody nodes={nodes} />
```

Remove the now-unused `nodeView` import from `TreePanel.tsx` if it is no longer referenced there (the node rendering moved to `TreeBody`).

- [ ] **Step 5: Run — expect PASS (TreeBody + unchanged TreePanel/Workspace tests)**

Run: `cd apps/web && npx vitest run src/workspace/TreeBody.test.tsx src/workspace/TreePanel.test.tsx src/workspace/WorkspaceView.test.tsx`
Expected: PASS — `TreeBody` renders; `TreePanel` and `WorkspaceView` tests still green (same DOM text).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/workspace/TreeBody.tsx apps/web/src/workspace/TreeBody.test.tsx apps/web/src/workspace/TreePanel.tsx
git commit -m "refactor(web): extract TreeBody from TreePanel for reuse"
```

---

### Task 3: `MaterialPane`

**Files:**
- Create: `apps/web/src/workspace/MaterialPane.tsx`
- Test: `apps/web/src/workspace/MaterialPane.test.tsx`

**Interfaces:**
- Consumes: `api.listMaterials/createMaterial/fetchMaterialFromSeed/saveScratch`, `MaterialFetchError` (from `../api`).
- Produces: `MaterialPane({ taskId, seedUrl }: { taskId: string; seedUrl: string | null })` — self-contained; loads materials on mount, auto-fetches from seed when empty, falls back to a paste form, renders material tabs + blocks + debounced scratch.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/workspace/MaterialPane.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { Material } from "@mind-imprint/contracts";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return {
    ...real,
    api: { ...real.api, listMaterials: vi.fn(), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() },
  };
});

import { api, MaterialFetchError } from "../api";
import { MaterialPane } from "./MaterialPane";

const article: Material = {
  id: "m1", task_id: "t1", kind: "article", source: "fetched", title: "卫星图看中国变绿",
  source_url: "https://x", blocks: [{ id: "b0", text: "第一段。" }, { id: "b1", text: "第二段。" }], scratch: "", created_at: "1",
};

describe("MaterialPane", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders existing material blocks", async () => {
    (api.listMaterials as any).mockResolvedValue([article]);
    render(<MaterialPane taskId="t1" seedUrl="https://x" />);
    expect(await screen.findByText("第一段。")).toBeInTheDocument();
    expect(screen.getByText("第二段。")).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).not.toHaveBeenCalled();
  });

  it("auto-fetches from seed when empty and renders the result", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    (api.fetchMaterialFromSeed as any).mockResolvedValue(article);
    render(<MaterialPane taskId="t1" seedUrl="https://x" />);
    expect(await screen.findByText("第一段。")).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).toHaveBeenCalledWith("t1");
  });

  it("shows the paste fallback when the fetch fails, and creates on submit", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    (api.fetchMaterialFromSeed as any).mockRejectedValue(new MaterialFetchError("blocked"));
    (api.createMaterial as any).mockResolvedValue({ ...article, source: "pasted", blocks: [{ id: "b0", text: "我粘的。" }] });
    render(<MaterialPane taskId="t1" seedUrl="https://x" />);
    const box = await screen.findByPlaceholderText(/把材料贴进来/);
    fireEvent.change(box, { target: { value: "我粘的。" } });
    fireEvent.click(screen.getByText("加入材料"));
    await waitFor(() => expect(api.createMaterial).toHaveBeenCalledWith("t1", expect.objectContaining({ text: "我粘的。" })));
    expect(await screen.findByText("我粘的。")).toBeInTheDocument();
  });

  it("shows the paste fallback immediately when there is no seed", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    render(<MaterialPane taskId="t1" seedUrl={null} />);
    expect(await screen.findByPlaceholderText(/把材料贴进来/)).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/workspace/MaterialPane.test.tsx`
Expected: FAIL — `Failed to resolve import "./MaterialPane"`.

- [ ] **Step 3: Implement `MaterialPane`**

Create `apps/web/src/workspace/MaterialPane.tsx`:

```tsx
import { useEffect, useRef, useState } from "react";
import type { Material } from "@mind-imprint/contracts";
import { api, MaterialFetchError } from "../api";

type Phase = "loading" | "ready" | "paste";

export function MaterialPane({ taskId, seedUrl }: { taskId: string; seedUrl: string | null }) {
  const [materials, setMaterials] = useState<Material[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [phase, setPhase] = useState<Phase>("loading");
  const [pasteTitle, setPasteTitle] = useState("");
  const [pasteText, setPasteText] = useState("");
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    let cancelled = false;
    void (async () => {
      try {
        const existing = await api.listMaterials(taskId);
        if (cancelled) return;
        if (existing.length > 0) {
          setMaterials(existing);
          setActiveId(existing[0]!.id);
          setPhase("ready");
          return;
        }
        if (!seedUrl) {
          setPhase("paste");
          return;
        }
        try {
          const m = await api.fetchMaterialFromSeed(taskId);
          if (cancelled) return;
          setMaterials([m]);
          setActiveId(m.id);
          setPhase("ready");
        } catch (e) {
          if (cancelled) return;
          if (!(e instanceof MaterialFetchError)) {
            // Non-fetch errors also degrade to paste (never a blocking error).
          }
          setPhase("paste");
        }
      } catch {
        if (!cancelled) setPhase("paste");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [taskId, seedUrl]);

  async function submitPaste() {
    const text = pasteText.trim();
    if (!text) return;
    const kind: "article" | "draft" = seedUrl ? "article" : "draft";
    const m = await api.createMaterial(taskId, { kind, title: pasteTitle.trim() || "我的材料", text });
    setMaterials((prev) => [...prev, m]);
    setActiveId(m.id);
    setPasteTitle("");
    setPasteText("");
    setPhase("ready");
  }

  const active = materials.find((m) => m.id === activeId) ?? null;

  return (
    <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
      {materials.length > 0 && (
        <div style={{ flex: "none", padding: "12px 16px 10px", borderBottom: "1px solid #F2F3F7", display: "flex", gap: 8, overflowX: "auto" }}>
          {materials.map((m) => {
            const on = m.id === activeId;
            return (
              <button
                key={m.id}
                type="button"
                onClick={() => setActiveId(m.id)}
                style={{
                  flex: "none", padding: "7px 12px", borderRadius: 10, fontSize: 12, fontWeight: 600,
                  cursor: "pointer", whiteSpace: "nowrap", fontFamily: "inherit",
                  background: on ? "#EDEFF9" : "#F7F8FB", color: on ? "#2A3B7A" : "#8A92A3",
                  border: `1px solid ${on ? "#DFE3F4" : "#EEF0F4"}`,
                }}
              >
                {m.title}
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => setPhase("paste")}
            style={{ flex: "none", padding: "7px 11px", borderRadius: 10, fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap", fontFamily: "inherit", color: "#9AA1B0", background: "#fff", border: "1px dashed #E1E4ED" }}
          >
            ＋ 加材料
          </button>
        </div>
      )}

      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "14px 16px 24px" }}>
        {phase === "loading" && <div style={{ fontSize: 13, color: "#9AA1B0" }}>正在载入材料…</div>}

        {phase === "paste" && (
          <div>
            <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.6, marginBottom: 12 }}>
              没能自动读取这个链接。把你正在读的材料贴进来，印记就能在上面陪你圈画。
            </div>
            <input
              value={pasteTitle}
              onChange={(e) => setPasteTitle(e.target.value)}
              placeholder="给材料起个名字（可留空）"
              style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 10, padding: "10px 12px", fontSize: 13.5, color: "#1C2333", background: "#fff", outline: "none", marginBottom: 10 }}
            />
            <textarea
              value={pasteText}
              onChange={(e) => setPasteText(e.target.value)}
              rows={10}
              placeholder="把材料贴进来——文章正文，或你自己的草稿。"
              style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 10, padding: "11px 13px", fontSize: 14, lineHeight: 1.7, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }}
            />
            <button
              type="button"
              onClick={() => void submitPaste()}
              style={{ marginTop: 10, background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 10, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
            >
              加入材料
            </button>
          </div>
        )}

        {phase === "ready" && active && <MaterialBody key={active.id} taskId={taskId} material={active} />}
      </div>
    </div>
  );
}

function MaterialBody({ taskId, material }: { taskId: string; material: Material }) {
  const [scratch, setScratch] = useState(material.scratch);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  function onScratch(v: string) {
    setScratch(v);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      void api.saveScratch(taskId, material.id, v).catch(() => {});
    }, 600);
  }

  return (
    <div>
      <div style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "20px 22px" }}>
        <div style={{ fontSize: 17, fontWeight: 800, color: "#1C2333", lineHeight: 1.5 }}>{material.title}</div>
        <div style={{ marginTop: 14 }}>
          {material.blocks.map((b) => (
            <p key={b.id} style={{ fontSize: 15, lineHeight: 2.1, color: "#2B3346", margin: "0 0 14px" }}>
              {b.text}
            </p>
          ))}
        </div>
      </div>
      <div style={{ marginTop: 14, background: "#FBF7EF", border: "1px solid #F0E6D2", borderRadius: 12, padding: "13px 15px" }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: "#8A6520", marginBottom: 8 }}>随手记</div>
        <textarea
          value={scratch}
          onChange={(e) => onScratch(e.target.value)}
          rows={3}
          placeholder="临时的想法、要去查的东西、一个反例……"
          style={{ width: "100%", border: "none", borderRadius: 8, padding: "9px 11px", fontSize: 13, lineHeight: 1.6, color: "#5C4A22", background: "#fff", outline: "none", resize: "vertical" }}
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run — expect PASS + typecheck**

Run: `cd apps/web && npx vitest run src/workspace/MaterialPane.test.tsx && npx tsc --noEmit`
Expected: PASS (4 tests); tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/MaterialPane.tsx apps/web/src/workspace/MaterialPane.test.tsx
git commit -m "feat(web): MaterialPane — fetch/paste + blocks + scratch"
```

---

### Task 4: `RightPanel` + workspace swap

**Files:**
- Create: `apps/web/src/workspace/RightPanel.tsx`
- Modify: `apps/web/src/workspace/WorkspaceView.tsx`
- Modify: `apps/web/src/workspace/WorkspaceView.test.tsx`
- Test: `apps/web/src/workspace/RightPanel.test.tsx`

**Interfaces:**
- Consumes: `TreeBody` (Task 2), `MaterialPane` (Task 3).
- Produces: `RightPanel({ nodes, taskId, seedUrl }: { nodes?: ProcessNode[]; taskId: string; seedUrl: string | null })` — a self-contained sidebar owning its open/tab/width state, rendering a 材料/过程树 segmented header + drag divider + collapse, with `MaterialPane` or `TreeBody` in the body.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/workspace/RightPanel.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { ProcessNode } from "./processTree";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return { ...real, api: { ...real.api, listMaterials: vi.fn().mockResolvedValue([]), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() } };
});

import { RightPanel } from "./RightPanel";

const nodes: ProcessNode[] = [
  { id: "root", kind: "task_root", title: "任务根" } as ProcessNode,
  { id: "n1", kind: "card_use", title: "用了 CRAAP", sub: "信息素养" } as ProcessNode,
];

describe("RightPanel", () => {
  beforeEach(() => vi.clearAllMocks());

  it("defaults to the process-tree tab", () => {
    render(<RightPanel nodes={nodes} taskId="t1" seedUrl={null} />);
    expect(screen.getByText("用了 CRAAP")).toBeInTheDocument();
  });

  it("switches to the material tab", async () => {
    render(<RightPanel nodes={nodes} taskId="t1" seedUrl={null} />);
    fireEvent.click(screen.getByRole("tab", { name: "材料" }));
    // no seed → paste fallback surfaces
    expect(await screen.findByPlaceholderText(/把材料贴进来/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/workspace/RightPanel.test.tsx`
Expected: FAIL — `Failed to resolve import "./RightPanel"`.

- [ ] **Step 3: Implement `RightPanel`**

Create `apps/web/src/workspace/RightPanel.tsx`:

```tsx
import { useRef, useState } from "react";
import type { ProcessNode } from "./processTree";
import { TreeBody } from "./TreeBody";
import { MaterialPane } from "./MaterialPane";

type Tab = "material" | "tree";
const MIN_W = 320;
const MAX_W = 640;

export function RightPanel({ nodes = [], taskId, seedUrl }: { nodes?: ProcessNode[]; taskId: string; seedUrl: string | null }) {
  const [open, setOpen] = useState(true);
  const [tab, setTab] = useState<Tab>("tree");
  const [width, setWidth] = useState(360);
  const drag = useRef<{ startX: number; startW: number } | null>(null);

  function onDragStart(e: React.MouseEvent) {
    drag.current = { startX: e.clientX, startW: width };
    const onMove = (ev: MouseEvent) => {
      if (!drag.current) return;
      const next = drag.current.startW - (ev.clientX - drag.current.startX);
      setWidth(Math.max(MIN_W, Math.min(MAX_W, next)));
    };
    const onUp = () => {
      drag.current = null;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  if (!open) {
    return (
      <button
        type="button"
        aria-label="展开侧栏"
        onClick={() => setOpen(true)}
        style={{ width: 46, flex: "none", background: "#fff", display: "flex", flexDirection: "column", alignItems: "center", paddingTop: 16, gap: 14, cursor: "pointer", border: "none", borderLeft: "1px solid #EAECF2" }}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#6B7384" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
        <span style={{ writingMode: "vertical-rl", fontSize: "12.5px", fontWeight: 700, color: "#6B7384", letterSpacing: ".08em" }}>材料 / 过程树</span>
      </button>
    );
  }

  const segBtn = (active: boolean): React.CSSProperties => ({
    flex: 1, display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
    padding: "6px 10px", borderRadius: 7, fontSize: 12.5, fontWeight: 700, cursor: "pointer",
    fontFamily: "inherit", border: "none",
    background: active ? "#fff" : "transparent", color: active ? "#2A3B7A" : "#8A92A3",
    boxShadow: active ? "0 1px 2px rgba(20,30,60,.08)" : "none",
  });

  return (
    <>
      <div onMouseDown={onDragStart} style={{ width: 7, flex: "none", cursor: "col-resize", display: "flex", alignItems: "center", justifyContent: "center", background: "transparent" }}>
        <div style={{ width: 3, height: 34, borderRadius: 2, background: "#D6DAE4" }} />
      </div>
      <div style={{ width: `${width}px`, flex: "none", background: "#fff", borderLeft: "1px solid #EAECF2", display: "flex", flexDirection: "column" }}>
        <div style={{ height: 54, flex: "none", display: "flex", alignItems: "center", gap: 10, padding: "0 14px 0 16px", borderBottom: "1px solid #EFF0F5" }}>
          <div style={{ flex: 1, display: "flex", gap: 3, background: "#F1F2F5", borderRadius: 9, padding: 3 }}>
            <button type="button" role="tab" aria-selected={tab === "material"} onClick={() => setTab("material")} style={segBtn(tab === "material")}>材料</button>
            <button type="button" role="tab" aria-selected={tab === "tree"} onClick={() => setTab("tree")} style={segBtn(tab === "tree")}>过程树</button>
          </div>
          <button type="button" aria-label="折叠侧栏" onClick={() => setOpen(false)} style={{ flex: "none", cursor: "pointer", color: "#9AA1B0", padding: 6, borderRadius: 7, display: "flex", background: "none", border: "none" }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </button>
        </div>
        {tab === "tree" ? <TreeBody nodes={nodes} /> : <MaterialPane taskId={taskId} seedUrl={seedUrl} />}
      </div>
    </>
  );
}
```

- [ ] **Step 4: Swap `RightPanel` into `WorkspaceView`**

In `apps/web/src/workspace/WorkspaceView.tsx`:

4a. Replace the import `import { TreePanel } from "./TreePanel";` with `import { RightPanel } from "./RightPanel";`.

4b. Remove the `const [treeOpen, setTreeOpen] = useState(true);` line (RightPanel owns its open state now). Keep the other `useState`.

4c. Replace the `<TreePanel open={treeOpen} onToggle={() => setTreeOpen((v) => !v)} nodes={treeNodes} />` line with:

```tsx
        <RightPanel nodes={treeNodes} taskId={taskId} seedUrl={task?.seed ?? null} />
```

- [ ] **Step 5: Update `WorkspaceView.test.tsx`**

`WorkspaceView.test.tsx` imports/asserts around `TreePanel`. Open it and: (a) if it `vi.mock`s or imports `./TreePanel`, retarget to `./RightPanel` (or drop the mock and let the real `RightPanel` render — but then it needs the `../api` mock); (b) since the workspace now renders `RightPanel` whose 材料 pane calls `api.listMaterials` on mount, add an `../api` mock at the top of the file so the pane doesn't hit the network:

```tsx
vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return { ...real, api: { ...real.api, listMaterials: vi.fn().mockResolvedValue([]), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() } };
});
```

Keep every existing assertion that checks tree content — the default tab is 过程树, so tree nodes still render. If a test asserted the collapsed-tree toggle via `TreePanel`'s exact `aria-label`, update it to `RightPanel`'s labels (`折叠侧栏` / `展开侧栏`). Run the file and fix any remaining reference to the old `TreePanel` props.

- [ ] **Step 6: Run — expect PASS (RightPanel + WorkspaceView) + typecheck**

Run: `cd apps/web && npx vitest run src/workspace/RightPanel.test.tsx src/workspace/WorkspaceView.test.tsx && npx tsc --noEmit`
Expected: PASS both files; tsc clean.

- [ ] **Step 7: Full web suite**

Run: `cd apps/web && npx vitest run`
Expected: entire web suite green (the new material tests + all prior tests).

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/workspace/RightPanel.tsx apps/web/src/workspace/RightPanel.test.tsx apps/web/src/workspace/WorkspaceView.tsx apps/web/src/workspace/WorkspaceView.test.tsx
git commit -m "feat(web): tabbed 材料/过程树 RightPanel + workspace swap"
```

---

## Self-Review

**Spec coverage:** API client with `MaterialFetchError` mapping (T1) ✅; 材料/过程树 tabbed draggable sidebar (T4) ✅; MaterialPane fetch→paste fallback + blocks + scratch (T3) ✅; blocks render as plain `<p>` (T3) ✅; TreeBody reuse without duplicating tree rendering (T2) ✅; no store-schema change (MaterialPane local state) ✅; frontend-only ✅.

**Placeholder scan:** No TBD/TODO; complete code in every code step. The empty `if (!(e instanceof MaterialFetchError)) { }` branch in MaterialPane is intentional (both fetch-error and other-error degrade to paste); it documents the distinction without dead behavior — acceptable, but a reviewer may suggest collapsing it (fine to simplify to a single `setPhase("paste")`).

**Type consistency:** `MaterialFetchError` defined in T1 (`materials.ts`), re-exported from `../api`, consumed in T3/T4. `Material` (contract) fields (`blocks: {id,text}[]`, `title`, `scratch`, `source_url`) used consistently. `RightPanel({nodes,taskId,seedUrl})` produced in T4 matches the `WorkspaceView` call site (`seedUrl={task?.seed ?? null}`). `TreeBody({nodes})` produced T2, consumed T4.

**Ordering:** T1 client → T2 TreeBody → T3 MaterialPane (needs T1) → T4 RightPanel (needs T2+T3) + workspace swap.

## Note for the executor

- `MaterialPane`'s empty `if (!(e instanceof MaterialFetchError))` block is intentional documentation (both fetch-error and other-error degrade to paste). If the task reviewer prefers, collapse the catch to a single `setPhase("paste")` — behavior is identical. Do not add an error toast (spec: fetch failure is never a blocking error).
