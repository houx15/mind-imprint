import { test, expect, Page } from "@playwright/test";
import { registerStudent, openRail, createPaper, coachSend, uniqueEmail } from "./helpers";

// J-walk — a fresh student walks ONE writing project through ALL stations
// S0→S6 entirely through the UI (real DeepSeek model via the backend gateway),
// finishes it, and confirms the flagship 你的思维印记 report generates.
//
// This is the pure-UI counterpart to journey-studio-project.spec.ts (which
// drives the back half over the API against the seeded Phoebe project). Here a
// brand-new student registers, creates a project via the 新建论文 funnel, and
// every station's gate is satisfied by real clicks / form fills / coach
// messages / tool-card fills. Station advancement is AUTOMATIC (no advance
// button): producing a station's gate items re-derives the gate server-side and
// unlocks the next station. Card surfacing is deterministic (a pure graph
// predicate, SurfaceCardCandidates), so a coach message at the right graph
// state reliably surfaces the expected card.
//
// The walk is long and includes 4 live-model steps (信源体检 / 论证体检 /
// 整稿体检 / finish). Timeouts are generous; playwright.config gives retries:1.

const API = "http://localhost:8080/api/v1";
const JOIN_CODE = "DEMO-0001"; // seeded Demo Class

const SRC_A = "碳中和年度进展报告";
const SRC_B = "全球碳排放趋势观察";

// ── API helpers (page.request shares the UI session cookie) ──────────────────

async function firstProjectId(page: Page): Promise<string> {
  const r = await page.request.get(`${API}/projects`);
  const j = await r.json();
  const rows = Array.isArray(j) ? j : j.projects ?? j.items ?? [];
  expect(rows.length, "student should have exactly one created project").toBeGreaterThan(0);
  return rows[0].id;
}

type Station = { code: string; state: string; gate?: { total: number; passed: number } };

async function getStations(page: Page, id: string): Promise<Station[]> {
  const r = await page.request.get(`${API}/projects/${id}`);
  expect(r.ok(), "GET /projects/{id} should succeed").toBeTruthy();
  const j = await r.json();
  return j.stations as Station[];
}

function stationOf(sts: Station[], code: string): Station | undefined {
  return sts.find((s) => s.code === code);
}

// Poll the projection until the given station reaches one of `states`.
async function waitStationState(page: Page, id: string, code: string, states: string[], timeout = 120_000) {
  await expect
    .poll(async () => states.includes(stationOf(await getStations(page, id), code)?.state ?? ""), {
      timeout,
      intervals: [1500],
    })
    .toBe(true);
}

// Poll the projection until the given station's gate has >= n items passed.
async function waitGatePassed(page: Page, id: string, code: string, n: number, timeout = 180_000) {
  await expect
    .poll(async () => stationOf(await getStations(page, id), code)?.gate?.passed ?? 0, { timeout, intervals: [1500] })
    .toBeGreaterThanOrEqual(n);
}

// Switch the station-rail view. The rail item div carries title={station.name}
// (StationRail.tsx) — a selector nothing else on the page shares (the ViewFrame
// header uses a span; the S0 plan tracker uses plain text), so it disambiguates
// even when the station name doubles as a milestone-plan step ("立题" /
// "反思归档" both appear in the S0 plan tracker).
async function gotoStation(page: Page, name: string) {
  await page.locator(`[title="${name}"]`).click();
}

// ── Tool-card fills ──────────────────────────────────────────────────────────

