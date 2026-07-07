# Slice 4c · Course Player — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** The course player: 课程 grid → open a course → a header + stage stepper + per-step rendered content (teaching / challenge templates, fed by the render endpoint) + text ask-panel + prev/next/finish with progress saved.

**Architecture:** `api.renderCourseStep`; two pure presentational templates (`TeachingTemplate`, `ChallengeTemplate`); a `CoursePlayer` that loads the course + progress, renders the current step via the render endpoint, and drives navigation + `saveCourseProgress`; a small courses container (grid ↔ player) wired into `StudentApp`.

**Tech Stack:** React 18 + TS, Vitest + RTL. From `apps/web`: `npx vitest run`, `npx tsc`. Voice is deferred — the ask-panel is text-only (no voice button).

## Global Constraints

- **Frontend only.** Consumes the Slice-4a/4b course API (`getCourse`, `getCourseProgress`, `saveCourseProgress`, `renderCourseStep`). Client never calls the model — content comes from the render endpoint.
- **Templates are pure/presentational** (given content); `CoursePlayer` owns fetching + nav + progress.
- **UI copy Chinese, verbatim from the design** (`COURSE PLAYER`); palette per app. Voice button omitted (4-voice).
- The challenge template renders the step's `anchors` as a flat list of questions (like `AnnotationBranch`) + a reason textarea; answers are local (courses don't create card instances this MVP).
- Every task ends green + `tsc` clean, committed.

---

### Task 1: `renderCourseStep` client + step templates

**Files:**
- Modify: `apps/web/src/api/courses.ts`, `apps/web/src/api/index.ts`
- Create: `apps/web/src/shell/courses/TeachingTemplate.tsx`, `apps/web/src/shell/courses/ChallengeTemplate.tsx`
- Test: `apps/web/src/shell/courses/templates.test.tsx`

**Interfaces:**
- Produces: `api.renderCourseStep(courseId, ordinal): Promise<RenderedStep>`; `TeachingTemplate({content})`, `ChallengeTemplate({content})` where content is the parsed template content.

- [ ] **Step 1: Add `renderCourseStep` to the client**

In `apps/web/src/api/courses.ts` add (importing `RenderedStep`):

```ts
import type { Course, CourseSummary, CourseProgress, RenderedStep } from "@mind-imprint/contracts";
```
```ts
export async function renderCourseStep(courseId: string, ordinal: number): Promise<RenderedStep> {
  const r = await apiFetch<{ rendered: RenderedStep }>(`/api/v1/courses/${courseId}/steps/${ordinal}/render`, { method: "POST" });
  return r.rendered;
}
```
In `apps/web/src/api/index.ts`: add `RenderedStep` to the type import, `renderCourseStep` to the `./courses` import, the `ApiClient` interface (`renderCourseStep(courseId: string, ordinal: number): Promise<RenderedStep>;`), and the `api` object.

- [ ] **Step 2: Write the failing template test**

Create `apps/web/src/shell/courses/templates.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TeachingTemplate } from "./TeachingTemplate";
import { ChallengeTemplate } from "./ChallengeTemplate";

describe("course templates", () => {
  it("TeachingTemplate renders title, subtitle caption, and body", () => {
    render(<TeachingTemplate content={{ title: "先别急着信", subtitle: "停一下。", body: ["第一段。", "第二段。"], foreground_asset_id: null }} />);
    expect(screen.getByText("先别急着信")).toBeInTheDocument();
    expect(screen.getByText("停一下。")).toBeInTheDocument();
    expect(screen.getByText("第一段。")).toBeInTheDocument();
  });

  it("ChallengeTemplate renders the prompt + anchored questions, and reveals feedback on submit", () => {
    render(<ChallengeTemplate content={{
      title: "现在轮到你", prompt: "哪句是事实？", reason_hint: "说说理由",
      anchors: [{ id: "a0", material_id: "m0", block_id: "b0", start: 0, end: 0, quote: "美航局发现", dimension: "权威性", author: "ai", question: "作者是权威吗？", answer: "" }],
    }} />);
    expect(screen.getByText("现在轮到你")).toBeInTheDocument();
    expect(screen.getByText("作者是权威吗？")).toBeInTheDocument();
    expect(screen.queryByText(/很好，你已经开始像个核查者/)).toBeNull();
    fireEvent.click(screen.getByText("提交我的判断"));
    expect(screen.getByText(/很好，你已经开始像个核查者/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/templates.test.tsx`
