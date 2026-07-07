# Slice 4d · Course Report — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** The course-completion report — a summary (hero, stats, what-you-learned, challenge review, actions) shown when a student finishes a course, wired into the player's finish control.

**Architecture:** A self-contained `CourseReport` (fetches the course + progress, derives the summary); `CoursePlayer` gains an `onFinish` callback (distinct from `onExit`); `CoursesContainer` gains a `report` state (grid → player → report → grid) + threads an `onGoPortal` from `StudentApp` for the "去批判思维工作台" action.

**Tech Stack:** React 18 + TS, Vitest + RTL. From `apps/web`: `npx vitest run`, `npx tsc`.

## Global Constraints

- **Frontend only.** Report data is derived from the already-available course (`getCourse`) + progress (`getCourseProgress`) — no new backend. Course-level SOLO eval + note-export are **deferred** (this MVP report shows structural stats + learnings + challenge review + actions).
- UI copy Chinese, verbatim from the design (`COURSE REPORT`); palette per app.
- Every task ends green + `tsc` clean, committed.

---

### Task 1: `CourseReport` component

**Files:**
- Create: `apps/web/src/shell/courses/CourseReport.tsx`
- Test: `apps/web/src/shell/courses/CourseReport.test.tsx`

**Interfaces:**
- Produces: `CourseReport({ courseId, onBackToCourses, onGoPortal }: { courseId: string; onBackToCourses: () => void; onGoPortal: () => void })` — fetches the course + progress, renders the completion summary.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/shell/courses/CourseReport.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { Course } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, getCourse: vi.fn(), getCourseProgress: vi.fn() } };
});

import { api } from "../../api";
import { CourseReport } from "./CourseReport";

const course: Course = {
  id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 3,
  steps: [
    { id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "建立停一下的习惯", assets: [], challenge_type: null, authored_content: {} },
    { id: "s1", course_id: "co1", ordinal: 1, kind: "teaching", purpose: "教横向溯源", assets: [], challenge_type: null, authored_content: {} },
    { id: "s2", course_id: "co1", ordinal: 2, kind: "challenge", purpose: "亲手核查一处断言", assets: [], challenge_type: "verify_claim", authored_content: { title: "现在轮到你" } },
  ],
};

describe("CourseReport", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourse as any).mockResolvedValue(course);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 2, completed_ordinals: [0, 1, 2], updated_at: "" });
  });

  it("renders the hero, stats, learnings and challenge review", async () => {
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("学习报告 · 课程完成")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument(); // 用时 stat
    expect(screen.getByText("建立停一下的习惯")).toBeInTheDocument(); // a learning
    expect(screen.getByText("你学到了什么")).toBeInTheDocument();
  });

  it("wires the two actions", async () => {
    const onBack = vi.fn(); const onPortal = vi.fn();
    render(<CourseReport courseId="co1" onBackToCourses={onBack} onGoPortal={onPortal} />);
    await screen.findByText("一条网络信息，该不该信");
    fireEvent.click(screen.getByText("返回课程"));
    expect(onBack).toHaveBeenCalled();
    fireEvent.click(screen.getByText(/去批判思维工作台/));
    expect(onPortal).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/CourseReport.test.tsx`
Expected: FAIL — `CourseReport` missing.

- [ ] **Step 3: Implement `CourseReport`**

Create `apps/web/src/shell/courses/CourseReport.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { Course } from "@mind-imprint/contracts";
import { api } from "../../api";

function Stat({ value, label, color }: { value: string; label: string; color?: string }) {
  return (
    <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "16px 14px", textAlign: "center" }}>
      <div style={{ fontSize: 22, fontWeight: 800, color: color ?? "#1C2333" }}>{value}</div>
      <div style={{ fontSize: 12, color: "#8A92A3", fontWeight: 600, marginTop: 3 }}>{label}</div>
    </div>
  );
}