// CRAAP (annotate primitive, rail). The anchor generator guarantees exactly one
// anchor per tag (LLM output is tag-validated, else it falls back to the five
// tag anchors), so every "<dimension>-answer" textarea + the risk note is all
// completion needs. The active card's unique marker is 「作用与风险（自己写）」
// (the CraapPlaceholder mimics 「工具卡 · CRAAP」/「评估这条来源」, so those can't
// mark the live card apart from the placeholder).
async function fillCraap(page: Page) {
  // L3 (elicit) anchors need the student to write the question herself first.
  const questions = page.locator('input[aria-label$="-question"]');
  for (let i = 0; i < (await questions.count()); i++) {
    await questions.nth(i).fill("针对这条来源，我想核实它在这一维上是否站得住？");
  }
  const answers = page.locator('textarea[aria-label$="-answer"]');
  await expect(answers.first()).toBeVisible({ timeout: 30_000 });
  const n = await answers.count();
  expect(n, "CRAAP should surface at least one tag anchor").toBeGreaterThan(0);
  for (let i = 0; i < n; i++) {
    await answers.nth(i).fill("这一维我逐条核对过：出处、时间与可验证性都能对上，可以谨慎采信。");
  }
  await page
    .getByPlaceholder(/这条来源在你的论证里起什么作用/)
    .fill("它为我的核心论点提供关键数据，但要注意发布时间较早，且发布方可能存在立场偏差。");
  // L2/L3 (locate/elicit) anchors need a located span OR the "找不到合适的句子"
  // escape before the card can lock. Take the escape on every anchor that
  // offers it — always available, never a wall (铁律 2).
  const escapes = page.getByRole("button", { name: "找不到合适的句子" });
  while ((await escapes.count()) > 0) {
    await escapes.first().click();
  }
  const lock = page.getByRole("button", { name: "锁定，进下一条" });
  await expect(lock).toBeEnabled({ timeout: 15_000 });
  await lock.click();
  await expect(page.getByText("作用与风险（自己写）")).toHaveCount(0, { timeout: 60_000 });
}

// SIFT (compare primitive, rail). Every textarea + single_choice must be
// answered AND a real, different lateral source picked. Textareas have no
// aria-label (placeholder = field label), so target by placeholder regex.
async function fillSift(page: Page) {
  await expect(page.getByText("工具卡 · SIFT")).toBeVisible({ timeout: 30_000 });
  await page.getByPlaceholder(/你的第一反应是什么/).fill("我的第一反应是先别急着相信，也别急着转发，先弄清它到底是谁发的。");
  await page.getByPlaceholder(/这个来源是谁/).fill("我另开标签查了发布方，它是一家有公开背景的研究机构，不是营销号。");
  await page.getByPlaceholder(/这个独立来源怎么说同一件事/).fill("我找到一份独立机构的数据，对同一趋势给出了一致的结论，可以互相印证。");
  await page.getByPlaceholder(/最原始的出处是哪里/).fill("原始出处是该机构公开发布的年度统计报告，可在其官网直接查到。");
  await page
    .getByPlaceholder(/你现在怎么判断这条说法/)
    .fill("横向查过之后我更有把握了：核心趋势成立，但要给出具体数字时仍需标注口径与年份。");
  // single_choice: relation (find step) + tier_after (trace step)
  await page.getByRole("button", { name: "印证", exact: true }).click();
  await page.getByRole("button", { name: "一手报道", exact: true }).click();
  // lateral source: the only candidate (everything except the card's own source)
  await page.getByTestId("lateral-material-picker").getByRole("button").first().click();
  const lock = page.getByRole("button", { name: "锁定这张卡" });
  await expect(lock).toBeEnabled({ timeout: 15_000 });
  await lock.click();
  await expect(page.getByText("工具卡 · SIFT")).toHaveCount(0, { timeout: 60_000 });
}