Expected: FAIL — templates don't exist.

- [ ] **Step 4: Implement `TeachingTemplate`**

Create `apps/web/src/shell/courses/TeachingTemplate.tsx`:

```tsx
export type TeachingContent = { title: string; subtitle: string; body: string[]; foreground_asset_id: string | null };

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

export function TeachingTemplate({ content }: { content: TeachingContent }) {
  return (
    <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
      <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-0.01em", lineHeight: 1.3 }}>{content.title}</div>
      <div style={{ marginTop: 16, height: 148, borderRadius: 16, background: "linear-gradient(135deg,#EDEFF9 0%,#F3F0EC 100%)", border: "1px solid #EAECF2", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 11 }}>
        <div style={{ width: 54, height: 54, borderRadius: 16, background: "#fff", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 6px 16px rgba(42,59,122,.12)" }}>
          <svg width="27" height="27" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
        </div>
        <div style={{ fontSize: 14, color: "#5B6373", fontWeight: 600, maxWidth: 520, textAlign: "center", padding: "0 20px" }}>{content.subtitle}</div>
      </div>
      <div style={{ marginTop: 20 }}>
        {content.body.map((p, i) => (
          <div key={i} style={{ fontSize: 15.5, lineHeight: 1.85, color: "#2B3346", marginBottom: 15 }}>{p}</div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Implement `ChallengeTemplate`**

Create `apps/web/src/shell/courses/ChallengeTemplate.tsx`:

```tsx
import { useState } from "react";
import type { Anchor } from "@mind-imprint/contracts";

export type ChallengeContent = { title: string; prompt: string; reason_hint: string; anchors: Anchor[] };

