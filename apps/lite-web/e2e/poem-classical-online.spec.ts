import { test, expect } from "@playwright/test";

/**
 * 诗词和文言文，在**线上**真的走一遍。
 *
 * 产品负责人 2026-09-23：「since we will face reading poems/文言文 in chinese
 * reading…… these are very important scene in junior study.」
 * 以及：「please always think from the students' perspective and really decide
 * if the function is useful, attractive, and educative for a real student,
 * instead of just "all green", "no 50x".」
 *
 * 🚨 所以这一条断言的不是「没报错」，而是**她那一屏上真的换了东西**：
 *   - 体裁判得出来（不再落到记叙 / 说明上）；
 *   - 清单换成了这一体裁自己的那一套（有 shape、有它自己的标注步骤）；
 *   - 段落工具条上出现了只有这一体裁才有的那几件（字词释义 / 句法 / 意象），
 *     而别的体裁上没有。
 *
 * 用的是产品负责人给的那两份语料里的真篇目（《咏雪》/《世说新语》，
 * 《江雪》），不是我编的文本 —— 编出来的文本判得准，不说明真语料判得准。
 */

const API = process.env.E2E_API_BASE_URL ?? "https://mind-api.uni-robot.cn";

/** 《咏雪》—— 语料 wenyan-3001，初中第七卷上册。 */
const YONGXUE = `谢太傅寒雪日内集，与儿女讲论文义。

俄而雪骤，公欣然曰："白雪纷纷何所似？"兄子胡儿曰："撒盐空中差可拟。"兄女曰："未若柳絮因风起。"公大笑乐。

即公大兄无奕女，左将军王凝之妻也。`;

/** 《江雪》柳宗元 —— 五言绝句，小学/初中都读。 */
const JIANGXUE = `千山鸟飞绝，万径人踪灭。

孤舟蓑笠翁，独钓寒江雪。`;

type Plan = { routineKey?: string; tasks?: { kind: string }[] };

async function openReading(
  ctx: import("@playwright/test").APIRequestContext,
  title: string,
  text: string,
) {
  const made = await ctx.post(`${API}/api/v1/readings`, { data: { title, lang: "zh" } });
  expect(made.ok(), `建阅读失败 ${made.status()} ${await made.text()}`).toBeTruthy();
  const id = (await made.json()).id as string;

  const put = await ctx.put(`${API}/api/v1/readings/${id}/source`, { data: { title, text } });
  expect(put.ok(), `贴正文失败 ${put.status()} ${await put.text()}`).toBeTruthy();

  const gen = await ctx.post(`${API}/api/v1/readings/${id}/plan`, { data: {} });
  expect(gen.ok(), `排读法失败 ${gen.status()} ${await gen.text()}`).toBeTruthy();

  const plan = (await (await ctx.get(`${API}/api/v1/readings/${id}/plan`)).json()) as Plan;
  const tools = (await (
    await ctx.get(`${API}/api/v1/readings/${id}/blocks/tools`)
  ).json()) as unknown;
  return { id, plan, tools };
}

function toolIDs(tools: unknown): string[] {
  const rows = Array.isArray(tools) ? tools : ((tools as { tools?: unknown[] })?.tools ?? []);
  return (rows as { id?: string }[]).map((t) => t.id ?? "").filter(Boolean);
}

test("文言文：《咏雪》判成 classical，清单和工具都换了", async ({ request }) => {
  const { plan, tools } = await openReading(request, "咏雪", YONGXUE);
  const kinds = (plan.tasks ?? []).map((t) => t.kind);
  const ids = toolIDs(tools);

  // eslint-disable-next-line no-console
  console.log(`《咏雪》 routineKey=${plan.routineKey} 步骤=${kinds.join(",")}\n  工具=${ids.join(",")}`);

  expect(plan.routineKey, "《咏雪》没有落到文言文那套读法上").toBe("zh-classical");
  // 🚨 整篇那一层要有 shape —— 文言文最常见的形状是先叙后议，
  // 而她要学会的正是认出那个「议」从哪一句开始。
  expect(kinds, "清单里没有 shape").toContain("shape");
  // 🚨 只有文言文才有的两件工具真的到了她的工具条上。
  expect(ids, "工具条上没有字词释义").toContain("classical_words");
  expect(ids, "工具条上没有句法").toContain("classical_syntax");
  expect(ids, "文言文上不该有诗词的意象工具").not.toContain("poem_images");
});

test("诗词：《江雪》判成 poem，没有排序板那一步", async ({ request }) => {
  const { plan, tools } = await openReading(request, "江雪", JIANGXUE);
  const kinds = (plan.tasks ?? []).map((t) => t.kind);
  const ids = toolIDs(tools);

  // eslint-disable-next-line no-console
  console.log(`《江雪》 routineKey=${plan.routineKey} 步骤=${kinds.join(",")}\n  工具=${ids.join(",")}`);

  expect(plan.routineKey, "《江雪》没有落到诗词那套读法上").toBe("zh-poem");
  expect(kinds, "清单里没有 shape").toContain("shape");
  // 🚨 一首诗上没有「几件事的先后」可排。摆一块排序板给她，等于要她把
  // 「千山鸟飞绝 / 万径人踪灭」排出先后 —— 那不是这首诗里的东西。
  expect(kinds, "诗词上排了事件顺序那一步").not.toContain("sequence");
  expect(ids, "工具条上没有意象").toContain("poem_images");
  expect(ids, "诗词上不该有文言文的字词释义").not.toContain("classical_words");
});

// 🚨 反方向：一篇现代白话文上，那三件专用工具一件都不许出现。
// 一个按下去讲不出东西的按钮，会让她不信任整条工具条。
test("现代白话文上没有那三件专用工具", async ({ request }) => {
  const text = `海水为什么是咸的？

海水里的盐，一部分来自陆地。雨水冲刷岩石，把其中的矿物质带进河流，再由河流汇入海洋。

另一部分来自海底。海底火山和热液喷口不断向海水里释放矿物质。

水会蒸发，盐留下来，于是海水越来越咸。`;
  const { plan, tools } = await openReading(request, "海水为什么是咸的", text);
  const ids = toolIDs(tools);
  // eslint-disable-next-line no-console
  console.log(`说明文 routineKey=${plan.routineKey}\n  工具=${ids.join(",")}`);
  for (const id of ["classical_words", "classical_syntax", "poem_images"]) {
    expect(ids, `白话文的工具条上摆了 ${id}`).not.toContain(id);
  }
  // 原来那三件中文工具一件都没丢。
  for (const id of ["rhetoric", "examples", "structure"]) {
    expect(ids, `白话文上少了原有的 ${id}`).toContain(id);
  }
});
