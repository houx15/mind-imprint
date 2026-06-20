export const PHOEBE_VALUES: Record<string, Record<string, unknown>> = {
  sift_craap: {
    stop: "我想用这条信息支持「中国让地球更可持续」。",
    sources: [
      { name: "公众号《环球科技》", type: "自媒体", verdict: "存疑" },
      { name: "NASA Earth Observatory", type: "官方", verdict: "可信" },
      { name: "Nature Sustainability (IF 32.1)", type: "学者/机构", verdict: "可信" },
    ],
    better: "NASA 与 Nature Sustainability 指出变绿主要来自农业集约化与植树，并非整体生态改善。",
    trace: "https://www.nature.com/articles/s41893-019-0220-7",
  },
  concession: {
    thesis: "中国在很大程度上让地球更可持续。",
    counter: "中国是全球碳排放总量第一。",
    concede: "确实，中国的碳排放总量目前居全球首位。",
    rebut: "但其人均排放低于多数发达国家，且在可再生能源装机与植被恢复上贡献全球领先——总量第一不足以推翻其在可持续上的净贡献。",
  },
};
