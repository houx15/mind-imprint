#!/usr/bin/env node
/**
 * 觉醒协议 · 接口层走查。
 *
 * 它不开浏览器。这一条链子上真正会坏的东西全在服务端 —— 节点怎么推进、词有没有
 * 写回树、第二趟知不知道她已经有什么、报告里那几篇是不是真的文章 —— 而这些用
 * 接口打一遍又快又确定。界面另外用真浏览器看（AGENTS.md §测试只写逻辑测试）。
 *
 * 用法：
 *
 *     E2E_API_BASE=https://mind-api.uni-robot.cn \
 *     E2E_JOIN_CODE=G624-UXFE \
 *     node apps/lite-web/e2e/awakening-walk.mjs
 *
 * 🚨 线上的 Demo School 是 **pro**，拿 DEMO-0001 注册出来的账号打轻量版的路
 * 一律 404。线上要用轻量版体验班的 join code（见 e2e/freshAccount.ts 的注释）。
 *
 * 🚨 每个场景**自己注册一个账号**。共用一个种子账号的话，「她还没走过一趟」
 * 这个前提就由跑的顺序在守，而那守不住（同 freshAccount.ts 的教训）。
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");
const JOIN_CODE = process.env.E2E_JOIN_CODE ?? "G624-UXFE";

/* ── 一个带 cookie 的最小客户端 ─────────────────────────────────────────── */