export function ChallengeTemplate({ content }: { content: ChallengeContent }) {
  const [answers, setAnswers] = useState<string[]>(content.anchors.map((a) => a.answer));
  const [reason, setReason] = useState("");
  const [submitted, setSubmitted] = useState(false);

  return (
    <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
      <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-0.01em", lineHeight: 1.3 }}>{content.title}</div>
      <div style={{ marginTop: 16, display: "inline-flex", alignItems: "center", gap: 7, fontSize: 12, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "5px 12px", borderRadius: 999 }}>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#D98263" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M13 2L3 14h7l-1 8 10-12h-7z" /></svg>
        现在轮到你
      </div>
      <div style={{ fontSize: 15.5, lineHeight: 1.8, color: "#2B3346", marginTop: 14 }}>{content.prompt}</div>

      <div style={{ marginTop: 16 }}>
        {content.anchors.map((a, i) => (
          <div key={a.id} style={{ border: "1px solid #ECEEF3", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#fff" }}>
            <span style={{ display: "inline-flex", fontSize: 11, fontWeight: 700, padding: "3px 10px", borderRadius: 999, color: "#fff", background: "#7C6BB5" }}>{a.dimension}</span>
            <div style={{ fontSize: 14, lineHeight: 1.7, color: "#2B3346", fontWeight: 500, marginTop: 8 }}>{a.question}</div>
            <textarea value={answers[i]} onChange={(e) => setAnswers((prev) => prev.map((x, j) => (j === i ? e.target.value : x)))} rows={2}
              placeholder="写下你的判断……"
              style={{ width: "100%", marginTop: 9, border: "1px solid #E1E4ED", borderRadius: 9, padding: "9px 11px", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }} />
          </div>
        ))}
      </div>

      <div style={{ fontSize: 13, fontWeight: 600, color: "#3A4256", margin: "8px 0 9px" }}>{content.reason_hint || "写一句你的理由"}</div>
      <textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} placeholder="说说你为什么这么判断……"
        style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 12, padding: "12px 14px", fontSize: 14, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }} />

      {!submitted ? (
        <button type="button" onClick={() => setSubmitted(true)}
          style={{ marginTop: 12, background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 10, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>提交我的判断</button>
      ) : (
        <div style={{ marginTop: 16, background: "#E7F3EE", border: "1px solid #D3E9DF", borderRadius: 12, padding: "15px 17px", fontSize: 13.5, lineHeight: 1.72, color: "#2B4A3E" }}>
          很好，你已经开始像个核查者一样思考了——先分辨事实与情绪，再决定信不信。带着这份判断，继续下一步。
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 6: Run — expect PASS + tsc**

Run: `cd apps/web && npx vitest run src/shell/courses/templates.test.tsx && npx tsc --noEmit`
Expected: PASS (2 tests); tsc clean.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/api/courses.ts apps/web/src/api/index.ts apps/web/src/shell/courses/TeachingTemplate.tsx apps/web/src/shell/courses/ChallengeTemplate.tsx apps/web/src/shell/courses/templates.test.tsx
git commit -m "feat(web): course step templates (teaching + challenge) + renderCourseStep client"
```

---

### Task 2: `CoursePlayer`

**Files:**
- Create: `apps/web/src/shell/courses/CoursePlayer.tsx`
- Test: `apps/web/src/shell/courses/CoursePlayer.test.tsx`

**Interfaces:**
- Consumes: `api.getCourse/getCourseProgress/saveCourseProgress/renderCourseStep`, the templates (Task 1).
- Produces: `CoursePlayer({ courseId, onExit }: { courseId: string; onExit: () => void })` — header + stepper + current step + prev/next/finish, saving progress.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/shell/courses/CoursePlayer.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { Course } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, getCourse: vi.fn(), getCourseProgress: vi.fn(), saveCourseProgress: vi.fn(), renderCourseStep: vi.fn() } };
});

import { api } from "../../api";
import { CoursePlayer } from "./CoursePlayer";

const course: Course = {
  id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 2,
  steps: [
    { id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} },
    { id: "s1", course_id: "co1", ordinal: 1, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} },
  ],
};

describe("CoursePlayer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourse as any).mockResolvedValue(course);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 0, completed_ordinals: [], updated_at: "" });
    (api.saveCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0], updated_at: "" });
    (api.renderCourseStep as any).mockImplementation(async (_c: string, ord: number) => ({
      ordinal: ord, kind: "teaching", template: "teaching", source: "generated",
      content: { title: `第 ${ord} 步`, subtitle: "导语", body: ["正文。"], foreground_asset_id: null },
    }));
  });

  it("renders the first step and advances to the next on 下一步", async () => {
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} />);
    expect(await screen.findByText("第 0 步")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("下一步"));
    expect(await screen.findByText("第 1 步")).toBeInTheDocument();
    await waitFor(() => expect(api.saveCourseProgress).toHaveBeenCalled());
  });

  it("exits via the back control", async () => {
    const onExit = vi.fn();
    render(<CoursePlayer courseId="co1" onExit={onExit} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByText("课程"));
    expect(onExit).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursePlayer.test.tsx`
Expected: FAIL — `CoursePlayer` missing.

- [ ] **Step 3: Implement `CoursePlayer`**

Create `apps/web/src/shell/courses/CoursePlayer.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { Course, RenderedStep } from "@mind-imprint/contracts";
import { api } from "../../api";
import { TeachingTemplate, type TeachingContent } from "./TeachingTemplate";
import { ChallengeTemplate, type ChallengeContent } from "./ChallengeTemplate";

export function CoursePlayer({ courseId, onExit }: { courseId: string; onExit: () => void }) {
  const [course, setCourse] = useState<Course | null>(null);
  const [ordinal, setOrdinal] = useState(0);
  const [rendered, setRendered] = useState<RenderedStep | null>(null);
  const [completed, setCompleted] = useState<number[]>([]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const c = await api.getCourse(courseId);
      if (cancelled) return;
      setCourse(c);
      try {
        const p = await api.getCourseProgress(courseId);
        if (cancelled) return;
        setCompleted(p.completed_ordinals);
        setOrdinal(Math.min(p.current_ordinal, c.steps.length - 1));
      } catch { /* default 0 */ }
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  useEffect(() => {
    if (!course) return;
    let cancelled = false;
    setRendered(null);
    void api.renderCourseStep(courseId, ordinal).then((r) => { if (!cancelled) setRendered(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [course, courseId, ordinal]);

  if (!course) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在载入课程…</div>;

  const total = course.steps.length;
  const isLast = ordinal >= total - 1;

  function go(next: number) {
    const nextCompleted = Array.from(new Set([...completed, ordinal])).sort((a, b) => a - b);
    setCompleted(nextCompleted);
    void api.saveCourseProgress(courseId, { current_ordinal: next, completed_ordinals: nextCompleted }).catch(() => {});
    setOrdinal(next);
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%" }}>
      {/* header */}
      <div style={{ flex: "none", background: "#fff", borderBottom: "1px solid #EFF0F5" }}>
        <div style={{ height: 50, display: "flex", alignItems: "center", padding: "0 20px", gap: 13 }}>
          <div onClick={onExit} style={{ display: "flex", alignItems: "center", gap: 6, color: "#6B7384", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: "6px 10px", borderRadius: 8 }}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
            课程
          </div>
          <div style={{ width: 1, height: 20, background: "#EAECF2" }} />
          <span style={{ flex: "none", width: 8, height: 8, borderRadius: 3, background: "#2A3B7A" }} />
          <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{course.title}</span>
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 9 }}>
            <span style={{ fontSize: 11.5, fontWeight: 600, color: "#AEB4C2", background: "#F2F3F8", padding: "2px 9px", borderRadius: 999 }}>{ordinal + 1} / {total}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4, padding: "0 20px 11px" }}>
          {course.steps.map((s, i) => (
            <div key={s.id} style={{ flex: 1, height: 4, borderRadius: 3, background: i < ordinal || completed.includes(i) ? "#4C9A82" : i === ordinal ? "#2A3B7A" : "#E7E9F0" }} />
          ))}
        </div>
      </div>

      {/* body */}
      <div style={{ flex: 1, minHeight: 0, position: "relative", display: "flex", flexDirection: "column", background: "#F3F4F8", overflowY: "auto" }}>
        <div style={{ flex: 1, padding: "36px 40px 60px" }}>
          {!rendered ? (
            <div style={{ maxWidth: 700, margin: "0 auto", color: "#9AA1B0" }}>印记正在为你准备这一页…</div>
          ) : rendered.template === "challenge" ? (
            <ChallengeTemplate content={rendered.content as ChallengeContent} />
          ) : (
            <TeachingTemplate content={rendered.content as TeachingContent} />
          )}
        </div>

        {/* nav */}
        {ordinal > 0 && (
          <div aria-label="上一步" onClick={() => setOrdinal(ordinal - 1)} style={{ position: "absolute", left: 14, top: "44%", width: 40, height: 40, borderRadius: "50%", background: "#fff", border: "1px solid #E7E9F0", boxShadow: "0 3px 12px rgba(20,30,60,.10)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", color: "#6B7384" }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
          </div>
        )}
        {!isLast ? (
          <div aria-label="下一步" onClick={() => go(ordinal + 1)} style={{ position: "absolute", right: 14, top: "44%", width: 44, height: 44, borderRadius: "50%", background: "#2A3B7A", boxShadow: "0 5px 16px rgba(42,59,122,.28)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6" /></svg>
          </div>
        ) : (
          <div aria-label="完成课程" onClick={() => { go(ordinal); onExit(); }} style={{ position: "absolute", right: 14, top: "44%", width: 44, height: 44, borderRadius: "50%", background: "#4C9A82", boxShadow: "0 5px 16px rgba(76,154,130,.30)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
        )}
      </div>
    </div>
  );
}
```

(The finish control currently exits to the grid; Slice 4d replaces `onExit` on the last step with a course-report transition.)

- [ ] **Step 4: Run — expect PASS + tsc**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursePlayer.test.tsx && npx tsc --noEmit`
Expected: PASS (2 tests); tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/courses/CoursePlayer.tsx apps/web/src/shell/courses/CoursePlayer.test.tsx
git commit -m "feat(web): CoursePlayer — stepper + per-step render + nav + progress"
```

---

### Task 3: Wire grid → player in the courses surface

**Files:**
- Modify: `apps/web/src/shell/courses/CoursesView.tsx` (card `onOpen`)
- Create: `apps/web/src/shell/courses/CoursesContainer.tsx`
- Modify: `apps/web/src/shell/StudentApp.tsx` (render `CoursesContainer` for the `courses` tab)
- Test: `apps/web/src/shell/courses/CoursesContainer.test.tsx`

**Interfaces:**
- Produces: `CoursesContainer` — grid ↔ player state; `CoursesView` gains an `onOpenCourse(id)` prop.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/shell/courses/CoursesContainer.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { Course } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, listCourses: vi.fn(), getCourseProgress: vi.fn(), getCourse: vi.fn(), saveCourseProgress: vi.fn(), renderCourseStep: vi.fn() } };
});

import { api } from "../../api";
import { CoursesContainer } from "./CoursesContainer";

const summary = { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 1 };
const course: Course = { ...summary, steps: [{ id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} }] };

