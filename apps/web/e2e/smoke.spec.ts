import { test, expect } from "@playwright/test";

// Proves the harness wiring before the expensive live-model journey:
// the web dev server serves the app, and the API liveness endpoint answers.
test("stack is up: app loads and API is healthy", async ({ page, request }) => {
  await page.goto("/");
  // The unauthenticated app boots to the auth screen (login card title 登录).
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible();

  // Liveness is at the server root (not under /api), so hit :8080 directly.
  const res = await request.get("http://localhost:8080/healthz");
  expect(res.status()).toBe(200);
  expect((await res.text()).trim()).toBe("ok");
});
