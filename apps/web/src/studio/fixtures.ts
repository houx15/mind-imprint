import type { MaterialSource } from "@mind-imprint/contracts";
import type { StudioState } from "./state";

// The 0457 calibration scenario (AGENTS.md): Phoebe, "To what extent is China
// making the world more environmentally sustainable?" — dramatized at station
// S4 (论证构建), mid-way through fixing an "orphan evidence" flag on the
// argument map. Content is drawn from the real scenario materials and the
// design binding docs/design/思维印记_工作区.dc.html (STA ~L2092, EQ ~L2286,
// S4META ~L1947, GA ~L2239). Real content throughout — no lorem ipsum.
//
// The dossier's own material fixture (blog + NASA paper) was server-projected
// as of Slice 6b (migration 0020 seeds them) and its frontend copy deleted —
// MATERIAL_FIXTURE below is the wire-shaped (MaterialSource) equivalent kept
// only for this file's client-side story fixture / the dev harness.

const BLOG_ID = "src-blog-china-greening";
const NASA_ID = "src-nasa-nature-sustainability";

export const MATERIAL_FIXTURE: MaterialSource[] = [
  {
    id: BLOG_ID,
    title: "《卫星图看中国变绿》",
    sourceUrl: "https://mp.weixin.qq.com/s/china-greening-satellite",
    kind: "article",
    origin: "fetched",
    blocks: [
      {
        id: "b1",
        text: "过去二十年里发生了一件几乎没人注意到的事：根据 NASA 卫星数据，地球比 2000 年整整绿了一圈，而这背后最大的推手，是中国。",
      },
      {
        id: "b2",
        text: "变化大到能从太空里看见。2000 到 2017 年间，NASA 的 MODIS 卫星记录到全球绿叶面积增加了 5%，相当于新增了一整片亚马逊雨林那么大的绿色；仅占全球陆地面积 9% 的中国和印度，就贡献了这其中三分之一以上的增量。",
      },
      {
        id: "b3",
        text: "很难不把这读成一个信号：那个曾经和雾霾、燃煤电厂划等号的国家，如今悄悄成了地球变绿背后最大的力量——中国的环保政策，正在起效。",
      },
    ],
    locked: false,
    role: "触发关注的入口——数据引用听着权威，但结论被作者悄悄放大了，需要横向核实。",
    tier: "二手转述",
    takeaway: "",
    timeSpentS: 240,
    lateralRead: false,
    anchors: [
      {
        id: "span-blog-authority",
        material_id: BLOG_ID,
        block_id: "b1",
        start: 21,
        end: 33,
        quote: "根据 NASA 卫星数据",
        dimension: "权威性",
        author: "ai",
        question: "「根据 NASA 卫星数据」——这条往上追，原始出处是谁？能找到 NASA 或论文本身吗，还是只是这篇公众号自己转述的？",
        answer: "",
      },
      {
        id: "span-blog-purpose",
        material_id: BLOG_ID,
        block_id: "b3",
        start: 0,
        end: 63,
        quote: "很难不把这读成一个信号",
        dimension: "目的性",
        author: "ai",
        question: "作者把「变绿」直接等同于「环保政策奏效」「更可持续」，这个推论站得住吗？有没有被这篇文章悄悄绕开的对立事实（比如碳排放）？",
        answer: "",
      },
    ],
  },
  {
    id: NASA_ID,
    title: "Chen et al. (2019), Nature Sustainability",
    sourceUrl: "https://doi.org/10.1038/s41893-019-0220-7",
    kind: "paper",
    origin: "fetched",
    blocks: [
      {
        id: "b1",
        text: "基于 NASA MODIS 卫星 2000–2017 年数据：全球绿叶面积净增 5%，中国、印度合计贡献全球净增量的三分之一以上；增量主要来自农业集约化耕作与大规模植树工程，而非森林自然恢复。",
      },
    ],
    locked: true,
    role: "第一手数据来源，证实了「卫星观测到变绿」这件事本身是真的，但没有说这等于「更可持续」——变绿主要来自农业集约化与植树造林，论文本身并未涉及碳排放。",
    tier: "一手论文",
    takeaway:
      "NASA 与 Nature Sustainability 指出：卫星数据确认地球在变绿，中国是最大贡献者之一，但主要机制是农业集约化与人工造林，不是整体生态系统改善——论文本身不支持「中国让地球更可持续」这个更大的结论，也没有讨论碳排放。",
    timeSpentS: 610,
    lateralRead: false,
    anchors: [],
  },
];

