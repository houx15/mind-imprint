// Existing marketing-site case and asset keys.
type Station = {
  k: [string, string];
  h: [string, string];
  p: [string, string];
  /** Media key without extension; `clip` says whether it moves. */
  asset: string;
  clip?: boolean;
  alt: [string, string];
};

export const stations: Station[] = [
  {
    k: ["课程", "The course"],
    h: ["情境化的知识输入", "Knowledge, delivered in a situation"],
    p: [
      "通过案例学习，系统了解顶尖名校对学术写作的标准。",
      "She learns what the best universities actually ask of academic writing — through a case, not a checklist.",
    ],
    asset: "case/01-course-v1.webp",
    alt: ["课程页：哈佛与剑桥招生网站对书面作品的原文要求", "The course page: what Harvard and Cambridge really say about written work"],
  },
  {
    k: ["立题", "Framing"],
    h: ["与 AI 一同讨论研究议题", "Talking the question through with the AI"],
    p: [
      "AI 引导学生把宽泛的想法，逐步定义成一个具体的研究主题。",
      "The AI walks her from a broad interest to a subject specific enough to research.",
    ],
    asset: "case/02-proposal-v1",
    clip: true,
    alt: ["立题：教练把模糊的主题追问成可回答的研究问题", "Framing: the coach pressing a vague topic into an answerable question"],
  },
  {
    k: ["研究", "Research"],
    h: ["系统化的溯源检索", "Searching, and tracing what she finds"],
    p: [
      "引导学生追溯每一条信息的来源，把检索做成研究。",
      "She is walked back to where each claim came from, until searching becomes research.",
    ],
    asset: "case/03-research-v1",
    clip: true,
    alt: ["溯源检索：从二手转述追回一手来源", "Tracing a claim from a second-hand retelling back to its source"],
  },
  {
    k: ["研究", "Research"],
    h: ["兔子洞收集岔路的探索", "The rabbit hole keeps the detours"],
    p: [
      "兔子洞图谱记录下学生的探索轨迹——包括那些没有走通的岔路。",
      "The rabbit-hole map records where she went, including the turnings that led nowhere.",
    ],
    asset: "case/04-warrenmap-v1.webp",
    alt: ["兔子洞图谱：问题、挂在问题下的文献，与未归类的来源", "The rabbit-hole map: questions, the sources hung beneath them, and what is still unfiled"],
  },
  {
    k: ["阅读", "Reading"],
    h: ["深入考察数据、信息、来源", "Reading the data, the claim and the source closely"],
    p: [
      "AI 用学科透镜和工具包，陪学生一起深度阅读。",
      "The AI reads with her, through a subject lens and the tools that go with it.",
    ],
    asset: "case/05-guided-reading-v1.webp",
    alt: ["阅读室：原文、学科透镜与逐段的阅读工具", "The reading room: the source, a subject lens, and the tools for each passage"],
  },
  {
    k: ["写作", "Writing"],
    h: ["AI 只提供引导，每个词都自己写", "The AI guides; every word is hers"],
    p: [
      "AI 在需要的时候给出引导性问题和写作反馈，正文一个字也不代写。",
      "It asks the question that unsticks her and marks up what she has written. It writes none of the prose.",
    ],
    asset: "case/06-writing-v1",
    clip: true,
    alt: ["写作面：引导卡、阅读笔记与 AI 批注，正文由学生自己填", "The writing surface: a guiding card, her notes, the AI's annotations — and her own prose"],
  },
  {
    k: ["评估", "Assessment"],
    h: ["详细的过程评估报告，记录每次尝试", "A report of the process, attempt by attempt"],
    p: [
      "成稿之后，她自己先写下复盘。系统随后生成九个板块的评估报告。",
      "With the paper finished she writes her own review first. The system then generates the nine-section report.",
    ],
    asset: "case/07-evaluation-v1",
    clip: true,
    alt: ["评估报告：里程碑、数出来的计数，与九个板块", "The report: milestones, counted figures, and the nine sections"],
  },
];