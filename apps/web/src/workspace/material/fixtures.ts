import type { AnnotateState } from "@mind-imprint/contracts";

export type SourceFixture = {
  id: string;
  name: string;
  type: string;
  tier: string;
  craapLabel: string;
  role: string;
  locked: boolean;
  view: "article" | "summary";
  meta?: string;
  blocks: { id: string; text: string }[];
  takeaway?: string;
  annotate: AnnotateState;
};

// The calibration scenario (AGENTS.md): Phoebe, "中国是否让地球变得更可持续？" —
// a self-media blog riffing on real NASA/Boston University satellite findings
// (Chen et al. 2019, Nature Sustainability, DOI 10.1038/s41893-019-0220-7),
// then the actual NASA/Nature Sustainability finding itself as the source she
// traces back to. Real content, no lorem ipsum.

const BLOG_ID = "src-blog-china-greening";
const NASA_ID = "src-nasa-nature-sustainability";

const blogArticle: SourceFixture = {
  id: BLOG_ID,
  name: "《卫星图看中国变绿》",
  type: "自媒体",
  tier: "二手转述",
  craapLabel: "存疑",
  role: "触发关注的入口——数据引用听着权威，但结论被作者悄悄放大了，需要横向核实。",
  locked: false,
  view: "article",
  meta: "环球科技观察 · 公众号 · 3 天前",
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
  annotate: {
    material_id: BLOG_ID,
    spans: [
      {
        id: "span-blog-authority",
        block_ref: "b1",
        range: { start: 21, end: 33 },
        tag: "权威性",
        note: "「根据 NASA 卫星数据」——这条往上追，原始出处是谁？能找到 NASA 或论文本身吗，还是只是这篇公众号自己转述的？",
        author: "ai",
      },
      {
        id: "span-blog-purpose",
        block_ref: "b3",
        range: { start: 0, end: 63 },
        tag: "目的性",
        note: "作者把「变绿」直接等同于「环保政策奏效」「更可持续」，这个推论站得住吗？有没有被这篇文章悄悄绕开的对立事实（比如碳排放）？",
        author: "ai",
      },
    ],
  },
};

const nasaSummary: SourceFixture = {
  id: NASA_ID,
  name: "Chen et al. (2019), Nature Sustainability",
  type: "学者/机构",
  tier: "一手论文",
  craapLabel: "可信",
  role: "第一手数据来源，证实了「卫星观测到变绿」这件事本身是真的，但没有说这等于「更可持续」——变绿主要来自农业集约化与植树造林，论文本身并未涉及碳排放。",
  locked: true,
  view: "summary",
  meta: "Nature Sustainability · 影响因子 32.1 · 2019-02-11 · DOI 10.1038/s41893-019-0220-7",
  blocks: [
    {
      id: "b1",
      text: "基于 NASA MODIS 卫星 2000–2017 年数据：全球绿叶面积净增 5%，中国、印度合计贡献全球净增量的三分之一以上；增量主要来自农业集约化耕作与大规模植树工程，而非森林自然恢复。",
    },
  ],
  takeaway:
    "NASA 与 Nature Sustainability 指出：卫星数据确认地球在变绿，中国是最大贡献者之一，但主要机制是农业集约化与人工造林，不是整体生态系统改善——论文本身不支持「中国让地球更可持续」这个更大的结论，也没有讨论碳排放。",
  annotate: {
    material_id: NASA_ID,
    spans: [],
  },
};

export const SOURCE_FIXTURES: SourceFixture[] = [blogArticle, nasaSummary];
