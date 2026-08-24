import { test, expect } from "@playwright/test";
import { login, PHOEBE } from "./helpers";

// J-studio — a writing project's back half → the flagship 你的思维印记.
//
// The seeded Phoebe project (0018, status 'active') is driven through the
// lifecycle's back half over the REAL API (Playwright's request context shares
// the browser session cookie set by the UI login — still real HTTP → API → DB →
// live flagship model), then the outcome is UI-verified under 项目 → 评估报告.
// This proves the flagship 过程评估 (你的思维印记) generates end-to-end on the
// project path. Idempotent: a re-run (retry) sees the report already ready and
// just re-verifies.
//
// The report is read via GET /projects/{id}/evaluation-report — the three-state
// envelope (null | {status:"generating"|"failed"} | {status:"ready",report}) —
// NOT the retired /assessment path the stale spec polled.
const API = "http://localhost:8080/api/v1";
const DRAFT = [
  "中国是否让地球更可持续，必须分两面看。本文论点：中国在可再生能源上的贡献是实质性的，但碳排放总量仍是严重的反例，所以答案是部分是、部分否。",
  "支持面：中国是全球最大的可再生能源投资国，风电与光伏装机世界第一。NASA 发表在《自然·可持续性》的卫星研究显示，过去二十年全球新增绿化有相当一部分来自中国的植树造林。这些是可核查的权威来源。",
  "反例面：中国目前是全球碳排放总量第一，煤炭在能源结构中占比仍高。只讲绿化而回避排放总量，论证就会失衡。一个诚实的让步是：中国人均排放低于部分发达国家，但总量的绝对值决定了它对全球气候的直接影响。",
  "结论：把中国简单判为可持续或不可持续都是过度简化。更准确的判断是：它在特定维度上做出全球领先的贡献，同时在排放总量上仍是最大的挑战来源；评估净效应要同时看这两条证据线。",
].join("\n\n");

test("J-studio: project lifecycle → 整稿体检 → finish → 你的思维印记", async ({ page }) => {
  // Generous: the back half runs a live SSE review then an ASYNC flagship report
  // (4 flagship calls) that can take a few minutes under load, plus the poll.
  test.setTimeout(420_000);

  // 1. UI login (Phoebe) — sets the session cookie the API request context reuses.
  await login(page, PHOEBE.email, PHOEBE.password);

  // 2. Find her seeded project.
  const projects = await (await page.request.get(`${API}/projects`)).json();
  const rows = Array.isArray(projects) ? projects : projects.projects ?? projects.items ?? [];
  const projectId: string = rows[0].id;
  expect(projectId).toBeTruthy();

  // Reads the three-state evaluation-report envelope; returns the parsed body.
  const readReport = async () => {
    const res = await page.request.get(`${API}/projects/${projectId}/evaluation-report`);
    if (!res.ok()) return null;
    return (await res.json()) as { status?: string; report?: { depth?: unknown; autonomy?: unknown } } | null;
  };

  // 3. Drive the back half over the real API. Idempotent + retry-safe: skip the
  //    whole drive if the report is already ready, AND skip re-finalizing if a
  //    prior attempt (or a Playwright retry) already kicked off generation — the
  //    report envelope reads "generating" then, and re-POSTing finish would 409.
  const existing = await readReport();
  const alreadyReady = existing?.status === "ready";
  const alreadyGenerating = existing?.status === "generating";
  if (!alreadyReady && !alreadyGenerating) {
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

    // Complete the reflection (mirrors 完成回顾) — finish's server-side gate
    // refuses to archive until project_reflection.done is true.
    const refl = await page.request.put(`${API}/projects/${projectId}/reflection-doc`, {
      data: {
        answers: [
          "回看这一程，我最大的收获是学会把证据放在它能支撑的范围里说话。",
          "我一开始想直接下结论，后来学会先追问来源到底能证明什么。",
          "撞到反例时我没有回避，而是把它写成让步段。",
          "如果重来，我会更早地区分口径，避免把年度排放和累计责任混在一起。",
          "带走的一点：强证据不只是可信，还得有边界、有位置。",
        ],
        done: true,
      },
    });
    expect(refl.ok()).toBeTruthy();

    // 完成写作 gate: finish refuses (422 writing_not_finished) unless the ESSAY
    // doc is locked, and finish-writing itself refuses (422 draft_empty) unless
    // the essay's edit buffer carries real content. So fill the buffer, then
    // finish writing, before finalizing.
    const buf = await page.request.put(`${API}/projects/${projectId}/buffer?doc=essay`, { data: { content: DRAFT } });
    expect(buf.ok()).toBeTruthy(); // 204
    const fw = await page.request.post(`${API}/projects/${projectId}/finish-writing?doc=essay`);
    expect(fw.ok()).toBeTruthy(); // 200

    // finish → kicks off the flagship 你的思维印记 report ASYNC and a detached
    // goroutine generates it. 202 {status:"evaluating"} on a fresh finalize; 409
    // if it was already finalizing (a prior attempt / retry) — both are fine, we
    // poll the report either way.
    const finish = await page.request.post(`${API}/projects/${projectId}/finish`, { timeout: 150_000 });
    expect([202, 409]).toContain(finish.status());
  }

  // Poll the three-state envelope until the async flagship report reads "ready"
  // (skipped only if it was already ready). ~280s window — the live flagship
  // report can take a few minutes under load.
  if (!alreadyReady) {
    let ready = false;
    for (let i = 0; i < 70; i++) {
      const body = await readReport();
      if (body?.status === "ready") {
        ready = true;
        // A real report envelope, not an empty stub.
        expect(Array.isArray(body.report?.depth)).toBeTruthy();
        expect(Array.isArray(body.report?.autonomy)).toBeTruthy();
        break;
      }
      if (body?.status === "failed") throw new Error("evaluation report generation failed");
      await page.waitForTimeout(4000);
    }
    expect(ready, "flagship report generated async within timeout").toBeTruthy();
  }

  // 4. UI: the report surfaces under 项目 → 评估报告 (ReportsView, after the nav
  //    restructure — the retired 成长报告 tab is gone). The empty state is gone
  //    and the report is openable.
  await page.getByRole("tab", { name: "项目" }).click();
  await page.getByRole("button", { name: "评估报告" }).click();
  await expect(page.getByRole("heading", { name: "你的思维印记" })).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText("还没有报告")).toHaveCount(0, { timeout: 15_000 });
});