export const STUDIO_FIXTURE: StudioState = {
  project: {
    title: "To what extent is China making the world more environmentally sustainable?",
    qualLabel: "0457 个人报告",
  },
  stations: [
    { code: "S0", name: "任务解码", view: "评估", state: "done" },
    { code: "S1", name: "立题", view: "结构", state: "done" },
    { code: "S2", name: "视角与素材", view: "素材", state: "done" },
    { code: "S3", name: "信源评估", view: "素材", state: "done", backflow: true },
    { code: "S4", name: "论证构建", view: "结构", state: "current", gate: { total: 5, passed: 2 } },
    { code: "S5", name: "成稿打磨", view: "写作", state: "locked" },
    { code: "S6", name: "反思归档", view: "评估", state: "locked" },
  ],
  activeStation: "S4",
  focusMode: false,
  coach: {
    anchor: "论证图 · 治理决心主张",
    messages: [
      {
        kind: "flag",
        label: "孤儿证据",
        body: "图上有一处「孤儿证据」：你收集了「可再生投资全球第一」，却没连到任何主张。它到底在替你证明什么？",
      },
      { kind: "student", body: "它想证明中国是认真在转型的。" },
      {
        kind: "ai",
        // `tag` stays a bare criterion — never "锚定 D5". `anchor` is the
        // separate carried-forward field: CoachRail renders it as its own
        // "锚定 {anchor}" label next to the tag chip (matching the label
        // already shown in the header status pill / DispositionCard).
        tag: "D5",
        anchor: "论证图 · 治理决心主张",
        body: "那就把它连到「治理决心」那条主张下——不过那条现在是「裸主张」，还没有证据。这两处正好互相补上。补完，门禁第①条就过了。",
      },
    ],
    equipment: [
      { id: "eq-steelman", name: "钢人卡", spont: "提示后", meth: "concession", materialId: "" },
      { id: "eq-concession-para", name: "让步段卡", spont: "自发", meth: "concession", materialId: "" },
      { id: "eq-toulmin-map", name: "Toulmin 图", spont: "自发", meth: "concession", materialId: "" },
      { id: "eq-sift-lateral", name: "SIFT 横向阅读", spont: "自发", meth: "sift_craap", materialId: "" },
      { id: "eq-craap-five", name: "CRAAP 五维", spont: "提示后", meth: "sift_craap", materialId: "" },
      { id: "eq-lateral-check", name: "横向核查", spont: "自发", meth: "sift_craap", materialId: "" },
    ],
  },
  views: {
    material: MATERIAL_FIXTURE,
    structure: [
      {
        id: "claim",
        role: "核心主张",
        status: "done",
        preview:
          "中国的环保治理呈现真实且持续增强的决心，但存量排放问题尚未解决——「趋势变好」不等于「问题已解决」。",
      },
      {
        id: "warrant",
        role: "理据 · 推理",
        status: "empty",
      },
      {
        id: "evidence",
        role: "支撑证据",
        status: "active",
        question: "挑一条证据，用自己的话概括它如何支撑主张。",
      },
      {
        id: "counter",
        role: "反方 · 钢人",
        status: "done",
        preview: "中国是全球碳排放总量第一的国家——这是任何「更可持续」论证都绕不开的硬事实。",
      },
      {
        id: "concession",
        role: "让步 · 转折",
        status: "empty",
      },
    ],
    writing: {
      draft:
        "这篇文章想讨论一个常见的说法：中国是否让地球更可持续。\n\n" +
        "卫星数据显示，2000 年以来地球明显变绿，其中中国的贡献最大。照这个趋势，可以说中国正在让整个地球更可持续。当然，有人会说中国的碳排放总量是全球第一，但这并不能抹掉绿化的成绩。",
      mode: "edit",
    },
    review: [
      { table: "表A", total: 2, lit: 2, note: "问题聚焦、可回答", level: "full" },
      { table: "表B", total: 3, lit: 3, note: "本地—国家—全球三层齐备", level: "full" },
      { table: "表C", total: 3, lit: 2, note: "两方视角在，反方仍偏薄", level: "partial" },
      { table: "表D", total: 4, lit: 3, note: "已溯到一手源；仍有 1 条孤儿证据", level: "partial" },
      { table: "表E", total: 4, lit: 2, note: "「变绿→可持续」的跳步还没补上", level: "partial" },
      { table: "表F", total: 3, lit: 1, note: "对来源风险的评估还不足", level: "partial" },
      { table: "表G", total: 2, lit: 0, note: "S6 尚未开始", level: "empty" },
      { table: "表H", total: 3, lit: 3, note: "结构清楚、语言干净", level: "full" },
    ],
    onboarding: {
      restatePrompt:
        "这次要写的是一篇个人报告：「中国在多大程度上让世界变得更具环境可持续性？」（0457 全球社会中的环境系统与社会）。用你自己的话说说，这道题到底在问什么，你打算怎么回答，以及评分标准里你觉得最容易被忽略的是哪一条。",
      rubricRows: [
        {
          official: "Analysis of different perspectives",
          plain: "能从不同视角分析，不只罗列观点",
          weak: true,
        },
        {
          official: "Use & evaluation of evidence / sources",
          plain: "用可信来源，并说清它可不可信",
          weak: true,
        },
        {
          official: "Personal response & reflection",
          plain: "给出自己的判断，并回看研究过程",
          weak: true,
        },
        {
          official: "Communication & organisation",
          plain: "结构清楚、表达清晰",
          weak: false,
        },
      ],
      planSteps: ["立题", "找素材", "评估来源", "搭论证", "成稿", "反思归档"],
    },
  },
};
