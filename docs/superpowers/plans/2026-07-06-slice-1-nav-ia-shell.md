# Slice 1 · Nav / IA Shell — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reposition the student app from a flat 3-tab shell (`任务 / 记录 / 设置`) to the 4-pillar shell (`课程 / 批判思维 / 我的评估 / 设置`), reskin the working-portal home to "项目" (project) vocabulary, and render the real Courses grid from a one-course mock fixture.

**Architecture:** Frontend-only React changes in `apps/web`. `LeftRail` grows to 4 tabs; `StudentApp` routes a new `courses` tab to a new `CoursesView` that reads a throwaway mock fixture; `DirectoryView` gets copy-only "项目" reskin + a `userName` prop + an empty state. No backend, contracts, store, or API changes.

**Tech Stack:** React 18 + TypeScript, Vite, Vitest + @testing-library/react + jest-dom (setup at `src/test/setup.ts`). Tests run from `apps/web` with `npx vitest run <path>`.

## Global Constraints

- **Frontend only.** No edits to `apps/api`, `packages/contracts`, `src/store/*`, or `src/api/*`. No new network calls.
- **UI copy is Chinese, verbatim from the binding design** `思维印记 工作区.dc.html`. Code identifiers/comments are English.
- **Internal tab key `tasks` is retained** for the 批判思维 portal — only its label changes. `TabKey`/`Tab` becomes `"courses" | "tasks" | "records" | "settings"`.
- **Palette tokens:** indigo `#2A3B7A`, active-pill `#EDEFF9`, ink `#1C2333`, muted `#6B7384`/`#8A92A3`/`#9AA1B0`, hairline `#EAECF2`, bg `#F3F4F8`, green `#4C9A82`. Icons: `viewBox="0 0 24 24"`, `stroke-width` 2 (rail) / 1.9–2.4 (cards), size per markup.
- **Course cards are inert on click in this slice** (the player is Slice 4): handlers are no-ops, no navigation.
- Every task ends green (`npx vitest run <changed test file>`) and is committed.

---

### Task 1: LeftRail — four pillar tabs

**Files:**
- Modify: `apps/web/src/shell/LeftRail.tsx`
- Test: `apps/web/src/shell/LeftRail.test.tsx`

**Interfaces:**
- Produces: `LeftRail` with `TabKey = "courses" | "tasks" | "records" | "settings"`; props unchanged `{ tab: TabKey; onTab: (t: TabKey) => void }`. Rendered tab labels, in order: `课程`, `批判思维`, `我的评估`, `设置`.

- [ ] **Step 1: Rewrite the test to the new tab set**

Replace the entire contents of `apps/web/src/shell/LeftRail.test.tsx` with:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LeftRail } from "./LeftRail";

