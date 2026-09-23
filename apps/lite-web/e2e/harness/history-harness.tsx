import { useState } from "react";
import { createRoot } from "react-dom/client";
import { ReadingHistoryPanel } from "../../src/readings/ReadingHistoryPanel";
import type { Reading } from "../../src/api/readings";
import { useLiteTheme } from "../../src/shared/useLiteTheme";
import { AccentProvider } from "@/ui";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import "../../src/index.css";
import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";

/**
 * 「我的阅读」那张列表的看图台 —— 专为**把一篇收起来**那颗按钮存在。
 *
 * 产品负责人 2026-09-23 第 3 条：「阅读列表里旧的也没办法删除。」
 *
 * 🚨 为什么必须在真浏览器里按一次：memory
 * `control-that-is-not-wired-2026-09-22` —— 那一轮一屏交出三个假控件，其中
 * 一个的 click 被外层的 onPointerDown 吞掉，**只有真浏览器抓得到**。
 * 这次的形状正是同一类：整行原来是一个 `<button>`，收起来那颗要摆在它旁边。
 * 按钮套按钮既不合法、里面那一下也会被外面那一下吃掉 —— 所以外层换成了 div。
 * 这台看图台要证的就是：按收起来**不会顺手把那一行点开**。
 *
 * 🚨 主题照 LiteApp 的真做法装（AccentProvider + useLiteTheme）。
 */

function reading(over: Partial<Reading> = {}): Reading {
  return {
    id: "r1",
    title: "一件小事",
    lang: "zh",
    status: "active",
    hasSource: true,
    createdAt: "2026-09-20T02:00:00Z",
    updatedAt: "2026-09-22T02:00:00Z",
    lastActivityAt: "2026-09-22T02:00:00Z",
    finishedAt: null,
    ...over,
  } as Reading;
}

const SEED: Reading[] = [
  reading({ id: "r1", title: "一件小事" }),
  reading({ id: "r2", title: "粘错了的那一篇（分段全乱）" }),
  reading({
    id: "r3",
    title: "读完的那一篇",
    status: "finished",
    finishedAt: "2026-09-21T02:00:00Z",
  }),
];

function Stage() {
  const { themeStyle } = useLiteTheme();
  const [rows, setRows] = useState<Reading[]>(SEED);
  // 「她点开了哪一行」—— 这一位就是那条判据：按收起来时它必须不动。
  const [opened, setOpened] = useState<string>("");

  return (
    <div style={{ ...themeStyle, padding: 24, background: "var(--mk-paper)", minHeight: "100vh" }}>
      <p data-opened style={{ fontSize: 12 }}>{opened}</p>
      <ReadingHistoryPanel
        open
        onClose={() => {}}
        readings={rows}
        error={null}
        onSelect={(r) => setOpened(r.id)}
        onArchive={(r) => {
          setRows((cur) => cur.filter((x) => x.id !== r.id));
        }}
      />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <AccentProvider presets={LITE_ACCENT_PRESETS} initialAccent={LITE_ACCENT_PRESETS[0]!.id}>
    <Stage />
  </AccentProvider>,
);
