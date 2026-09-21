import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 线上验 R2：**开头段不再被要求补一个具体的例子**。
 *
 * 🚨 同事 2026-09-20 的意见 9，原话：
 *
 *	「这个第一段的分析，这个具体的举例写在了第二段和第三段，但是 ai 在分析
 *	  第一段的时候没有进行关联，也不知道开头段只是一个引子的作用，
 *	  给出了错误的分析结果。」
 *
 * 这条走的是**真的线上**：真模型、真数据库、真的那条 `AI审阅这一段`。
 * 服务端那一半（整篇上下文、按 kind 分派、两项减法）由 Go 测试守着；
 * 这一条守的是它们接起来之后，屏幕上到底写了什么。
 *
 * 🚨 这条会花一次真的模型调用。它只跑一次，不重试（online.config 里
 * retries: 0）—— 一次 flake 值得报告，不值得付两次钱。
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

test("段落：开头段写完主张之后，印记不再要求它自带一个具体的例子", async ({ browser }) => {
  test.setTimeout(240_000);

  const ctx = await freshAccount(browser, "comment-verdict");
  const page = await ctx.newPage();
  page.on("dialog", (d) => void d.dismiss());

  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "学校应不应该允许学生带手机", lang: "zh" },
  });
  expect(created.ok(), `创建写作失败：${created.status()}`).toBeTruthy();
  const writingId = (await created.json()).id as string;

  const setup = await ctx.request.put(`${API}/api/v1/writings/${writingId}/setup`, {
    data: { lang: "zh", targetWords: 500, note: "" },
  });
  expect(setup.ok(), `设定失败：${setup.status()}`).toBeTruthy();

  // 同事截图里那一篇的结构。
  const put = await ctx.request.put(`${API}/api/v1/writings/${writingId}/outline`, {
    data: {
      outline: [
        { text: "学校应该允许学生带手机", kind: "thesis", depth: 0 },
        { text: "放学能联系家长", kind: "point", depth: 1 },
        { text: "学习上能查不会的题", kind: "point", depth: 1 },
        { text: "允许带，但老师同意才能拿出来", kind: "closing", depth: 0 },
      ],
    },
  });
  expect(put.ok(), `摆图失败：${put.status()}`).toBeTruthy();

  const outline = (await (await ctx.request.get(`${API}/api/v1/writings/${writingId}/outline`)).json())
    .outline as { id: string; kind: string; position: number; text: string }[];
  const thesis = outline.find((o) => o.kind === "thesis")!;
  const points = outline.filter((o) => o.kind === "point");
  expect(thesis, "中心论点那一条不见了").toBeTruthy();
  expect(points.length, "两条分论点不见了").toBe(2);

  // 三段正文：开头是两句判断，具体的事在后面两段里 —— 截图里就是这个形状。
  const opening = "手机可以帮助我们联系家长，也能用来学习。所以我觉得学校应该允许学生带手机。";
  const bodies = [
    "上周三五点半我放学等车，公交迟迟不来，我用手机给我妈打了电话，她开车来接的我。",
    "有一次英语作业里有个单词我不认识，我用手机查了意思，还听了发音，才看懂那句话。",
  ];
  // 段落是**整批**存的（PUT /snippets 收 {snippets: [...]}）。
  const saved = await ctx.request.put(`${API}/api/v1/writings/${writingId}/snippets`, {
    data: {
      snippets: [thesis, ...points].map((node, i) => ({
        outlineId: node.id,
        position: node.position,
        text: i === 0 ? opening : bodies[i - 1]!,
      })),
    },
  });
  expect(saved.ok(), `存段落失败：${saved.status()} ${await saved.text()}`).toBeTruthy();

  // 拿开头那一段去要一次意见 —— 这就是截图里她按的那颗按钮。
  const snippets = (await (await ctx.request.get(`${API}/api/v1/writings/${writingId}/snippets`)).json())
    .snippets as { id: string; outlineId: string | null; text: string }[];
  const openingSnippet = snippets.find((s) => s.outlineId === thesis.id);
  expect(openingSnippet, "开头那一段没存上").toBeTruthy();

  const res = await ctx.request.post(
    `${API}/api/v1/writings/${writingId}/snippets/${openingSnippet!.id}/comment`,
    { timeout: 180_000 },
  );
  expect(res.ok(), `要意见失败：${res.status()} ${await res.text()}`).toBeTruthy();
  const comment = (await res.json()).comment as {
    verdict?: string;
    summary: string;
    points: { kind?: string; text: string; action?: string }[];
  };

  // eslint-disable-next-line no-console
  console.log("线上回的：", JSON.stringify(comment, null, 2));

  // 分级要有，而且是闭表里的那三个之一。
  expect(["pass", "polish", "revise"], `verdict=${comment.verdict}`).toContain(comment.verdict);

  // 🚨 这就是同事那句话：开头**不该**被要求补一个具体的例子 ——
  // 那两件事就写在后面两段里，而服务端现在把它们喂给了模型。
  const issues = comment.points.filter((p) => p.kind === "issue");
  const said = issues.map((p) => `${p.text}${p.action ?? ""}`).join("\n");
  for (const bad of ["具体的事", "举一个例子", "举个例子", "没有例子", "缺少例子"]) {
    expect(said, `还在要求开头补例子（「${bad}」）：\n${said}`).not.toContain(bad);
  }

  await ctx.close();
});
