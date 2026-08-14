# PDF Report Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the disabled/absent `导出 PDF` button on the student report page and the teacher report view into a working client-side print of a richer, methodology-transparent 过程评估报告 (cover + shared axiom + per-part intros + rubric anchor ladders as color-schema reference).

**Architecture:** A print-only React tree (`EvaluationReportPrint`) is mounted on demand via a `createPortal` into `document.body` inside a `.report-print-root` wrapper; a global print stylesheet hides the app (`#root`) and shows only that wrapper during `@media print`, then `window.print()` opens the browser's Save-as-PDF. No router (the app is state-driven), no backend, no LLM, no re-fetch — both surfaces already hold the ready `EvaluationReport`. All methodology text comes from `DUALAXIS_MODEL` in `@mind-imprint/contracts` (the same `dualaxis.json` Go embeds).

**Tech Stack:** React + TypeScript + Vite, inline `var(--mk-*)` design tokens (matches `TeacherReportView`), `react-dom` `createPortal`, `@testing-library/react` + vitest.

## Global Constraints

Every task's requirements implicitly include these:

- **Client-side only.** No changes under `apps/api`, no new endpoints, no new contract fields. `EvaluationReport` and `DUALAXIS_MODEL` already exist and are imported.
- **No LLM.** The document is 100% deterministic from the locked report JSON + the static rubric.
- **Chinese** content throughout.
- **No exact level/band, ever.** Never print `depth[].level` or `autonomy[].band` as a number, and never mark a single "you-are-here" rung/cell. The student's position is conveyed **by color only** (ramp chip). The L1–L4 ladder / 0–5 band strip appear as neutral, color-coded **reference**. No total, no composite score (`两轴永不合成总分`).
- **Reuse tokens, no new visual language.** Use `var(--mk-*)` tokens and the ramps `DEPTH_RAMP` (4 colors, index `level-1`) / `AUTONOMY_RAMP` (6 colors, index `band`) from `apps/web/src/shell/report/EvaluationReport/tokens.ts`.
- **Verbatim evidence.** Render `evidence[].quote` verbatim (length is fine).
- **Every commit compiles.** Each task wires its new components into `EvaluationReportPrint` so the build stays green; the TS gate is `npm run typecheck` (from `apps/web`), **not** `npm run build` (esbuild skips typecheck). Also run web vitest.

## File Structure

All new files under `apps/web/src/shell/report/print/`:

- `print.css` — screen/print visibility swap + page-break rules (imported once from `main.tsx`).
- `ReportPrintButton.tsx` — the `导出 PDF` button + portal + `window.print()` trigger + `afterprint` cleanup.
- `EvaluationReportPrint.tsx` — the document tree; composes the sections (grows task by task).
- `parts.tsx` — shared presentational helpers: `Cover`, `AboutReport`, `PrintSection`, `PrintTable`, `EvidenceList`.
- `sections.tsx` — `SummarySection`, `TimelineSection`, `MaterialsSection`, `PromptLensSection`, `ToolUsageSection`, `RisksSection`.
- `axis.tsx` — `DepthSection`, `AutonomySection` (the color-schema reference blocks).
- Co-located tests: `print.test.tsx` (button/trigger), `document.test.tsx` (sections).

Modified:
- `apps/web/src/main.tsx` — `import "./shell/report/print/print.css";`
- `apps/web/src/shell/report/EvaluationReportPage.tsx` — swap the disabled button for `ReportPrintButton` when ready.
- `apps/web/src/console/TeacherReportView.tsx` — add `ReportPrintButton` in the header when ready.

Reused (do not modify): `EvaluationReport`/`DUALAXIS_MODEL`/`DepthDims`/`AutonomySignals` from `@mind-imprint/contracts`; `DEPTH_RAMP`/`AUTONOMY_RAMP` from `../EvaluationReport/tokens`; `Button`/`Icon` from `@/ui`; the fixture `MOCK_EVALUATION_REPORT` from `../EvaluationReport/__fixtures__/mock`.

---

### Task 1: Print pipeline — button, portal, stylesheet, cover, both surfaces wired

**Files:**
- Create: `apps/web/src/shell/report/print/print.css`
- Create: `apps/web/src/shell/report/print/parts.tsx` (Cover only for now)
- Create: `apps/web/src/shell/report/print/EvaluationReportPrint.tsx`
- Create: `apps/web/src/shell/report/print/ReportPrintButton.tsx`
- Create: `apps/web/src/shell/report/print/print.test.tsx`
- Modify: `apps/web/src/main.tsx`
- Modify: `apps/web/src/shell/report/EvaluationReportPage.tsx:133-135`
- Modify: `apps/web/src/console/TeacherReportView.tsx` (header block ~line 120)

**Interfaces:**
- Produces: `ReportPrintButton({ report: EvaluationReport })` — a self-contained button both surfaces drop in. `EvaluationReportPrint({ report: EvaluationReport })` — the document tree (Cover only in this task). `Cover({ report })` from `parts.tsx`.

- [ ] **Step 1: Write `print.css`**

