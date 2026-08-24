import { test, expect } from "@playwright/test";
import { login } from "./helpers";

// J-teacher — the teacher console: 班级 → 班级周报 (D2, live composer) → drill into
// a student → 家长报告 (live composer). Uses the seeded teacher wu.teacher (owns
// the IBDP class with seeded week data + evaluations). Live flagship on the two
// compose calls (weekly prose + parent prose) — the path the maxTokens fix
// repaired.
const TEACHER = { email: "wu.teacher@demo.mindimprint.local", password: "phoebe-dev-pass" };

test("J-teacher: 班级 → 周报 → 学生 → 家长报告", async ({ page }) => {
  test.setTimeout(180_000);
  await login(page, TEACHER.email, TEACHER.password);

  // 1. 班级 → open the seeded class. Opening it auto-composes the weekly prose
  //    (D2 first-open-wins), a flagship call.
  await page.getByRole("tab", { name: "班级" }).click();
  await page.getByText(/IBDP/).first().click();

  // 2. 班级周报 (default subtab): header + a real comment (not the 暂未生成
  //    placeholder) — proves the live weekly composer works. Wait DIRECTLY for
  //    the end state (placeholder gone) with a budget that comfortably exceeds
  //    flagship latency, rather than coupling a network-response wait to a
  //    shorter assertion window (which flaked under contention). This passes as
  //    soon as compose lands — however long it takes — and only fails if the
  //    composer genuinely cannot produce prose (a real product problem).
  await expect(page.getByText(/班级周报 ·/)).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText("本周点评暂未生成")).toHaveCount(0, { timeout: 150_000 });

  // 3. 全部学生 → drill into a student → detail renders (stats). The parent-
  //    stage-report projection (old step 4) was retired from the API + UI, so the
  //    journey ends here — the teacher console's live class-week + drill-in path.
  await page.getByRole("button", { name: "全部学生" }).click();
  await page.locator("tr", { hasText: "林" }).first().click();
  await expect(page.getByText(/生成能力报告/)).toBeVisible({ timeout: 15_000 });
});
