import { useState } from "react";
import { createRoot } from "react-dom/client";
import { PlanningView } from "../../src/writings/PlanningView";
import type { WritingOutlineItem } from "../../src/api/writingRoom";
import type { LiteMessage } from "../../src/api/readingRoom";
import type { Writing } from "../../src/api/writings";
import "../../src/index.css";

import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import type { CSSProperties } from "react";

// 见 flow-harness.tsx 里那段：lite 的青色由 React 那一层作为内联样式给，
// 光 import 一份 CSS 不够。不装上，看图台把每一屏都画成 pro 的珊瑚红。
const LITE_ACCENT_STYLE = Object.fromEntries(
  Object.entries(LITE_ACCENT_PRESETS[0]!.scale).map(([step, value]) => [`--mk-theme-accent-${step}`, value]),
) as CSSProperties;

/**
 * 构思那一屏的看图台 —— 同事 2026-09-22 的意见 1 那一屏。
 *
 *	「选择题目进入写作后，无法看到完整的题目。想要看完整的题目还需要退出重新
 *	  搜索，可能不利于学生**边看题目边构思**。」
 *
 * 2026-09-21 加的那一栏题目（PromptSidebar）只挂在段落和成稿两步上 ——
 * 构思和行文各自在房间外壳之前 early-return，于是它们还挂着那行
 * line-clamp-2 的小字。而「边看题目边构思」说的正是这一屏。
 *
 * 这个看图台只为一件事存在：**在真浏览器里看那一栏和对话、图三块同时在屏幕
 * 上的样子**。三栏挤不挤、题目折起来之后正文有没有变宽、那张图还剩多少地方
 * —— 这些是几何，jsdom 一个字都证明不了。
 *
 * 它假接口（照 flow-harness 的做法），所以不连后端、不要账号。
 */

const PROMPT =
  "阅读下面的材料，根据要求写作。\n" +
  "词语是表达思想情感的载体，也是展现社会生活变化的窗口。当前，世界之变、时代之变、" +
  "历史之变正以前所未有的方式展开。青年是常为新的，在你的成长过程中，你对哪一个词语的" +
  "理解发生了变化？这变化和你的成长有什么关系？\n" +
  "请结合自身经历，写一篇文章。要求：选准角度，确定立意，明确文体，自拟标题；" +
  "不要套作，不得抄袭；不少于800字。";

const WRITING = {
  id: "w1",
  title: "成功这个词",
  lang: "zh",
  stage: "outline",
  targetWords: 800,
  structureKey: "",
  setupAt: "2026-09-22T00:00:00Z",
  origin: "here",
  assignedPrompt: PROMPT,
  status: "active",
  createdAt: "2026-09-22T00:00:00Z",
  updatedAt: "2026-09-22T00:00:00Z",
  lastActivityAt: "2026-09-22T00:00:00Z",
  finishedAt: null,
  revisingAt: null,
} as unknown as Writing;

// 同事截图里那张图，照着摆 —— 包括被摆错的那一条（意见 3）。
const SEED: WritingOutlineItem[] = [
  { id: "t", text: "我想写「成功」这个词，我对它的理解发生了变化", role: "", kind: "thesis", depth: 0, position: 0 },
  { id: "p1", text: "成功的定义太窄，成功的人就太少", role: "", kind: "point", depth: 1, position: 1 },
  { id: "p2", text: "人人有自己的贡献，平凡尽责也是成功", role: "", kind: "point", depth: 1, position: 2 },
  { id: "r1", text: "外卖员辛勤付出让人按时吃上饭，是成功", role: "", kind: "reference", depth: 2, position: 3 },
  { id: "r2", text: "黑心商家哪怕赚很多钱，也是失败", role: "", kind: "reference", depth: 2, position: 4 },
];

const MESSAGES: LiteMessage[] = [
  {
    seq: 1,
    role: "ai",
    content:
      "老师布置的这篇文章，是让你从成长经历里选一个词，讲讲你对它的理解怎么变了。我们先一起把要说的想清楚、排好顺序，再动笔。你最先想到的是哪个词？",
    createdAt: "",
  },
  { seq: 2, role: "student", content: "我想写「成功」。小时候觉得成功就是考第一。", createdAt: "" },
  {
    seq: 3,
    role: "ai",
    content: "好，那这篇要让读者相信的是什么？用一句话说说你现在怎么看「成功」。",
    createdAt: "",
  },
];

// 假接口：这一屏只会打开场和回合两条路，以及保存提纲。
const realFetch = window.fetch;
window.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  if (url.includes("/opening")) {
    return new Response(JSON.stringify({ reply: "" }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }
  if (url.includes("/outline")) {
    const body = JSON.parse(String(init?.body ?? "{}"));
    (window as unknown as { lastPut?: unknown }).lastPut = body;
    // 回一份按 kind 重算过深度的清单，和服务端一样。
    const depths: Record<string, number> = {
      thesis: 0, opening: 0, closing: 0,
      point: 1, counter: 1, scene: 1, turn: 1, feeling: 1,
      evidence: 2, reference: 2, reasoning: 2, rebuttal: 2, gap: 2, detail: 2,
    };
    const outline = (body.outline as { text: string; kind: string; source?: string }[]).map((n, i) => ({
      id: `n${i}`,
      text: n.text,
      role: "",
      kind: n.kind,
      depth: depths[n.kind] ?? 1,
      position: i,
      source: n.source ?? "",
    }));
    return new Response(JSON.stringify({ outline }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }
  return realFetch(input, init);
}) as typeof window.fetch;

function Harness() {
  const [outline, setOutline] = useState(SEED);
  const [messages, setMessages] = useState(MESSAGES);

  return (
    <div className="lite-student" style={{ ...LITE_ACCENT_STYLE, height: "100vh", background: "var(--mk-paper)" }}>
      <PlanningView
        writing={WRITING}
        messages={messages}
        outline={outline}
        onMessages={setMessages}
        onOutline={setOutline}
        onDone={() => {}}
        onBack={() => {}}
      />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