```css
/* Screen: the print document is never visible in the app. */
@media screen {
  .report-print-root { display: none; }
}

/* Print: hide the live app, show only the print document. */
@media print {
  body > *:not(.report-print-root) { display: none !important; }
  .report-print-root { display: block; }
  .report-print-root,
  .report-print-root * {
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }
  .print-section { break-inside: avoid; }
  .print-page-break { break-before: page; }
  .print-avoid-break { break-inside: avoid; }
}

/* Document base (applies in both modes; only shown during print). */
.report-print-root {
  background: #ffffff;
  color: var(--mk-ink);
  font-family: inherit;
}
.report-print-root .print-page {
  max-width: 760px;
  margin: 0 auto;
  padding: 32px 40px;
}
```

- [ ] **Step 2: Add `Cover` to `parts.tsx`**

```tsx
import type { EvaluationReport } from "@mind-imprint/contracts";

export function Cover({ report }: { report: EvaluationReport }) {
  const b = report.basics;
  const range = [b.startDate?.slice(0, 10), b.endDate ? b.endDate.slice(0, 10) : "进行中"]
    .filter(Boolean)
    .join(" — ");
  return (
    <section className="print-section print-page" data-testid="print-cover" style={{ paddingTop: 96 }}>
      <div style={{ fontSize: 13, fontWeight: 700, color: "var(--mk-accent-600)", letterSpacing: ".08em" }}>
        思维印记 · 过程评估报告
      </div>
      <h1 style={{ fontSize: 30, fontWeight: 800, color: "var(--mk-ink)", margin: "18px 0 10px", lineHeight: 1.25 }}>
        {b.title}
      </h1>
      <div style={{ fontSize: 15, color: "var(--mk-secondary)", marginBottom: 40 }}>
        {report.student.name}
      </div>
      <dl style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "8px 20px", fontSize: 13, color: "var(--mk-secondary)", margin: 0 }}>
        <dt style={{ color: "var(--mk-muted)" }}>项目类型</dt><dd style={{ margin: 0 }}>{b.type}</dd>
        <dt style={{ color: "var(--mk-muted)" }}>起止日期</dt><dd style={{ margin: 0 }}>{range}</dd>
        <dt style={{ color: "var(--mk-muted)" }}>生成日期</dt><dd style={{ margin: 0 }}>{report.generatedAt.slice(0, 10)}</dd>
      </dl>
      <p style={{ marginTop: 56, fontSize: 12, color: "var(--mk-muted)", lineHeight: 1.7, borderTop: "1px solid var(--mk-border)", paddingTop: 16 }}>
        本报告记录学生与 AI 协作中的思考过程，是事件级证据的整理，不构成人级评分。两轴（认知深度、智识自主）永不合成总分。
      </p>
    </section>
  );
}
```

- [ ] **Step 3: Write `EvaluationReportPrint.tsx` (Cover only)**

```tsx
import type { EvaluationReport } from "@mind-imprint/contracts";
import { Cover } from "./parts";

export function EvaluationReportPrint({ report }: { report: EvaluationReport }) {
  return (
    <div className="report-print-root-inner">
      <Cover report={report} />
    </div>
  );
}
```

- [ ] **Step 4: Write `ReportPrintButton.tsx`**

```tsx
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { Button } from "@/ui";
import { EvaluationReportPrint } from "./EvaluationReportPrint";

export function ReportPrintButton({ report }: { report: EvaluationReport }) {
  const [printing, setPrinting] = useState(false);

  useEffect(() => {
    if (!printing) return;
    const done = () => setPrinting(false);
    window.addEventListener("afterprint", done);
    // Print after the portal has painted this frame.
    const id = window.setTimeout(() => window.print(), 0);
    return () => {
      window.removeEventListener("afterprint", done);
      window.clearTimeout(id);
    };
  }, [printing]);

  return (
    <>
      <Button variant="secondary" size="sm" onClick={() => setPrinting(true)}>
        导出 PDF
      </Button>
      {printing &&
        createPortal(
          <div className="report-print-root">
            <EvaluationReportPrint report={report} />
          </div>,
          document.body,
        )}
    </>
  );
}
```

- [ ] **Step 5: Import the stylesheet in `main.tsx`**

Add after `import "./index.css";`:

```tsx
import "./shell/report/print/print.css";
```

- [ ] **Step 6: Wire the student page** — `EvaluationReportPage.tsx`, replace the disabled placeholder (currently lines 133–135):

```tsx
import { ReportPrintButton } from "@/shell/report/print/ReportPrintButton";
// ...
{state.status === "ready" ? (
  <ReportPrintButton report={state.report} />
) : (
  <Button variant="secondary" size="sm" disabled title="报告就绪后可导出">
    导出 PDF
  </Button>
)}
```

- [ ] **Step 7: Wire the teacher view** — `TeacherReportView.tsx`, in the header's right column (the `<div style={{ display: "flex", ... justifyContent: "space-between" }}>` block around line 120, which currently has only a left child) add a right child:

```tsx
import { ReportPrintButton } from "@/shell/report/print/ReportPrintButton";
// ...inside the justify-between row, after the left <div>:
{state.status === "ready" ? <ReportPrintButton report={state.report} /> : null}
```

- [ ] **Step 8: Write `print.test.tsx`**

