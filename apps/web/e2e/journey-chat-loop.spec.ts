import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";

// J3 — Chat loop: coach-alone thread → live reply → (optional link→card offer) →
// generate a per-conversation 思维印记.
// The chat surface is coach-alone (planner off). Card-offer is classifier-gated
// (a link may or may not summon CRAAP under the live model), so it is attempted
// but not required. Assessment is flagship (can take a while).
const JOIN_CODE = "DEMO-0001";

test("J3: chat — thread → live reply → generate 思维印记", async ({ page }) => {
  test.setTimeout(180_000);
  const email = uniqueEmail("j3-student");
  await registerStudent(page, { name: "E2E 聊天", email, code: JOIN_CODE });

  // 1. Open 聊天 → empty state.
  await openRail(page, "聊天");
  await expect(page.getByText("还没有对话——把你正在想的、卡住的、好奇的说给它听。")).toBeVisible({ timeout: 15_000 });

  // Create the thread FIRST (see Finding C: sending with no active thread races
  // the load-messages effect and drops the first reply). With the thread already
  // active, the send doesn't change activeThreadId, so no race.
  await page.getByRole("button", { name: "新对话" }).first().click();

  const composer = page.getByPlaceholder("把你正在想的、卡住的、好奇的，说给它听……");
  const send = page.getByLabel("发送");
  const reportBtn = page.getByRole("button", { name: "生成本次对话的思维印记" });

  // 2. Send a plain message → thread auto-creates, coach replies live.
  const turn1 = page.waitForResponse(
    (r) => /\/chat\/threads\/[^/]+\/turn$/.test(r.url()) && r.request().method() === "POST",
    { timeout: 90_000 },
  );
  await composer.fill("我在纠结一个问题：刷到的养生说法到底能不能信？我该怎么想清楚？");
  await send.click();
  expect((await turn1).status()).toBe(200);
  // A landed assistant reply enables the report control (canOpenReport).
  await expect(reportBtn).toBeEnabled({ timeout: 60_000 });

  // 3. Optional: a message with a link MAY summon a CRAAP offer (classifier-gated,
  //    live variance). If it appears, dispose it (skip counts); otherwise proceed.
  const turn2 = page.waitForResponse(
    (r) => /\/chat\/threads\/[^/]+\/turn$/.test(r.url()) && r.request().method() === "POST",
    { timeout: 90_000 },
  );
  await composer.fill("我看到这篇文章说这个说法是对的：https://example.com/yangsheng — 我该不该信它？");
  await send.click();
  await turn2;
  const accept = page.getByRole("button", { name: "接受" });
  if (await accept.count()) {
    await accept.first().click();
    const skip = page.getByRole("button", { name: "跳过这张卡" });
    const submit = page.getByRole("button", { name: "提交并钉到过程树" });
    await expect(skip.or(submit).first()).toBeVisible({ timeout: 10_000 });
    if (await skip.count()) await skip.first().click();
    else await submit.first().click();
  }

  // 4. Generate the per-conversation 思维印记 (flagship). Header opens the report;
  //    the report's own button triggers the assessment POST. (Finding D fixed:
  //    the assess call now has adequate MaxTokens, so the flagship report is no
  //    longer truncated to empty.)
  await reportBtn.first().click();
  await expect(page.getByText("本次对话 · 思维印记")).toBeVisible({ timeout: 10_000 });
  const asmt = page.waitForResponse(
    (r) => /\/chat\/threads\/[^/]+\/assessment$/.test(r.url()) && r.request().method() === "POST",
    { timeout: 120_000 },
  );
  await page.getByRole("button", { name: "生成本次对话的思维印记" }).last().click();
  expect((await asmt).status()).toBe(200);
  // The report renders (generation finished ⇒ 返回对话 available).
  await expect(page.getByRole("button", { name: "返回对话" })).toBeVisible({ timeout: 120_000 });
});
