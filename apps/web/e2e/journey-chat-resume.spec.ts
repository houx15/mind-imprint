import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";

// J-resume — mid-interruption + resume. A student holds a multi-round chat,
// leaves (full page reload = close tab / come back later), and returns to find
// the transcript rehydrated and the thread continuable. This exercises the
// server-side persistence + mount rehydration path (listThreads →
// activeThreadId=list[0] → getMessages) and, crucially, proves the Finding C
// fix (pendingLocalThreadRef consumed on first activation) does NOT suppress a
// later reload: a returning student MUST see their history.
const JOIN_CODE = "DEMO-0001";

const ROUND_1 = "我在纠结一个问题：网上刷到的减肥说法到底能不能信？";
const ROUND_2 = "如果我想自己判断，第一步应该先看什么？";
const ROUND_3 = "那如果不同来源说法互相矛盾呢？";
const ROUND_4 = "谢谢，那我们回到最开始那个减肥说法——我该怎么给它打分？";

async function sendRound(page: import("@playwright/test").Page, text: string) {
  const composer = page.getByPlaceholder("把你正在想的、卡住的、好奇的，说给它听……");
  const send = page.getByLabel("发送");
  const turn = page.waitForResponse(
    (r) => /\/chat\/threads\/[^/]+\/turn$/.test(r.url()) && r.request().method() === "POST",
    { timeout: 90_000 },
  );
  await composer.fill(text);
  await send.click();
  expect((await turn).status()).toBe(200);
}

test("J-resume: 3-round chat → leave (reload) → transcript rehydrated → resume 4th round", async ({ page }) => {
  test.setTimeout(240_000);
  const email = uniqueEmail("jresume-student");
  await registerStudent(page, { name: "E2E 续聊", email, code: JOIN_CODE });

  await openRail(page, "聊天");
  await expect(page.getByText("还没有对话——把你正在想的、卡住的、好奇的说给它听。")).toBeVisible({ timeout: 15_000 });

  // Three rounds. First send is into the empty surface (Finding C path).
  await sendRound(page, ROUND_1);
  await sendRound(page, ROUND_2);
  await sendRound(page, ROUND_3);

  // All three of the student's messages are on screen before leaving.
  await expect(page.getByText(ROUND_1)).toBeVisible();
  await expect(page.getByText(ROUND_2)).toBeVisible();
  await expect(page.getByText(ROUND_3)).toBeVisible();
  // At least one assistant reply landed (report control becomes enabled).
  await expect(page.getByRole("button", { name: "生成本次对话的思维印记" })).toBeEnabled({ timeout: 60_000 });

  // ── Leave and come back: a full reload drops all in-memory state; the app
  //    must reconstruct the session from the server (session cookie survives). ──
  await page.reload();
  await openRail(page, "聊天");

  // The transcript is rehydrated from the server — every student turn is back.
  // This is the resume guarantee, and it proves the Finding C flag did not
  // wrongly suppress the reload on return.
  await expect(page.getByText(ROUND_1)).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(ROUND_2)).toBeVisible();
  await expect(page.getByText(ROUND_3)).toBeVisible();

  // The thread is still continuable: a 4th round posts to the SAME thread and
  // gets a live reply.
  await sendRound(page, ROUND_4);
  await expect(page.getByText(ROUND_4)).toBeVisible();

  // Reload once more and confirm the 4th round persisted too (full durability).
  await page.reload();
  await openRail(page, "聊天");
  await expect(page.getByText(ROUND_4)).toBeVisible({ timeout: 30_000 });
});