```tsx
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MOCK_EVALUATION_REPORT } from "../EvaluationReport/__fixtures__/mock";
import { ReportPrintButton } from "./ReportPrintButton";

afterEach(cleanup);

describe("ReportPrintButton", () => {
  it("mounts the print document and calls window.print on click", async () => {
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => {});
    vi.useFakeTimers();
    render(<ReportPrintButton report={MOCK_EVALUATION_REPORT} />);
    fireEvent.click(screen.getByText("导出 PDF"));
    expect(document.querySelector(".report-print-root")).not.toBeNull();
    expect(screen.getByTestId("print-cover")).toBeInTheDocument();
    vi.runAllTimers();
    expect(printSpy).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
    printSpy.mockRestore();
  });

  it("removes the print document on afterprint", async () => {
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => {});
    render(<ReportPrintButton report={MOCK_EVALUATION_REPORT} />);
    fireEvent.click(screen.getByText("导出 PDF"));
    expect(document.querySelector(".report-print-root")).not.toBeNull();
    window.dispatchEvent(new Event("afterprint"));
    expect(document.querySelector(".report-print-root")).toBeNull();
    printSpy.mockRestore();
  });

  it("renders the cover identity from the report", () => {
    render(<ReportPrintButton report={MOCK_EVALUATION_REPORT} />);
    fireEvent.click(screen.getByText("导出 PDF"));
    expect(screen.getByText(MOCK_EVALUATION_REPORT.basics.title)).toBeInTheDocument();
    expect(screen.getByText(MOCK_EVALUATION_REPORT.student.name)).toBeInTheDocument();
  });
});
```

- [ ] **Step 9: Run tests + typecheck**

Run from `apps/web`: `npx vitest run src/shell/report/print` then `npm run typecheck`.
Expected: print tests PASS; typecheck 0 errors.

- [ ] **Step 10: Commit**

```bash
git add apps/web/src/shell/report/print apps/web/src/main.tsx apps/web/src/shell/report/EvaluationReportPage.tsx apps/web/src/console/TeacherReportView.tsx
git commit -m "feat(report-pdf): client-side print pipeline + cover + both surfaces wired"
```

---

### Task 2: Shared helpers + 关于这份报告 (axiom) + 综述

**Files:**
- Modify: `apps/web/src/shell/report/print/parts.tsx` (add `AboutReport`, `PrintSection`, `PrintTable`, `EvidenceList`)
- Create: `apps/web/src/shell/report/print/sections.tsx` (add `SummarySection`)
- Modify: `apps/web/src/shell/report/print/EvaluationReportPrint.tsx`
- Create: `apps/web/src/shell/report/print/document.test.tsx`

**Interfaces:**
- Consumes: `Cover` (Task 1).
- Produces: `AboutReport({ report })`, `PrintSection({ n, title, intro?, breakBefore?, children })`, `PrintTable({ head, rows })`, `EvidenceList({ evidence })` in `parts.tsx`; `SummarySection({ report })` in `sections.tsx`.

- [ ] **Step 1: Add shared helpers to `parts.tsx`**

```tsx
import type { EvidenceItem } from "@mind-imprint/contracts";
import { DUALAXIS_MODEL } from "@mind-imprint/contracts";

export function PrintSection({
  n, title, intro, breakBefore, children,
}: {
  n: string; title: string; intro?: React.ReactNode; breakBefore?: boolean; children: React.ReactNode;
}) {
  return (
    <section
      className={`print-section print-page${breakBefore ? " print-page-break" : ""}`}
      style={{ marginBottom: 30 }}
      data-testid={`print-section-${n}`}
    >
      <div style={{ display: "flex", alignItems: "baseline", gap: 10, marginBottom: intro ? 6 : 12 }}>
        <span style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-accent-600)" }}>{n}</span>
        <h2 style={{ fontSize: 19, fontWeight: 800, color: "var(--mk-ink)", margin: 0 }}>{title}</h2>
      </div>
      {intro ? (
        <p style={{ fontSize: 12.5, color: "var(--mk-muted)", lineHeight: 1.65, margin: "0 0 14px" }}>{intro}</p>
      ) : null}
      {children}
    </section>
  );
}

const thStyle: React.CSSProperties = {
  textAlign: "left", padding: "7px 9px", borderBottom: "1.5px solid var(--mk-border)",
  fontSize: 11.5, fontWeight: 700, color: "var(--mk-secondary)", background: "var(--mk-paper)",
};
const tdStyle: React.CSSProperties = {
  padding: "7px 9px", borderBottom: "1px solid var(--mk-border)", fontSize: 12,
  color: "var(--mk-ink)", verticalAlign: "top",
};

export function PrintTable({ head, rows }: { head: string[]; rows: React.ReactNode[][] }) {
  return (
    <table style={{ width: "100%", borderCollapse: "collapse" }} className="print-avoid-break">
      <thead>
        <tr>{head.map((h, i) => <th key={i} style={thStyle}>{h}</th>)}</tr>
      </thead>
      <tbody>
        {rows.map((r, ri) => (
          <tr key={ri}>{r.map((c, ci) => <td key={ci} style={tdStyle}>{c}</td>)}</tr>
        ))}
      </tbody>
    </table>
  );
}

export function EvidenceList({ evidence }: { evidence: EvidenceItem[] }) {
  if (evidence.length === 0) return null;
  return (
    <ul style={{ listStyle: "none", margin: "12px 0 0", padding: 0, display: "flex", flexDirection: "column", gap: 12 }}>
      {evidence.map((ev, i) => (
        <li key={i} className="print-avoid-break">
          <blockquote style={{ margin: 0, borderLeft: "2px solid var(--mk-lake)", paddingLeft: 10, fontSize: 12.5, color: "var(--mk-ink)" }}>
            {ev.quote}
          </blockquote>
          <div style={{ marginTop: 5, fontSize: 11.5, color: "var(--mk-muted)" }}>
            {ev.stage ? `${ev.stage} · ` : null}
            {ev.observation}
          </div>
          {ev.boundary ? (
            <div style={{ marginTop: 3, fontSize: 11.5, color: "var(--mk-danger)" }}>边界：{ev.boundary}</div>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

export function AboutReport({ report }: { report: import("@mind-imprint/contracts").EvaluationReport }) {
  void report;
  return (
    <PrintSection n="—" title="关于这份报告">
      <p style={{ fontSize: 13, color: "var(--mk-ink)", lineHeight: 1.7, margin: "0 0 12px" }}>
        {DUALAXIS_MODEL.axiom}
      </p>
      <p style={{ fontSize: 12.5, color: "var(--mk-secondary)", lineHeight: 1.7, margin: 0 }}>
        本报告用两个轴描述过程：<b>认知深度 D</b>——思考推进得有多深；<b>智识自主 A</b>——是否由学生自己驱动认知。
        两轴各自独立，永不合成一个总分；每个维度的具体读法写在对应章节的开头。
      </p>
    </PrintSection>
  );
}
```

