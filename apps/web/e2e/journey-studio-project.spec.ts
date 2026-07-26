import { test, expect } from "@playwright/test";
import { login, openRail, PHOEBE } from "./helpers";

// J-studio — the writing project's back half → the flagship 你的思维印记.
//
// The seeded Phoebe project sits at S4 写作 (the seed already walked S0→S3 — the
// acceptance mainline). The studio UI enforces station-locking: 回看/finish is
// not reachable from S4 without walking each station's gates through the UI (a
// separate, large UI-journey effort). So this journey drives the lifecycle's
// back half over the REAL API (Playwright's request context shares the browser
// session cookie set by the UI login — still real HTTP → API → DB → live
// flagship model), and then UI-verifies the outcome in 成长报告. This proves the
// flagship 过程评估 (你的思维印记) generates end-to-end on the project path — the
// gate that Findings D + E were blocking. Idempotent: re-running (retry) sees the
// project already assessed and just re-verifies.
const API = "http://localhost:8080/api/v1";
const DRAFT = [
  "中国是否让地球更可持续，必须分两面看。本文论点：中国在可再生能源上的贡献是实质性的，但碳排放总量仍是严重的反例，所以答案是部分是、部分否。",
  "支持面：中国是全球最大的可再生能源投资国，风电与光伏装机世界第一。NASA 发表在《自然·可持续性》的卫星研究显示，过去二十年全球新增绿化有相当一部分来自中国的植树造林。这些是可核查的权威来源。",
  "反例面：中国目前是全球碳排放总量第一，煤炭在能源结构中占比仍高。只讲绿化而回避排放总量，论证就会失衡。一个诚实的让步是：中国人均排放低于部分发达国家，但总量的绝对值决定了它对全球气候的直接影响。",
  "结论：把中国简单判为可持续或不可持续都是过度简化。更准确的判断是：它在特定维度上做出全球领先的贡献，同时在排放总量上仍是最大的挑战来源；评估净效应要同时看这两条证据线。",
].join("\n\n");

test("J-studio: project lifecycle → commit → 整稿体检 → finish → 你的思维印记", async ({ page }) => {
  test.setTimeout(240_000);

  // 1. UI login (Phoebe) — sets the session cookie the API request context reuses.
  await login(page, PHOEBE.email, PHOEBE.password);

  // 2. Find her seeded project.
  const projects = await (await page.request.get(`${API}/projects`)).json();
  const rows = Array.isArray(projects) ? projects : projects.projects ?? projects.items ?? [];
  const projectId: string = rows[0].id;
  expect(projectId).toBeTruthy();

  // 3. Drive the back half over the real API (idempotent: skip if already assessed).
  const existing = await page.request.get(`${API}/projects/${projectId}/assessment`);
  const already = existing.ok() && (await existing.text()).includes("depthAxis");
  if (!already) {
    // commit a draft snapshot
    const snap = await page.request.post(`${API}/projects/${projectId}/snapshots`, { data: { content: DRAFT } });
    expect(snap.ok()).toBeTruthy();
    const snapshotId: string = (await snap.json()).id;

    // 整稿体检 (live flagship review) — SSE; sets the whole_draft_review gate.
    const review = await page.request.post(`${API}/projects/${projectId}/snapshots/${snapshotId}/review`, {
      headers: { Accept: "text/event-stream" },
      timeout: 120_000,
    });
    expect(review.status()).toBe(200);

    // finish → generates the flagship 你的思维印记 report.
    const finish = await page.request.post(`${API}/projects/${projectId}/finish`, { timeout: 150_000 });
    expect(finish.status()).toBe(200);
    const report = await finish.json();
    // RL-5 / shape: a real dual-axis report, not an empty stub.
    expect(JSON.stringify(report)).toContain("depthAxis");
    expect(JSON.stringify(report)).toContain("narrative");
  }

  // 4. UI: 成长报告 · 学习记录 now shows the evaluated project (growth populated).
  await openRail(page, "成长报告");
  await page.getByRole("button", { name: "学习记录" }).click();
  await expect(page.getByText("还没有报告")).toHaveCount(0, { timeout: 15_000 });
});