function client() {
  let cookie = "";
  return async function call(method, path, body) {
    const res = await fetch(`${API}${path}`, {
      method,
      headers: {
        "content-type": "application/json",
        ...(cookie ? { cookie } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const set = res.headers.getSetCookie?.() ?? [];
    for (const c of set) cookie = c.split(";")[0];
    const text = await res.text();
    let json = null;
    try {
      json = text ? JSON.parse(text) : null;
    } catch {
      /* 非 JSON 的回应照原样留在 text 里，报错时打出来 */
    }
    return { status: res.status, json, text };
  };
}

async function signUp(label) {
  const call = client();
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  const email = `awk-${label}-${tag}@demo.mindimprint.local`;
  const password = `awk-${tag}-pass`;
  const up = await call("POST", "/api/v1/auth/signup", {
    email,
    password,
    display_name: `走查 ${label}`,
    join_code: JOIN_CODE,
  });
  if (up.status >= 400) throw new Error(`注册失败：${up.status} ${up.text}`);
  const inn = await call("POST", "/api/v1/auth/signin", { email, password });
  if (inn.status >= 400) throw new Error(`登录失败：${inn.status} ${inn.text}`);
  return { call, email };
}

/* ── 学生的话 ───────────────────────────────────────────────────────────── */

// 一个真的会写字的学生。八问各一句，内容互相衔接。
const PHOEBE = [
  "最近老是刷到潮汐发电的视频，一个海湾里的闸门一开一合就能发电，我看了四十分钟还在看",
  "最吸引我的是那个闸门的节奏，它不是一直转，而是要等潮水到某个高度才动一次",
  "我家在海边，小时候赶海要看潮汐表，我一直觉得那张表很神奇，现在发现它跟发电是同一件事",
  "我想不通的是，既然潮汐这么规律，为什么全世界用潮汐发电的地方这么少",
  "为什么潮汐发电在少数海岸能建起来，在大多数海岸却建不起来？",
  "我需要先读懂潮差和地形的基础概念，再看一两个真的建成了的案例",
  "我猜是因为要有很大的潮差和很窄的海湾，但如果看到平缓海岸也有成功的例子，我会改想法",
  "我想做一个给同学看的图解，让他们一眼看出为什么我家那片海滩建不了",
];

// 第二趟的她。方向变了一点 —— 从潮汐挪到了做东西本身。
const PHOEBE_AGAIN = [
  "这学期我一直在折腾一个小水轮，用矿泉水瓶和竹签做的，做坏了六个",
  "最让我在意的是叶片角度，差五度出水量就完全不一样",
  "上次我说潮汐表很神奇，现在我自己在调那个角度，感觉是同一种事情",
  "我想不通为什么书上给的最佳角度在我这里不成立",
  "在什么条件下，书上给的叶片最佳角度会失效？",
  "我需要看流体力学里关于叶片的基础解释，还有别人做的失败记录",
  "我猜是因为我的水流速度比书上假设的小很多，看到流速数据我就能确认",
  "我想写一份给同学的制作笔记，把六次失败都记下来",
];

// 第二趟的她，但**方向没变** —— 还是潮汐和海岸，只是又往前走了一点。
//
// 🚨 这一份不是多余的。PHOEBE_AGAIN 故意换了方向，于是选词落不到同一个词上，
// 「又出现一次的词强度要涨」这条判据就一直被跳过 —— 而那正是这次重建最核心的
// 承诺（重做是接着长，不是从头来）。要验它，就得有一个方向没变的学生。
const PHOEBE_SAME = [
  "这两个月我还在看潮汐发电，又找了三个海湾的资料，越看越想弄明白",
  "最吸引我的还是闸门什么时候开这件事，它要等潮水到一个高度才动",
  "我家在海边，赶海要看潮汐表，那张表和发电站看的是同一件事",
  "我想不通为什么潮汐这么规律，全世界真的建起来的潮汐电站却这么少",
  "为什么同样的潮差，有的海岸建得起来，有的海岸建不起来？",
  "我需要先弄懂潮差和海湾地形的关系，再看两个建成的案例",
  "我猜是因为要有很窄的海湾，但如果平缓海岸也有成功的例子，我就得改想法",
  "我想给同学做一张图，让他们看出我家那片海滩为什么建不了",
];

// 一个还说不清的学生。每一问都很薄 —— 服务端应该换个问法再问一次。
const THIN = ["不知道", "说不清", "没想过", "都可以", "随便", "不知道", "没什么", "都喜欢"];

/* ── 走一趟 ─────────────────────────────────────────────────────────────── */

const log = (...a) => console.log(...a);
const fail = (msg) => {
  console.error(`  ✗ ${msg}`);
  failures.push(msg);
};
const ok = (msg) => log(`  ✓ ${msg}`);
const failures = [];

/** 存一次进度。整份状态都要带上 —— 服务端整行覆盖。 */
async function save(call, id, state) {
  const r = await call("PUT", `/api/v1/awakening/${id}`, state);
  if (r.status >= 400) throw new Error(`存进度失败（${state.stage}）：${r.status} ${r.text}`);
  return r.json;
}

function blankState() {
  return {
    stage: "boot",
    route: "",
    navigator: "",
    energyProfile: {},
    talent: {},
    lensChoice: "",
    challengeChoice: "",
    archiveAttempts: 0,
    observerQuestion: "",
  };
}

/** 把八问答完。返回每一轮服务端回的东西。 */
async function answerAll(call, id, answers, { maxTurns = 20 } = {}) {
  const turns = [];
  for (let i = 0; i < maxTurns; i++) {
    const node = turns.length === 0 ? 0 : turns[turns.length - 1].nextNode;
    if (node >= 8) break;
    const text = answers[Math.min(node, answers.length - 1)];
    const r = await call("POST", `/api/v1/awakening/${id}/turn`, { text });
    if (r.status >= 400) throw new Error(`第 ${i + 1} 轮失败：${r.status} ${r.text}`);
    turns.push(r.json);
    if (r.json.done) break;
  }
  return turns;
}

/* ── 场景 ───────────────────────────────────────────────────────────────── */

/** 1 · 一个新学生，走观察者那条长路（看完 AI 底牌再加入）。 */
async function scenarioNewStudentLongRoute() {
  log("\n【1】新学生 · 走完整条路（含 AI 底牌）");
  const { call } = await signUp("new-long");

  let s = await call("GET", "/api/v1/awakening");
  if (s.json.taken !== false) fail(`刚注册的学生 taken 应为 false，得到 ${s.json.taken}`);
  if (s.json.open !== null) fail("刚注册的学生不该有未走完的一趟");
  ok("刚注册：没走过，也没有未完成的一趟");

  const started = await call("POST", "/api/v1/awakening");
  if (started.status >= 400) throw new Error(`开一趟失败：${started.status} ${started.text}`);
  const run = started.json;
  if (run.attemptNo !== 1) fail(`第一趟的 attemptNo 应为 1，得到 ${run.attemptNo}`);
  if (!run.openingAsk) fail("第一屏没有开场问题");
  ok(`开了一趟（attempt ${run.attemptNo}）`);

  // 空树上的第一问应当是默认问法，不该提到任何一个词。
  if (run.openingAsk.includes("上次")) fail(`空树的第一问不该说「上次」：${run.openingAsk}`);
  ok("空树的第一问是默认问法");

  const st = blankState();
  for (const stage of ["world", "warning", "archive", "deck", "rejoin"]) {
    Object.assign(st, { stage, route: "observer" });
    if (stage === "archive") st.archiveAttempts = 2; // 她试了两次才选对
    await save(call, run.id, st);
  }
  ok("走完了序章 → 档案 → 底牌 → 重新决定（archiveAttempts 记下了 2 次）");

  Object.assign(st, {
    stage: "navigator",
    route: "joined",
    navigator: "SAGE",
    energyProfile: { focus: "弄清楚规则（分析 / 推理 / 整理）", domains: [{ id: "system", name: "弄清楚规则", short: "分析 / 推理 / 整理", score: 3 }] },
  });
  await save(call, run.id, st);
  st.stage = "terminal";
  await save(call, run.id, st);
  ok("选了 SAGE，能量方向已存");

  const turns = await answerAll(call, run.id, PHOEBE);
  const failed = turns.filter((t) => t.failed).length;
  if (failed > 0) fail(`有 ${failed} 轮模型没回上来`);
  if (!turns[turns.length - 1].done) fail("八问没有走完");
  ok(`八问走完，共 ${turns.length} 轮，${failed} 轮失败`);

  st.stage = "talent";
  st.lensChoice = "wonder";
  st.challengeChoice = "compare";
  st.talent = {
    selected: ["logic", "explore", "making", "organize", "visual"],
    lanes: { energy: ["logic", "explore"], learned: ["organize"], latent: ["making", "visual"] },
  };
  await save(call, run.id, st);

  const rep = await call("POST", `/api/v1/awakening/${run.id}/finish`);
  if (rep.status >= 400) throw new Error(`生成报告失败：${rep.status} ${rep.text}`);
  const report = rep.json.report;
  checkReport(report, { attemptNo: 1, expectDiff: false });

  // 树真的长了吗。
  const tree = await call("GET", "/api/v1/interest/tree");
  const words = tree.json?.keywords ?? [];
  if (report.pursuing.length > 0 && words.length === 0) {
    fail("报告说长了词，但树上一个都没有");
  } else if (report.pursuing.length > 0) {
    ok(`树上现在有 ${words.length} 个词：${words.map((w) => w.textZh).join("、")}`);
    // 来源必须记在这一趟上。
    const src = words[0]?.sources ?? [];
    if (!src.some((x) => x.refId === run.id)) {
      fail(`第一个词的来源里没有这一趟的 id（来源：${JSON.stringify(src)}）`);
    } else {
      ok("词的来源指向这一趟");
    }
  }

  // 走完之后状态翻过来了没有。
  s = await call("GET", "/api/v1/awakening");
  if (s.json.taken !== true) fail("走完之后 taken 应为 true");
  if (s.json.open !== null) fail("走完之后不该还有未完成的一趟");
  if (s.json.latestReportRunId !== run.id) fail("latestReportRunId 对不上");
  ok("走完之后：taken=true，没有未完成的一趟，报告可直接打开");

  return { call, runId: run.id, report };
}

/** 2 · 中途退出，第二天回来接着走。 */
async function scenarioResume() {
  log("\n【2】中途退出 · 回来接着走");
  const { call } = await signUp("resume");
  const run = (await call("POST", "/api/v1/awakening")).json;

  const st = blankState();
  Object.assign(st, {
    stage: "navigator",
    route: "joined",
    navigator: "KIRO",
    energyProfile: { focus: "动手做出来（搭建 / 拆解 / 改进）" },
  });
  await save(call, run.id, st);

  // 「关掉页面」= 换一个新的客户端，只带同一个账号的 cookie。这里直接重查。
  const s = await call("GET", "/api/v1/awakening");
  if (!s.json.open) {
    fail("未走完的一趟没有被返回");
    return;
  }
  if (s.json.open.id !== run.id) fail("返回的不是同一趟");
  if (s.json.open.stage !== "navigator") fail(`接着走的那一屏应是 navigator，得到 ${s.json.open.stage}`);
  if (s.json.open.navigator !== "KIRO") fail("她选的助手没有留下来");
  if (s.json.taken !== false) fail("中途退出不算「做过了」");
  ok("回来时落在 navigator，助手和能量方向都还在，且不算做过");

  // 再点一次「开始」不该开第二趟。
  const again = (await call("POST", "/api/v1/awakening")).json;
  if (again.id !== run.id) fail("再点一次开始，开出了新的一趟");
  else ok("再点一次开始，回到同一趟");
}

/** 3 · 一个说不清的学生：服务端换问法再问，报告照实说。 */
async function scenarioThinAnswers() {
  log("\n【3】说不清的学生 · 换问法再问，报告照实说");
  const { call } = await signUp("thin");
  const run = (await call("POST", "/api/v1/awakening")).json;
  const st = blankState();
  Object.assign(st, { stage: "terminal", route: "joined", navigator: "NOVA" });
  await save(call, run.id, st);

  const turns = await answerAll(call, run.id, THIN, { maxTurns: 24 });
  const retries = turns.filter((t) => t.retry).length;
  if (retries === 0) fail("每一问都很薄，却一次换问法都没有发生");
  else ok(`发生了 ${retries} 次换问法（共 ${turns.length} 轮）`);

  // 同一个节点最多问两轮，不能把她卡死。
  const perNode = {};
  for (const t of turns) perNode[t.nodeIndex] = (perNode[t.nodeIndex] ?? 0) + 1;
  const stuck = Object.entries(perNode).filter(([, n]) => n > 2);
  if (stuck.length > 0) fail(`有节点问了超过两轮：${JSON.stringify(stuck)}`);
  else ok("没有节点把她卡住（每个节点最多两轮）");

  st.stage = "talent";
  st.talent = { selected: [], lanes: { energy: [], learned: [], latent: [] } };
  await save(call, run.id, st);
  const rep = await call("POST", `/api/v1/awakening/${run.id}/finish`);
  const report = rep.json.report;
  if (report.pursuing.length > 0) {
    // 允许，但每个词的 evidence 必须真的出自她写的那几句。
    for (const w of report.pursuing) {
      if (!THIN.join("\n").includes(w.evidence)) {
        fail(`长出了一个 evidence 不在她原话里的词：${w.zh} / ${w.evidence}`);
      }
    }
    ok(`长出了 ${report.pursuing.length} 个词，evidence 都查得到`);
  } else {
    ok("一个词都没长出来 —— 报告会照实说，这是对的");
  }
}

/** 4 · 老学生再走一趟：第一问从她的词出发，报告带上和上次的差。 */
async function scenarioRetake(first) {
  log("\n【4】老学生 · 再走一趟");
  const { call, report: firstReport } = first;

  const s = await call("GET", "/api/v1/awakening");
  if (s.json.taken !== true) fail("她应该已经走过一趟");

  const run = (await call("POST", "/api/v1/awakening")).json;
  if (run.attemptNo !== 2) fail(`第二趟的 attemptNo 应为 2，得到 ${run.attemptNo}`);
  ok(`开了第二趟（attempt ${run.attemptNo}）`);

  // 🚨 这就是那条回路：第一问必须从她树上已有的词出发。
  if (firstReport.pursuing.length > 0) {
    const hers = firstReport.pursuing.map((w) => w.zh);
    const mentions = hers.some((w) => run.openingAsk.includes(w));
    if (!mentions) {
      fail(`第二趟的第一问没有提到她已有的词（${hers.join("、")}）：${run.openingAsk}`);
    } else {
      ok(`第二趟的第一问从她的词出发：${run.openingAsk.slice(0, 48)}…`);
    }
  } else {
    // 第一趟一个词都没长出来时，第二趟仍然该从头问 —— 这也是对的。
    if (run.openingAsk.includes("上次")) fail("树上没有词，却说了「上次」");
    else ok("第一趟没长出词，第二趟仍然从头问（判据看树，不看次数）");
  }

  const st = blankState();
  Object.assign(st, { stage: "terminal", route: "joined", navigator: "NOVA" });
  await save(call, run.id, st);
  const turns = await answerAll(call, run.id, PHOEBE_AGAIN);
  if (!turns[turns.length - 1].done) fail("第二趟八问没走完");
  else ok(`第二趟八问走完（${turns.length} 轮）`);

  st.stage = "talent";
  st.talent = {
    selected: ["making", "logic", "explore", "body", "organize"],
    lanes: { energy: ["making", "body"], learned: ["organize"], latent: ["logic", "explore"] },
  };
  await save(call, run.id, st);

  const rep = await call("POST", `/api/v1/awakening/${run.id}/finish`);
  const report = rep.json.report;
  checkReport(report, { attemptNo: 2, expectDiff: true });

  if (report.diff) {
    // 🚨 这里逐个兜底不是防御性代码，是判据的一部分：Go 的 nil 切片会
    // marshal 成 null，而前端对它调 .join()。走查必须**看得见** null，
    // 而不是在它上面崩掉。
    const stronger = report.diff.stronger;
    const grown = report.diff.new;
    if (stronger === null || grown === null) {
      fail(`diff 里有 null（前端会在它上面崩掉）：${JSON.stringify(report.diff)}`);
    }
    ok(`本次变化：变强 ${(stronger ?? []).length} 个，新长 ${(grown ?? []).length} 个`);
    if (!report.diff.previousQuestion) fail("本次变化那一块没有带上上次的问题");
    else ok(`带上了上次的问题：${report.diff.previousQuestion.slice(0, 30)}…`);
  }

  // confirm 的那几个强度必须比 1 高 —— 那正是「重做是再长几个词」。
  const confirmed = report.pursuing.filter((w) => w.verdict === "confirm");
  if (confirmed.length > 0) {
    const weak = confirmed.filter((w) => w.strength < 2);
    if (weak.length > 0) fail(`有 confirm 的词强度仍是 1：${weak.map((w) => w.zh).join("、")}`);
    else ok(`${confirmed.length} 个词又出现了一次，强度都涨到了 2 以上`);
  } else {
    log("  · 这一趟没有 confirm（她的方向确实变了），跳过强度检查");
  }
}

/** 5 · 观察者：她选择不继续，树上不该多出任何东西。 */
async function scenarioObserverOptOut() {
  log("\n【5】观察者 · 选择不继续");
  const { call } = await signUp("observer");
  const run = (await call("POST", "/api/v1/awakening")).json;
  const st = blankState();
  for (const stage of ["world", "warning", "archive", "deck", "rejoin", "observer"]) {
    Object.assign(st, { stage, route: "observer" });
    if (stage === "observer") st.observerQuestion = "bias";
    await save(call, run.id, st);
  }
  const s = await call("GET", "/api/v1/awakening");
  if (s.json.taken !== false) fail("观察者没有走完，不该算「做过了」");
  if (!s.json.open || s.json.open.observerQuestion !== "bias") fail("她带走的那个问题没有留下来");
  else ok("落在观察者那一屏，带走的问题已记下，且不算做过");

  const tree = await call("GET", "/api/v1/interest/tree");
  if ((tree.json?.keywords ?? []).length !== 0) fail("她还没答任何问题，树上不该有词");
  else ok("树上没有任何词 —— 没答问题就不该有");
}

/**
 * 6 · 方向没变的重做：又出现的那个词必须**接着长**，不是从头来。
 *
 * 这条回路的核心承诺就在这里。场景 4 的她换了方向，于是一个 confirm 都没有、
 * 强度那条判据被整段跳过 —— 走查绿着，而最重要的那件事没人验过。
 */
async function scenarioSameDirectionRetake() {
  log("\n【6】方向没变的重做 · 又出现的词要接着长");
  const { call } = await signUp("confirm");

  // ── 第一趟 ──
  const run1 = (await call("POST", "/api/v1/awakening")).json;
  const st = blankState();
  Object.assign(st, { stage: "terminal", route: "joined", navigator: "NOVA" });
  await save(call, run1.id, st);
  const t1 = await answerAll(call, run1.id, PHOEBE);
  if (!t1[t1.length - 1].done) {
    fail("第一趟八问没走完");
    return;
  }
  st.stage = "talent";
  st.talent = { selected: ["logic", "explore"], lanes: { energy: ["logic"], learned: [], latent: ["explore"] } };
  await save(call, run1.id, st);
  const r1 = (await call("POST", `/api/v1/awakening/${run1.id}/finish`)).json.report;
  const first = r1.pursuing.map((w) => w.zh);
  if (first.length === 0) {
    if (r1.selectionFailed) {
      fail("第一趟选词失败（不是她写得不够 —— 是我们没跑成），这条判据没法验");
    } else {
      fail("第一趟一个词都没长出来，这条判据没法验（换一份答案）");
    }
    return;
  }
  ok(`第一趟长出：${first.join("、")}`);

  // ── 第二趟，同一个方向 ──
  const run2 = (await call("POST", "/api/v1/awakening")).json;
  if (run2.attemptNo !== 2) fail(`第二趟的 attemptNo 应为 2，得到 ${run2.attemptNo}`);
  const st2 = blankState();
  Object.assign(st2, { stage: "terminal", route: "joined", navigator: "NOVA" });
  await save(call, run2.id, st2);
  const t2 = await answerAll(call, run2.id, PHOEBE_SAME);
  if (!t2[t2.length - 1].done) {
    fail("第二趟八问没走完");
    return;
  }
  st2.stage = "talent";
  st2.talent = { selected: ["logic", "explore"], lanes: { energy: ["logic"], learned: [], latent: ["explore"] } };
  await save(call, run2.id, st2);
  const r2 = (await call("POST", `/api/v1/awakening/${run2.id}/finish`)).json.report;
  ok(`第二趟长出：${r2.pursuing.map((w) => w.zh).join("、") || "（无）"}`);

  // 判据一：她说的还是同一件事，就该有词被判成 confirm。
  const confirmed = r2.pursuing.filter((w) => w.verdict === "confirm");
  if (confirmed.length === 0) {
    fail(
      `她两趟说的是同一件事（第一趟：${first.join("、")}），第二趟却一个 confirm 都没有 —— ` +
        `这意味着重做永远不会让已有的词变强，回路是断的`,
    );
    return;
  }
  ok(`${confirmed.length} 个词又出现了一次：${confirmed.map((w) => w.zh).join("、")}`);

  // 判据二：confirm 的词强度必须高于 1。
  const weak = confirmed.filter((w) => w.strength < 2);
  if (weak.length > 0) fail(`有 confirm 的词强度仍是 1：${weak.map((w) => `${w.zh}=${w.strength}`).join("、")}`);
  else ok(`它们的强度都涨到了 2 以上：${confirmed.map((w) => `${w.zh}=${w.strength}`).join("、")}`);

  // 判据三：机制本身 —— 树上那个词要挂着**两趟各一条**来源。
  const tree = await call("GET", "/api/v1/interest/tree");
  const words = tree.json?.keywords ?? [];
  const hit = words.find((w) => confirmed.some((c) => c.zh === w.textZh));
  if (!hit) {
    fail(`树上找不到又出现的那个词：${confirmed.map((c) => c.zh).join("、")}`);
  } else {
    const refs = new Set((hit.sources ?? []).map((x) => x.refId));
    if (!refs.has(run1.id) || !refs.has(run2.id)) {
      fail(`「${hit.textZh}」的来源没有同时指向两趟（来源：${[...refs].join("、")}）`);
    } else {
      ok(`「${hit.textZh}」挂着两趟各一条来源 —— 强度是这么涨上来的`);
    }
  }

  // 判据四：报告的「本次变化」要把它算作变强，而不是新长。
  const stronger = r2.diff?.stronger ?? [];
  const missing = confirmed.filter((c) => !stronger.includes(c.zh));
  if (missing.length > 0) fail(`「本次变化」里没把 ${missing.map((c) => c.zh).join("、")} 算作变强`);
  else ok(`「本次变化」把它算作变强：${stronger.join("、")}`);
}

/* ── 报告的共用检查 ─────────────────────────────────────────────────────── */

function checkReport(report, { attemptNo, expectDiff }) {
  if (!report) {
    fail("没有拿到报告");
    return;
  }
  if (report.attemptNo !== attemptNo) fail(`报告的 attemptNo 应为 ${attemptNo}，得到 ${report.attemptNo}`);

  // 她自己写的那两句必须**原样**在报告里。
  if (!report.question) fail("报告里没有她的问题");
  else ok(`她的问题原样带上了：${report.question.slice(0, 28)}…`);
  if (!report.workConcept) fail("报告里没有她的作品设想");

  // 每个词的 evidence 必须是她原话的子串。
  const corpus = (report.answers ?? []).join("\n");
  for (const w of report.pursuing) {
    if (!corpus.includes(w.evidence)) fail(`词「${w.zh}」的 evidence 不在她的原话里：${w.evidence}`);
  }
  // 每条驱动力同样。
  for (const d of report.drivers) {
    if (!corpus.includes(d.evidence)) fail(`驱动力「${d.label}」的 evidence 不在她的原话里：${d.evidence}`);
  }
  ok(`${report.pursuing.length} 个词 / ${report.drivers.length} 条驱动力，evidence 全部查得到`);

  // 🚨 零词有两种来路，而只有一种是允许的。selectionFailed 说明是**我们**没
  // 跑成（调用失败 / 回话读不懂），那时报告会对她说「你写得不够具体」——
  // 2026-09-19 线上真的发生过。走查必须把这一种叫出来。
  if (report.selectionFailed) {
    fail("选词那一步失败了（报告会把我们的故障说成她写得不够具体）");
  }

  // 推荐的必须是真文章：有 slug，而且有命中的学科（补位的一律不该出现）。
  for (const a of report.readings) {
    if (!a.slug) fail("推荐里有一篇没有 slug");
    if (!a.why || a.why.length === 0) fail(`推荐「${a.zhTitle || a.title}」没有命中学科 —— 那是补位的，不该出现在报告里`);
  }
  if (report.readings.length > 0) {
    ok(`推了 ${report.readings.length} 篇真文章：${report.readings.map((a) => a.zhTitle || a.title).join(" / ")}`);
  } else {
    ok("库里没有对得上的材料，报告会照实说（这是允许的结果）");
  }

  if (expectDiff && !report.diff) fail("第二趟应该有「本次变化」那一块");
  if (!expectDiff && report.diff) fail("第一趟不该有「本次变化」那一块");
}

/* ── 跑 ─────────────────────────────────────────────────────────────────── */

const only = process.argv[2];

const scenarios = {
  async all() {
    const first = await scenarioNewStudentLongRoute();
    await scenarioResume();
    await scenarioThinAnswers();
    await scenarioRetake(first);
    await scenarioObserverOptOut();
    await scenarioSameDirectionRetake();
  },
};

(async () => {
  log(`觉醒协议走查 → ${API}（join code ${JOIN_CODE}）`);
  const started = Date.now();
  try {
    await (scenarios[only] ?? scenarios.all)();
  } catch (e) {
    console.error(`\n走查中断：${e instanceof Error ? e.message : String(e)}`);
    failures.push(String(e));
  }
  log(`\n用时 ${((Date.now() - started) / 1000).toFixed(1)}s`);
  if (failures.length > 0) {
    console.error(`\n${failures.length} 处失败：`);
    for (const f of failures) console.error(`  · ${f}`);
    process.exit(1);
  }
  log("\n全部通过。");
})();
