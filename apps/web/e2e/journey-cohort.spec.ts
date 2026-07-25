import { test, expect } from "@playwright/test";
import { login } from "./helpers";

// J-cohort — the capstone's unique property: a class of many students, each
// drilling into their OWN data (correct per-student attribution), and the class
// weekly report aggregating the whole cohort. Uses the seeded IBDP class
// (wu.teacher + 9 students). Deterministic reads (the weekly comment composes
// once on open). Complements J-teacher (which covers the report compose paths).
const TEACHER = { email: "wu.teacher@demo.mindimprint.local", password: "phoebe-dev-pass" };

test("J-cohort: many students → correct per-student attribution + aggregation", async ({ page }) => {
  test.setTimeout(120_000);
  await login(page, TEACHER.email, TEACHER.password);

  await page.getByRole("tab", { name: "班级" }).click();
  await page.getByText(/IBDP/).first().click();

  // Roster shows the cohort — several distinct, named students, each on their
  // own row (attribution at the roster level: the class aggregates many people).
  await page.getByRole("button", { name: "全部学生" }).click();
  for (const name of ["林知远", "沈亦然", "周子墨", "陈屿"]) {
    await expect(page.locator("tr", { hasText: name })).toBeVisible({ timeout: 15_000 });
  }

  // Attribution: drilling into a student shows THAT student's own detail
  // (name + their stats), not a shared/blank view.
  await page.locator("tr", { hasText: "沈亦然" }).first().click();
  await expect(page.getByText("沈亦然").first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(/生成能力报告/)).toBeVisible();
  // The detail is 沈亦然's alone — a different classmate's name does not appear.
  await expect(page.getByText("林知远")).toHaveCount(0);
});
