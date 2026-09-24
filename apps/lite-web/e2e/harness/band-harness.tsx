import { createRoot } from "react-dom/client";
import { AccentProvider } from "@/ui/accent";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import { CommentPanel } from "../../src/writings/CommentPanel";
import type { Comment } from "../../src/api/writingRoom";
import "../../src/index.css";
import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";

document.body.classList.add("lite-student-theme");
for (const [step, value] of Object.entries(LITE_ACCENT_PRESETS[0]!.scale)) {
  document.body.style.setProperty(`--mk-theme-accent-${step}`, value);
}

/**
 * 「当前档位」的看图台。
 *
 * 产品负责人 2026-09-25 要档位给学生看。这一块全部是版式问题：
 * 数字要大得一眼看见，但**旁边那句话和底下那行小字不能被它压掉** ——
 * 一个孤零零的「第 3 档」会被读成考试分数，而它说的是「你现在卡在结构」。
 * 一条 getByText 这三件事一件都分不出来，所以在真浏览器里看
 *（AGENTS.md：UI 用真浏览器看）。
 *
 * 摆四屏：卡在立意（最低档）、只剩字句（高档）、四层都干净（满档），
 * 以及**单段那一条**——它不该有档位。
 */

const BASE: Comment = {
  id: "c1",
  scope: "draft",
  verdict: "polish",
  snippetId: null,
  summary: "你写了校园浪费这件事，给了一个自己数过的数字。",
  points: [
    {
      kind: "issue",
      layer: 3,
      symptom: "structure_order",
      quote: "这很浪费。这不应该。",
      text: "这两句说的是同一件事，中间没有往前推进。",
      action: "请把其中一句换成「浪费到什么程度」的具体交代。",
    },
  ],
  createdAt: "2026-09-25T02:00:00Z",
  sourceText: "",
  layer_verdicts: { "1": "pass", "2": "pass", "3": "polish", "4": "unchecked" },
} as unknown as Comment;

function band(n: number, label: string, layers: Record<string, string>): Comment {
  return {
    ...BASE,
    band: n,
    bandLabel: label,
    bandNote: "档位按「现在卡在哪一层」算：立意 → 材料 → 结构 → 字句，越靠前越要紧。它不是分数。",
    layer_verdicts: layers,
  } as unknown as Comment;
}

function Case({ title, comment }: { title: string; comment: Comment }) {
  return (
    <section style={{ marginBottom: 28 }} data-case={title}>
      <p style={{ fontSize: 12, color: "var(--mk-muted)", margin: "0 0 6px" }}>{title}</p>
      <div className="lite-student" style={{ maxWidth: 520 }}>
        <div className="flex flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-4">
          <CommentPanel comment={comment} onTrace={() => {}} />
        </div>
      </div>
    </section>
  );
}

function Stage() {
  return (
    <div style={{ padding: 26, maxWidth: 620, margin: "0 auto" }}>
      <Case
        title="卡在立意 —— 最低档"
        comment={band(1, "先把观点定下来", { "1": "revise", "2": "unchecked", "3": "unchecked", "4": "unchecked" })}
      />
      <Case
        title="只剩字句 —— 内容和结构已经立住"
        comment={band(4, "内容和结构已经立住，剩下字句", { "1": "pass", "2": "pass", "3": "pass", "4": "polish" })}
      />
      <Case
        title="四层都干净 —— 满档"
        comment={{
          ...band(5, "四层都没有挑出问题", { "1": "pass", "2": "pass", "3": "pass", "4": "pass" }),
          verdict: "pass",
          points: [],
        } as unknown as Comment}
      />
      <Case
        title="对照：单段那一条没有档位（五档是给一整篇用的尺子）"
        comment={{ ...BASE, scope: "block" } as unknown as Comment}
      />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <AccentProvider initialAccent="teal" presets={LITE_ACCENT_PRESETS}>
    <div className="lite-student" style={{ background: "var(--mk-paper)", minHeight: "100vh" }}>
      <Stage />
    </div>
  </AccentProvider>,
);