describe("CoursesContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listCourses as any).mockResolvedValue([summary]);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 0, completed_ordinals: [], updated_at: "" });
    (api.getCourse as any).mockResolvedValue(course);
    (api.renderCourseStep as any).mockResolvedValue({ ordinal: 0, kind: "teaching", template: "teaching", source: "generated", content: { title: "开场", subtitle: "s", body: ["b"], foreground_asset_id: null } });
  });

  it("opens the player when a course is clicked, and returns to the grid", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习"));
    expect(await screen.findByText("开场")).toBeInTheDocument(); // player step
    fireEvent.click(screen.getByText("课程")); // back
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // grid header
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesContainer.test.tsx`
Expected: FAIL — `CoursesContainer` missing; `CoursesView` card has no open handler.

- [ ] **Step 3: Add `onOpenCourse` to `CoursesView`**

In `apps/web/src/shell/courses/CoursesView.tsx`: add an `onOpenCourse?: (id: string) => void` prop to `CoursesView`; pass it to `CourseCard` and wire the card's CTA button + card container `onClick` to `onOpenCourse?.(course.id)`. (The `CourseCard` already renders a CTA button — give it `onClick={() => onOpen()}` where `onOpen` is threaded from the prop.)

- [ ] **Step 4: Create `CoursesContainer`**

Create `apps/web/src/shell/courses/CoursesContainer.tsx`:

```tsx
import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";

export function CoursesContainer() {
  const [activeCourseId, setActiveCourseId] = useState<string | null>(null);
  if (activeCourseId) {
    return <CoursePlayer courseId={activeCourseId} onExit={() => setActiveCourseId(null)} />;
  }
  return <CoursesView onOpenCourse={(id) => setActiveCourseId(id)} />;
}
```

- [ ] **Step 5: Wire into `StudentApp`**

In `apps/web/src/shell/StudentApp.tsx`: replace the `import { CoursesView } from "./courses/CoursesView";` import with `import { CoursesContainer } from "./courses/CoursesContainer";`, and change the `{tab === "courses" && <CoursesView />}` branch to `{tab === "courses" && <CoursesContainer />}`.

- [ ] **Step 6: Run — expect PASS + full suite + tsc**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesContainer.test.tsx src/shell/courses/CoursesView.test.tsx src/shell/StudentApp.test.tsx && npx tsc --noEmit`
Expected: PASS. Then full suite: `cd apps/web && npx vitest run` → green. (Update `StudentApp.test.tsx` / `CoursesView.test.tsx` only if the new `onOpenCourse` optional prop or the container wrapper changes an assertion — the `课程` tab still renders the grid header, so the existing nav test should pass; if `StudentApp.test` now needs the extra course-API mocks to reach the grid, add them.)

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/shell/courses/CoursesView.tsx apps/web/src/shell/courses/CoursesContainer.tsx apps/web/src/shell/StudentApp.tsx apps/web/src/shell/courses/CoursesContainer.test.tsx apps/web/src/shell/StudentApp.test.tsx apps/web/src/shell/courses/CoursesView.test.tsx
git commit -m "feat(web): courses grid → player navigation (CoursesContainer)"
```

---

## Self-Review

**Spec coverage:** render client + templates (T1) ✅; player with stepper + per-step render + nav + progress (T2) ✅; grid→player routing (T3) ✅; teaching template (subtitle-as-caption, body) + challenge template (anchored questions + reason + feedback) ✅; ask-panel voice button omitted (deferred) — the text ask-panel itself is deferred to a follow-up (see note). Course report is 4d.

**Placeholder scan:** none — complete code. The ask-panel (问印记) is intentionally NOT built here (see note) to bound the slice; flagged, not a silent gap.

**Type consistency:** `renderCourseStep(courseId, ordinal): Promise<RenderedStep>` (T1) consumed in `CoursePlayer` (T2). `TeachingContent`/`ChallengeContent` produced in T1, cast from `rendered.content` in T2. `CoursePlayer({courseId,onExit})` (T2) used by `CoursesContainer` (T3). `CoursesView` `onOpenCourse` (T3) drives the container.

**Ordering:** T1 (client+templates) → T2 (player, consumes T1) → T3 (routing, consumes T2).

## Note for executor / roadmap
- The **ask-panel ("问印记" — a text question box with course-page context)** is deliberately deferred to a small follow-up (call it 4c-ask) to keep this slice to the core player. The design's voice button stays out until 4-voice. If you want it in-scope, it's a right-sidebar panel calling a (future) course-ask endpoint — not built here.
- On the last step, "完成课程" currently calls `onExit` (returns to grid). Slice 4d wires it to the course report instead.
