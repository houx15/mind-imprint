import type { EvaluationReport } from "@mind-imprint/contracts";

/**
 * The guided-tour demo project's evaluation report (P2 Task 5).
 *
 * This is the report the guided tour (P3) walks section by section, so every
 * one of the 9 sections is filled to real depth. Its content is coherent with
 * the demo project seeded in migration 0082 — Phoebe's
 * "To what extent is China making the world more environmentally sustainable?"
 * — quoting the same essay, materials (Chen et al. 2019 Nature Sustainability,
 * IEA World Energy Investment, Global Carbon Project) and reflection answers.
 *
 * IMPORTANT: this object is the SINGLE SOURCE OF TRUTH for the seeded row. The
 * exact `JSON.stringify(demoEvaluationReport)` is pasted into migration 0082's
 * `evaluation_report` INSERT, and `test/tour/demoEvaluationReport.test.ts`
 * asserts the migration's JSON is byte-identical to this object (and that both
 * satisfy the strict EvaluationReport Zod). If you edit this object, regenerate
 * the migration JSON (see the test for the exact serialization) or the test
 * fails. Keep the shape identical to the mock fixture, which the schema
 * already accepts.
 *
 * Deliberately contains NO ASCII apostrophes so the JSON pastes into a Postgres
 * single-quoted `::jsonb` literal without escaping.
 */
