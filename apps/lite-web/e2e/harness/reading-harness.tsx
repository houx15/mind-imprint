import { useState } from "react";
import { createRoot } from "react-dom/client";
import { AccentProvider } from "@/ui/accent";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import { ReadingPlanDial } from "../../src/readings/ReadingPlanDial";
import { SelectionTools } from "../../src/readings/SelectionTools";
import { ReadingHarvest } from "../../src/readings/ReadingHarvest";
import type { LiteAnnotation, ReadingBlockTool, ReadingTask } from "../../src/api/readingRoom";
import "../../src/index.css";
import "@/ui/themes/lite.css";
// Mirror the real shell's portal scope as well as its accent provider.
document.body.classList.add("lite-student-theme");
for (const [step, value] of Object.entries(LITE_ACCENT_PRESETS[0]!.scale)) {
  document.body.style.setProperty(`--mk-theme-accent-${step}`, value);
}
const anchor = new URLSearchParams(location.search);
// 🚨 展开视图那一块的规则住在这里，而它是 `LiteApp.tsx` 导入的，不是
// `index.css`。第一版看图台只导了后者，于是那一块画出来**完全没有样式**
// （徽章没了、圆点竖着排），而五条断言全绿 —— 正是 AGENTS.md 那条：
// 看一眼图，别只看断言。
import "../../src/learning/student-surfaces.css";

/**
 * 一次性的看图台 —— 2026-09-22 那六条改动里，**看得见**的那四处。
 *
 * 🚨 为什么不是 jsdom：这四处坏掉的方式全是布局上的。盘搬进页签行之后是不是
 * 还在那一行里、浮层会不会掉出屏幕、划选工具条上多了两颗按钮之后会不会折行、
 * 展开视图那一排圆点在十五步的时候会不会撑破 —— 一条 `getByText` 全部照常
 * 通过。AGENTS.md 那条：UI 用真浏览器看。
 *
 * 🚨 `AccentProvider` 一定要装。lite 的青色是**内联**给的，不在 CSS 里；
 * 不装的话每一屏都画成 pro 的珊瑚红，而那会让人照着一张错颜色的图去查配色
 * （[[control-that-is-not-wired-2026-09-22]] 里栽过）。
 *
 * 用法见 `reading-harness.spec.ts`。
 */

/** 十五步，和产品负责人那张截图一样长 —— 那排圆点撑不撑得住要看长的那一份。 */
const PLAN: ReadingTask[] = Array.from({ length: 15 }, (_, i) =>
  ({
    id: `t${i + 1}`,
    position: i + 1,
    kind: "read",
    label:
      i === 3
        ? "通读第10–11段·例外与转折"
        : `通读第${i * 2 + 1}–${i * 2 + 2}段·这一部分在做什么`,
    detail: i === 3 ? "读这两段，看作者在哪一句上承认了例外。" : "",
    blockId: i === 3 ? "b10" : "",
    status: i < 3 ? "done" : "pending",
    completedAt: null,
  }) as ReadingTask,
);

const TOOLS: ReadingBlockTool[] = [
  { id: "lookup", label: "查词", subject: "word" } as ReadingBlockTool,
  { id: "grammar", label: "语法", subject: "sentence" } as ReadingBlockTool,
];

const EXCERPTS: LiteAnnotation[] = [
  {
    id: "a1",
    blockId: "b3",
    span: { start: 0, end: 52 },
    quote: "Some professors fear that ChatGPT could lead to cheating.",
    note: "",
    createdAt: "2026-09-22T01:00:00Z",
  },
  {
    id: "a2",
    blockId: "b10",
    span: { start: 4, end: 40 },
    quote: "Not all people are like this all the time.",
    note: "",
    createdAt: "2026-09-22T01:04:00Z",
  },
];

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={{ padding: 16, borderBottom: "1px solid var(--mk-border)" }}>
      <h2 style={{ margin: "0 0 10px", fontSize: 13, color: "var(--mk-muted)" }}>{title}</h2>
      {children}
    </section>
  );
}

