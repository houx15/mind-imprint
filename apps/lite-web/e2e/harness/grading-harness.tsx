import { useReducer, useState } from "react";
import { createRoot } from "react-dom/client";
import { GradingCard } from "../../src/teacher/GradingCard";
import { gradingContentReducer, type GradingMode } from "../../src/teacher/gradingLogic";
import type { GradingContent, Rubric } from "../../src/api/gradings";
import { useLiteTheme } from "../../src/shared/useLiteTheme";
import { AccentProvider } from "@/ui";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import "../../src/index.css";
import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";

/**
 * 批改卡的看图台 —— 专为**「依据」那个弹层**存在。
 *
 * 产品负责人点名要的就是它：「and we also need to tell teacher the rationale
 * or the real logic of our comment there (maybe a modal?)」。
 *
 * 🚨 为什么要真浏览器：memory `control-that-is-not-wired-2026-09-22` ——
 * 一个按钮长得像能用，不等于它接线了。那一轮一屏交出三个假控件，其中一个
 * 的 click 被外层的 onPointerDown 吞掉，**只有真浏览器抓得到**。
 *
 * 🚨 主题照 LiteApp 的真做法装，不自己拼一份。
 * 第一版我手搓了一份 `--mk-theme-accent-*` 内联样式挂在外层 div 上，
 * 结果整屏画成 pro 的珊瑚红 —— 因为真正起作用的是
 * `AccentProvider` + `useLiteTheme()`（后者还往 body 上加
 * `lite-student-theme`，弹层那条 `[role="dialog"]` 的规则就挂在它下面）。
 * 看图台自己脏了会被当成产品的配色出了问题
 * （[[observation-tool-is-the-bug-2026-09-12]]），所以这里用真的那一套。
 *
 * 三条意见故意摆成三种来源状态，因为按钮是**有条件**渲染的
 * （gradingLogic.pointHasBasis）：
 *   1. 维度 + 毛病 + 原句都有 —— 弹层三行齐
 *   2. 只有维度（毛病没匹配上，被 SanitizeProvenance 清掉）—— 弹层两行
 *   3. 两样都没有（老师自己写的一条）—— **不该有按钮**
 */

const RUBRIC: Rubric = {
  scale: "letter",
  focus: "重点看论证",
  dimensions: [
    { name: "内容", note: "观点与材料" },
    { name: "结构", note: "段落与顺序" },
    { name: "语言", note: "用词与句子" },
    { name: "书写规范", note: "标点与格式" },
  ],
};

const CONTENT: GradingContent = {
  overall: {
    grade: "B+",
    comment: "你把「坚持」这个观点立住了，两个例子也都写了具体的时间和地点。现在差的是把例子和观点之间那一步讲出来。",
  },
  dimensions: [
    { name: "内容", grade: "B+", comment: "例子具体，但和观点的关系还没说明。" },
    { name: "结构", grade: "A-", comment: "三段各有分工，顺序清楚。" },
    { name: "语言", grade: "B", comment: "有两处长句可以断开。" },
    { name: "书写规范", grade: "A", comment: "标点和格式都规范。" },
  ],
  points: [
    {
      kind: "issue",
      quote: "王羲之练字把池水都染黑了，后来他的字被称为天下第一行书。",
      text: "例子摆到这里就结束了：读者看到了他很刻苦、字也很好，但中间那一步——长年练习怎样让笔法一点点变好——还没有写出来。这是分析句的位置。",
      action: "请在这句话后面补一两句，说明日复一日的练习和他后来的成就之间是什么关系。",
      source: "ai",
      dimension: "内容",
      symptom: "举了例子，没有解释",
    },
    {
      kind: "issue",
      quote: "他每天都练字，练了很久很久，一直练到后来大家都说他写得非常非常好。",
      text: "这句话里连着用了两组重复的程度词，句子也一直没有停下来。",
      action: "把这句拆成两句，并删掉其中一组重复的程度词。",
      source: "ai",
      dimension: "语言",
      symptom: "",
    },
    {
      kind: "good",
      quote: "那天下午我把字帖摊开，第一笔就写歪了。",
      text: "开头从一个具体的动作写起，读者一下就进到画面里了。",
      action: null,
      source: "teacher",
      dimension: "",
      symptom: "",
    },
  ],
};

function Card() {
  // 和 LiteShell / LiteTeacherShell 同一条路：主题变量由 useLiteTheme 给，
  // 它同时把 `lite-student-theme` 加到 body 上。
  const { themeStyle } = useLiteTheme();
  const [content, dispatch] = useReducer(gradingContentReducer, CONTENT);
  const [mode, setMode] = useState<GradingMode>("edit");
  return (
    <div
      style={{ ...themeStyle, padding: 24, maxWidth: 900, margin: "0 auto", background: "var(--mk-paper)" }}
    >
      <GradingCard
        rubric={RUBRIC}
        content={content}
        mode={mode}
        onMode={setMode}
        unmarked={new Set<number>()}
        onEdit={dispatch}
        onPickQuote={() => {}}
        onShowQuote={() => {}}
      />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <AccentProvider presets={LITE_ACCENT_PRESETS} initialAccent={LITE_ACCENT_PRESETS[0]!.id}>
    <Card />
  </AccentProvider>,
);