describe("LeftRail", () => {
  it("renders the four pillar tabs in order", () => {
    render(<LeftRail tab="tasks" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["课程", "批判思维", "我的评估", "设置"]);
  });

  it("marks the active tab via aria-selected", () => {
    render(<LeftRail tab="records" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    const records = tabs.find((t) => t.textContent?.includes("我的评估"))!;
    expect(records.getAttribute("aria-selected")).toBe("true");
  });

  it("fires onTab when nav items are clicked", () => {
    const onTab = vi.fn();
    render(<LeftRail tab="tasks" onTab={onTab} />);
    fireEvent.click(screen.getByText("课程"));
    expect(onTab).toHaveBeenCalledWith("courses");
    fireEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });
});
```

- [ ] **Step 2: Run the test — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/LeftRail.test.tsx`
Expected: FAIL — the current rail renders `任务/记录/设置`, so `toEqual([...])` and the `课程` click assertions fail.

- [ ] **Step 3: Update `LeftRail.tsx`**

In `apps/web/src/shell/LeftRail.tsx`:

3a. Replace the stale provenance comment block at the top (lines 1–4) with:

```tsx
// LEFT RAIL — four pillars per binding design 思维印记 工作区.dc.html (left-rail section).
// Tabs: 课程 (courses) / 批判思维 (tasks portal) / 我的评估 (records) / 设置 (settings).
```

3b. Change the `TabKey` type to:

```tsx
type TabKey = "courses" | "tasks" | "records" | "settings";
```

3c. Replace the entire `NAV_ITEMS` array with the four items below (courses first, tasks relabelled 批判思维 with a star icon, records relabelled 我的评估 with a bar-chart icon, settings unchanged):

```tsx
const NAV_ITEMS: NavItem[] = [
  {
    key: "courses",
    label: "课程",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 5.5A2.5 2.5 0 016.5 3H20v15H6.5A2.5 2.5 0 004 20.5z" />
        <path d="M20 18v3H6.5A2.5 2.5 0 014 18.5" />
        <path d="M9 7.5h7M9 11h5" />
      </svg>
    ),
  },
  {
    key: "tasks",
    label: "批判思维",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z" />
      </svg>
    ),
  },
  {
    key: "records",
    label: "我的评估",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 20V10M6 20v-5M18 20V6" />
        <path d="M3 20h18" />
      </svg>
    ),
  },
  {
    key: "settings",
    label: "设置",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
      </svg>
    ),
  },
];
```

No other change is needed — `BOX_BASE`/`ACTIVE_BOX`/`ACTIVE_ICON_STROKE`/`ACTIVE_LABEL` constants and the `LeftRail` render body already drive styling and `aria-selected` from `NAV_ITEMS` + the `tab` prop.

- [ ] **Step 4: Run the test — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/LeftRail.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/LeftRail.tsx apps/web/src/shell/LeftRail.test.tsx
git commit -m "feat(web): LeftRail 4-pillar nav (课程/批判思维/我的评估/设置)"
```

---

### Task 2: CoursesView + mock course fixture

**Files:**
- Create: `apps/web/src/shell/courses/fixtures.ts`
- Create: `apps/web/src/shell/courses/CoursesView.tsx`
- Test: `apps/web/src/shell/courses/CoursesView.test.tsx`

**Interfaces:**
- Produces: `export function CoursesView(): JSX.Element` (no props); `export interface MockCourse`, `export type CourseToneKind`, `export const MOCK_COURSES: MockCourse[]`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/shell/courses/CoursesView.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoursesView } from "./CoursesView";

describe("CoursesView", () => {
  it("renders the header and the mock course card", () => {
    render(<CoursesView />);
    expect(screen.getByText("系统地学会一种思考方式")).toBeInTheDocument();
    expect(screen.getByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("3 个任务 · 4 个工具")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument();
    expect(screen.getByText("未开始")).toBeInTheDocument();
    expect(screen.getByText("开始学习")).toBeInTheDocument();
  });

  it("clicking the course CTA is an inert no-op that does not throw", () => {
    render(<CoursesView />);
    expect(() => fireEvent.click(screen.getByText("开始学习"))).not.toThrow();
  });
});
```

- [ ] **Step 2: Run the test — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesView.test.tsx`
Expected: FAIL — `Failed to resolve import "./CoursesView"` (module does not exist yet).

- [ ] **Step 3: Create the fixture**

Create `apps/web/src/shell/courses/fixtures.ts`:

```ts
// SLICE-1 MOCK FIXTURE — throwaway. Replaced by real backend course data in Slice 4.
// Do not treat as a source of truth; it exists only to exercise the Courses grid UI.

export type CourseToneKind = "not_started" | "in_progress" | "done";

export interface MockCourse {
  id: string;
  branch: string; // pillar label shown as the figure chip, e.g. "批判性思维"
  title: string;
  blurb: string;
  meta: string; // "3 个任务 · 4 个工具"
  time: string; // "约 40 分钟"
  progressPct: number | null; // null → hide the progress bar
  tone: string; // "未开始" | "进行中" | "已学完"
  toneKind: CourseToneKind;
  ctaLabel: string; // "开始学习" | "继续" | "回顾"
}

export const MOCK_COURSES: MockCourse[] = [
  {
    id: "c-info-trust",
    branch: "批判性思维",
    title: "一条网络信息，该不该信",
    blurb:
      "从一句「卫星图显示中国让地球变绿」出发，跟着印记学会横向溯源、辨识来源、拆穿断言——把「随手一信」变成「查过再信」。",
    meta: "3 个任务 · 4 个工具",
    time: "约 40 分钟",
    progressPct: null,
    tone: "未开始",
    toneKind: "not_started",
    ctaLabel: "开始学习",
  },
];
```

- [ ] **Step 4: Create the view**

Create `apps/web/src/shell/courses/CoursesView.tsx`:

```tsx
import type { CSSProperties } from "react";
import type { MockCourse, CourseToneKind } from "./fixtures";
import { MOCK_COURSES } from "./fixtures";

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

function toneStyle(kind: CourseToneKind): CSSProperties {
  const base: CSSProperties = {
    display: "inline-flex",
    alignItems: "center",
    gap: 5,
    fontSize: 12,
    fontWeight: 700,
    padding: "5px 11px",
    borderRadius: 999,
  };
  if (kind === "done") return { ...base, color: "#4C9A82", background: "#E7F3EE" };
  if (kind === "in_progress") return { ...base, color: "#2A3B7A", background: "#EDEFF9" };
  return { ...base, color: "#8A92A3", background: "#F1F2F5" };
}

function CourseCard({ course, onOpen }: { course: MockCourse; onOpen: () => void }) {
  return (
    <div
      onClick={onOpen}
      style={{
        display: "flex",
        flexDirection: "column",
        background: "#fff",
        border: "1px solid #EAECF2",
        borderRadius: 18,
        overflow: "hidden",
        cursor: "pointer",
        boxShadow: "0 1px 3px rgba(20,30,60,.04)",
      }}
    >
      {/* figure band */}
      <div
        style={{
          position: "relative",
          height: 120,
          background: "linear-gradient(135deg,#EDEFF9,#F3F0EC)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <span
          style={{
            position: "absolute",
            top: 14,
            left: 16,
            fontSize: 11,
            fontWeight: 700,
            color: "#2A3B7A",
            background: "rgba(255,255,255,.85)",
            padding: "4px 10px",
            borderRadius: 999,
          }}
        >
          {course.branch}
        </span>
        <div
          style={{
            width: 56,
            height: 56,
            borderRadius: 16,
            background: "#fff",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            boxShadow: "0 6px 16px rgba(42,59,122,.12)",
          }}
        >
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
            <path d={STAR} />
          </svg>
        </div>
      </div>

      {/* body */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: "18px 22px 20px" }}>
        <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333", lineHeight: 1.4 }}>{course.title}</div>
        <div style={{ fontSize: 13, color: "#6B7384", lineHeight: 1.66, marginTop: 8 }}>{course.blurb}</div>

        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 14, fontSize: 12, color: "#8A92A3", fontWeight: 600 }}>
          <span>{course.meta}</span>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="9" />
              <path d="M12 7v5l3 2" />
            </svg>
            {course.time}
          </span>
        </div>

        {course.progressPct !== null && (
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
            <div style={{ flex: 1, height: 6, background: "#EEF0F4", borderRadius: 999, overflow: "hidden" }}>
              <div style={{ width: `${course.progressPct}%`, height: "100%", background: "#2A3B7A" }} />
            </div>
            <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 700, flex: "none" }}>{course.progressPct}%</span>
          </div>
        )}

        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginTop: "auto", paddingTop: 16 }}>
          <span style={toneStyle(course.toneKind)}>
            {course.toneKind === "done" && (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
                <path d="M20 6L9 17l-5-5" />
              </svg>
            )}
            {course.tone}
          </span>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onOpen();
            }}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              background: "#2A3B7A",
              color: "#fff",
              border: "none",
              padding: "9px 15px",
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 700,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            {course.ctaLabel}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
              <path d="M5 12h14M13 6l6 6-6 6" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}

