import { test, expect } from "@playwright/test";
import { registerStudent, openRail, createPaper, coachSend, uniqueEmail } from "./helpers";

// J1 — New student, first arrival → first value.
// Proves the real arrive→act→see loop on the CURRENT refactored UI:
//   empty states → create a paper → the coach replies live → the paper is now
//   visible in the 工作室 directory.
// Truthful to the product: 成长报告 is populated by EVALUATIONS, so it stays
// empty for a brand-new in-progress project (finish+assessment is exercised by
// the student-project function spec, not here). Card-summon in the studio is
// station+material gated, so it is NOT forced here either.
const JOIN_CODE = "DEMO-0001"; // seeded Demo Class

test("J1: new student — empty → create paper → coach replies → paper listed", async ({ page }) => {
  const email = uniqueEmail("j1-student");

  // 1. Register with a class join code → lands on 工作室 (StudentApp default tab).
  await registerStudent(page, { name: "E2E 新生", email, code: JOIN_CODE });
  await expect(page.getByRole("tab", { name: "工作室" })).toBeVisible();

  // 2. Empty states before doing anything — no crash on any surface.
  await expect(page.getByText("还没有论文。点「新建论文」，贴上你的任务，就能开始。")).toBeVisible();
  await openRail(page, "成长报告");
  await expect(page.getByText("还没有报告")).toBeVisible();
  await page.getByRole("button", { name: "工具卡" }).click();
  await expect(page.getByText("还没有收集到工具卡")).toBeVisible();
  await page.getByRole("button", { name: "能力素养" }).click();
  await expect(page.getByText("还没有足够的数据")).toBeVisible();

  // 3. Create a paper via the funnel → workspace opens (coach composer present).
  await openRail(page, "工作室");
  await createPaper(page, {
    title: "TOK · 中国与可持续",
    prompt:
      "我在写 TOK：中国是否让地球更可持续？我找到一篇文章链接，想先判断它可不可信再用。https://example.com/china-sustainability",
  });

  // 4. Live coach turn: the AI 陪练 replies. Assert the round-trip at the network
  //    level (POST /turn → 200) — the robust signal that the live coach worked.
  const turnResp = page.waitForResponse(
    (r) => r.url().includes("/turn") && r.request().method() === "POST",
    { timeout: 60_000 },
  );
  await coachSend(page, "帮我看看这篇文章，我该从哪里开始判断它可不可信？");
  expect((await turnResp).status()).toBe(200);
  // Composer returns to ready after the reply streams in.
  await expect(page.getByPlaceholder("把你的想法发给印记……")).toBeEnabled({ timeout: 60_000 });

  // 5. See the result: reload → 工作室 directory now lists the created paper
  //    (arrive→act→see, persisted through the API).
  await page.goto("/");
  await expect(page.getByRole("tab", { name: "工作室" })).toBeVisible();
  const row = page.getByTestId("directory-project-row");
  await expect(row.filter({ hasText: "TOK · 中国与可持续" })).toBeVisible({ timeout: 15_000 });

  // 6. Truth-check: 成长报告 is still empty — in-progress work is NOT a report
  //    (成长报告 is populated by evaluations, exercised in the project spec).
  await openRail(page, "成长报告");
  await expect(page.getByText("还没有报告")).toBeVisible();
});
