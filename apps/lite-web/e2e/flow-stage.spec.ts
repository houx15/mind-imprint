import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 线上验 R3 的行文这一步（同事 2026-09-20 的意见 4）。
 *
 * 🚨 这条守的是**迁移**那一半，看图台守不到：
 *
 *  1. `stage = "flow"` 真的存得进去 —— 0099 把 `CHECK (stage IN …)` 写在列
 *     定义上，0184 要放开它。只改 Go 的枚举而漏了约束，她点进行文的第一次
 *     保存就是一个数据库层的冲突，以一个读不懂的 500 出现在她面前。
 *  2. `writing_outline.method` 和 `writing.structure_key` 真的落库、真的读得回来。
 *
 * 界面那一半（四张卡、下拉、这一步不写正文）由 `e2e/harness/flow-harness.spec.ts`
 * 在真浏览器里守着，不连后端、跑得快。
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

test("行文：stage 存得进去，结构和方法都落得了库", async ({ browser }) => {
  test.setTimeout(180_000);

  const ctx = await freshAccount(browser, "flow-stage");

  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "学校应不应该允许学生带手机", lang: "zh" },
  });
  expect(created.ok(), `创建写作失败：${created.status()}`).toBeTruthy();
  const writingId = (await created.json()).id as string;

  const setup = await ctx.request.put(`${API}/api/v1/writings/${writingId}/setup`, {
    data: { lang: "zh", targetWords: 800, note: "" },
  });
  expect(setup.ok(), `设定失败：${setup.status()}`).toBeTruthy();

  const put = await ctx.request.put(`${API}/api/v1/writings/${writingId}/outline`, {
    data: {
      outline: [
        { text: "学校应该允许学生带手机", kind: "thesis", depth: 0 },
        { text: "放学能联系家长", kind: "point", depth: 1 },
        { text: "学习上能查不会的题", kind: "point", depth: 1 },
      ],
    },
  });
  expect(put.ok(), `摆图失败：${put.status()}`).toBeTruthy();

  // 🚨 第一条：stage = flow 存得进去吗。0184 没放开那条 CHECK 的话，这里 500。
  const staged = await ctx.request.post(`${API}/api/v1/writings/${writingId}/stage`, {
    data: { stage: "flow" },
  });
  expect(
    staged.ok(),
    `切到行文失败：${staged.status()} ${await staged.text()} —— 多半是 0099 那条 stage CHECK 没被 0184 放开`,
  ).toBeTruthy();
  expect((await staged.json()).stage).toBe("flow");

  // 那四条论证结构读得到，而且每一条都带一句示范。
  const structures = await ctx.request.get(`${API}/api/v1/writings/${writingId}/flow/structures`);
  expect(structures.ok(), `读结构失败：${structures.status()}`).toBeTruthy();
  const body = (await structures.json()) as {
    structures: { id: string; name: string; example: string }[];
    methods: { id: string; name: string }[];
  };
  expect(body.structures.map((s) => s.name).sort()).toEqual(["并列式", "层进式", "总分式", "对照式"].sort());
  for (const s of body.structures) {
    expect(s.example, `${s.name} 没有示范`).not.toBe("");
  }
  // 论证方法里要有教辅那七种里新补的几个。
  const methodNames = body.methods.map((m) => m.name);
  for (const want of ["举例论证", "引用论证", "比喻论证", "类比论证", "归谬论证", "因果论证"]) {
    expect(methodNames, `论证方法里缺 ${want}`).toContain(want);
  }
  // 🚨 整篇层的四条**不许**出现在这个下拉里 ——「这一段用总分式」是句错话。
  for (const bad of ["总分式", "并列式", "层进式", "对照式"]) {
    expect(methodNames, `整篇层的 ${bad} 漏进了每一块的方法下拉`).not.toContain(bad);
  }

  // 🚨 第二条：结构和方法落得了库、读得回来。
  const outline = (await (await ctx.request.get(`${API}/api/v1/writings/${writingId}/outline`)).json())
    .outline as { id: string; kind: string; text: string }[];
  const firstPoint = outline.find((o) => o.kind === "point")!;
  expect(firstPoint, "分论点不见了").toBeTruthy();

  const saved = await ctx.request.put(`${API}/api/v1/writings/${writingId}/flow`, {
    data: {
      structureKey: "struct_parallel",
      methods: { [firstPoint.id]: "point_quote" },
    },
  });
  expect(saved.ok(), `存行文失败：${saved.status()} ${await saved.text()}`).toBeTruthy();

  const back = (await (await ctx.request.get(`${API}/api/v1/writings/${writingId}/outline`)).json())
    .outline as { id: string; method?: string }[];
  expect(back.find((o) => o.id === firstPoint.id)?.method, "这一块的论证方法没存上").toBe("point_quote");

  const wr = await (await ctx.request.get(`${API}/api/v1/writings/${writingId}`)).json();
  expect(wr.structureKey, "整篇的论证结构没存上").toBe("struct_parallel");

  // 编出来的 id 清空，不整份拒绝 —— 她别的改动不该被一个认不出的值拖下水。
  const junk = await ctx.request.put(`${API}/api/v1/writings/${writingId}/flow`, {
    data: { structureKey: "我编的一种", methods: { [firstPoint.id]: "struct_total_part" } },
  });
  expect(junk.ok(), "认不出的值该被清空，不该整份拒绝").toBeTruthy();
  const after = (await (await ctx.request.get(`${API}/api/v1/writings/${writingId}/outline`)).json())
    .outline as { id: string; method?: string }[];
  expect(after.find((o) => o.id === firstPoint.id)?.method, "整篇层的结构不该能标在一块上").toBe("");

  await ctx.close();
});
