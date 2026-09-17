import { describe, expect, it } from "vitest";
import type { LiteMessage } from "@lite/api/readingRoom";
import { harvestBoards, harvestWords, harvestWritings } from "@lite/readings/ReadingHarvest";
import { isOneWord, toolsForSelection } from "@lite/readings/SelectionTools";

/**
 * 产品负责人 2026-09-18：
 *
 *   > reading notes is also: it should not only be lens. it should be the
 *   > grammar/words/my thoughts etc. it's my real reading 成果
 *   > when I select a word or a sentence, my selection disappears and cannot
 *   > ask a word's meaning
 *
 * 这里测的是那两件事里读代码看不出对错的部分：从转写里把她摆过的板、写过的话
 * 认回来（认错了这一页就少一样东西），以及「划的这几个字该配哪几件工具」。
 */

function ai(seq: number, card: unknown): LiteMessage {
  return { seq, role: "ai", content: "给你一块板。", createdAt: "", payload: { card } } as LiteMessage;
}

function mine(seq: number, answer: unknown): LiteMessage {
  return { seq, role: "student", content: "", createdAt: "", payload: { answer } } as LiteMessage;
}

const SPANS = [
  { blockId: "b1", quote: "The storm reached the coast on Friday." },
  { blockId: "b2", quote: "Officials ordered an evacuation on Wednesday." },
];

describe("阅读成果：从转写里认回她摆过的板", () => {
  it("标注板按格子分组，只留她真放进去的那几句", () => {
    const card = { type: "label_roles", prompt: "判断下列句子属于哪一类。", options: SPANS, labels: ["事实", "引述"] };
    const msgs = [
      ai(1, card),
      mine(2, {
        type: "label_roles",
        prompt: card.prompt,
        choice: "事实：\nThe storm reached the coast on Friday.",
      }),
    ];
    expect(harvestBoards(msgs)).toEqual([
      { kind: "label", prompt: card.prompt, groups: [{ bin: "事实", quotes: ["The storm reached the coast on Friday."] }] },
    ]);
  });

  it("排序板留下她排的先后", () => {
    const card = { type: "order_events", prompt: "请按事情发生的先后排列。", options: SPANS };
    const msgs = [
      ai(1, card),
      mine(2, {
        type: "order_events",
        prompt: card.prompt,
        choice: "第1：\nOfficials ordered an evacuation on Wednesday.\n第2：\nThe storm reached the coast on Friday.",
      }),
    ];
    expect(harvestBoards(msgs)[0]?.order).toEqual([
      "Officials ordered an evacuation on Wednesday.",
      "The storm reached the coast on Friday.",
    ]);
  });

  it("没答的板不算成果；配不上卡片的作答也不编一块板出来", () => {
    const card = { type: "label_roles", prompt: "p", options: SPANS, labels: ["事实"] };
    expect(harvestBoards([ai(1, card)])).toEqual([]);
    expect(harvestBoards([mine(2, { type: "label_roles", prompt: "p", choice: "事实：\nx" })])).toEqual([]);
  });
});

describe("阅读成果：她自己写的和她学过的词", () => {
  it("段落工具底下写的那几段是她的话", () => {
    const msgs = [
      mine(1, { type: "block_tool", prompt: "> 【印记问】仿写 · 第3段：先给场景再讲原理", choice: "我家楼下的操场……" }),
      mine(2, { type: "short_text", prompt: "你怎么看？", choice: "我觉得证据不够。" }),
    ];
    expect(harvestWritings(msgs)).toEqual([
      { prompt: "> 【印记问】仿写 · 第3段：先给场景再讲原理", text: "我家楼下的操场……" },
    ]);
  });

  it("同一个词只留一张卡，大小写不算两个词", () => {
    const words = harvestWords([
      { blockId: "b1", tool: "vocabulary", body: "", words: [{ term: "scramble", meaning: "抢着做" }] },
      { blockId: "b2", tool: "lookup", body: "", words: [{ term: "Scramble", meaning: "抢着做" }, { term: "levee", meaning: "堤坝" }] },
    ] as never);
    expect(words.map((w) => w.term)).toEqual(["scramble", "levee"]);
  });
});

describe("划选之后该配哪几件工具", () => {
  const tools = [
    { id: "translate", label: "翻译" },
    { id: "lookup", label: "查词", subject: "word" },
    { id: "grammar", label: "语法", subject: "sentence" },
  ];

  it("一个词给查词，一句话给语法", () => {
    expect(toolsForSelection(tools, "prolonged").map((t) => t.id)).toEqual(["lookup"]);
    expect(toolsForSelection(tools, "The storm reached the coast.").map((t) => t.id)).toEqual(["grammar"]);
    // 整段工具（翻译）不上这条工具条：它讲的是整段，不是她划的那几个字。
    expect(toolsForSelection(tools, "prolonged").some((t) => t.id === "translate")).toBe(false);
  });

  it("中文按字数判，英文按空白判", () => {
    expect(isOneWord("热岛效应")).toBe(true);
    expect(isOneWord("城市热岛效应很明显")).toBe(false);
    expect(isOneWord("well-being")).toBe(true);
    expect(isOneWord("  ")).toBe(false);
  });

  it("目录里一件都配不上就不该有这条工具条", () => {
    expect(toolsForSelection([{ id: "rhetoric", label: "成语修辞" }], "热岛")).toEqual([]);
  });
});