// One coach turn → the graph predicate surfaces a card. Returns "source" once
// filled, or "other" when a non-source card (project-scoped Toulmin) surfaces —
// which is exactly the signal that S3's source-evaluation work is complete (no
// per-material CRAAP/SIFT candidate remains). The Toulmin proposal is left
// unopened for S4 to pick up.
async function surfaceAndFillSourceCard(page: Page, n: number): Promise<"source" | "other"> {
  await coachSend(page, `我准备好评估信源了（第 ${n} 步），帮我核对这条来源可不可信。`);
  const open = page.getByRole("button", { name: "打开", exact: true });
  await expect(open, "a tool card should surface after a coach turn at S3").toBeVisible({ timeout: 150_000 });
  // Identify the proposal BEFORE opening it — scoped to the CardProposalBubble
  // (the 打开 button's ancestor bubble div), NOT the whole page: coach reply
  // prose in the thread routinely mentions "信源辨识卡"/"横向核查卡", so a
  // page-wide getByText would falsely read a source card and open the Toulmin.
  const bubbleText = await open.locator("xpath=ancestor::div[2]").innerText();
  if (!/信源辨识卡|横向核查卡/.test(bubbleText)) {
    return "other"; // Toulmin (or another project card) — S3 source work done
  }
  await open.click();
  // Disambiguate CRAAP vs SIFT by their active-card markers.
  const craap = page.getByText("作用与风险（自己写）");
  const sift = page.getByText("工具卡 · SIFT");
  await expect(craap.or(sift)).toBeVisible({ timeout: 30_000 });
  if ((await craap.count()) > 0) await fillCraap(page);
  else await fillSift(page);
  return "source";
}

// Toulmin (graph primitive, CENTER pane at S4). Expand each slot, cite a locked
// source for needSrc slots, write >=12 runes, then lock all five.
async function fillToulminSlot(page: Page, role: string, text: string, needSrc: boolean) {
  await page.getByText(role, { exact: true }).click();
  if (needSrc) {
    await page.getByRole("button", { name: SRC_A }).first().click();
  }
  await page.getByPlaceholder(/用你自己的话写/).fill(text);
}

// ── Draft builder (CJK-aware; backend CountWords counts each Han char as 1) ───

function cjkCount(s: string): number {
  let c = 0;
  for (const ch of s) {
    const cp = ch.codePointAt(0)!;
    if ((cp >= 0x4e00 && cp <= 0x9fff) || (cp >= 0x3040 && cp <= 0x30ff) || (cp >= 0xff00 && cp <= 0xffef)) c++;
  }
  return c;
}

function buildDraft(): string {
  const sents = [
    "中国是否让地球更可持续，必须分两面看，不能只挑对自己有利的证据。",
    "支持面上，中国是全球最大的可再生能源投资国，风电与光伏装机容量多年位居世界第一。",
    "卫星研究显示，过去二十年全球新增绿化有相当一部分来自中国的植树造林工程。",
    "这些结论来自可核查的权威来源，经过横向比对后依然成立，可以作为论证的证据线。",
    "反例面上，中国目前仍是全球碳排放总量第一，煤炭在能源结构中的占比依然偏高。",
    "只讲绿化和装机而回避排放总量，论证就会失衡，也经不起考官的追问与质疑。",
    "一个诚实的让步是：中国人均排放低于部分发达国家，但总量的绝对值决定了它的直接影响。",
    "把中国简单判为可持续或不可持续都是过度简化，真实情况远比一句结论复杂。",
    "更准确的判断是：它在特定维度上做出全球领先的贡献，同时在排放总量上仍是最大挑战。",
    "评估净效应时要同时看这两条证据线，并明确标注每个数字的口径、年份与来源层级。",
  ];
  const paras: string[] = [];
  let i = 0;
  while (cjkCount(paras.join("\n\n")) < 1650) {
    paras.push(sents[i % sents.length] + sents[(i + 1) % sents.length] + sents[(i + 2) % sents.length]);
    i++;
  }
  return paras.join("\n\n");
}

// ── Add a pasted source via the 素材 dossier's 添加信源 form ──────────────────
async function addPastedSource(page: Page, title: string) {
  await page.getByRole("button", { name: "添加信源" }).first().click();
  await page.getByRole("button", { name: "粘贴正文" }).click();
  await page.getByPlaceholder("标题（必填）").fill(title);
  await page
    .getByPlaceholder("把正文粘贴进来……")
    .fill(
      `${title}。这份材料给出了中国在可再生能源装机与碳排放总量上的关键数据，` +
        "包含风电光伏装机、植树造林面积与年度排放趋势，并注明了统计口径与发布年份，" +
        "可以作为评估「中国是否让地球更可持续」这一问题的证据来源。",
    );
  await page.getByPlaceholder("一句话说说你从这条里读到了什么……").fill("它同时给出了正面贡献与排放总量两条线索。");
  await page.getByRole("radio", { name: "机构报告" }).check();
  await page.getByRole("button", { name: "加入信源档案" }).click();
  // The form collapses back to the 添加信源 affordance on success.
  await expect(page.getByRole("button", { name: "加入信源档案" })).toHaveCount(0, { timeout: 20_000 });
}

