import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * guidance-matrix —— 「不同文体、不同语言，拿到的带法真的不一样吗」。
 *
 * 这是二期到四期那条**轴**本身的验收。别的走查各自只站在矩阵的一格里
 * （teaching-r4 走中文记叙、writing-walk 走中文议论），所以「四格拿到的
 * 是同一份」这种事，它们全绿也照样发现不了。
 *
 * 🚨 这条要抓的就是那个形状：轴建起来了，但只通到浅处。
 * 2026-09-22 那次是 `vocab.Structures` 根本不看语言；2026-09-23 跑这条
 * 之前量出来的是 `writingGenreOf` 的记叙文词表全是中文子串，于是**每一篇
 * 英文**都被判成议论文 —— 四格里有一格根本到不了。
 *
 * 判据不钉模型的原话（那是每轮都会变的），钉的是**服务端决定的东西**：
 * 开出来的块属于哪一套、术语用的是哪一种。
 *
 * 跑法：
 *   npx playwright test -c e2e/online.config.ts guidance-matrix
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

const ARGUMENT_KINDS = ["thesis", "point", "counter", "rebuttal", "reasoning"];
const NARRATIVE_KINDS = ["scene", "detail", "turn", "feeling"];

type Cell = {
  name: string;
  lang: "zh" | "en";
  idea: string;
  say: string;
  expect: "argument" | "narrative";
};

// 四格。题目都挑成「这一篇显然是什么」，因为文体只能从题目推
// （没有下拉，见 writing_setup.go）。
const CELLS: Cell[] = [
  {
    name: "zh × 议论文",
    lang: "zh",
    idea: "学校是否应该允许学生带手机",
    say: "我觉得应该允许，因为可以联系家长，也能用来查资料。",
    expect: "argument",
  },
  {
    name: "zh × 记叙文",
    lang: "zh",
    idea: "记一次难忘的经历",
    say: "我想写那天下大雨，我爸来补习班接我，我在楼道口等了很久。",
    expect: "narrative",
  },
  {
    name: "en × 议论文",
    lang: "en",
    idea: "Should schools allow students to bring phones?",
    say: "I think they should, because students can reach their parents and look things up.",
    expect: "argument",
  },
  {
    name: "en × 记叙文",
    lang: "en",
    idea: "Write about a day you will never forget",
    say: "I want to write about the afternoon my grandmother taught me to ride a bike.",
    expect: "narrative",
  },
];