export function CourseReport({ courseId, onBackToCourses, onGoPortal }: { courseId: string; onBackToCourses: () => void; onGoPortal: () => void }) {
  const [course, setCourse] = useState<Course | null>(null);
  const [completed, setCompleted] = useState<number[]>([]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const c = await api.getCourse(courseId);
      let comp: number[] = [];
      try { comp = (await api.getCourseProgress(courseId)).completed_ordinals; } catch { /* none */ }
      if (cancelled) return;
      setCompleted(comp);
      setCourse(c);
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  if (!course) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的学习报告…</div>;

  const challenges = course.steps.filter((s) => s.kind === "challenge");
  const learnings = course.steps.filter((s) => s.kind === "teaching" && s.purpose).map((s) => s.purpose);

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        {/* hero */}
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>学习报告 · 课程完成</div>
            <div style={{ fontSize: 24, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>{course.title}</div>
            <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>{course.branch} · 恭喜你走完这一程，下面是你留下的印记。</div>
          </div>
        </div>

        {/* stats */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 12, marginTop: 16 }}>
          <Stat value={course.time_label} label="用时" />
          <Stat value={`${completed.length} / ${course.steps.length}`} label="阶段完成" />
          <Stat value={`${challenges.length}`} label="挑战通过" color="#D98263" />
          <Stat value={`${course.tools_count}`} label="工具收集" color="#4C9A82" />
        </div>

        {/* learned */}
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>你学到了什么</div>
          {learnings.map((l, i) => (
            <div key={i} style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 10 }}>
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 2 }}><path d="M20 6L9 17l-5-5" /></svg>
              <span style={{ fontSize: 14.5, color: "#2B3346", lineHeight: 1.6 }}>{l}</span>
            </div>
          ))}
        </div>

        {/* challenges */}
        {challenges.length > 0 && (
          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>挑战回顾</div>
            {challenges.map((c) => (
              <div key={c.id} style={{ display: "flex", gap: 12, alignItems: "flex-start", padding: "12px 0", borderBottom: "1px solid #F3F4F7" }}>
                <div style={{ flex: "none", width: 30, height: 30, borderRadius: 9, background: "#FBEEE7", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#D98263" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M13 2L3 14h7l-1 8 10-12h-7z" /></svg>
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{c.purpose || "挑战"}</div>
                </div>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 5 }}><path d="M20 6L9 17l-5-5" /></svg>
              </div>
            ))}
          </div>
        )}

        {/* actions */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 22 }}>
          <button type="button" onClick={onBackToCourses} style={{ flex: "none", background: "#fff", border: "1px solid #E1E4ED", color: "#6B7384", fontSize: 14, fontWeight: 700, padding: "13px 20px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>返回课程</button>
          <button type="button" onClick={onGoPortal} style={{ flex: 1, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: "#4C9A82", color: "#fff", border: "none", fontSize: 14.5, fontWeight: 700, padding: 13, borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>
            去批判思维工作台，用起来
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run — expect PASS + tsc**

Run: `cd apps/web && npx vitest run src/shell/courses/CourseReport.test.tsx && npx tsc --noEmit`
Expected: PASS (2 tests); tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/courses/CourseReport.test.tsx
git commit -m "feat(web): CourseReport — completion summary (hero, stats, learnings, challenge review)"
```

---

### Task 2: Wire finish → report

**Files:**
- Modify: `apps/web/src/shell/courses/CoursePlayer.tsx` (add `onFinish`)
- Modify: `apps/web/src/shell/courses/CoursesContainer.tsx` (grid → player → report)
- Modify: `apps/web/src/shell/StudentApp.tsx` (thread `onGoPortal` → switch to 批判思维 tab)
- Test: `apps/web/src/shell/courses/CoursesContainer.test.tsx` (extend)

**Interfaces:**
- Consumes: `CourseReport` (Task 1).
- Produces: `CoursePlayer` gains `onFinish: () => void`; `CoursesContainer` gains optional `onGoPortal?: () => void` and a `report` view.

- [ ] **Step 1: Extend the container test**

Add to `apps/web/src/shell/courses/CoursesContainer.test.tsx` (inside the existing `describe`, and ensure `getCourse`/`getCourseProgress`/`renderCourseStep` are mocked in `beforeEach` — for the single-step seed course, opening + clicking finish reaches the report):

```tsx
  it("shows the course report after finishing the last step", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习"));
    // single-step course → the finish (完成课程) control is shown immediately
    fireEvent.click(await screen.findByLabelText("完成课程"));
    expect(await screen.findByText("学习报告 · 课程完成")).toBeInTheDocument();
    fireEvent.click(screen.getByText("返回课程"));
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // back to grid
  });
```

(The existing `beforeEach` already mocks `listCourses`/`getCourseProgress`/`getCourse`/`renderCourseStep`; the single-step `course` fixture means `isLast` is true on step 0, so 完成课程 shows immediately.)

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesContainer.test.tsx`
Expected: FAIL — finishing exits to the grid (no report state yet).

- [ ] **Step 3: Add `onFinish` to `CoursePlayer`**

In `apps/web/src/shell/courses/CoursePlayer.tsx`: add `onFinish` to the props (`{ courseId, onExit, onFinish }: { courseId: string; onExit: () => void; onFinish: () => void }`), and change the finish control's handler from `onClick={() => { go(ordinal); onExit(); }}` to `onClick={() => { go(ordinal); onFinish(); }}`. Leave the back-button `onExit` and everything else unchanged.

- [ ] **Step 4: Add the report view to `CoursesContainer`**

Replace `apps/web/src/shell/courses/CoursesContainer.tsx` with:

```tsx
import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { CourseReport } from "./CourseReport";

type View = { name: "grid" } | { name: "player"; courseId: string } | { name: "report"; courseId: string };

export function CoursesContainer({ onGoPortal }: { onGoPortal?: () => void }) {
  const [view, setView] = useState<View>({ name: "grid" });

  if (view.name === "player") {
    return (
      <CoursePlayer
        courseId={view.courseId}
        onExit={() => setView({ name: "grid" })}
        onFinish={() => setView({ name: "report", courseId: view.courseId })}
      />
    );
  }
  if (view.name === "report") {
    return (
      <CourseReport
        courseId={view.courseId}
        onBackToCourses={() => setView({ name: "grid" })}
        onGoPortal={() => (onGoPortal ? onGoPortal() : setView({ name: "grid" }))}
      />
    );
  }
  return <CoursesView onOpenCourse={(id) => setView({ name: "player", courseId: id })} />;
}
```

- [ ] **Step 5: Thread `onGoPortal` from `StudentApp`**

In `apps/web/src/shell/StudentApp.tsx`: change the courses branch to pass an `onGoPortal` that switches to the 批判思维 (tasks) tab:

```tsx
        {tab === "courses" && <CoursesContainer onGoPortal={() => setTab("tasks")} />}
```

- [ ] **Step 6: Run — expect PASS + full suite + tsc**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesContainer.test.tsx src/shell/courses/CoursePlayer.test.tsx && npx tsc --noEmit`
Expected: PASS. Then `cd apps/web && npx vitest run` → full suite green. (If `CoursePlayer.test.tsx` breaks because it renders `CoursePlayer` without the new required `onFinish` prop, add `onFinish={vi.fn()}` to those renders.)

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/shell/courses/CoursePlayer.tsx apps/web/src/shell/courses/CoursesContainer.tsx apps/web/src/shell/StudentApp.tsx apps/web/src/shell/courses/CoursesContainer.test.tsx apps/web/src/shell/courses/CoursePlayer.test.tsx
git commit -m "feat(web): wire course finish → report → grid, and 去工作台 → 批判思维 tab"
```

---

## Self-Review

**Spec coverage:** CourseReport (hero, stats, learnings, challenge review, actions) (T1) ✅; finish → report → grid wiring + 去工作台 cross-tab (T2) ✅. Course-level SOLO eval + note-export deferred (noted). This completes Slice 4.

**Placeholder scan:** none — complete code.

**Type consistency:** `CourseReport({courseId,onBackToCourses,onGoPortal})` (T1) used by `CoursesContainer` (T2). `CoursePlayer` gains required `onFinish: () => void` (T2) — the CoursePlayer test renders must pass it (noted). `CoursesContainer` `onGoPortal?` (T2) threaded from `StudentApp`.

**Ordering:** T1 (report) → T2 (wire, consumes T1). T2 adds a required `onFinish` to `CoursePlayer` — its existing test renders must get `onFinish={vi.fn()}` (called out in T2 Step 6).

## Note for executor
- Adding the required `onFinish` prop to `CoursePlayer` will break its Slice-4c test renders (they pass only `courseId`+`onExit`). Add `onFinish={vi.fn()}` to those two renders as part of T2 (they're in `CoursePlayer.test.tsx`, which T2 already stages).