- [ ] **Step 2: Add `SummarySection` to `sections.tsx`**

```tsx
import type { EvaluationReport } from "@mind-imprint/contracts";
import { PrintSection, PrintTable } from "./parts";

export function SummarySection({ report }: { report: EvaluationReport }) {
  const c = report.basics.counters;
  const m = report.basics.milestones;
  const a = report.abstract;
  const milestoneRow = [
    ["立项", m.started], ["框架", m.frameworkFinished], ["提案", m.proposalFinished],
    ["初稿", m.writingFinished], ["完成", m.projectFinished],
  ]
    .filter(([, v]) => !!v)
    .map(([k, v]) => `${k} ${String(v).slice(0, 10)}`)
    .join(" · ");

  return (
    <PrintSection n="02" title="综述 · 学生画像" intro="以下为过程记录的确定性汇总，均由平台自动计数，非模型评分。">
      <p style={{ fontSize: 13, color: "var(--mk-ink)", lineHeight: 1.75, margin: "0 0 10px" }}>{a.overview}</p>
      <p style={{ fontSize: 12.5, color: "var(--mk-secondary)", lineHeight: 1.7, margin: "0 0 14px" }}>
        {a.materialSentence} {a.writingSentence} {a.aiSentence}
      </p>
      <PrintTable
        head={["AI 轮次", "材料阅读", "字数", "AI 批注", "编辑次数"]}
        rows={[[c.aiTurns, c.materialsRead, c.wordsWritten, c.aiCommentCount, c.editCount]]}
      />
      {milestoneRow ? (
        <div style={{ marginTop: 12, fontSize: 11.5, color: "var(--mk-muted)" }}>里程碑：{milestoneRow}</div>
      ) : null}
      {a.suggestionParagraph ? (
        <p style={{ marginTop: 14, fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{a.suggestionParagraph}</p>
      ) : null}
    </PrintSection>
  );
}
```

- [ ] **Step 3: Wire into `EvaluationReportPrint.tsx`**

```tsx
import { AboutReport, Cover } from "./parts";
import { SummarySection } from "./sections";
// ...
<Cover report={report} />
<AboutReport report={report} />
<SummarySection report={report} />
```