for (const cell of CELLS) {
  test(`写作矩阵：${cell.name} 拿到的是这一格自己的带法`, async ({ browser }) => {
    test.setTimeout(300_000);
    const ctx = await freshAccount(browser, `matrix-${cell.lang}-${cell.expect}`);

    const created = await ctx.request.post(`${API}/api/v1/writings`, {
      data: { idea: cell.idea, lang: cell.lang },
    });
    expect(created.ok(), `创建失败 ${created.status()}`).toBeTruthy();
    const id = (await created.json()).id as string;

    const setup = await ctx.request.put(`${API}/api/v1/writings/${id}/setup`, {
      data: { lang: cell.lang, targetWords: 800, note: "" },
    });
    expect(setup.ok(), `设定失败 ${setup.status()}`).toBeTruthy();

    const turn = await ctx.request.post(`${API}/api/v1/writings/${id}/plan/turn`, {
      data: { text: cell.say },
    });
    expect(turn.ok(), `立题一轮失败 ${turn.status()} ${await turn.text()}`).toBeTruthy();
    const reply = (await turn.json()).reply as string;

    const outline = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/outline`)).json())
      .outline as { kind: string; text: string }[];

    console.log(`\n──────── ${cell.name} ────────`);
    console.log(`题目：${cell.idea}`);
    console.log(`印记：${reply}`);
    console.log(`块：  ${outline.map((o) => `${o.kind}「${o.text}」`).join("  ") || "（这一轮没开块）"}`);

    const kinds = outline.map((o) => o.kind);
    const sawArgument = kinds.some((k) => ARGUMENT_KINDS.includes(k));
    const sawNarrative = kinds.some((k) => NARRATIVE_KINDS.includes(k));

    if (cell.expect === "narrative") {
      // 🚨 这一格的全部意义：不许拿到议论文的骨架。
      expect(
        sawArgument,
        `${cell.name} 开出了议论文的块（${kinds.join(",")}）—— 文体这条轴没走到这一格`,
      ).toBeFalsy();
    } else {
      expect(
        sawNarrative,
        `${cell.name} 开出了记叙文的块（${kinds.join(",")}）`,
      ).toBeFalsy();
    }

    // 语言这条轴：英文那两格要用英文术语跟她讲，中文那两格不许出现它们。
    // 术语逐字来自 prompts/api_writing_plan_lang.go。
    const EN_TERMS = ["thesis statement", "topic sentence", "commentary"];
    const hit = EN_TERMS.filter((t) => reply.toLowerCase().includes(t));
    console.log(`英文术语：${hit.length ? hit.join(", ") : "（无）"}`);
    if (cell.lang === "zh") {
      expect(
        hit,
        `中文那一篇的回复里出现了英文写作术语：${hit.join(", ")}`,
      ).toHaveLength(0);
    }

    await ctx.close();
  });
}

// ───────────────────────── 阅读：体裁这条轴 ─────────────────────────
//
// 阅读面的体裁不是从题目推的，是排读法那一轮由模型判的（reading_outline.go）。
// 判出来之后有两个下游：`pickRoutineForGenre` 换掉这一篇的步骤，
// `buildGenreCoachSection` 给带读提示词加上这一体裁那一节
// （议论文没有这一节，那是对的 —— 见 reading_genre.go 开头）。
//
// 所以这里钉两样**服务端决定**的东西：判出来的体裁、选中的 routine。
// 陪练说的话只打印出来给人读，不拿它当判据（那是每轮都会变的）。

type Article = { name: string; genre: string; routine: string; title: string; body: string };

const ARTICLES: Article[] = [
  {
    name: "议论文",
    genre: "argument",
    routine: "zh-scan-focus-lens",
    title: "该不该给中学生留书面作业",
    body: [
      "近年来，关于中学生书面作业的争论一直没有停下来。我认为，书面作业应当保留，但必须控制总量。",
      "首先，书面作业是把课堂知识变成个人能力的一道工序。听懂和会做是两件事，只有亲手写一遍，学生才知道自己哪一步是糊涂的。取消作业，等于取消了这道自检。",
      "其次，反对者担心作业挤占睡眠与运动。这个担心是成立的，但它针对的是作业的总量，不是作业本身。把每天的书面作业限制在一小时以内，既保留了练习，也守住了休息。",
      "有人提出用课堂练习完全替代课后作业。这种做法在小班里可行，但在一个五十人的班上，教师无法照顾到每个人的节奏，课后练习仍然是必要的补充。",
      "因此，真正要改的不是有没有作业，而是作业的量和质。把重复抄写换成少量而有思考含量的题目，这场争论才有出路。",
    ].join("\n\n"),
  },
  {
    name: "新闻报道",
    genre: "report",
    routine: "zh-report",
    title: "本市三座旧桥启动加固，预计年底完工",
    body: [
      "本报讯 记者从市交通局获悉，本市将于下月起对城东、城南、城西三座建成超过四十年的旧桥进行加固施工，预计今年年底完工。",
      "市交通局桥梁处处长李明在发布会上表示，三座桥梁在今年上半年的例行检测中被评为二类，主要问题是桥面铺装老化与部分支座位移。他说：「结构主体是安全的，这次加固属于预防性维护。」",
      "施工期间，三座桥将采取半幅封闭、交替通行的方式。城东桥每日早晚高峰不封闭，其余时段单向通行。",
      "在城南桥附近经营早点铺的王姓摊主对记者说，她担心封路会让客流减少，希望施工方能把围挡往外挪一些。市交通局回应称，将在施工前与沿线商户逐户沟通。",
      "截至发稿时，施工单位尚未公布每座桥的具体封闭时段。",
    ].join("\n\n"),
  },
  {
    name: "说明文",
    genre: "explain",
    routine: "zh-explain",
    title: "海水为什么是咸的",
    body: [
      "海水的咸味来自溶解在其中的盐类，其中含量最高的是氯化钠，也就是食盐的主要成分。",
      "这些盐分主要有两个来源。其一是陆地上的岩石。雨水中溶有二氧化碳，呈微弱的酸性，它在流过岩石时会缓慢地溶解其中的矿物质，把钠、钙、镁等离子带进河流，再由河流汇入海洋。",
      "其二是海底的火山与热液喷口。海水渗入洋壳，在高温下与岩石发生反应，重新喷出时带出了大量溶解物质。",
      "河水同样含有盐分，为什么尝不出咸味？因为河水中的盐分浓度极低，而且河水始终在流动、更新。海洋则不同：水分不断蒸发进入大气，盐分却留了下来。经过漫长的地质时间，盐分因此逐渐积累。",
      "海水的平均盐度约为百分之三点五，也就是每一千克海水中约含三十五克盐。不同海域的盐度并不相同：蒸发强烈而淡水补给少的海域盐度偏高，有大河注入的近海则偏低。",
    ].join("\n\n"),
  },
  {
    name: "英文记叙文",
    genre: "narrative",
    routine: "en-narrative",
    title: "The Last Bus",
    body: [
      "That winter I worked the night shift at a factory on the edge of town, and every night I had to catch the last bus home.",
      "One night I came out late. When I reached the stop my watch said 10:47. The timetable on the pole said 10:45. There was nobody under the sign, only a streetlight swinging in the wind. I thought I would be walking.",
      "I set off along the road. After about twenty minutes I heard an engine behind me. A bus pulled up beside me and the door opened. The driver was a man of about fifty. He said, \"Get in. I am running late tonight.\"",
      "I was the only passenger. He drove slowly, and after two stops I realised he kept watching the mirror, looking for anyone else running up behind us the way I had.",
      "When we reached my stop I asked him whether he did this every night. He smiled and said, \"Not every night. Only when it is very cold.\"",
      "I changed jobs later and never took that route again. But on the coldest nights I still think of that swinging streetlight, and the door that opened.",
    ].join("\n\n"),
  },
  {
    name: "记叙文",
    genre: "narrative",
    routine: "zh-narrative",
    title: "最后一班公交车",
    body: [
      "那年冬天，我在城郊的工厂上夜班，每天要赶最后一班公交回家。",
      "那天我出来得晚了。跑到站台的时候，表上是十点四十七分，末班车十点四十五。站牌下空无一人，只有一盏路灯在风里晃。我想，今天要走回去了。",
      "我沿着马路往家的方向走，走了大概二十分钟，身后忽然传来发动机的声音。一辆公交车停在我旁边，门开了。司机是个五十来岁的男人，他说：「上来吧，我今天晚点了。」",
      "车上只有我一个乘客。他开得很慢，过了两站，我才发现他其实一直在看后视镜——看有没有人像我一样在后面追。",
      "到站的时候我问他是不是每天都这样。他笑了一下说：「也不是每天。天太冷的时候才这样。」",
      "后来我换了工作，再没坐过那条线。但每年最冷的那几天，我总会想起那盏晃动的路灯，和那扇忽然打开的车门。",
    ].join("\n\n"),
  },
];

for (const art of ARTICLES) {
  test(`阅读矩阵：${art.name} 判得出体裁，并换上这一体裁的读法`, async ({ browser }) => {
    test.setTimeout(300_000);
    const ctx = await freshAccount(browser, `matrix-read-${art.genre}`);

    const created = await ctx.request.post(`${API}/api/v1/readings`, {
      data: { title: art.title },
    });
    expect(created.ok(), `创建阅读失败 ${created.status()} ${await created.text()}`).toBeTruthy();
    const id = (await created.json()).id as string;

    const src = await ctx.request.put(`${API}/api/v1/readings/${id}/source`, {
      data: { title: art.title, text: art.body },
    });
    expect(src.ok(), `贴原文失败 ${src.status()} ${await src.text()}`).toBeTruthy();

    const gen = await ctx.request.post(`${API}/api/v1/readings/${id}/plan`, { data: {} });
    expect(gen.ok(), `排读法失败 ${gen.status()} ${await gen.text()}`).toBeTruthy();

    const plan = await (await ctx.request.get(`${API}/api/v1/readings/${id}/plan`)).json();
    const steps = (plan.tasks ?? []) as { kind: string; title: string; detail: string }[];

    console.log(`\n──────── 阅读 · ${art.name} ────────`);
    console.log(`标题：${art.title}`);
    console.log(`routineKey：${plan.routineKey}   （${plan.routineName}）`);
    console.log(`步骤：`);
    for (const s of steps) console.log(`   - [${s.kind}] ${s.title ?? ""} ${s.detail ?? ""}`.trimEnd());

    // 🚨 钉的是 routineKey，不是体裁那个字。
    //
    // `GET /plan` 本来就不回体裁（只回 routineKey / routineName / tasks）——
    // 这是我第一版写错的地方，不是产品的。而且 routineKey 是更好的判据：
    // 体裁那个标签是中间产物，routineKey 才是**她真正拿到的那套读法**，
    // 由 pickRoutineForGenre 按体裁挑出来，一套体裁对一套读法。
    expect(
      plan.routineKey,
      `这一篇${art.name}拿到的读法是「${plan.routineKey}」，不是「${art.routine}」`
        + ` —— 体裁这条轴在阅读面没走到下游`,
    ).toBe(art.routine);

    await ctx.close();
  });
}
