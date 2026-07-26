import { test, expect } from "@playwright/test";
import { login } from "./helpers";

// Tenancy — a teacher must never read a student or class outside their own
// class. Every cross-tenant read returns 404 (hidden, never 200-with-data and
// never 403-which-confirms-existence). Deterministic, no model. Uses seeded org:
//   wu.teacher owns class 902 (IBDP); Phoebe (003) is in Demo Class 002.
const TEACHER = { email: "wu.teacher@demo.mindimprint.local", password: "phoebe-dev-pass" };
const API = "http://localhost:8080/api/v1";
const CLASS_OWN = "00000000-0000-0000-0000-000000000902";
const CLASS_FOREIGN = "00000000-0000-0000-0000-000000000002"; // Demo Class — not the teacher's
const PHOEBE = "00000000-0000-0000-0000-000000000003"; // enrolled in the foreign class

test("tenancy: a teacher gets 404 on every cross-class read", async ({ page }) => {
  await login(page, TEACHER.email, TEACHER.password);

  // Sanity: the teacher CAN read their own class (not 404) — proves the 404s
  // below are tenancy, not a broken route.
  const own = await page.request.get(`${API}/classes/${CLASS_OWN}`);
  expect(own.status()).toBe(200);

  // Foreign class detail / roster-report / weekly-report → 404.
  for (const path of [
    `/classes/${CLASS_FOREIGN}`,
    `/classes/${CLASS_FOREIGN}/roster-report`,
    `/classes/${CLASS_FOREIGN}/weekly-report`,
  ]) {
    const res = await page.request.get(`${API}${path}`);
    expect(res.status(), `${path} must be tenancy-hidden`).toBe(404);
  }

  // A student who is NOT in the teacher's class → 404 on detail + parent report,
  // whether addressed via the foreign class or the teacher's own class.
  for (const path of [
    `/classes/${CLASS_FOREIGN}/students/${PHOEBE}`,
    `/classes/${CLASS_OWN}/students/${PHOEBE}`,
    `/classes/${CLASS_FOREIGN}/students/${PHOEBE}/parent-stage-report/current`,
  ]) {
    const res = await page.request.get(`${API}${path}`);
    expect(res.status(), `${path} must be tenancy-hidden`).toBe(404);
  }
});
