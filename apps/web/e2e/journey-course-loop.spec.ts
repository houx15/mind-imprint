import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";
import { studentReply } from "./student-sim";

// J2 — Course runtime loop (start → page → ask → advance-gate).
//
// SCOPE NOTE: full course COMPLETION is blocked by Finding A
// (docs/2026-07-25-e2e-findings.md) — the live coach never emits the `advance`
// verb, so no session reaches `finished` regardless of engagement. This spec
// therefore verifies the course RUNTIME mechanics that DO work reliably:
// session start, step render + local paging, the 问印记 ask round-trip, and that
// a boundary `下一步` invokes the advance gate. Completion is a manual pre-deploy
// check until Finding A is resolved.
const JOIN_CODE = "DEMO-0001";
const COURSE_TITLE = "一条网络信息，该不该信";

test("J2: course runtime — open → page → ask → advance-gate fires", async ({ page }) => {
  test.setTimeout(180_000);
  const email = uniqueEmail("j2-student");
  await registerStudent(page, { name: "E2E 学员", email, code: JOIN_CODE });

  // 1. Open the course → player: nav + phase + step counter present.
  await openRail(page, "课程");
  await page.getByText(COURSE_TITLE).first().click();
  const nextBtn = page.getByLabel("下一步");
  await expect(nextBtn).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText("演示").first()).toBeVisible();
  await expect(page.getByText(/1\s*\/\s*3/)).toBeVisible();

  // 2. Page within 演示: 下一步 renders the next step locally (no gate here).
  await nextBtn.click();
  await expect(page.getByText(/2\s*\/\s*3/)).toBeVisible({ timeout: 15_000 });

  // 3. 问印记 ask round-trip (live): expand the panel, answer via the student-sim,
  //    assert the POST /session/ask succeeds and the student's turn renders.
  const expandAsk = page.getByLabel("展开问印记");
  if (await expandAsk.count()) await expandAsk.click();
  const askInput = page.getByPlaceholder("输入你的问题……");
  await expect(askInput).toBeVisible({ timeout: 10_000 });
  const reply = await studentReply("老师问：随手相信一条没核实的网络信息，可能有什么风险？");
  const askResp = page.waitForResponse(
    (r) => r.url().includes("/session/ask") && r.request().method() === "POST",
    { timeout: 90_000 },
  );
  await askInput.fill(reply);
  await page.getByLabel("发送").click();
  expect((await askResp).status()).toBe(200);
  await expect(page.getByTestId("ask-bubble").first()).toBeVisible({ timeout: 15_000 });

  // 4. Boundary advance: at the end of 演示, 下一步 invokes the advance gate.
  //    We assert the gate is INVOKED (POST /session/advance fires) — whether it
  //    grants the move is the live-coach decision tracked by Finding A.
  const advResp = page.waitForResponse(
    (r) => r.url().includes("/session/advance") && r.request().method() === "POST",
    { timeout: 90_000 },
  );
  await nextBtn.click(); // at ordinal 1 (last 演示 step) ⇒ this click is the boundary
  expect((await advResp).status()).toBe(200);
});
