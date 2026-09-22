import { useState } from "react";
import { createRoot } from "react-dom/client";
import { FlowStage } from "../../src/writings/FlowStage";
import { GuideBox } from "../../src/writings/GuideBox";
import { MiniMap } from "../../src/writings/MiniMap";
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

import type { Writing } from "../../src/api/writings";

/**
 * 一次性的看图台之二 —— R3 那三块新屏，在**真浏览器**里看一眼。
 *
 * 行文那块板（意见 4）、段落页左栏顶上那张缩略图（意见 5 的 UI）、
 * 引导框里的「这一段里的几步」（意见 5 的语言）。
 *
 * 它假接口：`fetch` 被替换成一个只认 `/flow/structures` 和 `/flow` 的桩，
 * 所以不连后端、不要账号。服务端那一半由 Go 测试守着。
 */

const SEED: WritingOutlineItem[] = [
  { id: "t", text: "学校应该允许学生带手机", role: "", kind: "thesis", depth: 0, position: 0 },
  { id: "p1", text: "放学能联系家长", role: "", kind: "point", depth: 1, position: 1, method: "point_pee" },
  { id: "e1", text: "上周三五点半那次", role: "", kind: "evidence", depth: 2, position: 2 },
  { id: "p2", text: "学习上能查不会的题", role: "", kind: "point", depth: 1, position: 3 },
  { id: "c", text: "允许带，但老师同意才能拿出来", role: "", kind: "closing", depth: 0, position: 4 },
];

const STRUCTURES = [
  { id: "struct_total_part", name: "总分式", definition: "先用一段把主张说清楚，后面每一段各撑住它的一面。", example: "开头一段说清「读书要读慢」；中间三段分别讲……" },
  { id: "struct_parallel", name: "并列式", definition: "几条分论点地位相同，从不同角度说同一件事。", example: "三段分别从时间、注意力、记忆三个角度说明慢读的好处。" },
  { id: "struct_progressive", name: "层进式", definition: "后一条建立在前一条上：是什么 → 为什么 → 怎么办。", example: "第一段说什么叫慢读，第二段说它为什么有用，第三段说怎么挤出这半小时。" },
  { id: "struct_contrast", name: "对照式", definition: "把正反两面摆在一起，差别本身就是论证。", example: "先用一大段写慢读的人两个月后还记得书里的话；再用短短一段写刷着读的人……" },
];

const METHODS = [
  { id: "point_pee", name: "举例论证", definition: "先说看法，再举一件具体的事，最后说清它为什么能说明那个看法。" },
  { id: "point_quote", name: "引用论证", definition: "引一句名言、一条权威数据来撑住你的看法。" },
  { id: "point_contrast", name: "对比论证", definition: "把两种情况放在一起比，差别本身就是论证。" },
  { id: "point_analogy", name: "类比论证", definition: "两件事在关键的那一点上相同，于是在结论上也该相同。" },
];

// 假接口：只认这一屏用到的那两条路。
const realFetch = window.fetch;
window.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  if (url.includes("/flow/structures")) {
    return new Response(JSON.stringify({ structures: STRUCTURES, methods: METHODS }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }
  if (url.includes("/flow") || url.includes("/outline")) {
    const body = JSON.parse(String(init?.body ?? "{}"));
    (window as unknown as { lastPut?: unknown }).lastPut = body;
    return new Response(JSON.stringify({ outline: SEED }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }
  return realFetch(input, init);
}) as typeof window.fetch;

/** harness 用的最小 writing —— 只有 PromptSidebar 读得到的那几个字段是真的。 */
const HARNESS_WRITING = {
  id: "w1",
  assignedPrompt:
    "阅读下面的材料，根据要求写作。词语是表达思想情感的载体，也是展现社会生活变化的窗口。在你的成长过程中，你对哪一个词语的理解发生了变化？请结合自身经历，写一篇文章。",
  targetWords: 800,
} as unknown as Writing;

function Harness() {
  const [outline, setOutline] = useState(SEED);
  const [structureKey, setStructureKey] = useState("");
  const [screen, setScreen] = useState<"flow" | "snippets">("flow");

  return (
    <div
      className="lite-student"
      style={{
        ...LITE_ACCENT_STYLE,
        height: "100vh",
        background: "var(--mk-paper)",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <div style={{ display: "flex", gap: 8, padding: 8, borderBottom: "1px solid var(--mk-border)" }}>
        <button data-testid="go-flow" onClick={() => setScreen("flow")}>
          行文
        </button>
        <button data-testid="go-snippets" onClick={() => setScreen("snippets")}>
          段落（左栏）
        </button>
        <span data-testid="structure">{structureKey || "(还没选)"}</span>
      </div>

      <div style={{ flex: 1, minHeight: 0 }}>
        {screen === "flow" ? (
          <FlowStage
            // 题目那一栏要一份 writing；harness 里给一个带题目的最小对象，
            // 这样左边那一栏也在这一屏上看得见。
            writing={HARNESS_WRITING}
            writingId="w1"
            outline={outline}
            structureKey={structureKey}
            onOutline={setOutline}
            onStructureKey={setStructureKey}
            onDone={() => {}}
            onBack={() => {}}
            onLocked={() => {}}
          />
        ) : (
          // 段落页的左栏：图在上、引导在下 —— 同事截图里那个箭头指的就是这个顺序。
          <aside
            style={{
              width: 320,
              height: "100%",
              overflowY: "auto",
              display: "flex",
              flexDirection: "column",
              gap: 16,
              padding: 16,
              background: "var(--mk-surface)",
              borderRight: "1px solid var(--mk-border)",
            }}
          >
            <h2 style={{ fontWeight: 600 }}>引导</h2>
            <MiniMap outline={outline} />
            <GuideBox
              guide={{
                job: "把你最强的那条理由展开成一段。",
                methods: [],
                questions: ["那天几点？在哪儿等的车？", "这件事凭什么说明学校该允许带手机？"],
              }}
              kind="point"
              onDismiss={() => {}}
              onDeepen={() => {}}
            />
          </aside>
        )}
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