export function CoursesView() {
  // Slice 1: course cards are inert; the course player arrives in Slice 4.
  const noop = () => {};
  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>课程</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6, letterSpacing: "-0.01em" }}>
          系统地学会一种思考方式
        </div>
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>
          每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去「批判思维」工作台把它用在你自己的问题上。
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20, marginTop: 28 }}>
          {MOCK_COURSES.map((c) => (
            <CourseCard key={c.id} course={c} onOpen={noop} />
          ))}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Run the test — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesView.test.tsx`
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/courses/
git commit -m "feat(web): Courses grid view + one-course mock fixture (inert cards)"
```

---

### Task 3: DirectoryView — "项目" vocabulary reskin

**Files:**
- Modify: `apps/web/src/shell/directory/DirectoryView.tsx`
- Test: `apps/web/src/shell/directory/DirectoryView.test.tsx`
- Modify: `apps/web/src/shell/AppShell.test.tsx` (old H1 assertions)
- Modify: `apps/web/e2e/golden-path.spec.ts` (H1 + placeholder copy; not run by vitest but kept truthful)

**Interfaces:**
- Produces: `DirectoryView` gains an optional `userName?: string` prop: `{ store: Store; onOpenTask: (taskId: string, openingText?: string) => void; userName?: string; now?: () => Date }`. Greeting renders `下午好，{userName ?? "Phoebe"} · 批判思维工作台`.

