import { useState } from "react";
import { createRoot } from "react-dom/client";
import { MindMap } from "../../src/writings/MindMap";
import { moveOutlineNode, type OutlineMoveMode } from "../../src/writings/outlineMove";
import { rekindChoices, rekindOutlineNode } from "../../src/writings/outlineRekind";
import { outlineGenreOf, type OutlineKind } from "../../src/writings/outlineKind";
import type { WritingOutlineItem } from "../../src/api/writingRoom";
import "../../src/index.css";

import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import type { CSSProperties } from "react";

// 🚨 看图台得自己把 lite 的配色装上，否则它画出来的每一屏都是红的。
//
// `--mk-accent-*` 的**基础值**是 pro 那边的珊瑚红（apps/web/src/index.css）。
// lite 的青色**不在 CSS 里** —— 它由 `AccentProvider` 把
// `--mk-theme-accent-*` 作为**内联样式**写在 `.lite-student` 那个 div 上
// （LiteApp.tsx / GuestTheme.tsx）。所以光加一个 class 不够，连 import 那份
// 主题 CSS 也不够：那份 CSS 写的是 `--mk-accent-500: var(--mk-theme-accent-500)`，
// 而那个变量要靠 React 那一层给。
//
// 2026-09-22 我差点照着红色的截图去查「配色是不是被改了」。
// **看图台自己脏了，产品没事**（[[observation-tool-is-the-bug-2026-09-12]]）。
const LITE_ACCENT_STYLE = Object.fromEntries(
  Object.entries(LITE_ACCENT_PRESETS[0]!.scale).map(([step, value]) => [`--mk-theme-accent-${step}`, value]),
) as CSSProperties;


/**
 * 一次性的看图台 —— 只为了**在真浏览器里**看那张思维导图。
 *
 * 🚨 为什么需要它：jsdom 没有真的 `setPointerCapture`，没有
 * `document.elementFromPoint`，也量不出 `getBoundingClientRect`。
 * 而拖动这件事全部建立在这三样上面 —— 「卡片上三分之一算放到旁边」这条判据
 * 是**几何**，单元测试一个字都证明不了它。
 * 这正是 AGENTS.md 说的那条：UI 用真浏览器看，不要靠 jsdom 断言。
 *
 * 它不连后端、不需要账号，所以跑一次是几秒钟的事。服务端那一半由 Go 测试和
 * `e2e/mindmap-drag.spec.ts`（线上）各自守着。
 *
 * 用法见 `mindmap-harness.spec.ts`。
 */

const SEED: WritingOutlineItem[] = [
  { id: "t", text: "校服不太好看，但早上省心，我还是愿意穿", role: "", kind: "thesis", depth: 0, position: 0 },
  { id: "p1", text: "早上不用想穿什么，省心", role: "", kind: "point", depth: 1, position: 1 },
  { id: "e1", text: "我早上经常起晚，穿校服不用挑衣服", role: "", kind: "evidence", depth: 2, position: 2 },
  { id: "p2", text: "校服不好看，想穿自己的衣服", role: "", kind: "point", depth: 1, position: 3 },
  // 🚨 同事 2026-09-20 截图里那一条：结尾被挂在了深度 1。
  // 它必须显示成「结尾」，不是「分论点 3」。
  { id: "c", text: "结尾回到「方便比好看值」，说得比开头更准", role: "", kind: "closing", depth: 1, position: 4 },
];

function Harness() {
  const [items, setItems] = useState<WritingOutlineItem[]>(SEED);
  const [log, setLog] = useState<string[]>([]);

  function onMove(draggedId: string, targetId: string, mode: OutlineMoveMode) {
    const next = moveOutlineNode(items, draggedId, targetId, mode);
    setLog((l) => [...l, `${draggedId} → ${targetId || "(空白)"} [${mode}] ${next ? "ok" : "拒绝"}`]);
    if (next) setItems(next);
  }

  /** 她自己改一条「是什么」—— 同事 2026-09-22 的意见 3。 */
  function onRekind(id: string, kind: OutlineKind) {
    const res = rekindOutlineNode(items, id, kind);
    setLog((l) => [...l, `${id} → ${kind} ${res.ok ? "ok" : "拒绝：" + res.why}`]);
    if (res.ok) setItems(res.items);
  }

  return (
    <div
      className="lite-student"
      style={{ ...LITE_ACCENT_STYLE, display: "flex", height: "100vh", background: "var(--mk-paper)" }}
    >
      <div style={{ flex: 1, minWidth: 0 }}>
        <MindMap
          items={items}
          justAdded={[]}
          onMove={onMove}
          onRemove={() => {}}
          onEdit={() => {}}
          onRekind={onRekind}
          kindChoices={rekindChoices(outlineGenreOf(items))}
        />
      </div>
      {/* 摊平之后的样子，给断言读 —— 屏幕上对了而数据错了是最坏的一种。 */}
      <pre
        data-testid="shape"
        style={{ width: 340, margin: 0, padding: 12, fontSize: 12, overflow: "auto", background: "var(--mk-surface)" }}
      >
        {items
          .slice()
          .sort((a, b) => a.position - b.position)
          .map((r) => `${"  ".repeat(r.depth)}${r.kind}: ${r.text.slice(0, 14)}`)
          .join("\n")}
        {"\n\n--- 拖动记录 ---\n"}
        {log.join("\n")}
      </pre>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