test("J-walk: fresh student walks S0→S6 through the UI → 你的思维印记 generates", async ({ page }) => {
  test.setTimeout(600_000);

  // ── Register a FRESH student (not seeded Phoebe) + create the project ───────
  const email = uniqueEmail("walk-student");
  await registerStudent(page, { name: "E2E 走查同学", email, code: JOIN_CODE });
  await expect(page.getByRole("tab", { name: "工作室" })).toBeVisible();

  await createPaper(page, {
    title: "中国是否让地球变得更可持续？",
    prompt:
      "我在写 TOK 论文：中国是否让地球更可持续？我要用可信来源、分不同视角分析、并诚实处理反例（碳排放总量）。",
  });

  const id = await firstProjectId(page);

  // Fail-safe: the journey composer may waive stations. Re-open any waived ones
  // so the whole S0→S6 chain is walkable through the UI.
  for (const s of await getStations(page, id)) {
    if (s.state === "waived") {
      await page.request.post(`${API}/projects/${id}/journey/reopen/${s.code}`);
    }
  }

  // ── S0 · 任务解码 ──────────────────────────────────────────────────────────
  // composeJourney may have waived S0 (the project then opens at S1); the API
  // reopen above un-waived it server-side, so switch the client view to S0.
  await gotoStation(page, "任务解码");
  // Restate (>=15 runes) + 记下我的理解. The fixture pre-selects 2 weakest rows,
  // so weakness_prediction>=2 is satisfied by the default picks on submit.
  await page
    .getByPlaceholder(/这道题到底在问什么/)
    .fill("这道题要我判断中国的行动到底让地球更可持续了没有，既要拿可信证据，也要诚实面对碳排放这个反例。");
  await page.getByRole("button", { name: "记下我的理解" }).click();
  await waitStationState(page, id, "S1", ["current", "done"], 90_000);

  // ── S1 · 立题 ──────────────────────────────────────────────────────────────
  // >=3 key terms each defined >=15 runes + >=1 核心论点 + >=1 检索方向.
  await gotoStation(page, "立题");
  const terms: [string, string][] = [
    ["可持续", "指在满足当代发展需要的同时，不损害后代满足其自身需要的能力，可用排放与资源指标衡量。"],
    ["中国的贡献", "特指中国在可再生能源装机、植树造林与减排政策上可核查的具体行动与数据结果。"],
    ["净效应", "指把正面贡献与负面排放两条证据线合并后，对全球可持续性的综合净影响判断。"],
  ];
  for (let i = 0; i < terms.length; i++) {
    await page.getByRole("button", { name: "添加一个关键词" }).click();
    await page.getByLabel("关键词", { exact: true }).nth(i).fill(terms[i][0]);
    await page.getByLabel("定义", { exact: true }).nth(i).fill(terms[i][1]);
  }
  await page.getByRole("button", { name: "添加一条论点" }).click();
  await page.getByLabel("核心论点", { exact: true }).first().fill("中国在可再生能源上贡献显著，但排放总量仍是严重反例，答案是部分是、部分否。");
  await page.getByRole("button", { name: "添加一条检索方向" }).click();
  await page.getByLabel("检索方向", { exact: true }).first().fill("权威机构报告");
  await page.getByRole("button", { name: "记下我的立题" }).click();
  await waitStationState(page, id, "S2", ["current", "done"], 90_000);

  // ── S2 · 视角与素材 ────────────────────────────────────────────────────────
  // >=2 perspectives; open a source (recon_logged); attest sources-per-perspective.
  await gotoStation(page, "视角与素材");
  const perspectives = [
    "国家视角：中国政府把可再生能源当作发展战略，看重装机规模与产业带动。",
    "全球视角：其他国家关注中国的碳排放总量对全球气候的直接影响。",
  ];
  for (let i = 0; i < perspectives.length; i++) {
    await page.getByRole("button", { name: "添加一条视角" }).click();
    await page.getByLabel("视角", { exact: true }).nth(i).fill(perspectives[i]);
  }
  await page.getByRole("button", { name: "记下我的视角" }).click();

  // Add source A (also used as a CRAAP target at S3) and open it for recon.
  await addPastedSource(page, SRC_A);
  await page.getByTestId("dossier-source-list").getByRole("button", { name: SRC_A }).click();
  await expect(page.getByText("返回信源列表")).toBeVisible({ timeout: 15_000 });
  await page.waitForTimeout(1500); // reading time > 0 so recon logs
  await page.getByText("返回信源列表").click();

  // Attest: every perspective has at least one source.
  const attest = page.locator('input[type="checkbox"]').first();
  await expect(attest).toBeEnabled({ timeout: 15_000 });
  await attest.check();
  await waitStationState(page, id, "S3", ["current", "done"], 90_000);

  // ── S3 · 信源评估 ──────────────────────────────────────────────────────────
  // Add a 2nd article (the SIFT lateral / 2nd CRAAP target). Then drive coach
  // turns: CRAAP each source (every_source_evaluated + source_risk_notes) and
  // SIFT once (cross_check). Machine+student items = 3 of 4 gate items.
  await gotoStation(page, "信源评估");
  await addPastedSource(page, SRC_B);

  // Drive coach turns until source-evaluation cards are exhausted (both sources
  // CRAAP'd → every_source_evaluated; one SIFT → cross_check). The loop stops
  // when the next surfaced card is no longer a source card (Toulmin).
  for (let i = 1; i <= 6; i++) {
    if ((await surfaceAndFillSourceCard(page, i)) === "other") break;
  }
  // cross_check + source_risk_notes both pass here. (every_source_evaluated is
  // NOT counted in the rail gauge — the studio projection builds its gate
  // GraphView with nil materials at apps/api/internal/studio/projection.go:87,
  // so that machine item reports Pass-but-not-Attempted; the real AdvanceAll
  // path uses LoadGraph WITH materials, so advancement is unaffected.)
  await waitGatePassed(page, id, "S3", 2, 60_000);

  // 信源体检 (live model) → records source_quality_spot_check → S3 done.
  // Live-model-gated: agent.ProposeSpotCheck (spotcheck.go) names the required
  // wire fields (target_id/evidence/missing/fix) so the model's items parse.
  const sourceSpotCheck = page.getByRole("button", { name: "信源体检" });
  await expect(sourceSpotCheck).toBeEnabled({ timeout: 15_000 });
  await sourceSpotCheck.click();
  await waitStationState(page, id, "S4", ["current", "done"], 150_000);

  // ── S4 · 论证构建 ──────────────────────────────────────────────────────────
  // Toulmin card surfaces (project-scoped: sources evaluated, no claim yet).
  await gotoStation(page, "论证构建");
  // The Toulmin proposal is usually already present (it surfaced at the end of
  // the S3 loop). If not, one coach turn brings it up.
  const openToulmin = page.getByRole("button", { name: "打开", exact: true });
  if ((await openToulmin.count()) === 0) {
    await coachSend(page, "我核完来源了，帮我把论证搭成结构。");
  }
  await expect(openToulmin, "Toulmin card should surface at S4").toBeVisible({ timeout: 150_000 });
  await openToulmin.click();
  await expect(page.getByText("核心主张")).toBeVisible({ timeout: 30_000 });

  await fillToulminSlot(page, "核心主张", "中国让地球更可持续这一判断只在特定维度上成立，需要分面看待。", false);
  await fillToulminSlot(page, "理据 · 推理", "因为可再生能源装机世界第一，说明其减排投入是实质性的而非口号。", true);
  await fillToulminSlot(page, "支撑证据", "机构报告显示中国风电光伏装机与植树造林规模均居全球前列。", true);
  await fillToulminSlot(page, "反方 · 钢人", "反方最硬的一张牌是：中国碳排放总量全球第一，煤炭占比仍偏高。", false);
  await fillToulminSlot(page, "让步 · 转折", "我承认排放总量确实第一，但人均与趋势数据表明其减排方向明确。", true);

  const lockToulmin = page.getByRole("button", { name: "全部锁定，完成论证" });
  await expect(lockToulmin).toBeEnabled({ timeout: 15_000 });
  await lockToulmin.click();
  await expect(page.getByText("全部锁定，完成论证")).toHaveCount(0, { timeout: 60_000 });
  await waitGatePassed(page, id, "S4", 3, 60_000); // concession + warrants + steelman

  // 论证体检 (live model) → records warrant_quality_spot_check → S4 done.
  const argSpotCheck = page.getByRole("button", { name: "论证体检" });
  await expect(argSpotCheck).toBeEnabled({ timeout: 15_000 });
  await argSpotCheck.click();
  await waitStationState(page, id, "S5", ["current", "done"], 150_000);

  // ── S5 · 成稿打磨 ──────────────────────────────────────────────────────────
  await gotoStation(page, "成稿打磨");
  const draft = buildDraft();
  expect(cjkCount(draft), "draft must land in the 1500–2000 word budget").toBeGreaterThanOrEqual(1500);
  expect(cjkCount(draft), "draft must land in the 1500–2000 word budget").toBeLessThanOrEqual(2000);

  const editor = page.locator("textarea:not([placeholder])").first();
  await expect(editor).toBeVisible({ timeout: 15_000 });
  await editor.fill(draft);
  await page.getByRole("button", { name: "提交快照 · 定格这一稿" }).click();

  // 整稿体检 (live flagship review) → whole_draft_review.
  const orderReview = page.getByRole("button", { name: "整稿体检" });
  await expect(orderReview).toBeEnabled({ timeout: 15_000 });
  await orderReview.click();
  await expect(page.getByText("整稿体检 · 段落 ⇄ 评分表")).toBeVisible({ timeout: 180_000 });

  // citations_matched attestation (in the review block). The checkbox is
  // controlled (checked={citationsMatched}) and only flips true after the
  // attest POST + refetch lands, so click and wait for the async state rather
  // than .check() (which verifies checked-ness synchronously and would race).
  const citations = page.locator("#citations-matched");
  await citations.click();
  await expect(citations).toBeChecked({ timeout: 20_000 });
  await waitStationState(page, id, "S6", ["current", "done"], 90_000);

  // ── S6 · 反思归档 ──────────────────────────────────────────────────────────
  await gotoStation(page, "反思归档");
  await page
    .getByTestId("reflection-textarea")
    .fill("这次研究让我学会先核来源、再分视角，最后诚实处理反例，而不是只挑对自己有利的证据。");
  await page.getByTestId("reflection-submit").click();
  await page.getByTestId("declaration-sign").click();
  await expect(page.getByTestId("declaration-signed-note")).toBeVisible({ timeout: 15_000 });

  // Finish → generates the flagship 你的思维印记 report (live, up to ~150s).
  const finishResp = page.waitForResponse(
    (r) => r.url().includes(`/projects/${id}/finish`) && r.request().method() === "POST",
    { timeout: 180_000 },
  );
  await page.getByRole("button", { name: "完成任务 · 归档" }).click();
  expect((await finishResp).status(), "finish should generate the report").toBe(200);

  // ── Verify the flagship report generated: 成长报告 · 学习记录 is populated ───
  await openRail(page, "成长报告");
  await page.getByRole("button", { name: "学习记录" }).click();
  await expect(page.getByText("还没有报告")).toHaveCount(0, { timeout: 20_000 });
});
