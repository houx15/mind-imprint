import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 线上验 R4 —— 那位语文老师的讲义装进陪练之后，真的到得了她那边吗。
 *
 * 🚨 这条守的是**看图台和 Go 测试都守不到的那一半**：文体这条轴要从
 * 「她建了一篇什么」一路传到立题、传到段落意见，中间隔着 HTTP、隔着
 * 一次真的模型调用。任何一段漏了，本地全绿而线上照旧拿议论文的词去量
 * 一篇记叙文。
 *
 * 跑法：
 *   npx playwright test -c e2e/online.config.ts teaching-r4
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

test("记叙文：立题开出的是场景，不是中心论点", async ({ browser }) => {
  test.setTimeout(240_000);
  const ctx = await freshAccount(browser, "r4-narrative");

  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "记一次难忘的经历", lang: "zh" },
  });
  expect(created.ok(), `创建写作失败：${created.status()}`).toBeTruthy();
  const id = (await created.json()).id as string;

  const setup = await ctx.request.put(`${API}/api/v1/writings/${id}/setup`, {
    data: { lang: "zh", targetWords: 800, note: "" },
  });
  expect(setup.ok(), `设定失败：${setup.status()}`).toBeTruthy();

  const wr0 = await (await ctx.request.get(`${API}/api/v1/writings/${id}`)).json();
  console.log("题目 =", JSON.stringify(wr0.title), "| lang =", wr0.lang, "| stage =", wr0.stage);

  // 真的走一轮立题。
  const turn = await ctx.request.post(`${API}/api/v1/writings/${id}/plan/turn`, {
    data: { text: "我想写那天下大雨，我爸来补习班接我的事。我在楼道口等了很久。" },
  });
  expect(turn.ok(), `立题一轮失败：${turn.status()} ${await turn.text()}`).toBeTruthy();
  const body = await turn.json();
  console.log("印记说：", body.reply);

  const outline = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/outline`)).json())
    .outline as { kind: string; text: string }[];
  console.log("图上：", outline.map((o) => `${o.kind}=${o.text}`).join(" | "));

  // 🚨 这一篇上开出来的块不该是议论文的。中心论点／分论点在一篇记叙文里
  // 无处安放 —— R4 之前这就是她会拿到的东西。
  const argumentKinds = ["thesis", "point", "counter", "rebuttal", "reference", "reasoning"];
  for (const row of outline) {
    expect(
      argumentKinds,
      `记叙文那一篇开出了议论文的块「${row.kind}」（${row.text}）—— 文体这条轴没接到线上`,
    ).not.toContain(row.kind);
  }

  await ctx.close();
});

test("议论文：一段有例子没分析，印记叫得出「分析句」这个名字", async ({ browser }) => {
  test.setTimeout(240_000);
  const ctx = await freshAccount(browser, "r4-analysis");

  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "坚持的意义", lang: "zh" },
  });
  expect(created.ok(), `创建写作失败：${created.status()}`).toBeTruthy();
  const id = (await created.json()).id as string;

  await ctx.request.put(`${API}/api/v1/writings/${id}/setup`, {
    data: { lang: "zh", targetWords: 800, note: "" },
  });

  const put = await ctx.request.put(`${API}/api/v1/writings/${id}/outline`, {
    data: {
      outline: [
        { text: "坚持能让一个人走得很远", kind: "thesis", depth: 0 },
        { text: "坚持让普通的才能变得出众", kind: "point", depth: 1 },
      ],
    },
  });
  expect(put.ok(), `摆图失败：${put.status()}`).toBeTruthy();

  const outline = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/outline`)).json())
    .outline as { id: string; kind: string }[];
  const point = outline.find((o) => o.kind === "point")!;

  await ctx.request.post(`${API}/api/v1/writings/${id}/stage`, { data: { stage: "snippets" } });

  // 一段标准的「观点句 + 材料句，然后就没了」—— 缺的是分析句。
  // 🚨 段落是 PUT（整批 upsert），不是 POST —— 打错方法回的是 405，
  // 看起来像接口没了。
  const snip = await ctx.request.put(`${API}/api/v1/writings/${id}/snippets`, {
    data: {
      snippets: [
        {
          outlineId: point.id,
          position: 0,
          text: "坚持让普通的才能变得出众。王羲之九岁开始练字，无论严寒酷暑还是刮风下雨都不间断，他在绍兴兰亭的一个水池边练字，池水都被他洗笔砚染黑了。他的字千百年来被人们奉为瑰宝。",
        },
      ],
    },
  });
  expect(snip.ok(), `存段落失败：${snip.status()} ${await snip.text()}`).toBeTruthy();

  const snippets = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/snippets`)).json())
    .snippets as { id: string; text: string }[];
  expect(snippets.length, "段落没存上").toBeGreaterThan(0);
  const sid = snippets[0].id;

  const res = await ctx.request.post(`${API}/api/v1/writings/${id}/snippets/${sid}/comment`);
  expect(res.ok(), `请印记看一看失败：${res.status()} ${await res.text()}`).toBeTruthy();
  const comment = (await res.json()).comment as {
    verdict: string;
    summary: string;
    points: { kind: string; text: string; action: string }[];
  };
  console.log("verdict =", comment.verdict);
  console.log("总评 =", comment.summary);
  for (const p of comment.points) console.log(`  [${p.kind}] ${p.text} / ${p.action}`);

  // 分级是 R2 定的三档之一。
  expect(["pass", "polish", "revise"]).toContain(comment.verdict);

  // 🚨 正题：缺的那一句，它叫得出名字吗。
  // R2 上线那天真模型说的是「还差一句把它和主张连起来的话」—— 说的是缺什么，
  // 没说那一句叫什么。讲义给了它三个名字。
  const all = comment.summary + comment.points.map((p) => p.text + p.action).join("");
  expect(
    all.includes("分析句") || all.includes("阐释句"),
    `印记没叫出缺的那一句的名字。检查表里的五句型没到线上。它说的是：\n${all}`,
  ).toBeTruthy();

  await ctx.close();
});