- [ ] **Step 1: Update the DirectoryView test to new copy + empty state**

Replace the entire contents of `apps/web/src/shell/directory/DirectoryView.test.tsx` with:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { createStore } from "../../store/createStore";
import { DirectoryView } from "./DirectoryView";

vi.mock("../../api", () => ({
  api: {
    listTasks: vi.fn(async () => []),
    createTask: vi.fn(async (i: any) => ({ id: "t-new", title: i.title, seed: i.seed, status: "active", created_at: "1", last_active_at: "1" })),
  },
}));

describe("DirectoryView", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("renders the project vocabulary and empty-state hint", async () => {
    const store = createStore({});
    render(<DirectoryView store={store} onOpenTask={vi.fn()} />);
    expect(screen.getByText("你想搞懂什么？")).toBeInTheDocument();
    expect(screen.getByText("进行中的项目")).toBeInTheDocument();
    expect(screen.getByText("0 个项目")).toBeInTheDocument();
    expect(screen.getByText("还没有项目，从上面开一个吧。")).toBeInTheDocument();
  });

  it("uses the provided userName in the greeting", () => {
    const store = createStore({});
    render(<DirectoryView store={store} onOpenTask={vi.fn()} userName="Alex" />);
    expect(screen.getByText("下午好，Alex · 批判思维工作台")).toBeInTheDocument();
  });

  it("creates a project via the API and opens it with the opening text", async () => {
    const store = createStore({});
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.change(screen.getByPlaceholderText(/把你正纠结的问题/), { target: { value: "中国是否让地球更可持续？https://x" } });
    fireEvent.click(screen.getByText("开新项目"));
    await waitFor(() => expect(onOpenTask).toHaveBeenCalledWith("t-new", "中国是否让地球更可持续？https://x"));
  });

  it("empty input does not call the API or open a project", async () => {
    const store = createStore({});
    const onOpenTask = vi.fn();
    const { api } = await import("../../api");
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.click(screen.getByText("开新项目"));
    await new Promise((r) => setTimeout(r, 50));
    expect(onOpenTask).not.toHaveBeenCalled();
    expect(api.createTask).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the test — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/directory/DirectoryView.test.tsx`
Expected: FAIL — `你想搞懂什么？`, `进行中的项目`, the empty hint, the `userName` greeting, and the `开新项目` button do not exist yet.

- [ ] **Step 3: Apply the copy + prop + empty-state edits in `DirectoryView.tsx`**

3a. Change the component signature and destructure (currently `{ store, onOpenTask, now }`):

```tsx
export function DirectoryView({
  store,
  onOpenTask,
  userName,
  now,
}: {
  store: Store;
  onOpenTask: (taskId: string, openingText?: string) => void;
  userName?: string;
  now?: () => Date;
}) {
```

3b. Replace the greeting eyebrow line (currently `下午好，Phoebe`):

```tsx
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>
          下午好，{userName ?? "Phoebe"} · 批判思维工作台
        </div>
```

3c. Replace the H1 text `今天你在尝试什么？` with `你想搞懂什么？` (keep the surrounding `<div>` styles).

3d. Replace the subtitle block (currently "你有任何想讨论的作业、课题、信息、资料，都可以来找我哦" with `fontWeight: 500, fontSize: 15`) with:

```tsx
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>
          每一个项目是你正在思考的一件事——可以随时离开，回来接着想。想搞懂新的东西时，开一个新项目。
        </div>
```

3e. Replace the input `placeholder` with:

```
开一个新项目——把你正纠结的问题写下来，带上你自己的东西（链接、草稿、本子上的话）。
```

3f. Replace the start-button label text `开始` with `开新项目` (keep the trailing arrow `<svg>` and button styles).

3g. Replace the section header + count (currently `进行中的任务` / `{taskCount} 个任务`):

```tsx
          <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>进行中的项目</div>
          <div style={{ fontSize: 13, color: "#9AA1B0", fontWeight: 600 }}>{taskCount} 个项目</div>
```

3h. Add an empty-state hint. Immediately **after** the grid `</div>` that closes `tasks.map(...)`, add:

```tsx
        {taskCount === 0 && (
          <div style={{ fontSize: 13, color: "#9AA1B0", marginTop: 14 }}>
            还没有项目，从上面开一个吧。
          </div>
        )}
```

- [ ] **Step 4: Fix the two other tests/specs that assert the old copy**

4a. In `apps/web/src/shell/AppShell.test.tsx`, replace **both** occurrences (lines ~29 and ~85) of:

```tsx
    await waitFor(() => expect(screen.getByText("今天你在尝试什么？")).toBeInTheDocument());
```

with:

```tsx
    await waitFor(() => expect(screen.getByText("你想搞懂什么？")).toBeInTheDocument());
```

4b. In `apps/web/e2e/golden-path.spec.ts`, update the copy references (kept truthful; e2e is not run in this cycle):
- `page.getByText("今天你在尝试什么？")` → `page.getByText("你想搞懂什么？")`
- `page.getByPlaceholder("把你正在纠结的问题写下来——带上你自己的东西（链接、草稿、本子上的话）。")` → `page.getByPlaceholder("开一个新项目——把你正纠结的问题写下来，带上你自己的东西（链接、草稿、本子上的话）。")`

- [ ] **Step 5: Run the affected vitest files — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/directory/DirectoryView.test.tsx src/shell/AppShell.test.tsx`
Expected: PASS (all tests in both files).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/directory/DirectoryView.tsx apps/web/src/shell/directory/DirectoryView.test.tsx apps/web/src/shell/AppShell.test.tsx apps/web/e2e/golden-path.spec.ts
git commit -m "feat(web): directory reskin to 项目 vocabulary + userName greeting + empty state"
```

---

### Task 4: StudentApp — route Courses tab + thread userName

**Files:**
- Modify: `apps/web/src/shell/StudentApp.tsx`
- Test: `apps/web/src/shell/StudentApp.test.tsx` (create)

**Interfaces:**
- Consumes: `CoursesView` (Task 2), `LeftRail` with the `courses` tab (Task 1), `DirectoryView`'s `userName` prop (Task 3).
- Produces: `StudentApp` renders `CoursesView` when the `课程` tab is active and passes `session.getUser()?.display_name` into `DirectoryView`.

- [ ] **Step 1: Write the failing integration test**

Create `apps/web/src/shell/StudentApp.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { createStore } from "../store/createStore";
import { createSession } from "./session";
import { StudentApp } from "./StudentApp";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return { ...real, api: { ...real.api, listTasks: vi.fn(async () => []) } };
});

function makeSession() {
  let s = "{}";
  const storage = { getItem: () => s, setItem: (_: string, v: string) => { s = v; } };
  const session = createSession({ storage });
  session.setUser({
    id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
    avatar_color: "#2A3B7A", school: { id: "s1", name: "Demo" }, classes: [],
  });
  return session;
}

describe("StudentApp", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("starts on the 批判思维 directory with the user's name", () => {
    const store = createStore({});
    render(<StudentApp store={store} session={makeSession()} onLogout={() => {}} />);
    expect(screen.getByText("你想搞懂什么？")).toBeInTheDocument();
    expect(screen.getByText("下午好，Phoebe · 批判思维工作台")).toBeInTheDocument();
  });

  it("switches to the Courses tab and renders the course grid", () => {
    const store = createStore({});
    render(<StudentApp store={store} session={makeSession()} onLogout={() => {}} />);
    fireEvent.click(screen.getByText("课程"));
    expect(screen.getByText("系统地学会一种思考方式")).toBeInTheDocument();
    expect(screen.getByText("一条网络信息，该不该信")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/StudentApp.test.tsx`
Expected: FAIL — clicking `课程` renders nothing (no `courses` branch), so `系统地学会一种思考方式` is not found.

- [ ] **Step 3: Wire the new tab in `StudentApp.tsx`**

3a. Add the import (next to the other view imports):

```tsx
import { CoursesView } from "./courses/CoursesView";
```

3b. Widen the `Tab` type:

```tsx
type Tab = "courses" | "tasks" | "records" | "settings";
```

3c. Add the courses branch and thread `userName`. Replace the render body's tab branches so they read:

```tsx
        {tab === "courses" && <CoursesView />}
        {tab === "tasks" && taskView === "directory" && (
          <DirectoryView
            store={store}
            userName={session.getUser()?.display_name}
            onOpenTask={(id, opening) => {
              setActiveTaskId(id);
              setOpeningMessage(opening);
              setTaskView("workspace");
            }}
          />
        )}
```

(Leave the `tasks`+`workspace`, `records`, and `settings` branches unchanged.)

- [ ] **Step 4: Run the test — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/StudentApp.test.tsx`
Expected: PASS (2 tests).

- [ ] **Step 5: Full-suite + typecheck sanity**

Run: `cd apps/web && npx vitest run && npx tsc --noEmit`
Expected: entire web suite green; no type errors.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/StudentApp.tsx apps/web/src/shell/StudentApp.test.tsx
git commit -m "feat(web): route Courses tab in StudentApp + thread userName to directory"
```

---

## Self-Review

**Spec coverage:**
- LeftRail 4 tabs + icons + active tokens → Task 1. ✅
- StudentApp routes `courses` → Task 4. ✅
- CoursesView real grid from mock fixture, inert cards, one seed course with exact fields → Task 2. ✅
- DirectoryView "项目" copy reskin + `userName` prop + empty state → Task 3. ✅
- Frontend-only, no backend/contracts/store change → no task touches those. ✅
- Tests per component (LeftRail, CoursesView, DirectoryView, StudentApp) → Tasks 1–4. ✅
- Ripple: AppShell.test + DirectoryView.test + LeftRail.test + e2e golden-path copy → Tasks 1 & 3. ✅

**Placeholder scan:** No TBD/TODO; every code step contains full code or exact string swaps. ✅

**Type consistency:** `TabKey`/`Tab` = `"courses" | "tasks" | "records" | "settings"` in both LeftRail (Task 1) and StudentApp (Task 4). `MockCourse`/`CourseToneKind`/`MOCK_COURSES` defined in Task 2 and consumed only there. `DirectoryView` `userName?: string` defined in Task 3 and passed in Task 4 via `session.getUser()?.display_name` (`MeUser.display_name: string`). ✅

**Ordering:** 1 (rail) → 2 (courses) → 3 (directory prop) → 4 (wire, depends on 1+2+3). ✅
