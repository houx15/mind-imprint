import type { ChatMessage } from "@/studio/reading/readingLoop";

/**
 * The guided-tour demo project's ReadingRoom replay (P6, Task 4).
 *
 * A short, REAL exchange seeded into the read-only demo room so the tour can
 * open the actual `ReadingRoom` surface (not a mock screenshot) and show what
 * "read together" looks like — Phoebe using the CRAAP source-evaluation lens
 * to trace a self-media claim back to Chen et al. (2019, Nature
 * Sustainability), the same paper the seeded demo project (migration 0082)
 * and `demoEvaluationReport.ts` are built around.
 *
 * 铁律① (AI never writes the student's essay): every assistant turn here
 * only helps her EVALUATE the source (asks the CRAAP questions, reports back
 * what the lens surfaces) — it never drafts a sentence of her essay, and the
 * closing student turn is HER OWN judgment about what to cite and how.
 *
 * Rendered via `ReadingRoom`'s `initialMessages` prop with `demoMode` — the
 * room is read-only, so this transcript is the whole "conversation" a
 * visitor sees; it never appends to (nothing can be sent in demo mode).
 */
export const demoReadingTranscript: ChatMessage[] = [
  {
    id: "demo-r1",
    role: "student",
    kind: "text",
    body: "我在读的这篇是一个公众号转述的：「NASA 卫星数据显示，中国让地球变绿了」。这个说法能直接信吗？我想用它支持我的论点。",
  },
  {
    id: "demo-r2",
    role: "assistant",
    kind: "lens",
    body: "这是一手论文经过自媒体转述后的说法——转述本身可能悄悄改变了论文的结论边界。用「信源辨识卡」从时效、权威、目的三个维度体检一下这条来源，比直接判断「信不信」更靠谱。",
    cardName: "信源辨识卡 CRAAP / CRRAAB",
  },
  {
    id: "demo-r3",
    role: "assistant",
    kind: "text",
    body: "体检结果：顺着这篇公众号文章往上追，能找到它引用的一手论文——Chen et al. (2019, Nature Sustainability)，用 NASA 卫星数据测的全球叶面积指数变化，权威和时效都站得住。但「目的」这一栏要注意：论文原文说的是全球变绿面积净增约 5%，中国和印度合计贡献超三分之一，而且明确把这归因于「农业集约化（约 32%）+ 人工造林（约 42%）」——论文从未说过「更可持续」。「让地球变绿了」是自媒体自己加的结论，不是论文的结论。",
  },
  {
    id: "demo-r4",
    role: "student",
    kind: "text",
    body: "明白了——那我引用的时候不能直接写「中国让地球更可持续了」，得写成「Chen et al. 发现中国的植被覆盖净增长，主要来自农业集约化和人工造林」，而且要单独引 Chen et al. 这篇一手论文，不能只引那篇公众号转述。至于「更可持续」这个更大的判断，我还得再去查排放数据才能下结论。",
  },
];