function Harness() {
  const [log, setLog] = useState<string[]>([]);
  const note = (s: string) => setLog((l) => [...l, s]);

  return (
    <AccentProvider initialAccent="teal" presets={LITE_ACCENT_PRESETS}>
      <div className="lite-student" style={{ background: "var(--mk-paper)", minHeight: "100vh" }}>
        {/* 页签行 + 盘，和阅读室里那一行同一个结构。 */}
        <Panel title="① 右栏页签行上的盘（折起来）。悬停给清单，点击就地摊开展开视图。">
          <div
            data-testid="coachcol"
            className="mk-reading-room__coach"
            style={{ maxWidth: 560, background: "var(--mk-surface)", padding: 12, borderRadius: 12 }}
          >
            <div className="mk-lite-coachtabs" role="tablist" aria-label="右栏视图">
              <button type="button" role="tab" aria-selected className="is-active">
                印记
              </button>
              <button type="button" role="tab" aria-selected={false}>
                阅读成果
                <span className="mk-lite-coachtabs__count">6</span>
              </button>
              <div className="mk-lite-coachtabs__spacer" />
              <ReadingPlanDial tasks={PLAN} onLocate={(b) => note(`定位原文 → ${b}`)} />
            </div>
            {/* 印记 说话的地方 —— 这一块以前被步骤条压到只剩一条缝。 */}
            <div style={{ padding: "14px 4px", fontSize: 14, lineHeight: 1.9 }}>
              第10–11段里，哪一句在承认例外？请在文章里划选那一句。
              <button type="button" className="mk-jumpblock" onClick={() => note("跳到第 10 段")}>
                跳到第 10 段
              </button>
            </div>
          </div>
        </Panel>

        <Panel title="② 划选之后那条工具条。摘抄 / 放入对话框 一直在；查词、语法看她划的是词还是句。">
          <div style={{ position: "relative", height: 120 }}>
            <SelectionTools
              quote="Not all people are like this all the time."
              at={{ x: Number(anchor.get("x") ?? 320), y: Number(anchor.get("y") ?? 40) }}
              tools={TOOLS}
              excerpted={false}
              excerptable
              onPick={(t) => note(`工具 ${t}`)}
              onExcerpt={() => note("摘抄")}
              onSendToCoach={() => note("放入对话框")}
              onDismiss={() => note("关闭")}
            />
          </div>
        </Panel>

        <Panel title="③ 同一句已经摘过了：那颗按钮变成「已摘抄」，按不动。">
          <div style={{ position: "relative", height: 120 }}>
            <SelectionTools
              quote="fear"
              at={{ x: 320, y: 300 }}
              tools={TOOLS}
              excerpted
              excerptable
              onPick={() => {}}
              onExcerpt={() => note("不该被按到")}
              onSendToCoach={() => note("放入对话框")}
              onDismiss={() => {}}
            />
          </div>
        </Panel>

        <Panel title="④ 只有摘要的那一篇：摘抄整颗不出现，别的照常。">
          <div style={{ position: "relative", height: 120 }}>
            <SelectionTools
              quote="Not all people are like this all the time."
              at={{ x: 900, y: 40 }}
              tools={TOOLS}
              excerpted={false}
              excerptable={false}
              onPick={(t) => note(`工具 ${t}`)}
              onExcerpt={() => note("不该被按到")}
              onSendToCoach={() => note("放入对话框")}
              onDismiss={() => {}}
            />
          </div>
        </Panel>

        <Panel title="⑤ 阅读成果那一页上摘抄的那一节。每一条能回原文，也能交给印记。">
          <div style={{ maxWidth: 520 }}>
            <ReadingHarvest
              notes={[]}
              messages={[]}
              outcomes={[]}
              excerpts={EXCERPTS}
              ordinalOf={(b) => (b === "b3" ? 3 : 10)}
              onLocate={(b) => note(`回到原文 ${b}`)}
              onDiscuss={(b, q) => note(`和印记说 ${b}：${q.slice(0, 12)}…`)}
            />
          </div>
        </Panel>

        <pre data-testid="log" style={{ margin: 0, padding: 12, fontSize: 12 }}>
          {log.join("\n")}
        </pre>
      </div>
    </AccentProvider>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