- [ ] **Step 4: Write `document.test.tsx` (this task's cases)**

```tsx
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { DUALAXIS_MODEL } from "@mind-imprint/contracts";
import { MOCK_EVALUATION_REPORT as R } from "../EvaluationReport/__fixtures__/mock";
import { EvaluationReportPrint } from "./EvaluationReportPrint";

afterEach(cleanup);

describe("EvaluationReportPrint — front matter", () => {
  it("renders the shared axiom and the 综述 counters", () => {
    render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText(DUALAXIS_MODEL.axiom)).toBeInTheDocument();
    expect(screen.getByText("关于这份报告")).toBeInTheDocument();
    expect(screen.getByText("综述 · 学生画像")).toBeInTheDocument();
    expect(screen.getByText(String(R.basics.counters.aiTurns))).toBeInTheDocument();
  });
});
```

- [ ] **Step 5: Run tests + typecheck + commit**

`npx vitest run src/shell/report/print && npm run typecheck` (from `apps/web`). Then:

```bash
git add apps/web/src/shell/report/print
git commit -m "feat(report-pdf): shared helpers + 关于这份报告 axiom + 综述 section"
```

---

### Task 3: 过程时间线 + 材料清单

**Files:**
- Modify: `apps/web/src/shell/report/print/sections.tsx` (add `TimelineSection`, `MaterialsSection`)
- Modify: `apps/web/src/shell/report/print/EvaluationReportPrint.tsx`
- Modify: `apps/web/src/shell/report/print/document.test.tsx`

**Interfaces:**
- Consumes: `PrintSection`, `PrintTable` (Task 2).
- Produces: `TimelineSection({ events })`, `MaterialsSection({ materials })`.

- [ ] **Step 1: Add both sections to `sections.tsx`**

```tsx
import type { EventEntry, MaterialEntry } from "@mind-imprint/contracts";

const EVENT_KIND_LABEL: Record<EventEntry["kind"], string> = {
  chat: "对话", reading: "阅读", graph: "探索", writing: "写作", review: "回顾", milestone: "里程碑",
};

export function TimelineSection({ events }: { events: EventEntry[] }) {
  return (
    <PrintSection n="03" title="过程时间线" intro="下表由平台的事件轨迹自动生成，按时间顺序还原学生这个项目的关键动作。">
      <PrintTable
        head={["时间", "阶段", "摘要", "AI 轮次"]}
        rows={events.map((e) => [e.ts.slice(0, 10), EVENT_KIND_LABEL[e.kind] ?? e.kind, e.summary, e.aiTurns])}
      />
    </PrintSection>
  );
}

export function MaterialsSection({ materials }: { materials: MaterialEntry[] }) {
  return (
    <PrintSection
      n="04"
      title="材料清单"
      intro="材料清单记录学生用过的每一份来源，以及对它的溯源体检：来源等级与可信度、最终判定（用 / 存疑 / 弃用）、这份材料用在了哪里、以及它能支持什么、不能支持什么。这是论证是否有据的地基。"
    >
      <PrintTable
        head={["来源", "加入时间", "最终判定", "用在哪", "备注", "局限"]}
        rows={materials.map((m) => [
          m.url ? `${m.source}（${m.url}）` : m.source,
          m.addedAt.slice(0, 10),
          m.finalStatus,
          m.usedIn?.label ?? "—",
          m.comment || "—",
          m.cannotSupport || "—",
        ])}
      />
    </PrintSection>
  );
}
```

- [ ] **Step 2: Wire into `EvaluationReportPrint.tsx`** (after `SummarySection`):

```tsx
import { MaterialsSection, SummarySection, TimelineSection } from "./sections";
// ...
<TimelineSection events={report.events} />
<MaterialsSection materials={report.materials} />
```

- [ ] **Step 3: Add test cases to `document.test.tsx`**

```tsx
describe("EvaluationReportPrint — timeline & materials", () => {
  it("renders section intros and material rows", () => {
    render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("过程时间线")).toBeInTheDocument();
    expect(screen.getByText("材料清单")).toBeInTheDocument();
    if (R.materials[0]) {
      expect(screen.getByText(new RegExp(R.materials[0].source))).toBeInTheDocument();
    }
  });
});
```

- [ ] **Step 4: Run tests + typecheck + commit**

```bash
git add apps/web/src/shell/report/print
git commit -m "feat(report-pdf): 过程时间线 + 材料清单 sections with intros"
```

---

### Task 4: 认知深度 D — anchor-ladder reference (color schema, no asserted rung)

**Files:**
- Create: `apps/web/src/shell/report/print/axis.tsx` (add `DepthSection`)
- Modify: `apps/web/src/shell/report/print/EvaluationReportPrint.tsx`
- Modify: `apps/web/src/shell/report/print/document.test.tsx`

**Interfaces:**
- Consumes: `PrintSection`, `EvidenceList` (Task 2); `DEPTH_RAMP` from `../EvaluationReport/tokens`; `DepthDims` from `@mind-imprint/contracts`.
- Produces: `DepthSection({ dims: DepthDimResult[] })`.

- [ ] **Step 1: Write `DepthSection` in `axis.tsx`**

```tsx
import type { DepthDimResult } from "@mind-imprint/contracts";
import { DepthDims } from "@mind-imprint/contracts";
import { DEPTH_RAMP } from "../EvaluationReport/tokens";
import { EvidenceList, PrintSection } from "./parts";

const RUNGS = ["L1", "L2", "L3", "L4"] as const;

function DepthDimBlock({ d }: { d: DepthDimResult }) {
  const model = DepthDims().find((x) => x.id === d.id);
  const anchors = model?.anchors;
  // Position by COLOR ONLY — never print d.level, never mark a "current" rung.
  const chip = DEPTH_RAMP[d.level - 1] ?? DEPTH_RAMP[DEPTH_RAMP.length - 1];
  return (
    <div className="print-avoid-break" style={{ marginBottom: 20, borderTop: "1px solid var(--mk-border)", paddingTop: 14 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
        <span
          data-color-chip={d.id}
          aria-hidden
          style={{ width: 16, height: 16, borderRadius: 4, background: chip, flexShrink: 0 }}
        />
        <span style={{ fontSize: 12, fontWeight: 800, color: "var(--mk-secondary)" }}>{d.id}</span>
        <span style={{ fontSize: 15, fontWeight: 700, color: "var(--mk-ink)" }}>{model?.name ?? d.id}</span>
      </div>
      {model?.means ? (
        <div style={{ marginTop: 3, fontSize: 11.5, color: "var(--mk-muted)" }}>{model.means}</div>
      ) : null}

      {anchors ? (
        <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 6 }}>
          {RUNGS.map((r, i) => (
            <div key={r} style={{ display: "flex", gap: 8, alignItems: "flex-start" }}>
              <span
                aria-hidden
                style={{ width: 4, alignSelf: "stretch", borderRadius: 2, background: DEPTH_RAMP[i], flexShrink: 0 }}
              />
              <span style={{ fontSize: 11, fontWeight: 700, color: "var(--mk-secondary)", width: 22, flexShrink: 0 }}>{r}</span>
              <span style={{ fontSize: 11.5, color: "var(--mk-secondary)", lineHeight: 1.55 }}>{anchors[r]}</span>
            </div>
          ))}
        </div>
      ) : null}
      {d.id === "D6" && model?.reflectionRule ? (
        <div style={{ marginTop: 8, fontSize: 11, color: "var(--mk-muted)", fontStyle: "italic" }}>{model.reflectionRule}</div>
      ) : null}

      <p style={{ marginTop: 12, fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{d.summary}</p>
      <EvidenceList evidence={d.evidence} />
      {d.suggestion ? (
        <div style={{ marginTop: 12, background: "var(--mk-butter-bg)", borderRadius: 6, padding: "9px 11px", fontSize: 12, color: "var(--mk-ink)" }}>
          <b style={{ color: "var(--mk-butter-fg)" }}>建议：</b>{d.suggestion}
        </div>
      ) : null}
    </div>
  );
}

export function DepthSection({ dims }: { dims: DepthDimResult[] }) {
  return (
    <PrintSection
      n="05"
      title="认知深度 D · 想得有多深"
      breakBefore
      intro="认知深度看学生把思考推进到多深。每个维度给出 L1→L4 四级参照锚点——由浅到深的定性描述，不是 1–4 打分，也不与其它轴合成总分。学生所处的位置只用颜色深浅示意，本版本不标注具体档位。"
    >
      {dims.map((d) => <DepthDimBlock key={d.id} d={d} />)}
    </PrintSection>
  );
}
```

- [ ] **Step 2: Wire into `EvaluationReportPrint.tsx`**

```tsx
import { DepthSection } from "./axis";
// ...after MaterialsSection:
<DepthSection dims={report.depth} />
```

- [ ] **Step 3: Add test cases (the color-schema invariants)**

```tsx
import { DEPTH_RAMP } from "../EvaluationReport/tokens";

describe("EvaluationReportPrint — 认知深度 D", () => {
  it("shows all four L1–L4 anchors as reference, marks no current rung, prints no level number", () => {
    const { container } = render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("认知深度 D · 想得有多深")).toBeInTheDocument();
    // No "you-are-here" marker exists anywhere by construction.
    expect(container.querySelector("[data-current], [data-current-rung]")).toBeNull();
    const d0 = R.depth[0]!;
    // Color chip carries the position, using the ramp for the (never-printed) level.
    const chip = container.querySelector(`[data-color-chip="${d0.id}"]`) as HTMLElement | null;
    expect(chip).not.toBeNull();
    expect(chip!.style.background).toContain(DEPTH_RAMP[d0.level - 1]!.toLowerCase());
    // All four rubric anchors for D1 render.
    const { DepthDims } = await import("@mind-imprint/contracts");
    const model = DepthDims().find((x) => x.id === d0.id)!;
    for (const rung of ["L1", "L2", "L3", "L4"] as const) {
      expect(screen.getByText(model.anchors[rung])).toBeInTheDocument();
    }
  });
});
```

> Note: make the `it` callback `async` for the dynamic `import`, or hoist `import { DepthDims } from "@mind-imprint/contracts"` to the top of the test file. `HTMLElement.style.background` normalizes hex to lowercase; compare case-insensitively.

- [ ] **Step 4: Run tests + typecheck + commit**

```bash
git add apps/web/src/shell/report/print
git commit -m "feat(report-pdf): 认知深度 D — ladder reference by color, no asserted rung"
```

---

### Task 5: 智识自主 A — band-scale reference (color schema, no asserted band)

**Files:**
- Modify: `apps/web/src/shell/report/print/axis.tsx` (add `AutonomySection`)
- Modify: `apps/web/src/shell/report/print/EvaluationReportPrint.tsx`
- Modify: `apps/web/src/shell/report/print/document.test.tsx`

**Interfaces:**
- Consumes: `PrintSection`, `EvidenceList`; `AUTONOMY_RAMP` from `../EvaluationReport/tokens`; `AutonomySignals`, `DUALAXIS_MODEL` from `@mind-imprint/contracts`.
- Produces: `AutonomySection({ dims: AutonomyDimResult[] })`.

- [ ] **Step 1: Write `AutonomySection` in `axis.tsx`**

```tsx
import type { AutonomyDimResult } from "@mind-imprint/contracts";
import { AutonomySignals, DUALAXIS_MODEL } from "@mind-imprint/contracts";
import { AUTONOMY_RAMP } from "../EvaluationReport/tokens";

function BandStrip() {
  // Color-coded 0–5 scale as reference; NO cell is marked as the student's.
  return (
    <div aria-hidden style={{ display: "flex", borderRadius: 4, overflow: "hidden", width: 132 }}>
      {AUTONOMY_RAMP.map((color, i) => (
        <span key={i} style={{ height: 10, width: 22, background: color }} />
      ))}
    </div>
  );
}

function AutonomyDimBlock({ d }: { d: AutonomyDimResult }) {
  const model = AutonomySignals().find((x) => x.id === d.id);
  const chip = AUTONOMY_RAMP[d.band] ?? AUTONOMY_RAMP[AUTONOMY_RAMP.length - 1];
  return (
    <div className="print-avoid-break" style={{ marginBottom: 20, borderTop: "1px solid var(--mk-border)", paddingTop: 14 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
        <span
          data-color-chip={d.id}
          aria-hidden
          style={{ width: 16, height: 16, borderRadius: 4, background: chip, flexShrink: 0 }}
        />
        <span style={{ fontSize: 12, fontWeight: 800, color: "var(--mk-secondary)" }}>{d.id}</span>
        <span style={{ fontSize: 15, fontWeight: 700, color: "var(--mk-ink)" }}>{model?.name ?? d.id}</span>
      </div>
      {model?.means ? (
        <div style={{ marginTop: 3, fontSize: 11.5, color: "var(--mk-muted)" }}>{model.means}</div>
      ) : null}
      <div style={{ marginTop: 10, display: "flex", alignItems: "center", gap: 10 }}>
        <BandStrip />
        <span style={{ fontSize: 11, color: "var(--mk-muted)" }}>档 0 → 5（低 → 高）</span>
      </div>
      {model?.event ? (
        <div style={{ marginTop: 6, fontSize: 11.5, color: "var(--mk-secondary)", lineHeight: 1.55 }}>
          什么算一次可计事件：{model.event}
        </div>
      ) : null}

      <p style={{ marginTop: 12, fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{d.summary}</p>
      <EvidenceList evidence={d.evidence} />
      {d.suggestion ? (
        <div style={{ marginTop: 12, background: "var(--mk-butter-bg)", borderRadius: 6, padding: "9px 11px", fontSize: 12, color: "var(--mk-ink)" }}>
          <b style={{ color: "var(--mk-butter-fg)" }}>建议：</b>{d.suggestion}
        </div>
      ) : null}
    </div>
  );
}

export function AutonomySection({ dims }: { dims: AutonomyDimResult[] }) {
  return (
    <PrintSection
      n="06"
      title="智识自主 A · 是否自己驱动认知"
      breakBefore
      intro={`智识自主看学生是否自己驱动认知。${DUALAXIS_MODEL.autonomyBand} ${DUALAXIS_MODEL.opportunityRule} 学生所处的档位只用颜色深浅示意，本版本不标注具体档数。`}
    >
      {dims.map((d) => <AutonomyDimBlock key={d.id} d={d} />)}
    </PrintSection>
  );
}
```

> Note: the top-of-file imports (`PrintSection`, `EvidenceList`, `DepthDims`, `DEPTH_RAMP`) added in Task 4 remain; add the new imports (`AutonomySignals`, `DUALAXIS_MODEL`, `AUTONOMY_RAMP`, the `AutonomyDimResult` type).

- [ ] **Step 2: Wire into `EvaluationReportPrint.tsx`**

```tsx
import { AutonomySection, DepthSection } from "./axis";
// ...after DepthSection:
<AutonomySection dims={report.autonomy} />
```

- [ ] **Step 3: Add test cases**

```tsx
import { AUTONOMY_RAMP } from "../EvaluationReport/tokens";

describe("EvaluationReportPrint — 智识自主 A", () => {
  it("shows the band scale as reference, marks no cell, prints no band number", () => {
    const { container } = render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("智识自主 A · 是否自己驱动认知")).toBeInTheDocument();
    const a0 = R.autonomy[0]!;
    const chip = container.querySelector(`[data-color-chip="${a0.id}"]`) as HTMLElement | null;
    expect(chip).not.toBeNull();
    expect(chip!.style.background).toContain(AUTONOMY_RAMP[a0.band]!.toLowerCase());
    expect(container.querySelector("[data-current], [data-current-band]")).toBeNull();
  });
});
```

- [ ] **Step 4: Run tests + typecheck + commit**

```bash
git add apps/web/src/shell/report/print
git commit -m "feat(report-pdf): 智识自主 A — band-scale reference by color, no asserted band"
```

---

### Task 6: 提问透镜 + 工具卡与子代理 + 风险提示 (conditional)

**Files:**
- Modify: `apps/web/src/shell/report/print/sections.tsx` (add `PromptLensSection`, `ToolUsageSection`, `RisksSection`)
- Modify: `apps/web/src/shell/report/print/EvaluationReportPrint.tsx`
- Modify: `apps/web/src/shell/report/print/document.test.tsx`

**Interfaces:**
- Consumes: `PrintSection`, `PrintTable`; `DUALAXIS_MODEL` from `@mind-imprint/contracts`.
- Produces: `PromptLensSection({ promptLens })`, `ToolUsageSection({ toolUsage })`, `RisksSection({ risks })`.

- [ ] **Step 1: Add the three sections to `sections.tsx`**

```tsx
import type { PromptLens, RiskEntry, ToolUsageEntry } from "@mind-imprint/contracts";
import { DUALAXIS_MODEL } from "@mind-imprint/contracts";

const RISK_LABEL: Record<RiskEntry["type"], string> = {
  "ai-ghostwrite": "AI 代写", "missing-source": "缺来源", "argument-logic": "论证逻辑",
  "data-scope": "数据范围", "rabbit-hole-offtopic": "跑题兔子洞",
};

export function PromptLensSection({ promptLens }: { promptLens: PromptLens }) {
  const lensList = DUALAXIS_MODEL.lenses.map((l) => l.name).join("、");
  return (
    <PrintSection
      n="07"
      title="提问透镜"
      intro={`${DUALAXIS_MODEL.lensNote}它只读 AI 互动痕迹（如${lensList}），是给两轴补过程证据的旁证，不是第三根评分轴。`}
    >
      {promptLens.summary ? (
        <p style={{ fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7, margin: "0 0 12px" }}>{promptLens.summary}</p>
      ) : null}
      <PrintTable
        head={["阶段", "提问", "观察", "关联维度", "注意"]}
        rows={promptLens.prompts.map((p) => [
          p.stage, p.quote, p.observation, p.relatedDomains.join("、") || "—", p.attention ? "⚠" : "",
        ])}
      />
    </PrintSection>
  );
}

export function ToolUsageSection({ toolUsage }: { toolUsage: ToolUsageEntry[] }) {
  return (
    <PrintSection
      n="08"
      title="工具卡与子代理"
      intro="思维工具卡是把「思考」在对的时刻塞回给学生的交互卡片——触发是自动的，但打开由学生自己确认（我们不强迫、不操纵）。下表记录这个项目里被真正用到的工具卡与其用途。"
    >
      <PrintTable
        head={["工具", "阶段", "用途", "摘要"]}
        rows={toolUsage.map((t) => [t.name, t.stage, t.purpose, t.summary])}
      />
    </PrintSection>
  );
}

export function RisksSection({ risks }: { risks: RiskEntry[] }) {
  return (
    <PrintSection
      n="09"
      title="风险提示"
      intro="风险提示是过程中值得和学生聊一聊的信号，不是定论，也不影响任何评级。"
    >
      {risks.length === 0 ? (
        <p style={{ fontSize: 12.5, color: "var(--mk-secondary)", margin: 0 }}>本次评估未发现需要提示的风险行为。</p>
      ) : (
        <PrintTable
          head={["类型", "行为", "建议"]}
          rows={risks.map((r) => [RISK_LABEL[r.type] ?? r.type, r.behaviour, r.suggestion])}
        />
      )}
    </PrintSection>
  );
}
```

- [ ] **Step 2: Wire into `EvaluationReportPrint.tsx`** (after `AutonomySection`):

```tsx
import { MaterialsSection, PromptLensSection, RisksSection, SummarySection, TimelineSection, ToolUsageSection } from "./sections";
// ...
<PromptLensSection promptLens={report.promptLens} />
<ToolUsageSection toolUsage={report.toolUsage} />
<RisksSection risks={report.risks} />
```

- [ ] **Step 3: Add test cases (both risk branches)**

```tsx
describe("EvaluationReportPrint — lens, tools, risks", () => {
  it("renders lens + tool sections", () => {
    render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("提问透镜")).toBeInTheDocument();
    expect(screen.getByText("工具卡与子代理")).toBeInTheDocument();
  });

  it("shows the positive statement when risks are empty", () => {
    render(<EvaluationReportPrint report={{ ...R, risks: [] }} />);
    expect(screen.getByText("本次评估未发现需要提示的风险行为。")).toBeInTheDocument();
  });

  it("shows a risk table when risks are present", () => {
    const withRisk = {
      ...R,
      risks: [{ type: "missing-source" as const, behaviour: "引用了未溯源的网页", suggestion: "补一手出处" }],
    };
    render(<EvaluationReportPrint report={withRisk} />);
    expect(screen.getByText("引用了未溯源的网页")).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run the full print suite + typecheck**

From `apps/web`: `npx vitest run src/shell/report/print && npm run typecheck`. Expected: all print tests PASS; typecheck 0 errors. Also run the broader web suite once (`npx vitest run`) to confirm no regressions in the report/console areas.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/report/print
git commit -m "feat(report-pdf): 提问透镜 + 工具卡 + 风险提示 (conditional) — document complete"
```

---

## Self-Review Notes (for the executor)

- **Spec coverage:** cover (T1) · shared axiom (T2) · 综述 (T2) · timeline (T3) · materials + intro (T3) · D ladder-reference (T4) · A band-reference (T5) · lens + tools + risks intros, conditional empty-risk (T6) · both surfaces wired + print trigger (T1). 对标框架 appendix is a spec non-goal (deferred) — intentionally absent.
- **Color-schema invariant** (`level`/`band` never printed, no current-rung marker) is enforced by the structural test in T4/T5 (`[data-current*]` is null; only a `[data-color-chip]` carries position). Keep it: do not add a "current" marker or a printed digit in any later change.
- **Type/name consistency:** contract helpers used verbatim — `DepthDims()`, `AutonomySignals()`, `DUALAXIS_MODEL.{axiom,autonomyBand,opportunityRule,lenses,lensNote}`; ramps `DEPTH_RAMP` (len 4) / `AUTONOMY_RAMP` (len 6). `EventEntry.kind` and `RiskEntry.type` enums are mapped to Chinese labels locally (mirrors `EVENT_KIND`/`RISK_LABEL` in the on-screen `tokens.ts`).
- **Fixture dependency:** tests read `MOCK_EVALUATION_REPORT`; if a field a test asserts is absent/empty in the fixture, guard the assertion (as T3 does) rather than hardcoding fixture values.
- **Verify each `var(--mk-*)` token exists** (e.g. `--mk-butter-bg`, `--mk-butter-fg`, `--mk-lake`, `--mk-accent-600`) — they are used by the on-screen report/console; if any is missing, substitute the nearest existing token rather than inventing one.