export const demoEvaluationReport: EvaluationReport = {
  version: 1,
  reportId: "report:demo-china-sustainability",
  projectId: "00000000-0000-0000-0000-000000000200",
  student: { id: "00000000-0000-0000-0000-000000000003", name: "Phoebe" },

  basics: {
    title: "To what extent is China making the world more environmentally sustainable?",
    type: "个人报告 · IB DP 环境系统与社会",
    startDate: "2026-08-16T09:00:00Z",
    endDate: "2026-08-22T16:30:00Z",
    milestones: {
      started: "2026-08-16T09:00:00Z",
      frameworkFinished: "2026-08-17T11:00:00Z",
      proposalFinished: "2026-08-17T15:00:00Z",
      writingFinished: "2026-08-21T15:00:00Z",
      projectFinished: "2026-08-22T16:30:00Z",
    },
    counters: { aiTurns: 148, materialsRead: 5, wordsWritten: 934, aiCommentCount: 11, editCount: 14 },
  },

  abstract: {
    overview:
      "Phoebe 从一篇「NASA 卫星显示中国让地球变绿了」的自媒体文章出发，**没有直接接受这个反转叙事**，而是用 CRAAP 横向溯源把它追回一手论文 Chen et al. (2019, Nature Sustainability)，读出全球绿叶面积净增约 5%、中国与印度合计贡献超三分之一，并抓住论文的关键限定——变绿主因是农业集约化（约 32%）与人工造林（约 42%），论文本身从未声称「变绿=更可持续」。她为核心主张补上第二条独立证据（IEA 清洁能源投资全球第一），再主动去撞对自己不利的反例（中国自 2006 年起为全球最大碳排放国、煤电与新能源并行扩张），用让步段处理它。最关键的变化，是结论从「中国是否让地球变绿」收束为一个带限定条件的判断：**中国正在使世界更可持续，但这是进行时而非完成时**。",
    materialSentence:
      "共 5 条来源，为每条记录了来源层级、能证明什么与不能证明什么：一手论文与 IEA/GCP 数据进正文，自媒体文章只作溯源入口不作证据，煤电监测报告作第二条反例线。",
    writingSentence:
      "14 次打磨中不仅改措辞，更把 prove / saved the Earth / China caused 等越界表达收窄到论文口径，并把「趋势变好」与「问题已解决」两件事显式分开。",
    aiSentence:
      "让 AI 递 CRAAP 工具卡、做追问、检查越界、模拟答辩；明确要求 AI 不替写正文、只查论证与结构，最终判断与取舍署名归自己。",
    suggestionParagraph:
      "把 source log 做得更标准化——每条来源都固定填入 opened by、why opened、source type、claim supported、cannot support、status。这样答辩时你能更快说清每条证据的边界，也更容易证明最终判断归属于你自己而非 AI。",
    suggestionSentences: [
      "如果继续扩展：可单独讨论人均排放与历史累计排放，但不要在有限篇幅的正文里随手混入不同口径的数字。",
      "让步段之后再补一句「因此结论被如何限定」，让 qualified yes 的边界更外显。",
      "为每条来源标注其立场与潜在利益（如 GEM 的 NGO 立场），让视角识别更系统。",
      "把每次修订「改了什么、为什么」记成一句修订日志，答辩时可直接引用。",
    ],
    recommendedCourses: [
      {
        courseId: "course:source-triage",
        reason: "你已能给来源分配功能（进正文 / 进 source log / 作反例），这门课把它变成可复用的标准流程。",
      },
      {
        courseId: "course:data-scope",
        reason: "巩固「年度份额 / 历史累计 / 人均」的区分，避免变绿 5% 与排放 31% 这类不同口径的数字互相抵消。",
      },
    ],
  },

  events: [
    {
      ts: "2026-08-16T09:05:00Z",
      kind: "chat",
      summary:
        "从一篇「NASA 卫星显示中国让地球变绿」的自媒体文章进入，先不下结论，立下「未追到一手来源不写进论证」的红线，并把研究问题拆成变绿 / 投资 / 存量排放三条线。",
      aiTurns: 18,
      ref: { id: "message:t003", label: "研究问题成形", ts: "2026-08-16T09:25:00Z" },
    },
    {
      ts: "2026-08-17T10:14:00Z",
      kind: "reading",
      summary:
        "用 CRAAP 把自媒体文章横向溯源，追回 Chen et al. (2019, Nature Sustainability) 原论文，核实 5% 净增、中印超三分之一、以及农业集约化与造林的机制。",
      aiTurns: 34,
      ref: { id: "message:t048", label: "追回一手论文", ts: "2026-08-17T10:50:00Z" },
    },
    {
      ts: "2026-08-18T15:20:00Z",
      kind: "reading",
      summary:
        "为主张补第二条独立证据线：读 IEA World Energy Investment 2023（中国清洁能源投资全球第一），并主动检索到 Global Carbon Project 的排放核算作为反例。",
      aiTurns: 27,
    },
    {
      ts: "2026-08-19T11:00:00Z",
      kind: "graph",
      summary:
        "搭论证图，把「主张—证据—推理」连起来，主动把「中国是全球最大碳排放国」这个反例放进图里，而不是回避它。",
      aiTurns: 19,
      ref: { id: "message:t263", label: "主动撞反例", ts: "2026-08-19T11:40:00Z" },
    },
    {
      ts: "2026-08-20T13:02:00Z",
      kind: "writing",
      summary:
        "分段写引论 / 论证一 / 论证二 / 让步段 / 结论；列不能写的红线词，拒绝 AI 代写正文，只要结构提示与越界检查。",
      aiTurns: 21,
    },
    {
      ts: "2026-08-21T10:00:00Z",
      kind: "review",
      summary:
        "把 Review 定义为可答辩检查而非润色：逐项核对 claim / 数据口径 / 引用；确认煤电监测数据作为第二条反例线，形成 AI use statement。",
      aiTurns: 20,
      ref: { id: "message:t461", label: "claim 越界审查", ts: "2026-08-21T10:35:00Z" },
    },
    {
      ts: "2026-08-22T16:00:00Z",
      kind: "milestone",
      summary:
        "复盘：说清每条来源为什么用、能证明什么、不能证明什么；整理 essay / source log / 论证图 / reflection 提交包，完成过程评估。",
      aiTurns: 9,
    },
  ],

  materials: [
    {
      materialId: "material:wechat-greening",
      addedAt: "2026-08-16T09:05:00Z",
      source: "微信公众号《卫星图看中国变绿》",
      url: "https://mp.weixin.qq.com/s/demo-china-greening-tour",
      usedIn: null,
      finalStatus: "溯源入口 · 不作证据",
      comment:
        "真实看到的第一条材料。她识别出文章把「变绿」偷换成「更可持续」，把它降级为溯源入口，保留它是为了展示从二手转述追到一手论文的方法本身。",
      cannotSupport: "不能证明 NASA/论文原意，也不能证明中国让世界更可持续。",
    },
    {
      materialId: "material:chen-2019",
      addedAt: "2026-08-17T11:05:00Z",
      source: "Chen et al. (2019), Nature Sustainability",
      url: "https://doi.org/10.1038/s41893-019-0220-7",
      usedIn: { id: "graph:00000000-0000-0000-0000-000000000240", label: "核心主张" },
      finalStatus: "一手论文 · 锚点证据",
      comment:
        "从自媒体线索追回的一手来源，同行评审、数据可复核。确认全球绿叶面积净增约 5%、中国与印度合计贡献超三分之一；机制为农业集约化（约 32%）与人工造林（约 42%）。",
      cannotSupport: "只支持 leaf-area greening，不证明生物多样性、碳中和或整体环境可持续。",
    },
    {
      materialId: "material:iea-2023",
      addedAt: "2026-08-18T15:20:00Z",
      source: "IEA World Energy Investment 2023",
      url: "https://www.iea.org/reports/world-energy-investment-2023",
      usedIn: null,
      finalStatus: "权威机构 · 第二证据线",
      comment:
        "为「治理决心真实」提供独立于卫星数据的第二条证据：中国连续多年清洁能源投资全球第一，2023 年约占全球三成。与卫星数据交叉，不让单一证据独自承重。",
      cannotSupport: "投资额大不等于存量问题已解决，不能单独推出「已经可持续」。",
    },
    {
      materialId: "material:gcp-2023",
      addedAt: "2026-08-18T16:40:00Z",
      source: "Global Carbon Budget 2023 / Global Carbon Project",
      url: "https://globalcarbonproject.org/carbonbudget/",
      usedIn: { id: "graph:00000000-0000-0000-0000-000000000242", label: "让步段反例" },
      finalStatus: "一手核算 · 反例证据",
      comment:
        "自己检索到的反例数据：中国自 2006 年起为全球最大年度 CO2 排放国，2022 年约占全球 31%。她没有回避，而是写进让步段，用它给结论装限定条件。",
      cannotSupport: "annual share，不是历史累计、不是人均；不能抹掉 25%/5% 的变绿证据。",
    },
    {
      materialId: "material:gem-coal-2022",
      addedAt: "2026-08-18T17:10:00Z",
      source: "Global Energy Monitor & CREA — China coal power 2022",
      url: "https://globalenergymonitor.org/report/china-coal-2022/",
      usedIn: null,
      finalStatus: "NGO 报告 · 第二反例线",
      comment:
        "2022 年中国在扩张新能源的同时新核准大量燃煤电厂。她注意到 NGO 立场需留意，但煤电核准数据可交叉核实，用它说明这是「转型进行中」而非「已完成」。",
      cannotSupport: "立场性报告，不能单独作科学证明；煤电扩张不能推翻投资与变绿趋势。",
    },
  ],

  depth: [
    {
      id: "D1",
      level: 3,
      summary: "问题从「这真的假的、中国是不是主角」逐步被材料迫着收窄为 to what extent 的三维框架（变绿 / 投资 / 存量排放）。",
      evidence: [
        {
          id: "message:t003",
          ts: "2026-08-16T09:20:00Z",
          stage: "真实起点",
          quote: "我看到一篇文章说 NASA 卫星显示中国让地球变绿了，这能说明中国让世界更可持续吗？",
          observation: "用人话说出最初问题，不先套研究框架。",
          boundary: "此时只是直觉怀疑，尚未成为可检验的研究问题。",
        },
        {
          id: "message:t031",
          ts: "2026-08-16T09:25:00Z",
          stage: "研究问题形成",
          quote: "我要把「趋势在变好」和「问题已解决」这两件事分开，给出一个带限定条件的判断。",
          observation: "在多轮追问后才形成 to what extent 的正式框架。",
          boundary: "不是 AI 一开始就给出三维框架。",
        },
      ],
      suggestion: "下次可显式写出被排除的子问题及原因，让问题边界更透明。",
    },
    {
      id: "D2",
      level: 4,
      summary: "不仅找到来源，还记录来源层级、机制与不能证明的内容；自媒体文章被降级为入口、煤电报告被标注立场，说明来源纪律真实影响正文。",
      evidence: [
        {
          id: "message:t010",
          ts: "2026-08-16T09:35:00Z",
          stage: "真实起点",
          quote: "未追到一手来源的说法不写进论证。",
          observation: "自建来源准入标准。",
          boundary: "标准是自发定的，尚需验证后续是否真的坚持执行。",
        },
        {
          id: "message:t048",
          ts: "2026-08-17T10:50:00Z",
          stage: "追回一手论文",
          quote: "论文只说变绿，没说更可持续，主因是农业集约化和人工造林。",
          observation: "继续追到论文而非停在自媒体转述，并抓住机制限定。",
        },
        {
          id: "message:t461",
          ts: "2026-08-21T10:35:00Z",
          stage: "Review：claim 审查",
          quote: "煤电报告是 NGO 立场，我把它标成需留意，但核准数据可交叉核实。",
          observation: "对来源立场做显式标注，而非照单全收。",
          boundary: "立场性来源需与一手核算数据交叉，不单独承重。",
        },
      ],
      suggestion: "把「口径是否一致」也固定写进每条来源的备注。",
    },
    {
      id: "D3",
      level: 3,
      summary: "三类指标放进一个限定判断：变绿提供正证据，投资补强决心，存量排放限制外推——三者张力被显式处理而非互相抵消。",
      evidence: [
        {
          id: "message:t090",
          ts: "2026-08-18T15:25:00Z",
          stage: "补第二条证据",
          quote: "我需要第二条独立证据，不能只靠卫星数据这一条。",
          observation: "意识到单一证据线的脆弱，主动补 IEA 投资数据。",
          boundary: "投资与变绿都指向决心，但都不覆盖存量排放。",
        },
        {
          id: "message:t324",
          ts: "2026-08-20T14:05:00Z",
          stage: "让步段",
          quote: "投资额巨大不等于存量问题已经解决。",
          observation: "主张、证据与限制在让步段处闭合。",
        },
      ],
      suggestion: "让步段后补一句「因此结论如何被限定」。",
    },
    {
      id: "D4",
      level: 3,
      summary: "能区分自媒体转述、同行论文、权威机构报告与 NGO 监测的不同功能；自媒体只作入口，一手数据作锚点，反例作让步。",
      evidence: [
        {
          id: "message:t054",
          ts: "2026-08-17T10:55:00Z",
          stage: "追回一手论文",
          quote: "combined China and India, not China alone。",
          observation: "防止把中印合并口径写成中国单独贡献。",
          boundary: "只纠正了归因主体，尚未处理数据口径本身。",
        },
        {
          id: "message:t263",
          ts: "2026-08-19T11:40:00Z",
          stage: "搭论证图",
          quote: "我把最大碳排放国这个反例直接放进论证图里。",
          observation: "主动纳入不利证据而非回避。",
        },
      ],
      suggestion: "为每条来源标注其立场与潜在利益，让视角识别更系统。",
    },
    {
      id: "D5",
      level: 3,
      summary: "修订不是语言层面：多次把 prove / saved the Earth / China caused 等越界表达改成范围更准的说法。",
      evidence: [
        {
          id: "message:t078",
          ts: "2026-08-20T13:40:00Z",
          stage: "论证一",
          quote: "删「中国拯救了地球」→ 改为可证的 leaf-area 净增贡献。",
          observation: "修订是降低主张强度到证据能支撑的范围。",
          boundary: "改的是单句表达，需确认后续段落是否统一收紧。",
        },
        {
          id: "message:t308",
          ts: "2026-08-20T14:50:00Z",
          stage: "论证二 / 让步段",
          quote: "「prove sustainability」→「supports a qualified claim」；「已经可持续」→「进行时而非完成时」。",
          observation: "越界表达被逐一收窄到论文口径。",
        },
      ],
      suggestion: "把每次修订「改了什么、为什么」记成一句修订日志。",
    },
    {
      id: "D6",
      level: 4,
      summary: "反思落在方法而非结论：意识到「不被叙事俘获」是一套可练习的操作——溯源、交叉验证、主动证伪。",
      evidence: [
        {
          id: "message:t518",
          ts: "2026-08-22T15:40:00Z",
          stage: "回顾反思",
          quote: "这道题最大的收获不在结论，而在方法。",
          observation: "把项目经验抽象为可迁移方法。",
          boundary: "方法句本身不是证据，要看终检清单是否被真正执行。",
        },
        {
          id: "message:t519",
          ts: "2026-08-22T15:50:00Z",
          stage: "回顾反思",
          quote: "下一次遇到太过圆满的说法，我会先问：这个数据最初是从哪来的？",
          observation: "把方法转成一句可执行的自我提问。",
        },
        {
          id: "message:t520",
          ts: "2026-08-22T16:10:00Z",
          stage: "回顾反思",
          quote: "如果重来，我会更早地为核心主张找第二条独立证据线。",
          observation: "对证据结构做出具体反思。",
        },
      ],
      suggestion: "把「溯源—交叉验证—主动证伪」固定成一句可复用的话，迁移到下个项目。",
    },
  ],

  autonomy: [
    {
      id: "A1",
      band: 3,
      summary: "关键方向多次由她提出：自己追一手论文、自己补投资数据、要求不回避反例。",
      evidence: [
        {
          id: "message:t022",
          ts: "2026-08-17T10:05:00Z",
          stage: "追回一手论文",
          quote: "我不是让 AI 给我论文链接，而是自己用文章里的线索追回原论文。",
          observation: "检索路线归属清楚。",
          boundary: "AI 可帮术语，不替代追源路径。",
        },
        {
          id: "message:t090",
          ts: "2026-08-18T15:25:00Z",
          stage: "补第二条证据",
          quote: "我搜到了 IEA 的清洁能源投资数据，用来补强主张。",
          observation: "补证由研究问题需要驱动。",
        },
      ],
      suggestion: "把「为什么走这条线」也记一句，让方向选择可追溯。",
    },
    {
      id: "A2",
      band: 4,
      summary: "主动启动溯源、横向阅读、第二证据线检索与 Review 漏洞发现——CRAAP 工具卡参与推动，需保留触发方。",
      evidence: [
        {
          id: "message:t044",
          ts: "2026-08-17T10:40:00Z",
          stage: "追回一手论文",
          quote: "我用 CRAAP 逐项核对作者、日期、机构，再追到 DOI。",
          observation: "主动做来源身份核查。",
          boundary: "触发方：部分由 CRAAP 工具卡推动，非全部自发。",
        },
        {
          id: "message:t221",
          ts: "2026-08-18T16:45:00Z",
          stage: "补第二条证据",
          quote: "我主动去找了一个对自己不利的数据来撞。",
          observation: "主动寻找反例而非等待被质疑。",
        },
      ],
      suggestion: "保持——可把核查清单固化成模板。",
    },
    {
      id: "A3",
      band: 4,
      summary: "反复设边界：未追源不进论证、AI 不替写正文、列出不能写的红线词、给立场性来源标注。",
      evidence: [
        {
          id: "message:t285",
          ts: "2026-08-20T13:05:00Z",
          stage: "开稿边界",
          quote: "我先不开全文，请你只检查写作计划，不替我写。",
          observation: "AI 只能做边界检查，不能替生成终稿。",
          boundary: "AI 只能做边界检查，不能替生成终稿。",
        },
        {
          id: "message:t293",
          ts: "2026-08-20T13:20:00Z",
          stage: "开稿边界",
          quote: "列不能写的词：prove sustainability、saved the Earth、China caused。",
          observation: "语言红线直接约束写作。",
        },
      ],
      suggestion: "把边界口径固定成一句可复用的话，迁移到下个项目。",
    },
    {
      id: "A4",
      band: 3,
      summary: "没有编造反例，而是自己检索到真实反例（最大碳排放国、煤电扩张），拆成窄机制并要求答辩追问。",
      evidence: [
        {
          id: "message:t213",
          ts: "2026-08-18T16:50:00Z",
          stage: "反例处理",
          quote: "我先不反驳这个数据，只问它能限制我的结论到什么程度。",
          observation: "对抗是功能判断，不是立刻否定。",
        },
        {
          id: "message:t498",
          ts: "2026-08-22T16:12:00Z",
          stage: "专业答辩",
          quote: "请你像老师一样连续追问我每条证据的使用理由、功能和边界。",
          observation: "主动要求高压答辩模拟。",
        },
      ],
      suggestion: "让 AI 标注反例的信源强度，把检验做得更细。",
    },
    {
      id: "A5",
      band: 4,
      summary: "答辩段最清楚：逐一说明每条来源为什么用、为什么不用、能证明什么、不能证明什么。",
      evidence: [
        {
          id: "message:t499",
          ts: "2026-08-22T16:14:00Z",
          stage: "专业答辩",
          quote: "自媒体文章我放 source log，不放正文——它能说明我怎么进入问题，但不是证据。",
          observation: "清楚说明来源功能分配。",
        },
        {
          id: "message:t422",
          ts: "2026-08-21T18:00:00Z",
          stage: "最终正文自查",
          quote: "最终自查：正文没有把自媒体转述、煤电 NGO 立场当成独立结论证据。",
          observation: "自主排除不合格来源。",
          boundary: "不是 AI 替她删。",
        },
      ],
      suggestion: "在复盘里点名哪些判断完全由自己做出。",
    },
    {
      id: "A6",
      band: 3,
      summary: "同时拒绝 full yes 和 no：qualified yes 不是折中术，而是不同指标张力下的限制判断。",
      evidence: [
        {
          id: "message:t436",
          ts: "2026-08-21T10:15:00Z",
          stage: "Review：claim 审查",
          quote: "不能改成 full yes，因为最大碳排放国和煤电扩张会顶回来。",
          observation: "用证据张力约束主张强度。",
        },
        {
          id: "message:t437",
          ts: "2026-08-21T10:16:00Z",
          stage: "Review：claim 审查",
          quote: "也不能改成 no，因为变绿 5% 和投资第一不能被排放数据直接抹掉。",
          observation: "拒绝用一条证据抹掉另一条。",
        },
      ],
      suggestion: "记录一次「被证据改变主意」的具体时刻，作为迁移样本。",
    },
  ],

  promptLens: {
    summary:
      "提示词本身不分档。真正能成立为过程证据的，是提示词之后是否产生了溯源、草稿修改、Review 记录或答辩回应。Phoebe 的提问以「要求溯源工具 / 要求不替写 / 要求只查越界 / 要求模拟答辩」为主；少数「帮我总结这篇论文」这类自然请求，她随后自己读完并做了核对，因此不应自动判为低质量。",
    prompts: [
      {
        stage: "真实起点",
        quote: "我看到一篇文章说 NASA 卫星显示中国让地球变绿了，这能说明中国让世界更可持续吗？",
        ref: { id: "message:t003" },
        observation: "暴露真实材料起点与初始问题，不是让 AI 直接写文章——后续产生了溯源路径与 source log。",
        relatedDomains: ["D1", "A1"],
        attention: false,
      },
      {
        stage: "写作过程",
        quote: "我先不开全文，请你只检查写作计划，不替我写。",
        ref: { id: "message:t285" },
        observation: "把 AI 限定为计划检查者；后续段落功能与红线词由她自己设定。",
        relatedDomains: ["A3", "D5"],
        attention: false,
      },
      {
        stage: "Review",
        quote: "我把整文贴出来，先请你只检查 claim 有没有越界，不润色。",
        ref: { id: "message:t389" },
        observation: "把 AI 放在审阅者位置，触发了 claim / 数据口径逐项审查。",
        relatedDomains: ["D3", "A5"],
        attention: false,
      },
      {
        stage: "读论文",
        quote: "你能不能帮我总结这篇 Nature 论文。",
        ref: { id: "message:t042" },
        observation: "自然的求助点；她随后自己读完论文、核对了 5% 与机制数字，把 AI 的总结当索引而非结论——风险被她自己化解。",
        relatedDomains: ["A2", "D2"],
        attention: true,
      },
    ],
  },

  toolUsage: [
    {
      toolId: "card:craap",
      name: "CRAAP 体检",
      stage: "追源 · 补证",
      purpose: "对来源做 Currency/Relevance/Authority/Accuracy/Purpose 局部判断",
      summary: "帮助她把自媒体文章追回 Chen et al. 原论文，并核查 IEA 与 GCP 的机构身份与数据口径。",
    },
    {
      toolId: "card:source-triage",
      name: "溯源 / 横向阅读",
      stage: "追源",
      purpose: "停止判断、提取关键词、追到一手来源、横向核实",
      summary: "把「变绿」这条说法从二手转述追回同行评审论文，区分入口与证据。",
    },
    {
      toolId: "card:warrant",
      name: "论证解剖 / warrant",
      stage: "搭论证图",
      purpose: "拆 claim–evidence–reasoning 与缺失 warrant",
      summary: "把三条证据放回段落功能，而不是堆材料；构造让步段接住最大碳排放国这个反例。",
    },
    {
      toolId: "card:concession",
      name: "让步段",
      stage: "写作",
      purpose: "正面处理最强反例并给结论装限定条件",
      summary: "承认中国是全球最大碳排放国、煤电与新能源并行扩张，把它转成对结论的限定而非削弱。",
    },
    {
      toolId: "subagent:review",
      name: "审阅 / AI 伦理审计子代理",
      stage: "Review",
      purpose: "把 AI 从代写者改为审阅者、追问者、边界检查器",
      summary: "对全稿给出 11 条批注，学生采纳后形成 AI use statement 与 not-used-for 清单。",
    },
    {
      toolId: "subagent:reading-room",
      name: "阅读室子代理",
      stage: "追源",
      purpose: "学科透镜下逐句共读",
      summary: "以环境科学透镜共读 Chen et al. 论文，帮助分辨数据与结论、标出「dominates」等强动词。",
    },
  ],

  risks: [
    {
      type: "argument-logic",
      behaviour: "自媒体标题把「变绿」偷换成「更可持续」，存在被反转叙事俘获、写成强因果的风险。",
      ref: { id: "message:t006" },
      suggestion: "表扬她识别了风险并追回一手论文；正文动词回到论文口径，不把标题当证据。",
    },
    {
      type: "data-scope",
      behaviour: "变绿 5% / 中印超三分之一 与 排放约 31% 是不同口径，早期有互相抵消的风险。",
      ref: { id: "message:t098" },
      suggestion: "不能说 31% 反驳 5%，只能说它限制整体可持续的外推；陈述前先声明口径。",
    },
    {
      type: "missing-source",
      behaviour: "IEA 投资数据一度只记结论未记具体报告年份与口径，存在无法答辩的风险。",
      ref: { id: "message:t201" },
      suggestion: "为每条数据固定填入报告名、年份与页码/表号，答辩时可直接定位。",
    },
    {
      type: "argument-logic",
      behaviour: "煤电扩张（GEM，NGO 立场）若不与一手核算交叉，存在用立场性来源单独承重的风险。",
      ref: { id: "message:t455" },
      suggestion: "标注立场、与 GCP 核算交叉，把它定位为「转型进行中」的佐证而非独立证明。",
    },
    {
      type: "ai-ghostwrite",
      behaviour: "存在「帮我总结这篇论文」等自然请求，接近让 AI 代劳阅读与判断。",
      ref: { id: "message:t042" },
      suggestion: "她随后自己读完论文并核对数字，持续要求 AI 检查而非代写，AI use 边界可查。",
    },
  ],

  generatedAt: "2026-08-22T17:00:00Z",
};
