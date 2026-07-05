// Per Aspera — Institute long-read research content.
//
// Source docs (authoritative; do not fabricate facts/numbers/names beyond
// what they state — see the docs' own "打假/confirmed blanks" sections for
// where the record runs out):
//   1. docs/astranova/ad-astra-调研报告.html        ("Ad Astra 深度调研")
//   2. docs/astranova/astra-nova高中申请与真实案例.html ("高中申请 + 案例")
// Both were retrieved/compiled 2026-07-05; official-site content is dated
// to that scrape, media citations carry their own publish dates.
//
// Scope note: doc 1's §3 "同类高端创新学校对比" (Alpha School, Synthesis,
// Khan Lab School, Minerva, Sora, Nueva, Acton comparison table) is reserved
// for Task D2's `schools` export (/institute/schools) — it is intentionally
// NOT duplicated here so `adAstra` stays self-contained and D2 owns that
// comparison without a second, drifting copy.
//
// zh is the source of truth; en is an idiomatic (not literal) translation.
// Every URL from both docs (excluding the D2-reserved §3 table) is preserved
// somewhere below — either inline as a citation or in the closing
// `adAstra.sources` list. No "不是…而是" antithesis; positive declaratives.

import type { Bilingual } from "./site";

export interface Cite {
  label: Bilingual;
  url: string;
}

export interface Bullet {
  text: Bilingual;
  cites?: Cite[];
}

/* ════════════════════════════════════════════════════════════════════════
   Page meta
   ════════════════════════════════════════════════════════════════════════ */

export const meta: {
  eyebrow: Bilingual;
  title: Bilingual;
  lede: Bilingual;
  dateNote: Bilingual;
} = {
  eyebrow: { zh: "研究院 · 公开研究", en: "Institute · Public research" },
  title: {
    zh: "Ad Astra / Astra Nova 完全解读",
    en: "The Full Ad Astra / Astra Nova Story",
  },
  lede: {
    zh: "“Musk 的学校”其实是一个谱系里的三个不同实体——招生标准、学生画像、培养方式完全不同。这篇长文把它们分开讲清楚：谁在招什么样的孩子、官方说法与实际操作之间的落差、分年龄段怎么培养、高中项目的三档设计、真实的录取与被拒案例，以及网上流传的夸大和伪造内容。每一条事实都带来源链接——这是这篇报告存在的意义。",
    en: "\"Musk's school\" is actually three distinct entities in one lineage — with admissions standards, student profiles, and teaching methods that differ completely. This long-read separates them out: who admits which kids, the gap between official language and actual practice, how each age band is taught, the three-tier design of the new high-school program, real admit and reject cases, and the exaggerations and fabrications circulating online. Every claim carries a source link — that is the entire point of this report.",
  },
  dateNote: {
    zh: "调研日期：2026 年 7 月 5 日 · 所有事实均附来源链接 · 官网内容为当日直接抓取（astranova.org、adastraschool.org），媒体报道注明发表日期。",
    en: "Research date: July 5, 2026. Every fact carries a source link. Official-site content reflects that day's scrape (astranova.org, adastraschool.org); media reports are dated to their original publication.",
  },
};

/* ════════════════════════════════════════════════════════════════════════
   0 · Three-entity genealogy
   ════════════════════════════════════════════════════════════════════════ */

export interface GenealogyEntity {
  date: Bilingual;
  label: Bilingual;
  form: Bilingual;
  audience: Bilingual;
  cites: Cite[];
}

export const genealogy: {
  intro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual };
  entities: GenealogyEntity[];
  extra: Bullet;
} = {
  intro: {
    eyebrow: { zh: "先厘清", en: "First, untangle it" },
    title: { zh: "三个「Ad Astra」", en: "Three different \"Ad Astra\"s" },
    body: {
      zh: "调研时必须分开看——它们是同一谱系里三个招生和培养方式完全不同的实体。",
      en: "Researching this school means keeping these apart — three entities in the same lineage, with completely different admissions and teaching methods.",
    },
  },
  entities: [
    {
      date: { zh: "2014–2020", en: "2014–2020" },
      label: {
        zh: "Ad Astra（SpaceX 校内），已注销",
        en: "Ad Astra (on the SpaceX campus), now dissolved",
      },
      form: {
        zh: "线下实验学校，位于加州 Hawthorne SpaceX 总部内，Josh Dahn 主理。",
        en: "An in-person experimental school inside SpaceX's Hawthorne, California headquarters, run by Josh Dahn.",
      },
      audience: {
        zh: "7–14 岁，Musk 的 5 个儿子 + SpaceX 员工子女 + 少量洛杉矶高潜力孩子，约 30–50 人，免学费。",
        en: "Ages 7–14: Musk's five sons, SpaceX employees' children, and a small number of high-potential Los Angeles kids — roughly 30–50 students total, tuition-free.",
      },
      cites: [
        { label: { zh: "Wikipedia: Astra Nova School", en: "Wikipedia: Astra Nova School" }, url: "https://en.wikipedia.org/wiki/Astra_Nova_School" },
        { label: { zh: "astranova.org（2026-07-05 抓取）", en: "astranova.org (scraped 2026-07-05)" }, url: "https://www.astranova.org/" },
        { label: { zh: "Texas Tribune, 2025-01-13", en: "Texas Tribune, 2025-01-13" }, url: "https://www.texastribune.org/2025/01/13/texas-elon-musk-school-education-bastrop-county/" },
        { label: { zh: "ProPublica 非营利数据库（Ad Astra EIN 47-1480453，已提交注销申报）", en: "ProPublica nonprofit database (Ad Astra EIN 47-1480453, dissolution filed)" }, url: "https://projects.propublica.org/nonprofits/organizations/471480453" },
      ],
    },
    {
      date: { zh: "2020–今", en: "2020–present" },
      label: { zh: "Astra Nova（astranova.org）", en: "Astra Nova (astranova.org)" },
      form: {
        zh: "Ad Astra 原班教师团队独立后成立的在线非营利学校（WASC 认证）；Musk 与 Dahn 各持一半 IP，但 Musk 无财务权益。",
        en: "An online nonprofit school (WASC-accredited), founded by Ad Astra's original teaching team after they went independent. Musk and Dahn each hold half the IP, but Musk has no financial stake.",
      },
      audience: {
        zh: "初中 11–14 岁（315 名学生，来自 45 国）；高中 14–18 岁，2026 年 8 月开学。",
        en: "Middle school, ages 11–14 (315 students from 45 countries); high school, ages 14–18, opening August 2026.",
      },
      cites: [
        { label: { zh: "astranova.org（2026-07-05 抓取）", en: "astranova.org (scraped 2026-07-05)" }, url: "https://www.astranova.org/" },
      ],
    },
    {
      date: { zh: "2024–今", en: "2024–present" },
      label: { zh: "Ad Astra（德州 Bastrop）", en: "Ad Astra (Bastrop, Texas)" },
      form: {
        zh: "线下学校，Musk 基金会通过 X Foundation 注资约 1 亿美元，日常由 Xplor Education 运营，紧邻 Musk 的 SpaceX/Boring/X 园区。",
        en: "An in-person campus, funded with roughly $100M through Musk's X Foundation, run day-to-day by Xplor Education, adjacent to Musk's SpaceX/Boring/X campus.",
      },
      audience: {
        zh: "3–9 岁幼儿及小学低年级，2024–25 首届，学费补贴。",
        en: "Ages 3–9, preschool through lower elementary, first cohort 2024–25, tuition subsidized.",
      },
      cites: [
        { label: { zh: "adastraschool.org（2026-07-05 抓取）", en: "adastraschool.org (scraped 2026-07-05)" }, url: "https://www.adastraschool.org/" },
      ],
    },
  ],
  extra: {
    text: {
      zh: "另外两个衍生项目：Synthesis（Ad Astra 的团队模拟游戏课商业化，Josh Dahn + Chrisman Frank 于 2020 年创办）和 Conundrums（伦理思辨视频，2021 年与 ClassDojo 合作推向大众）。SpaceX 还在 Boca Chica 星港附近另行申请了一所约 2000 万美元的学校（独立项目，信息很少）。",
      en: "Two further spin-offs exist: Synthesis (the commercialized version of Ad Astra's team-simulation games, founded 2020 by Josh Dahn and Chrisman Frank) and Conundrums (ethics-reasoning videos, brought to the public in a 2021 partnership with ClassDojo). SpaceX has also separately filed for a roughly $20M school near its Boca Chica launch site (a distinct project, with very little public information).",
    },
    cites: [
      { label: { zh: "The Hustle", en: "The Hustle" }, url: "https://thehustle.co/meet-synthesis-the-edtech-startup-scaling-elon-musks-ad-astra-school" },
      { label: { zh: "synthesis.com/tutor", en: "synthesis.com/tutor" }, url: "https://www.synthesis.com/tutor" },
    ],
  },
};

/* ════════════════════════════════════════════════════════════════════════
   1 · Admissions — official vs. actual, per entity
   ════════════════════════════════════════════════════════════════════════ */

export interface AdmissionsBlock {
  title: Bilingual;
  official: Bullet[];
  actual: Bullet[];
  warn?: { title: Bilingual; body: Bilingual; cites: Cite[] };
}

export const admissions: {
  intro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual };
  blocks: AdmissionsBlock[];
} = {
  intro: {
    eyebrow: { zh: "谁能进去", en: "Who gets in" },
    title: { zh: "官方说法 vs. 实际操作", en: "What they say vs. what they actually do" },
    body: {
      zh: "三个实体各自的招生标准都写了官方版本；把官方措辞和实际执行摆在一起看，落差才看得清楚。",
      en: "All three entities publish an official version of their admissions standards. Placing that language next to what actually happens is where the gap becomes visible.",
    },
  },
  blocks: [
    {
      title: { zh: "1.1 · SpaceX 时期 Ad Astra（2014–2020）", en: "1.1 · SpaceX-era Ad Astra (2014–2020)" },
      official: [
        {
          text: {
            zh: "创始动机：Musk 把 5 个儿子从洛杉矶天才学校 Mirman School 撤出，挖来教师 Josh Dahn 办学。第一年 8 个孩子，Musk 的孩子占近三分之二。",
            en: "Founding motive: Musk pulled his five sons out of the Mirman School, a gifted school in Los Angeles, and hired away teacher Josh Dahn to start the school. The first year had 8 children, nearly two-thirds of them Musk's own.",
          },
          cites: [
            { label: { zh: "Quartz, 2018-11-30", en: "Quartz, 2018-11-30" }, url: "https://qz.com/1480109/the-three-questions-on-the-application-for-elon-musks-private-school" },
            { label: { zh: "Ars Technica, 2018-06-25", en: "Ars Technica, 2018-06-25" }, url: "https://arstechnica.com/science/2018/06/first-space-then-auto-now-elon-musk-quietly-tinkers-with-education/" },
          ],
        },
        {
          text: {
            zh: "不设年级：Musk 反对「让所有孩子像流水线一样同步升年级」，主张按天资和能力匹配教育。",
            en: "No grade levels: Musk objected to \"moving all children through grades on the same assembly line,\" arguing education should match a child's gifts and ability instead.",
          },
          cites: [{ label: { zh: "Business Insider, 2015-05-22", en: "Business Insider, 2015-05-22" }, url: "https://www.businessinsider.com/elon-musk-creates-a-grade-school-2015-5" }],
        },
        {
          text: {
            zh: "Astra Nova 官网今天的回顾性说法：「孩子确实聪明，但录取标准只有善良（kindness）、学习热情（eagerness to learn）、以及父母在 SpaceX 工作。」",
            en: "Astra Nova's site today, looking back: \"The kids really were bright, but the admissions bar was really just kindness, an eagerness to learn, and having a parent who worked at SpaceX.\"",
          },
          cites: [{ label: { zh: "astranova.org 创始人信, 2026-03-01", en: "astranova.org founders' letter, 2026-03-01" }, url: "https://www.astranova.org/" }],
        },
      ],
      actual: [
        {
          text: {
            zh: "2017 招生季：约 400 个家庭竞争约 12 个名额（录取率约 3%），申请者需通过儿童心理学家开发的推理测试。",
            en: "2017 admissions cycle: roughly 400 families competed for about 12 seats (an acceptance rate near 3%), and applicants had to pass a reasoning test developed by a child psychologist.",
          },
          cites: [
            { label: { zh: "Ars Technica, 2018-06-25", en: "Ars Technica, 2018-06-25" }, url: "https://arstechnica.com/science/2018/06/first-space-then-auto-now-elon-musk-quietly-tinkers-with-education/" },
            { label: { zh: "Washington Post, 2018-06-27", en: "Washington Post, 2018-06-27" }, url: "https://www.washingtonpost.com/technology/2018/06/27/elon-musk-created-secretive-laboratory-school-brilliant-kids-who-love-flamethrowers/" },
          ],
        },
        {
          text: {
            zh: "2018–19 公开申请（面向 8–13 岁、洛杉矶优先）的申请材料：家长信息表（会问是否在 SpaceX/Tesla/Boring Co. 工作）+ 可选家长陈述 + 一个体现「投入、野心与原创性」的个人项目 + 三选一的「synthesis」思辨题——Goldilocks（为人类殖民给 11 颗虚构行星排序）、The Eleventh Painting（给一幅失落名画的 5 位买家排序）、The Lake（在 6 个责任方——含一个「幕后操纵者」——之间分配环境灾难的责任）。",
            en: "The 2018–19 open application (ages 8–13, Los Angeles families given priority) asked for: a parent information form (which asked whether the family worked at SpaceX/Tesla/Boring Co.), an optional parent statement, a personal project demonstrating \"commitment, ambition, and originality,\" and one of three \"synthesis\" reasoning prompts: Goldilocks (rank 11 fictional planets for human colonization), The Eleventh Painting (rank 5 buyers for a lost masterpiece), or The Lake (assign responsibility for an environmental disaster among 6 parties, including a \"mastermind behind the scenes\").",
          },
          cites: [{ label: { zh: "Quartz, 2018-11-30", en: "Quartz, 2018-11-30" }, url: "https://qz.com/1480109/the-three-questions-on-the-application-for-elon-musks-private-school" }],
        },
        {
          text: {
            zh: "学生画像：约一半是 SpaceX 员工子女，其余是洛杉矶「高成就」孩子；混龄 7–14 岁；Dahn 的原话——「我们不断吸收能跟上大孩子的、最早慧的小孩子」。IRS 申报文件写明「由于极高的师生配比，学校可能永远不会超过 50 人」。全部免学费，由 Musk 个人出资（2014、2015 年各 47.5 万美元起，后增至每年逾百万美元）。",
            en: "Student profile: about half were SpaceX employees' children, the rest \"high-achieving\" Los Angeles kids, mixed ages 7–14. Dahn's own words: \"we kept pulling in the youngest kids who were precocious enough to keep up with the older ones.\" The IRS filing states the school \"may never exceed 50 students, given the extremely high staff-to-student ratio.\" Fully tuition-free, personally funded by Musk (starting at $475,000 each in 2014 and 2015, rising past $1M/year later).",
          },
          cites: [
            { label: { zh: "Ars Technica, 2018-06-25", en: "Ars Technica, 2018-06-25" }, url: "https://arstechnica.com/science/2018/06/first-space-then-auto-now-elon-musk-quietly-tinkers-with-education/" },
            { label: { zh: "ProPublica Form 990", en: "ProPublica Form 990" }, url: "https://projects.propublica.org/nonprofits/organizations/471480453" },
          ],
        },
      ],
    },
    {
      title: { zh: "1.2 · Astra Nova（现行，在线，11–18 岁）", en: "1.2 · Astra Nova (current, online, ages 11–18)" },
      official: [
        {
          text: {
            zh: "招生流程：① Conundrum 视频作答——学生从 NASA、Dragon、Ferry、Bird、Photo、Moonshot 等思辨题视频中选一个，上传 30 秒–2 分钟的视频（首选）或音频，展示推理过程，不要成绩单、不要标化考试、不收申请费（官网原话：收申请费「很蠢」）。② 家长信——一页以内，说明家庭在寻找什么样的学校。③ 提交表单 → 约一半申请者进入下一轮（小组面试）→ 终审。全年可申请，每年评审两次；当前截止日 2026-10-15（2027 年 1 月入学），11-01 小组面试，12-15 放榜。",
            en: "Admissions process: ① a Conundrum video response — students pick one reasoning prompt from a set that includes NASA, Dragon, Ferry, Bird, Photo, and Moonshot, and submit a 30-second-to-2-minute video (preferred) or audio recording of their reasoning; no transcript, no standardized test, no application fee (the site's own words: charging one \"is dumb\"). ② A parent letter, under one page, on what kind of school the family is looking for. ③ A submission form — about half of applicants advance to the next round (a group interview), then a final decision. Applications are open year-round with two review cycles a year; the current deadline is 2026-10-15 (for January 2027 enrollment), with group interviews 11-01 and decisions 12-15.",
          },
        },
        {
          text: {
            zh: "官方表述：「努力且热爱学习的学生」（students who work hard and love to learn）、「善良、好奇、敢闯的孩子」（kind, curious, and daring kids）。培养目标很明确：「让学生准备好为攻克世界最难问题的团队贡献价值」。",
            en: "The official phrasing: \"students who work hard and love to learn,\" \"kind, curious, and daring kids.\" The stated goal is explicit: \"preparing students to add value to teams tackling the world's hardest problems.\"",
          },
        },
      ],
      actual: [
        {
          text: {
            zh: "实际画像：315 名初中生来自 45 个国家，多为全球范围内的资优生/在家上学者；很多毕业生进入美国最挑剔的寄宿和走读中学。2020 年创校报道称目标学生为「善良、有动力、学业认真」的 8–14 岁孩子，当时的申请挑战包括污染归责伦理视频、虚拟画廊策展、挑选火星宇航员、策略游戏等。",
            en: "Actual profile: 315 middle-school students from 45 countries, largely globally sourced gifted kids and homeschoolers; many graduates go on to some of the most selective boarding and day schools in the US. A 2020 launch report described the target student as \"kind, motivated, academically serious\" ages 8–14, with application challenges at the time including a pollution-attribution ethics video, curating a virtual gallery, choosing a Mars astronaut, and strategy games.",
          },
          cites: [
            { label: { zh: "Daily Beast, 2020-07-05", en: "Daily Beast, 2020-07-05" }, url: "https://www.thedailybeast.com/would-you-pay-dollar7500-to-educate-your-kid-like-elon-musks" },
            { label: { zh: "NY Magazine \"SpaceX Cadets\", 2022-08", en: "NY Magazine \"SpaceX Cadets\", 2022-08" }, url: "https://nymag.com/intelligencer/2022/08/elon-musk-astra-nova-school.html" },
          ],
        },
        {
          text: {
            zh: "学费按每周课时分档（2026/27，初高中相同）：2 小时/周 $4,800；3–4 小时 $9,600；5–7 小时 $14,400；8–11 小时 $24,000；12–15 小时 $31,200；16+ 小时 $36,000/年。承诺满足 100% 经证实的经济需求（经 Clarity 系统核定，家庭年收入 ≤$75,000 免申请费）。",
            en: "Tuition is tiered by weekly hours (2026/27, same for middle and high school): $4,800/yr at 2 hrs/week; $9,600 at 3–4 hrs; $14,400 at 5–7 hrs; $24,000 at 8–11 hrs; $31,200 at 12–15 hrs; $36,000 at 16+ hrs. The school commits to meeting 100% of demonstrated financial need (verified through its Clarity system; families earning ≤$75,000/year have the application fee waived).",
          },
        },
      ],
    },
    {
      title: { zh: "1.3 · Ad Astra Bastrop（现行，线下，3–9 岁）", en: "1.3 · Ad Astra Bastrop (current, in-person, ages 3–9)" },
      official: [
        {
          text: {
            zh: "「向所有 3–9 岁儿童开放」，「基于优秀程度（merit）录取，不分种族肤色国籍」；没有公布任何笔试/面试/作品要求。硬性规则：9 月 1 日前满 6 岁才能升小学部；幼儿部需自主如厕；必须披露医疗/特殊需求，疑似残障儿童需接受「个体化评估」判断是否适配。",
            en: "\"Open to all children ages 3–9,\" admitted \"on merit, regardless of race, color, or national origin\"; no published test, interview, or portfolio requirement. Hard rules: a child must turn 6 by September 1 to move into elementary; preschool requires independent toileting; medical/special needs must be disclosed, and a child with a suspected disability undergoes an \"individualized evaluation\" to determine fit.",
          },
          cites: [{ label: { zh: "adastraschool.org/admissions（2026-07-05 抓取）", en: "adastraschool.org/admissions (scraped 2026-07-05)" }, url: "https://www.adastraschool.org/admissions" }],
        },
        {
          text: {
            zh: "2024–25 学年学费全额补贴（覆盖 8:00–15:00 全年教学 + 15:00–18:00 课后班 + 点心 + 材料费）；未来学费「与本地含延时班的私校持平」（奥斯汀地区同类蒙氏学校约 $20,000/年），并考虑经济需求。",
            en: "Tuition was fully subsidized for the 2024–25 school year (covering 8:00–15:00 instruction year-round, 15:00–18:00 aftercare, snacks, and materials); future tuition is planned to \"match local private schools with extended-day care\" (comparable Austin-area Montessori schools run about $20,000/year), with financial need considered.",
          },
          cites: [
            { label: { zh: "adastraschool.org/admissions", en: "adastraschool.org/admissions" }, url: "https://www.adastraschool.org/admissions" },
            { label: { zh: "Texas Tribune, 2025-01-13", en: "Texas Tribune, 2025-01-13" }, url: "https://www.texastribune.org/2025/01/13/texas-elon-musk-school-education-bastrop-county/" },
          ],
        },
        {
          text: {
            zh: "公布容量：幼儿部 18 人 + 小学低段 30 人 = 48 人；师生比 12:1 / 15:1。X Foundation 文件显示远期规划：小学扩至 54 人、可能招远程生、「最终建成从小学到大学的完整体系」。",
            en: "Published capacity: 18 preschool + 30 lower-elementary = 48 students total, staff ratios of 12:1 and 15:1. X Foundation filings show longer-range plans: expanding the elementary program to 54 students, possibly enrolling remote students, and eventually \"building a complete system from elementary school through college.\"",
          },
          cites: [
            { label: { zh: "官网 admissions", en: "adastraschool.org/admissions" }, url: "https://www.adastraschool.org/admissions" },
            { label: { zh: "Fortune, 2024-11-20", en: "Fortune, 2024-11-20" }, url: "https://fortune.com/2024/11/20/elon-musk-ad-astra-school-permit-montessori-bastrop-texas/" },
          ],
        },
      ],
      actual: [
        {
          text: {
            zh: "未发现任何公开的「已录取学生」画像；Fortune（2024-11）称连 Musk 自己的孩子是否就读都不清楚。目标人群被普遍解读为 Bastrop 园区（SpaceX/Boring/X/Snailbrook 员工社区）的员工子女——学校距 Musk 园区仅一街之隔——但官网没有写员工优先或地域限制。",
            en: "No public profile of an admitted student has surfaced; Fortune (2024-11) reported it wasn't even clear whether Musk's own children attend. The target population is widely read as employee families from the Bastrop campus community (SpaceX/Boring/X/Snailbrook) — the school sits just one street from Musk's campus — though the official site states no employee preference or geographic restriction.",
          },
          cites: [
            { label: { zh: "Texas Tribune, 2025-01-13", en: "Texas Tribune, 2025-01-13" }, url: "https://www.texastribune.org/2025/01/13/texas-elon-musk-school-education-bastrop-county/" },
            { label: { zh: "Forbes, 2024-07-31", en: "Forbes, 2024-07-31" }, url: "https://www.forbes.com/sites/sarahemerson/2024/07/31/elon-musks-experimental-school-in-texas-is-now-looking-for-students/" },
          ],
        },
        {
          text: {
            zh: "唯一见报的申请者案例：奥斯汀家长 Zarema Saldana（住在 1 小时车程外）2024 年 8 月为两个女儿申请，后对 NYT 表示惊讶学校并未如宣传运营：「他们本来有很大的计划。」",
            en: "The only applicant case that made the press: Austin parent Zarema Saldana (a one-hour drive away) applied for her two daughters in August 2024, and later told the NYT she was surprised the school wasn't operating as advertised: \"They had big plans.\"",
          },
          cites: [{ label: { zh: "The Cool Down 引 NYT, 2025-12-19", en: "The Cool Down, citing NYT, 2025-12-19" }, url: "https://www.thecooldown.com/green-business/ad-astra-school-elon-musk-texas/" }],
        },
      ],
      warn: {
        title: { zh: "官方宣传与实际运营有明显落差", en: "A clear gap between the marketing and the actual operation" },
        body: {
          zh: "德州执照（2024-11-14 发放）只允许 21 名儿童，申请文件预计首批仅 16 人；《纽约时报》2025-12-18 报道：文件显示该校实际以「约 10 名 5 岁以下儿童的日托」形态运营，并未按宣传开办小学，因执照反复补件、一名员工缺乏园长资质等原因至少推迟了两次开学。官网自 2024 年底后基本未更新，首页「正在接受 2024-25 申请」与招生页「招生已关闭」互相矛盾。",
          en: "The Texas license (issued 2024-11-14) permits only 21 children, with filings anticipating just 16 in the first cohort. The New York Times reported on 2025-12-18 that filings show the school actually operating as \"a daycare for roughly 10 children under age 5,\" not the elementary school it advertised — delayed at least twice by repeated licensing resubmissions and a staff member lacking the required director credential. The official site has been largely unupdated since late 2024, and its homepage (\"now accepting 2024–25 applications\") contradicts its own admissions page (\"admissions closed\").",
        },
        cites: [
          { label: { zh: "NYT, 2025-12-18", en: "NYT, 2025-12-18" }, url: "https://www.nytimes.com/2025/12/18/technology/elon-musk-daycare-school.html" },
          { label: { zh: "The Cool Down, 2025-12-19", en: "The Cool Down, 2025-12-19" }, url: "https://www.thecooldown.com/green-business/ad-astra-school-elon-musk-texas/" },
          { label: { zh: "Quartz", en: "Quartz" }, url: "https://qz.com/elon-musk-school-eduation-bastrop-texas-ad-astra" },
          { label: { zh: "Fortune, 2024-11-20", en: "Fortune, 2024-11-20" }, url: "https://fortune.com/2024/11/20/elon-musk-ad-astra-school-permit-montessori-bastrop-texas/" },
        ],
      },
    },
  ],
};

/* ════════════════════════════════════════════════════════════════════════
   2 · Age-band pedagogy
   ════════════════════════════════════════════════════════════════════════ */

export interface AgeStage {
  ageRange: Bilingual;
  title: Bilingual;
  bullets: Bullet[];
}

export const ageStages: {
  intro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual };
  stages: AgeStage[];
} = {
  intro: {
    eyebrow: { zh: "怎么培养", en: "How they teach" },
    title: { zh: "按年龄段拆开看", en: "Broken down by age band" },
    body: {
      zh: "同一个「Musk 式教育」标签下面，蒙氏幼儿园、无年级实验班和企业实战高中其实是完全不同的三套方法。",
      en: "Under the same \"Musk-style education\" label sit three completely different methods: a Montessori preschool, a gradeless experimental classroom, and a corporate-embedded high school.",
    },
  },
  stages: [
    {
      ageRange: { zh: "3–6 岁", en: "Ages 3–6" },
      title: { zh: "Ad Astra Bastrop 幼儿部（Primary）", en: "Ad Astra Bastrop Primary" },
      bullets: [
        {
          text: {
            zh: "谁来培养：日常运营外包给 Xplor Education（CEO Greg Marick，同时运营夏威夷 Hala Kahiki 蒙氏学校和 SpaceX Hawthorne 的 Discovery 幼儿园）；招聘要求首选 AMI 蒙氏主教文凭或 MACTE 认证 + 本科/硕士 + 5 年以上蒙氏带班经验；已知教师包括校长 Joana Fowler 和有约 10 年美国蒙氏经验的前中国电视记者 James (Jin) Lu；教师年薪约 $54,479（Indeed）。法人层面 CEO 是 Musk 的财富管家 Jared Birchall。",
            en: "Who teaches: day-to-day operations are outsourced to Xplor Education (CEO Greg Marick, who also runs Hawaii's Hala Kahiki Montessori school and SpaceX Hawthorne's Discovery preschool). The hiring bar prefers an AMI Montessori lead-teacher diploma or MACTE accreditation, a bachelor's/master's degree, and 5+ years of Montessori classroom experience. Known staff include principal Joana Fowler and James (Jin) Lu, a former Chinese TV journalist with about a decade of US Montessori experience; average teacher pay is about $54,479/year (per Indeed). At the corporate level, the CEO is Jared Birchall, Musk's personal wealth manager.",
          },
          cites: [
            { label: { zh: "Fortune, 2024-11-20", en: "Fortune, 2024-11-20" }, url: "https://fortune.com/2024/11/20/elon-musk-ad-astra-school-permit-montessori-bastrop-texas/" },
            { label: { zh: "Texas Tribune, 2025-01-13", en: "Texas Tribune, 2025-01-13" }, url: "https://www.texastribune.org/2025/01/13/texas-elon-musk-school-education-bastrop-county/" },
            { label: { zh: "Indeed", en: "Indeed" }, url: "https://www.indeed.com/cmp/Ad-Astra-Montessori-School/salaries/Teacher" },
            { label: { zh: "Xplor 招聘帖", en: "Xplor job listing" }, url: "https://www.wayup.com/i-j-Xplor-Education-733710150076336/" },
          ],
        },
        {
          text: {
            zh: "培养什么：感官探索、生活技能（系扣子、扫地、道歉、解决冲突）、地图与地球仪、算术「数到千位」、礼仪与专注力训练；大孩子带小孩子。纪律观源自心理学家 Alfred Adler 与 Rudolf Dreikurs（培养「负责、尊重、有办法」的孩子）。日程含主题式 STEM 活动、户外玩耍和午睡。",
            en: "What's taught: sensory exploration, life skills (buttoning, sweeping, apologizing, resolving conflict), maps and globes, counting into the thousands, manners and focus-building; older children mentor younger ones. The discipline philosophy draws on psychologists Alfred Adler and Rudolf Dreikurs (raising children who are \"responsible, respectful, and resourceful\"). The daily schedule includes themed STEM activities, outdoor play, and nap time.",
          },
          cites: [
            { label: { zh: "官网 Primary", en: "adastraschool.org/primary" }, url: "https://www.adastraschool.org/primary" },
            { label: { zh: "Bloomberg, 2024-12-17", en: "Bloomberg, 2024-12-17" }, url: "https://www.bloomberg.com/news/features/2024-12-17/elon-musk-s-new-texas-preschool-highlights-his-education-priorities" },
          ],
        },
        {
          text: {
            zh: "一个有趣的矛盾：官网声明「Ad Astra 不是蒙特梭利学校，但珍视蒙氏培训教师的经验」；而 Texas Tribune 调查发现其提交州政府的课程符合 AMS 蒙氏五大核心要素（混龄班、不间断工作时段、蒙氏教具、蒙氏教师……），并有 $21,000+ 的 Nienhuis 蒙氏教具采购发票。实质上就是一所蒙氏+STEM 幼儿园。",
            en: "One interesting contradiction: the official site states \"Ad Astra is not a Montessori school, but values the experience of Montessori-trained teachers\"; a Texas Tribune investigation found the curriculum it filed with the state matches all five core AMS Montessori pillars (mixed-age classrooms, uninterrupted work periods, Montessori materials, Montessori-trained teachers...), plus a $21,000+ invoice for Nienhuis Montessori equipment. In substance, it is a Montessori-plus-STEM preschool.",
          },
          cites: [{ label: { zh: "Texas Tribune, 2025-01-13", en: "Texas Tribune, 2025-01-13" }, url: "https://www.texastribune.org/2025/01/13/texas-elon-musk-school-education-bastrop-county/" }],
        },
      ],
    },
    {
      ageRange: { zh: "6–9 岁", en: "Ages 6–9" },
      title: { zh: "Ad Astra Bastrop 小学低段（Lower Elementary）", en: "Ad Astra Bastrop Lower Elementary" },
      bullets: [
        {
          text: {
            zh: "官网描述：项目制的真实世界挑战、动手建造、批判性思维、概念化（非死记硬背）的数学与科学、创造力、全球公民意识，课程逐年螺旋上升；六大支柱：个性化学习、精心排序的活动式课程、不间断工作时段、生活实践技能、混龄「家庭式」社区、终身学习热情。设施为 Bastrop 约 40 英亩前马场上改建的约 4000 平方英尺白色农舍，有篮球场和小游乐场。",
            en: "Per the official site: project-based real-world challenges, hands-on building, critical thinking, conceptual (not rote) math and science, creativity, and global citizenship, with a spiraling curriculum year over year. Six pillars: personalized learning, a carefully sequenced activity-based curriculum, uninterrupted work periods, practical life skills, a mixed-age \"family-style\" community, and a lifelong love of learning. The campus is a roughly 4,000-square-foot renovated white farmhouse on a 40-acre former horse ranch in Bastrop, with a basketball court and a small playground.",
          },
          cites: [
            { label: { zh: "官网 Lower Elementary", en: "adastraschool.org/lowerelementary" }, url: "https://www.adastraschool.org/lowerelementary" },
            { label: { zh: "官网首页", en: "adastraschool.org homepage" }, url: "https://www.adastraschool.org/" },
            { label: { zh: "KUT, 2025-01-13", en: "KUT, 2025-01-13" }, url: "https://www.kut.org/education/2025-01-13/elon-musk-ad-astra-school-education-bastrop-austin-texas" },
          ],
        },
        {
          text: {
            zh: "注意：如上文警示框所述，NYT（2025-12）报道该小学部截至当时并未实际运营，以上是官方设计蓝图，其落地实践目前仍缺乏经证实的记录。曾发布「小学 STEM 专员（Elementary STEM Specialist）」等职位（2024 年 7 月起聘）。",
            en: "A caveat: as the warning box above notes, the NYT (2025-12) reported this elementary program was not actually operating as of that report — the description above is the official design blueprint, and its classroom practice currently lacks any verified record. The school did post an \"Elementary STEM Specialist\" job listing starting July 2024.",
          },
          cites: [{ label: { zh: "Salary.com 招聘帖, 2024", en: "Salary.com job listing, 2024" }, url: "https://www.salary.com/job/xplor-education/elementary-stem-specialist-assistant-elementary-teacher/j202405181304167135276" }],
        },
      ],
    },
    {
      ageRange: { zh: "7–14 岁 · 历史", en: "Ages 7–14 · historical" },
      title: {
        zh: "SpaceX 时期 Ad Astra 的培养法",
        en: "SpaceX-era Ad Astra's teaching method",
      },
      bullets: [
        {
          text: {
            zh: "做减法：重科学、数学、工程与伦理；没有体育、没有音乐、没有外语（Musk 认为实时机器翻译即将到来）；课程每年重写，学生自选约一半内容；可以退出不喜欢的科目；混龄、无年级、几乎无考试无评分。",
            en: "Subtraction by design: heavy on science, math, engineering, and ethics; no PE, no music, no foreign language (Musk believed real-time machine translation was imminent); the curriculum was rewritten every year, with students choosing about half of the content themselves; students could opt out of subjects they disliked; mixed ages, no grade levels, almost no tests or grading.",
          },
          cites: [{ label: { zh: "Ars Technica, 2018-06-25", en: "Ars Technica, 2018-06-25" }, url: "https://arstechnica.com/science/2018/06/first-space-then-auto-now-elon-musk-quietly-tinkers-with-education/" }],
        },
        {
          text: {
            zh: "Geneva 模块（伦理与地缘政治模拟）：如为相互竞争的 AI 团队制定监管、美-中-朝三方核谈判模拟（「一名'朝鲜代表'把世界带向核浩劫——对那个孩子是真正震撼的一刻」）。",
            en: "The Geneva module (ethics and geopolitics simulation): examples include drafting regulation for competing AI teams and a US–China–North Korea nuclear-negotiation simulation (\"a 'North Korean representative' brought the world to nuclear catastrophe — a genuinely shocking moment for that kid\").",
          },
          cites: [{ label: { zh: "Washington Post, 2018-06-27", en: "Washington Post, 2018-06-27" }, url: "https://www.washingtonpost.com/technology/2018/06/27/elon-musk-created-secretive-laboratory-school-brilliant-kids-who-love-flamethrowers/" }],
        },
        {
          text: {
            zh: "A-Frame 实验室模块：气象气球、机器人格斗、「炸东西」；学生问能否给机器人加喷火器和电磁脉冲——「答案永远是可以……直到你把学校炸了为止」（Dahn）。",
            en: "The A-Frame lab module: weather balloons, robot combat, \"blowing things up\"; students asked whether they could add flamethrowers or EMPs to their robots — \"the answer is always yes... until you blow up the school\" (Dahn).",
          },
        },
        {
          text: {
            zh: "Synthesis 模拟课：每周团队策略游戏（后来独立成公司）。校内经济系统：自有货币「Astra」，孩子们交易、卖手工饼干、卖建站服务——学市场与激励。",
            en: "Synthesis simulation classes: a weekly team strategy game (later spun off into its own company). An in-school economy: its own currency, \"Astra,\" with kids trading, selling homemade cookies, and selling web-building services — learning markets and incentives firsthand.",
          },
        },
        {
          text: {
            zh: "Folio：每周一个深度研究课题（邮轮产业、士绅化等）；Symposium：TED 式演讲，2016 在 UCLA、2017 在 USC 面对数百名成人答辩。编程：Scheme、Swift、Scratch，配合 Codecademy/edX/Khan Academy 自学。",
            en: "Folio: a weekly deep-dive research topic (the cruise industry, gentrification, and more). Symposium: TED-style talks, delivered to hundreds of adults at UCLA in 2016 and USC in 2017. Coding: Scheme, Swift, and Scratch, paired with self-study on Codecademy/edX/Khan Academy.",
          },
        },
        {
          text: {
            zh: "师资：Josh Dahn 领衔（薪酬从 2016 财年 $130,417 涨至 2020 财年 $205,267），Form 990 记录的教师还有 Daniel Lakis、Tara Safronoff、Rosemary Rohde；Musk 任无薪主席。",
            en: "Faculty: led by Josh Dahn (pay rising from $130,417 in FY2016 to $205,267 in FY2020); Form 990 also records teachers Daniel Lakis, Tara Safronoff, and Rosemary Rohde; Musk served as unpaid chairman.",
          },
          cites: [{ label: { zh: "ProPublica Form 990", en: "ProPublica Form 990" }, url: "https://projects.propublica.org/nonprofits/organizations/471480453" }],
        },
      ],
    },
    {
      ageRange: { zh: "11–14 岁 · 现行", en: "Ages 11–14 · current" },
      title: { zh: "Astra Nova 初中", en: "Astra Nova Middle School" },
      bullets: [
        {
          text: {
            zh: "课程机制：每年 3 学期，每学期全新设计课程（「maniacally creative」是校方自我要求）；全部 Zoom 直播小班（6–16 人），周一至周五七个 1 小时时段自选；学生通常每周上 4–16 小时。课程单：menu.astranova.org。数学系统连贯（代数 I→ 微积分，每周 4 小时全年，另与 Art of Problem Solving 合作）。",
            en: "Curriculum mechanics: 3 terms per year, with an entirely new curriculum designed each term (\"maniacally creative\" is the school's own bar for itself); all live over Zoom in small classes of 6–16, with seven one-hour blocks Monday through Friday that students choose from; students typically take 4–16 hours a week. Course list: menu.astranova.org. Math runs as a coherent track — Algebra I through Calculus, 4 hours/week year-round, in partnership with Art of Problem Solving.",
          },
          cites: [{ label: { zh: "menu.astranova.org", en: "menu.astranova.org" }, url: "https://menu.astranova.org" }],
        },
        {
          text: {
            zh: "三档就读方式：A 补充型（2–7 小时/周，只上课）；B（8–11 小时/周，+每年 2 次线下营：圣迭戈海洋营、亨茨维尔太空营）；C 完整型（12+ 小时/周，+全年数学）。历年线下活动还包括卡特琳娜岛、圣巴巴拉、日内瓦 CERN；每年 6 月全校在洛杉矶办沙滩派对+博物馆日。",
            en: "Three enrollment tiers: A, supplemental (2–7 hrs/week, classes only); B (8–11 hrs/week, plus two annual in-person camps — a San Diego ocean camp and a Huntsville space camp); C, full-time (12+ hrs/week, plus year-round math). Past in-person trips have included Catalina Island, Santa Barbara, and CERN in Geneva; every June the whole school holds a beach party and museum day in Los Angeles.",
          },
        },
        {
          text: {
            zh: "培养内核（创始人信原话）：三个聚焦——对学习与复杂性的态度、在团队中高效协作的能力、做出伦理决策的能力，「每一门课和每一次体验都从这些原则出发」。中学部口号是培养「能让任何团队变得更好」的学生。",
            en: "The pedagogical core, in the founders' own words: three focuses — an attitude toward learning and complexity, the ability to collaborate effectively on a team, and the ability to make ethical decisions — \"every class and every experience starts from these principles.\" The middle school's motto is raising students who \"make any team better.\"",
          },
        },
        {
          text: {
            zh: "师资：「聘请处于领域前沿的教师」；三位联合创始人/联合执行总监均为 SpaceX 时期原班教师：Josh Dahn、Dr. Rosemary Rohde、Tara Safronoff。",
            en: "Faculty: the school hires \"teachers at the frontier of their field\"; all three co-founders/co-executive-directors were original SpaceX-era teachers — Josh Dahn, Dr. Rosemary Rohde, and Tara Safronoff.",
          },
          cites: [{ label: { zh: "astranova.org（2026-07-05 抓取）", en: "astranova.org (scraped 2026-07-05)" }, url: "https://www.astranova.org/" }],
        },
      ],
    },
  ],
};

/* ════════════════════════════════════════════════════════════════════════
   3 · High school program (tiers + Corporate Collaborative + timeline + tuition)
   ════════════════════════════════════════════════════════════════════════ */

export interface HighSchoolTier {
  name: Bilingual;
  hours: Bilingual;
  desc: Bilingual;
}

export const highSchool: {
  intro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual };
  tiers: HighSchoolTier[];
  corporateCollaborative: Bullet;
  timeline: { label: Bilingual; date: Bilingual }[];
  tuition: { hours: Bilingual; price: Bilingual }[];
  notes: Bullet[];
} = {
  intro: {
    eyebrow: { zh: "14–18 岁 · 2026 年 8 月新开", en: "Ages 14–18 · opening August 2026" },
    title: { zh: "Astra Nova 高中项目详解", en: "The Astra Nova high school program, in detail" },
    body: {
      zh: "应家庭 12 年来的头号请求而建，3–4 年可毕业授证的项目，「严格学术 + 真实世界经验」。",
      en: "Built in response to families' number-one request over 12 years: a 3–4-year program leading to a diploma, combining \"rigorous academics with real-world experience.\"",
    },
  },
  tiers: [
    {
      name: { zh: "开放选修 · 轻量", en: "Open elective · light" },
      hours: { zh: "2–7 小时/周", en: "2–7 hrs/week" },
      desc: { zh: "只上课，按兴趣自选课程。", en: "Classes only, chosen by interest." },
    },
    {
      name: { zh: "开放选修 · 加强", en: "Open elective · intensive" },
      hours: { zh: "8+ 小时/周", en: "8+ hrs/week" },
      desc: {
        zh: "自选课程 + 每年 2 个为期一周的线下 Intensive 集训（「从早到晚」）。",
        en: "Chosen courses plus two annual weeklong in-person Intensives (\"dawn to dusk\").",
      },
    },
    {
      name: { zh: "毕业轨道", en: "Graduation track" },
      hours: { zh: "12+ 小时/周", en: "12+ hrs/week" },
      desc: {
        zh: "硬核技术课（生物技术、物理、工程、人工智能）+ 人文课（伟大文学、哲学、思维模型、历史）+ Intensives + Corporate Collaborative。",
        en: "Hardcore technical courses (biotech, physics, engineering, AI) plus humanities (great literature, philosophy, mental models, history), plus Intensives, plus the Corporate Collaborative.",
      },
    },
  ],
  corporateCollaborative: {
    text: {
      zh: "Corporate Collaborative——每年一周嵌入一家真实公司（位于加州 El Segundo，合作方每年不同），在攻坚型公司内部解决真实问题、理解团队动态。提供成绩单与推荐信；学费与初中同档（12+ 小时/周约 $31,200–36,000/年）。首届申请截止 2026-10-15。",
      en: "The Corporate Collaborative: one week each year embedded inside a real company (based in El Segundo, California; the partner company changes yearly), solving real problems and learning team dynamics from the inside of a company under real pressure. Transcripts and letters of recommendation are provided; tuition sits at the same tier as middle school (roughly $31,200–36,000/year at 12+ hrs/week). The first cohort's application deadline is 2026-10-15.",
    },
    cites: [{ label: { zh: "astranova.org #high-school（2026-07-05 抓取）", en: "astranova.org #high-school (scraped 2026-07-05)" }, url: "https://www.astranova.org/#high-school" }],
  },
  timeline: [
    {
      label: { zh: "首届高中开学", en: "First cohort starts" },
      date: { zh: "2026-08-26", en: "2026-08-26" },
    },
    {
      label: { zh: "你面对的截止日", en: "Your deadline" },
      date: { zh: "2026-10-15", en: "2026-10-15" },
    },
    {
      label: { zh: "小组面试", en: "Group interview" },
      date: { zh: "2026-11-01", en: "2026-11-01" },
    },
    {
      label: { zh: "放榜", en: "Decisions" },
      date: { zh: "2026-12-15", en: "2026-12-15" },
    },
  ],
  tuition: [
    { hours: { zh: "2 小时/周", en: "2 hrs/week" }, price: { zh: "$4,800/年", en: "$4,800/yr" } },
    { hours: { zh: "3–4 小时/周", en: "3–4 hrs/week" }, price: { zh: "$9,600/年", en: "$9,600/yr" } },
    { hours: { zh: "5–7 小时/周", en: "5–7 hrs/week" }, price: { zh: "$14,400/年", en: "$14,400/yr" } },
    { hours: { zh: "8–11 小时/周", en: "8–11 hrs/week" }, price: { zh: "$24,000/年", en: "$24,000/yr" } },
    { hours: { zh: "12–15 小时/周", en: "12–15 hrs/week" }, price: { zh: "$31,200/年", en: "$31,200/yr" } },
    { hours: { zh: "16+ 小时/周", en: "16+ hrs/week" }, price: { zh: "$36,000/年", en: "$36,000/yr" } },
  ],
  notes: [
    {
      text: {
        zh: "首届生的申请已于 2026-04-15 截止（6-15 放榜）；你面对的 2026-10-15 截止日对应 2027 年 1 月 4 日入学（第二学期插班进入首届），申请表：Paperform「Astra Nova Application 26/27 October」。约一半申请者能走到小组面试这一轮；放榜时同步给出经 Clarity 核定的资助方案。",
        en: "The first cohort's own application closed 2026-04-15 (decisions 06-15); the 2026-10-15 deadline you'd face corresponds to enrolling January 4, 2027 (joining the first cohort partway through, at its second term), via the Paperform \"Astra Nova Application 26/27 October.\" About half of applicants reach the group-interview round; decisions come with a Clarity-verified aid package attached.",
      },
    },
    {
      text: {
        zh: "申请材料（与初中完全相同）：① 六选一 Conundrum（NASA / Dragon / Ferry / Bird / Photo / Moonshot）录 30 秒–2 分钟视频；② 一页以内家长信；③ 基本信息表。无成绩单、无标化、无申请费。",
        en: "Application materials (identical to middle school): ① a Conundrum, one of six (NASA / Dragon / Ferry / Bird / Photo / Moonshot), recorded as a 30-second-to-2-minute video; ② a parent letter under one page; ③ a basic information form. No transcript, no standardized test, no application fee.",
      },
    },
  ],
};

/* ════════════════════════════════════════════════════════════════════════
   4 · Real admit / reject cases
   ════════════════════════════════════════════════════════════════════════ */

export interface CaseEntry {
  tag: Bilingual;
  credibility: Bilingual;
  title: Bilingual;
  bullets: Bullet[];
  quote?: Bilingual;
  cites: Cite[];
}

export const cases: {
  intro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual };
  noHsCasesNote: Bilingual;
  admitted: CaseEntry[];
  rejected: CaseEntry[];
  officialEvidence: Bullet[];
  takeaways: Bullet[];
} = {
  intro: {
    eyebrow: { zh: "真实案例", en: "Real cases" },
    title: { zh: "谁被录取了，谁被拒了", en: "Who got in, who didn't" },
    body: {
      zh: "覆盖平台：Reddit（含存档库）、YouTube、X、Quora、Niche、博客、知乎、小红书（经转载）、公众号、B站、豆瓣、台湾亲子媒体，每条案例标注可信度。",
      en: "Platforms covered: Reddit (including archives), YouTube, X, Quora, Niche, blogs, Zhihu, Xiaohongshu (via reprints), WeChat public accounts, Bilibili, Douban, and Taiwanese parenting media — every case below is tagged with a credibility level.",
    },
  },
  noHsCasesNote: {
    zh: "先说结论：高中项目本身还没有任何「被录/被拒」案例——首届 4 月刚放榜、8 月才开学，全网（含中英文平台）尚无一例高中申请者的公开分享。但初中部 6 年来用的是同一套选拔（Conundrum 视频 → 小组面试 → 家庭面谈），以下案例可直接类推。",
    en: "The conclusion up front: there is no public \"admitted/rejected\" case for the high-school program itself yet — the first cohort's decisions only came out in April, and classes don't start until August, so no high-school applicant has publicly shared their experience anywhere online, in Chinese or English. Middle school has used the identical selection process for 6 years (Conundrum video → group interview → family interview), so the cases below apply directly by analogy.",
  },
  admitted: [
    {
      tag: { zh: "录取", en: "Admitted" },
      credibility: { zh: "可信度：高", en: "Credibility: high" },
      title: {
        zh: "潘博悦 Julia（中国合肥，2023 年录取）——全网最详实的一手案例",
        en: "Julia Pan Boyue (Hefei, China, admitted 2023) — the most detailed firsthand case online",
      },
      bullets: [
        {
          text: {
            zh: "画像：女，申请时 12 岁；公立小学 → 六年级转国际学校 → 退学在家自学半年 → 2023 年 6 月底获录取。妈妈是大学老师。WSDA（世界学者辩论）积分榜双第一辩手，办过辩论社团和公益讲座。",
            en: "Profile: female, 12 at the time of application; public elementary school, transferred to an international school in 6th grade, then withdrew to homeschool for half a year before being admitted in late June 2023. Her mother is a university teacher. A double first-place debater on the WSDA (World Scholar's Debate Academy) leaderboard, who ran a debate club and public-interest talks.",
          },
        },
        {
          text: {
            zh: "Conundrum 选题：NASA「太空投资」题（7 个投资方向怎么排序）。她的答案：优先投资航天相关的人文科技教育，理由是「未来 200 年大概率不会发生重大宇宙灾难」——注意这是论证结构取胜，不是「标准答案」。",
            en: "Conundrum choice: the NASA \"space investment\" prompt (how to rank 7 investment directions). Her answer prioritized investing in space-related humanities and tech education, reasoning that \"a major cosmic disaster is unlikely in the next 200 years\" — notably, what won here was the structure of her argument, not a \"correct answer.\"",
          },
        },
        {
          text: {
            zh: "第二轮小组面试（评委含 Josh Dahn）：无领导小组讨论。题目：一张「大树旁有污染工厂」的照片——是 PS 的还是真拍的？多数孩子答 PS，老师当场追问「如果我告诉你是真的呢？」——考的是即场思辨与改口的勇气，不是猜对。",
            en: "The second-round group interview (judges included Josh Dahn): a leaderless group discussion. The prompt: a photo of \"a polluting factory next to a big tree\" — Photoshopped or real? Most kids said Photoshopped; the teacher pressed on the spot, \"what if I told you it's real?\" — testing real-time reasoning and the courage to change one's mind, not guessing correctly.",
          },
        },
        {
          text: {
            zh: "入学后：因时差读半日制（约 4 小时/天），12 门课全 A；她自己吐槽最大短板是「缺少社交」。妈妈坦言申请前「网上几乎搜不到任何中国学生被录取的分享」，一度怀疑学校是不是「噱头大于内容」。",
            en: "After enrolling: due to the time difference she studied half-day (about 4 hrs/day), with straight As across 12 subjects; her own complaint is that the biggest gap is \"lack of socializing.\" Her mother admitted that before applying, \"you could find almost no story online of a Chinese student being admitted,\" and at one point doubted whether the school was \"more hype than substance.\"",
          },
        },
      ],
      cites: [
        { label: { zh: "外滩教育专访（腾讯新闻，2024-10-02）", en: "Bund Education interview (Tencent News, 2024-10-02)" }, url: "https://news.qq.com/rain/a/20241002A0143800" },
        { label: { zh: "WSDA 专访转载（2023-10-31）", en: "WSDA interview reprint (2023-10-31)" }, url: "https://www.jingsailian.com/news/567784.html" },
      ],
    },
    {
      tag: { zh: "过程实录", en: "Process record" },
      credibility: { zh: "可信度：中", en: "Credibility: medium" },
      title: {
        zh: "加拿大华人 William 家庭（2023 申请季）——申请过程实录",
        en: "The Chinese-Canadian William family (2023 cycle) — a real application-process record",
      },
      bullets: [
        {
          text: {
            zh: "移民加拿大 20 年，小儿子吴为刚满 13 岁申请。真实细节：儿子一早录完 Conundrum 视频，时长四分多钟（超过 2 分钟上限），还把调研过程整个录了进去——父亲坦言不确定是否合规，说明真实家庭也会犯规则层面的错。",
            en: "Immigrated to Canada 20 years ago; younger son Wu Wei, just turned 13, applied. A real detail: the son recorded his Conundrum video in one take, running over four minutes (past the 2-minute cap), and even recorded his own research/prep process into it — his father admits he wasn't sure this was compliant, showing that real families make rules-level mistakes too.",
          },
        },
        {
          text: {
            zh: "该社区（心想树成/puredu.top）的《Astra Nova 手册》逐字贴出了第二轮邀请邮件、录取邮件（2024/2025 学年）和新生说明会实录（Josh Dahn 开场「欢迎来到 Astra Nova 第十一年」），证明其社区内有家庭走完全程并被录取。",
            en: "That community's (puredu.top's) \"Astra Nova Handbook\" posted verbatim the second-round interview invitation email, the admission email (2024/2025 school year), and a transcript of the new-student orientation (Josh Dahn's opening: \"welcome to Astra Nova's eleventh year\") — proving a family in their community went through the whole process and was admitted.",
          },
        },
      ],
      cites: [
        { label: { zh: "puredu.top 分享实录", en: "puredu.top shared transcript" }, url: "https://puredu.top/futuredu-newstars/" },
        { label: { zh: "申请流程页", en: "Application-process page" }, url: "https://puredu.top/astranova-app/" },
        { label: { zh: "Astra Nova 手册", en: "Astra Nova handbook" }, url: "https://puredu.top/astranova/" },
      ],
      quote: {
        zh: "⚠️ 该站同时售卖「ANT（Astra Nova Test）备考课」，有导流动机，取事实、滤广告。",
        en: "⚠️ This site also sells \"ANT (Astra Nova Test) prep courses\" and has a referral incentive — take the facts, filter the advertising.",
      },
    },
    {
      tag: { zh: "录取", en: "Admitted" },
      credibility: { zh: "可信度：中高", en: "Credibility: medium-high" },
      title: {
        zh: "湾区 145+ IQ 男孩（在读后转出）",
        en: "The Bay Area 145+ IQ boy (enrolled, then transferred out)",
      },
      bullets: [
        {
          text: {
            zh: "Reddit r/Gifted（2025-02-11）：旧金山湾区家长自述——「我儿子在 Astra Nova 读 8 年级，IQ 145+，刚拿到 Davidson Academy（内华达州著名的 profoundly gifted 实体校）录取，我们要搬去 Reno。」",
            en: "Reddit r/Gifted (2025-02-11): a San Francisco Bay Area parent's self-report — \"my son is in 8th grade at Astra Nova, IQ 145+, just got into Davidson Academy (Nevada's well-known profoundly-gifted in-person school), we're moving to Reno.\"",
          },
        },
        {
          text: {
            zh: "信息量在于转出原因：发帖时 Astra Nova 还没有高中，8 年级读完就得另找出路——这正是校方 2026 年开高中要补的洞，也说明其现有生源画像（高智商测评、科技湾区、把 AN 当「资优过渡方案」的家庭）。",
            en: "The informative part is the reason for transferring out: at the time of posting, Astra Nova had no high school yet, so finishing 8th grade meant finding somewhere else — precisely the gap the 2026 high school launch is meant to fill; it also reflects the school's current student profile (high-IQ-tested, Bay Area tech families, using AN as a \"gifted bridge program\").",
          },
        },
      ],
      cites: [{ label: { zh: "r/Gifted 原帖", en: "r/Gifted original post" }, url: "https://reddit.com/r/Gifted/comments/1ims63x/" }],
    },
    {
      tag: { zh: "录取", en: "Admitted" },
      credibility: { zh: "可信度：中", en: "Credibility: medium" },
      title: { zh: "在读学生自述", en: "A current student's self-account" },
      bullets: [
        {
          text: {
            zh: "Reddit r/EnoughMuskSpam（2022-01，一个反 Musk 板块里的正面证言，反而更可信）：学生自述——「我在 Astra Nova 读了一学期，这是我上过最好的学校……课程选择极多，从海洋生物学到史前掠食者、产品与游戏设计……ANOVA 真的改变了我的人生。」",
            en: "Reddit r/EnoughMuskSpam (2022-01, a positive testimonial inside an anti-Musk subreddit — arguably more credible for it): a student's self-report — \"I did a semester at Astra Nova, best school I've attended... huge range of course choices, from marine biology to prehistoric predators, product and game design... ANOVA really changed my life.\"",
          },
        },
        {
          text: {
            zh: "另有具名在读生：华裔男孩 Winston Lin，家长在 r/composer（2021-02）发布其钢琴作曲作品并注明就读 Astra Nova——侧面反映生源特质（有严肃课外造诣的孩子）。",
            en: "Also a named enrolled student: Winston Lin, a Chinese-American boy whose parent posted his piano composition on r/composer (2021-02), noting he attends Astra Nova — indirect evidence of the student body's traits (kids with serious extracurricular depth).",
          },
        },
      ],
      cites: [
        { label: { zh: "学生自述", en: "Student self-account" }, url: "https://reddit.com/r/EnoughMuskSpam/comments/ngyuua/elon_musk_opened_a_school_back_in_2020_and_its/hsnpmse/" },
        { label: { zh: "Winston Lin", en: "Winston Lin" }, url: "https://reddit.com/r/composer/comments/lbgbfc/" },
      ],
    },
  ],
  rejected: [
    {
      tag: { zh: "被拒", en: "Rejected" },
      credibility: { zh: "可信度：中高", en: "Credibility: medium-high" },
      title: {
        zh: "唯一公开的被拒叙述（Ad Astra 时期）",
        en: "The only public rejection account (Ad Astra era)",
      },
      bullets: [
        {
          text: {
            zh: "两个信息点：① 走到了家庭面试环节仍被拒——面试≠稳录；② 最后一句是对这类学校最清醒的评价：生源筛选本身（家长画像）可能比教学法更能解释「出路好」。",
            en: "Two takeaways: ① reaching the family-interview stage still didn't guarantee admission; ② the last line is the clearest-eyed take on this entire category of school — parent-body selection itself may explain \"good outcomes\" better than the teaching method does.",
          },
        },
      ],
      quote: {
        zh: "「我们为孩子面试过 Ad Astra（SpaceX 园区那所）。体验很棒……Josh Dahn 是了不起的教育者和思考者。我们被拒了，谢天谢地——第二年他们把学费提到 5 万美元/年，事后看那条路对我们并不合适。话说回来，它没有任何成果数据，也许其实是一团糟。不过大概率不是——当年和我们一起研究这所学校的家长群体，本身就保证了这些孩子会有好出路。」",
        en: "\"We interviewed for Ad Astra (the one at the SpaceX campus). The experience was great... Josh Dahn is a remarkable educator and thinker. We got rejected, thank god — the next year they raised tuition to $50k/year, and in hindsight that path wasn't right for us. That said, it has no outcome data, maybe it really is a mess. But probably not — the community of parents researching this school alongside us was, on its own, enough to guarantee those kids good outcomes.\"",
      },
      cites: [{ label: { zh: "Reddit 原评论 (u/siberian, 2024-08)", en: "Reddit original comment (u/siberian, 2024-08)" }, url: "https://reddit.com/r/conservativeterrorism/comments/1ejvknb/conspiracy_theorist_elon_musk_about_to_open/lggvmjx/" }],
    },
    {
      tag: { zh: "未申请", en: "Never applied" },
      credibility: { zh: "望而却步型", en: "The \"gave up before applying\" type" },
      title: {
        zh: "湾区工程师爸爸的「DIY 替代方案」",
        en: "A Bay Area engineer dad's DIY alternative",
      },
      bullets: [
        {
          text: {
            zh: "Andy Jagoe（湾区科技从业者，博客 2021-04）：四年级儿子 CogAT 测到资优区间，认真研究申请后发现当季「只剩候补名额，听说连 SpaceX 员工都很难进（注：此句为传闻）」，最终没有申请——转而用 Astra Nova 开源的 Conundrums 和 3DM 课程在家「自建 Astra Nova」，并给儿子报了 Synthesis。",
            en: "Andy Jagoe (a Bay Area tech professional, blog post 2021-04): his 4th-grade son tested into the gifted range on the CogAT. After seriously researching an application, he found that season had \"only wait-list spots left, and heard even SpaceX employees' kids have a hard time getting in (note: this line is hearsay)\" — he ultimately didn't apply, and instead used Astra Nova's open-sourced Conundrums and 3DM materials to \"build his own Astra Nova\" at home, enrolling his son in Synthesis as well.",
          },
        },
        {
          text: {
            zh: "参考价值：给出了「申请不上/等不起」家庭的替代路径清单，也是理性家长研究这所学校的完整思考过程。",
            en: "Reference value: this lays out a full alternative-path checklist for families who \"can't get in / can't wait,\" and models a rational parent's complete research process.",
          },
        },
      ],
      cites: [{ label: { zh: "Andy Jagoe 博客, 2021-04-04", en: "Andy Jagoe blog, 2021-04-04" }, url: "https://andyjagoe.com/send-your-child-to-school-like-elon-musks/" }],
    },
    {
      tag: { zh: "过程材料", en: "Process material" },
      credibility: { zh: "公开样本", en: "Public samples" },
      title: {
        zh: "申请材料本身的公开样本（Conundrum 视频）",
        en: "Public samples of the application materials themselves (Conundrum videos)",
      },
      bullets: [
        {
          text: {
            zh: "YouTube 上存在孩子们实际提交/练习的 Conundrum 作答视频，可当范本研究：Narthana（@Narthana18，2020-10 起每周一条）「The Martian Conundrum Response」（1.97 万次播放）等系列；Leonid Vishnevskiy（俄语背景家庭，2021-02）「Response to The Lake Conundrum, Part I」；Reddit r/AskReddit（2020-05）一位申请者公开自己对招生题「The Lake」的排序推理（科学家 > 幕后操纵者 > 政客 > 排污者 > 媒体 > 选民）并求讨论。",
            en: "YouTube hosts real submitted or practice Conundrum-response videos worth studying as models: Narthana (@Narthana18, weekly since 2020-10), whose \"The Martian Conundrum Response\" (19,700 views) and other entries cover Martian, Chocolate, and Pizza; Leonid Vishnevskiy (a Russian-speaking family, 2021-02), \"Response to The Lake Conundrum, Part I\"; and a Reddit r/AskReddit (2020-05) applicant who publicly shared their ranking-and-reasoning for the admissions prompt \"The Lake\" (scientist > mastermind > politician > polluter > media > voters) and asked for discussion.",
          },
          cites: [
            { label: { zh: "Narthana · Martian", en: "Narthana · Martian" }, url: "https://youtu.be/iaCdzPTIKaM" },
            { label: { zh: "Narthana · Chocolate", en: "Narthana · Chocolate" }, url: "https://youtu.be/b9mq1QocaXM" },
            { label: { zh: "Narthana · Pizza", en: "Narthana · Pizza" }, url: "https://youtu.be/h3-1djQfK0I" },
            { label: { zh: "Leonid · The Lake, Part I", en: "Leonid · The Lake, Part I" }, url: "https://youtu.be/Mm_WO84swDk" },
            { label: { zh: "r/AskReddit 原帖 (2020-05)", en: "r/AskReddit original post (2020-05)" }, url: "https://reddit.com/r/AskReddit/comments/gqo600/how_would_you_guys_rank_the_the_order_of_blame/frtv8l4/" },
          ],
        },
        {
          text: {
            zh: "小红书用户 @张丫舞爪 发过第二轮群面亲历帖（「像求职群面，考当场应变、思辨和表达」）——小红书内容无法被外部索引，此帖经「阅读第一」文章截图证实存在；平台上还有家长晒 offer 的帖子。",
            en: "Xiaohongshu (RedNote) user @张丫舞爪 posted a firsthand account of the second-round group interview (\"like a job-hunting group interview — tests real-time reactions, reasoning, and expression\"); since Xiaohongshu content isn't externally indexed, this post is confirmed to exist only via a screenshot quoted in a \"阅读第一\" (Read First) article; the platform also has parents posting admission-offer screenshots.",
          },
          cites: [{ label: { zh: "阅读第一（腾讯新闻，2024-08-17）", en: "\"Read First\" (Tencent News, 2024-08-17)" }, url: "https://news.qq.com/rain/a/20240817A00V7J00" }],
        },
      ],
      cites: [],
    },
  ],
  officialEvidence: [
    {
      text: {
        zh: "NY Magazine《SpaceX Cadets》（2022-08，记者进课堂）：当时约 50 全日制 + 125 兼读学生，约一半是 homeschool 家庭；流程 = Conundrum 作答 → 「demo day」（试课/群面）→ 家庭面谈；Josh Dahn 原话：「我们不在乎 IQ。Elon 不会来教你的孩子。」43% 全日制学生拿资助。",
        en: "NY Magazine's \"SpaceX Cadets\" (2022-08, a reporter sat in class): at the time, about 50 full-time plus 125 part-time students, roughly half homeschool families; the process ran Conundrum response → \"demo day\" (trial class/group interview) → family interview. Josh Dahn's exact words: \"We don't care about IQ. Elon is not going to come teach your kid.\" 43% of full-time students received financial aid.",
      },
      cites: [{ label: { zh: "NY Mag", en: "NY Magazine" }, url: "https://nymag.com/intelligencer/2022/08/elon-musk-astra-nova-school.html" }],
    },
    {
      text: {
        zh: "City Journal（2023-05）：同一流程描述；当年学费 $33,500，33% 全日制家庭获资助。",
        en: "City Journal (2023-05): describes the same process; that year's tuition was $33,500, with 33% of full-time families receiving aid.",
      },
      cites: [{ label: { zh: "City Journal", en: "City Journal" }, url: "https://www.city-journal.org/article/the-next-frontier-in-stem-education" }],
    },
    {
      text: {
        zh: "官方口径（经心想树成手册转载的校方材料）：「我们目前的许多学生在被录取之前都重新申请过多次」——被拒后再申请是常态且不减分。",
        en: "The official line (via school materials reprinted in the puredu.top handbook): \"many of our current students reapplied multiple times before being admitted\" — reapplying after a rejection is normal and doesn't count against you.",
      },
      cites: [{ label: { zh: "puredu.top 手册", en: "puredu.top handbook" }, url: "https://puredu.top/astranova/" }],
    },
    {
      text: {
        zh: "Niche 档案页：13 名学生（美国口径）、6–9 年级、滚动招生、$0 申请费、「需要面试：是」、无任何测试要求、暂无家长评价。",
        en: "Niche's profile page: 13 students (US figure), grades 6–9, rolling admissions, $0 application fee, \"interview required: yes,\" no test requirement, no parent reviews yet.",
      },
      cites: [{ label: { zh: "Niche", en: "Niche" }, url: "https://www.niche.com/k12/astra-nova-school-ca/" }],
    },
  ],
  takeaways: [
    {
      text: {
        zh: "录取者共性（Julia、湾区男孩、Winston、NY Mag 课堂群像）：有一个「玩真的」的课外纵深（辩论双冠、作曲、机器人……），校方看重的正是这份真实投入；面对开放问题敢下判断、能给结构化理由、被追问时敢修正；家庭本身认同非传统路径（半数 homeschool）；英语口头表达能撑住全英文小组讨论（对中国孩子这是隐形门槛，Julia 的辩论背景正好命中）。",
        en: "Common traits among admitted students (Julia, the Bay Area boy, Winston, the NY Mag classroom snapshot): a genuinely deep extracurricular pursuit that's \"the real thing\" (double debate champion, composition, robotics...), and what the school weighs is exactly that authentic commitment; the willingness to take a position on an open question, give structured reasoning, and revise when pressed; a family that itself embraces the non-traditional path (about half are homeschoolers); and spoken English strong enough to hold up in an all-English group discussion — an invisible barrier for Chinese kids, one Julia's debate background hit directly.",
      },
    },
    {
      text: {
        zh: "Conundrum 视频（六选一）：没有标准答案，评的是推理过程。结构建议：立场 → 两三条理由（有取舍权衡）→ 主动承认弱点/例外。严格控制在 30 秒–2 分钟内（案例 B 的四分钟视频就是反面教材）；可先用官方 Conundrums 频道旧题每周练一条（Narthana 就是这么做的）。",
        en: "The Conundrum video (one of six): there's no standard answer — what's graded is the reasoning process. A suggested structure: take a position, give two or three reasons with real trade-offs, then proactively acknowledge a weakness or exception. Keep it strictly to 30 seconds to 2 minutes (the William family's four-minute video is the cautionary counter-example); practice weekly with older prompts from the official Conundrums YouTube channel, the way Narthana did.",
      },
      cites: [{ label: { zh: "官方 Conundrums 频道", en: "Official Conundrums channel" }, url: "https://www.youtube.com/c/astranovaschool" }],
    },
    {
      text: {
        zh: "家长信：一页以内、像邮件——写清家庭为什么选非传统学校、孩子是什么样的学习者。校方明确反感包装。11-01 小组面试：无领导讨论 + 教师当场追问反转（「如果我告诉你照片是真的呢？」）。练的是倾听他人、接住反转、当众改观点的能力。",
        en: "The parent letter: under one page, written like an email — explaining why the family is choosing a non-traditional school and what kind of learner the child is. The school explicitly dislikes packaging. The 11-01 group interview: a leaderless discussion plus teachers deliberately reversing the premise mid-question (\"what if I told you the photo is real?\"). It trains the ability to listen to others, absorb a reversal, and change one's stated view in front of the group.",
      },
    },
    {
      text: {
        zh: "被拒不是终点：校方明说很多在读生是多次复申进来的；每年两个评审窗口。选档策略：若目标是完整高中文凭，选 12+ 小时毕业轨道（$31,200–36,000/年）；若只是试水，可 2–7 小时低成本进入（$4,800–14,400/年），之后再升档。经济压力大就走 Clarity（承诺满足 100% 核定需求）。时差现实（对中国家庭）：课程按美西时段排（PST 七个 1 小时时段）；Julia 因时差只能半日制——毕业轨道要求 12+ 小时/周，先核对可选时段再定档。",
        en: "Rejection isn't the end: the school states outright that many current students reapplied multiple times, and there are two review windows a year. Tier strategy: if the goal is a full diploma, choose the 12+ hr/week graduation track ($31,200–36,000/yr); if just testing the waters, start at 2–7 hrs/week ($4,800–14,400/yr) and upgrade later. Under financial pressure, go through Clarity, which commits to meeting 100% of verified need. The time-zone reality for Chinese families: classes run on Pacific time (seven one-hour blocks); Julia went half-day because of the time difference — the graduation track needs 12+ hrs/week, so check available time slots before committing to a tier.",
      },
    },
  ],
};

/* ════════════════════════════════════════════════════════════════════════
   5 · Mythbusting
   ════════════════════════════════════════════════════════════════════════ */

export interface MythEntry {
  claim: Bilingual;
  reality: Bilingual;
  cites: Cite[];
}

export const mythbusting: {
  intro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual };
  entries: MythEntry[];
  blankPlatforms: Bilingual;
  scarcityNote: Bilingual;
} = {
  intro: {
    eyebrow: { zh: "打假区", en: "Setting the record straight" },
    title: { zh: "网上流传的夸大与伪造", en: "The exaggerations and fabrications circulating online" },
    body: {
      zh: "研究这类学校时，州执照文件、Form 990、FOIA 报道往往比官网更接近真相。",
      en: "When researching this kind of school, state license filings, Form 990s, and FOIA-obtained reporting tend to get closer to the truth than the official website does.",
    },
  },
  entries: [
    {
      claim: { zh: "伪造证言", en: "Fabricated testimonials" },
      reality: {
        zh: "网上流传的「Rachel，10 岁孩子家长」「Jordan，12 岁」等「家长评价」出自 papersowl.com（论文代写站内容农场）并被 go2tutors 等站转抄，无任何来源，按虚构处理。",
        en: "\"Parent reviews\" circulating online, like \"Rachel, parent of a 10-year-old\" or \"Jordan, 12,\" originate from papersowl.com (an essay-mill content farm) and get re-copied by sites like go2tutors. They have no traceable source and should be treated as fabricated.",
      },
      cites: [],
    },
    {
      claim: { zh: "中文机构软文泛滥", en: "Chinese-language marketing filler" },
      reality: {
        zh: "StudyIsland、新东方顾问博客、亨瑞、Dealmoon 及大量知乎机构专栏均为导流内容，反复引用「2017 录取率 3% 低于哈佛」（那是 SpaceX 时期线下校的数字，与现在的在线校无关），并普遍夸大 Musk 与学校的现有关系（Musk 无财务权益、孩子已不在校）。「阅读第一」的批判文是中文圈最清醒的参照。",
        en: "StudyIsland, New Oriental consultant blogs, Hengrui, Dealmoon, and many Zhihu institutional columns are largely referral content. They repeatedly cite \"a 2017 acceptance rate of 3%, lower than Harvard\" — a figure from the SpaceX-era in-person school, unrelated to today's online school — and routinely overstate Musk's current relationship to the school (Musk has no financial stake, and his children are no longer enrolled). \"Read First\"'s critique piece is the clearest-eyed Chinese-language source on this.",
      },
      cites: [{ label: { zh: "阅读第一批判文（2024-08-17）", en: "\"Read First\" critique (2024-08-17)" }, url: "https://news.qq.com/rain/a/20240817A00V7J00" }],
    },
    {
      claim: {
        zh: "「48 人创新校区」的宣传",
        en: "The \"48-student innovative campus\" marketing",
      },
      reality: {
        zh: "Bastrop Ad Astra 宣传 48 人的创新学校，实际（截至 2025 年底）是约 10 人的日托。调研此类学校时，州执照文件、Form 990、FOIA 报道往往比官网更接近真相。",
        en: "Bastrop Ad Astra advertised a 48-student innovative school; by late 2025, per reporting, it was actually about a 10-child daycare. When researching this kind of school, state license filings, Form 990s, and FOIA-obtained reporting tend to get closer to the truth than the official website does.",
      },
      cites: [
        { label: { zh: "NYT, 2025-12-18", en: "NYT, 2025-12-18" }, url: "https://www.nytimes.com/2025/12/18/technology/elon-musk-daycare-school.html" },
        { label: { zh: "X Foundation Form 990 (2023)", en: "X Foundation Form 990 (2023)" }, url: "https://www.documentcloud.org/documents/25484756-2023-990-x-foundation/" },
        { label: { zh: "KVUE", en: "KVUE" }, url: "https://www.kvue.com/article/news/local/elon-musks-ad-astra-montessori-school-permit-to-open-bastrop-county/269-22f51286-34cc-4349-9355-653f96910f65" },
      ],
    },
  ],
  blankPlatforms: {
    zh: "确认为空白的平台：X/Twitter（无索引案例）、Quora（帖子存在但无就读者回答）、Trustpilot/GreatSchools/PrivateSchoolReview（无评价）、Medium/Substack（无经历文）、豆瓣（零结果）、Davidson 资优论坛/Well-Trained Mind/Mumsnet/College Confidential（零讨论）、B站（仅 Dahn 专访，无申请 vlog）。Reddit 2025 下半年后的内容因抓取限制未能覆盖。",
    en: "Platforms confirmed to be blank: X/Twitter (no indexed cases), Quora (threads exist but no answers from actual attendees), Trustpilot/GreatSchools/PrivateSchoolReview (no reviews), Medium/Substack (no firsthand accounts), Douban (zero results), the Davidson gifted forum/Well-Trained Mind/Mumsnet/College Confidential (zero discussion), and Bilibili (only Dahn interview clips, no application vlogs). Reddit content from the second half of 2025 onward wasn't covered, due to archive-scraping limits.",
  },
  scarcityNote: {
    zh: "为何案例这么少：全球仅 315 名初中生、历年录取几十人/届，样本本来就小；且约半数为低调的 homeschool 家庭。全网（中英文）没有一例第一人称的线上校被拒帖——所以「被拒画像」只能从「约 50% 过首轮 + 面试后仍会拒（案例 E）+ 常见复申」三点反推。",
    en: "Why there are so few cases at all: globally there are only 315 middle-school students, and historically only a few dozen are admitted per cohort — the sample is inherently tiny, and roughly half are low-profile homeschool families. There is not one first-person rejection post about the online school anywhere on the web, in Chinese or English — so the \"what gets rejected\" picture can only be inferred from three data points: about 50% pass the first round, interviews can still end in rejection, and reapplying is common.",
  },
};

/* ════════════════════════════════════════════════════════════════════════
   Full source list — every citation above, deduplicated, grouped by category
   ════════════════════════════════════════════════════════════════════════ */

export const sourcesIntro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual } = {
  eyebrow: { zh: "来源", en: "Sources" },
  title: { zh: "完整来源清单", en: "The complete source list" },
  body: {
    zh: "这篇长文的每一条事实都来自下面某一个链接。我们把它们摊在这里，方便你自己核对。",
    en: "Every fact in this long-read traces back to one of the links below. We're laying them all out here so you can check them yourself.",
  },
};

export const sourceGroups: { heading: Bilingual; items: Cite[] }[] = [
  {
    heading: { zh: "官网（2026-07-05 抓取）", en: "Official sites (scraped 2026-07-05)" },
    items: [
      { label: { zh: "astranova.org", en: "astranova.org" }, url: "https://www.astranova.org/" },
      { label: { zh: "astranova.org · 高中版块", en: "astranova.org · high-school section" }, url: "https://www.astranova.org/#high-school" },
      { label: { zh: "adastraschool.org", en: "adastraschool.org" }, url: "https://www.adastraschool.org/" },
      { label: { zh: "adastraschool.org/admissions", en: "adastraschool.org/admissions" }, url: "https://www.adastraschool.org/admissions" },
      { label: { zh: "adastraschool.org/primary", en: "adastraschool.org/primary" }, url: "https://www.adastraschool.org/primary" },
      { label: { zh: "adastraschool.org/lowerelementary", en: "adastraschool.org/lowerelementary" }, url: "https://www.adastraschool.org/lowerelementary" },
      { label: { zh: "menu.astranova.org（课程单）", en: "menu.astranova.org (course list)" }, url: "https://menu.astranova.org" },
      { label: { zh: "官方 Conundrums YouTube 频道", en: "Official Conundrums YouTube channel" }, url: "https://www.youtube.com/c/astranovaschool" },
      { label: { zh: "synthesis.com/tutor", en: "synthesis.com/tutor" }, url: "https://www.synthesis.com/tutor" },
    ],
  },
  {
    heading: { zh: "SpaceX 时期 Ad Astra", en: "SpaceX-era Ad Astra" },
    items: [
      { label: { zh: "Wikipedia: Astra Nova School", en: "Wikipedia: Astra Nova School" }, url: "https://en.wikipedia.org/wiki/Astra_Nova_School" },
      { label: { zh: "ProPublica 非营利数据库（Form 990）", en: "ProPublica nonprofit database (Form 990)" }, url: "https://projects.propublica.org/nonprofits/organizations/471480453" },
      { label: { zh: "Ars Technica, 2018-06-25", en: "Ars Technica, 2018-06-25" }, url: "https://arstechnica.com/science/2018/06/first-space-then-auto-now-elon-musk-quietly-tinkers-with-education/" },
      { label: { zh: "Washington Post, 2018-06-27", en: "Washington Post, 2018-06-27" }, url: "https://www.washingtonpost.com/technology/2018/06/27/elon-musk-created-secretive-laboratory-school-brilliant-kids-who-love-flamethrowers/" },
      { label: { zh: "Quartz, 2018-11-30", en: "Quartz, 2018-11-30" }, url: "https://qz.com/1480109/the-three-questions-on-the-application-for-elon-musks-private-school" },
      { label: { zh: "Business Insider, 2015-05-22", en: "Business Insider, 2015-05-22" }, url: "https://www.businessinsider.com/elon-musk-creates-a-grade-school-2015-5" },
      { label: { zh: "Daily Beast, 2020-07-05", en: "Daily Beast, 2020-07-05" }, url: "https://www.thedailybeast.com/would-you-pay-dollar7500-to-educate-your-kid-like-elon-musks" },
      { label: { zh: "NY Magazine \"SpaceX Cadets\", 2022-08", en: "NY Magazine \"SpaceX Cadets\", 2022-08" }, url: "https://nymag.com/intelligencer/2022/08/elon-musk-astra-nova-school.html" },
      { label: { zh: "The Hustle（Synthesis 报道）", en: "The Hustle (on Synthesis)" }, url: "https://thehustle.co/meet-synthesis-the-edtech-startup-scaling-elon-musks-ad-astra-school" },
    ],
  },
  {
    heading: { zh: "Bastrop（德州新校区）", en: "Bastrop (the new Texas campus)" },
    items: [
      { label: { zh: "Fortune, 2024-11-20", en: "Fortune, 2024-11-20" }, url: "https://fortune.com/2024/11/20/elon-musk-ad-astra-school-permit-montessori-bastrop-texas/" },
      { label: { zh: "Bloomberg, 2024-12-17", en: "Bloomberg, 2024-12-17" }, url: "https://www.bloomberg.com/news/features/2024-12-17/elon-musk-s-new-texas-preschool-highlights-his-education-priorities" },
      { label: { zh: "Texas Tribune, 2025-01-13", en: "Texas Tribune, 2025-01-13" }, url: "https://www.texastribune.org/2025/01/13/texas-elon-musk-school-education-bastrop-county/" },
      { label: { zh: "KUT, 2025-01-13", en: "KUT, 2025-01-13" }, url: "https://www.kut.org/education/2025-01-13/elon-musk-ad-astra-school-education-bastrop-austin-texas" },
      { label: { zh: "Forbes, 2024-07-31", en: "Forbes, 2024-07-31" }, url: "https://www.forbes.com/sites/sarahemerson/2024/07/31/elon-musks-experimental-school-in-texas-is-now-looking-for-students/" },
      { label: { zh: "NYT, 2025-12-18", en: "NYT, 2025-12-18" }, url: "https://www.nytimes.com/2025/12/18/technology/elon-musk-daycare-school.html" },
      { label: { zh: "The Cool Down, 2025-12-19", en: "The Cool Down, 2025-12-19" }, url: "https://www.thecooldown.com/green-business/ad-astra-school-elon-musk-texas/" },
      { label: { zh: "Quartz（Bastrop）", en: "Quartz (Bastrop)" }, url: "https://qz.com/elon-musk-school-eduation-bastrop-texas-ad-astra" },
      { label: { zh: "KVUE", en: "KVUE" }, url: "https://www.kvue.com/article/news/local/elon-musks-ad-astra-montessori-school-permit-to-open-bastrop-county/269-22f51286-34cc-4349-9355-653f96910f65" },
      { label: { zh: "X Foundation Form 990 (2023)", en: "X Foundation Form 990 (2023)" }, url: "https://www.documentcloud.org/documents/25484756-2023-990-x-foundation/" },
      { label: { zh: "Indeed（教师薪资）", en: "Indeed (teacher salary)" }, url: "https://www.indeed.com/cmp/Ad-Astra-Montessori-School/salaries/Teacher" },
      { label: { zh: "Xplor Education 招聘帖", en: "Xplor Education job listing" }, url: "https://www.wayup.com/i-j-Xplor-Education-733710150076336/" },
      { label: { zh: "Salary.com 招聘帖 (2024)", en: "Salary.com job listing (2024)" }, url: "https://www.salary.com/job/xplor-education/elementary-stem-specialist-assistant-elementary-teacher/j202405181304167135276" },
    ],
  },
  {
    heading: { zh: "高中申请与录取/被拒案例", en: "High-school application and admit/reject cases" },
    items: [
      { label: { zh: "外滩教育 · Julia 专访（2024-10-02）", en: "Bund Education · Julia interview (2024-10-02)" }, url: "https://news.qq.com/rain/a/20241002A0143800" },
      { label: { zh: "WSDA 专访转载（2023-10-31）", en: "WSDA interview reprint (2023-10-31)" }, url: "https://www.jingsailian.com/news/567784.html" },
      { label: { zh: "puredu.top 分享实录", en: "puredu.top shared transcript" }, url: "https://puredu.top/futuredu-newstars/" },
      { label: { zh: "puredu.top 申请流程页", en: "puredu.top application-process page" }, url: "https://puredu.top/astranova-app/" },
      { label: { zh: "puredu.top · Astra Nova 手册", en: "puredu.top · Astra Nova handbook" }, url: "https://puredu.top/astranova/" },
      { label: { zh: "r/Gifted 湾区家长（2025-02-11）", en: "r/Gifted, Bay Area parent (2025-02-11)" }, url: "https://reddit.com/r/Gifted/comments/1ims63x/" },
      { label: { zh: "在读生自述（2022-01）", en: "Current-student self-account (2022-01)" }, url: "https://reddit.com/r/EnoughMuskSpam/comments/ngyuua/elon_musk_opened_a_school_back_in_2020_and_its/hsnpmse/" },
      { label: { zh: "Winston Lin（2021-02）", en: "Winston Lin (2021-02)" }, url: "https://reddit.com/r/composer/comments/lbgbfc/" },
      { label: { zh: "u/siberian 被拒叙述（2024-08）", en: "u/siberian's rejection account (2024-08)" }, url: "https://reddit.com/r/conservativeterrorism/comments/1ejvknb/conspiracy_theorist_elon_musk_about_to_open/lggvmjx/" },
      { label: { zh: "Andy Jagoe 博客（2021-04）", en: "Andy Jagoe blog (2021-04)" }, url: "https://andyjagoe.com/send-your-child-to-school-like-elon-musks/" },
      { label: { zh: "The Lake 作答帖（2020-05）", en: "\"The Lake\" answer thread (2020-05)" }, url: "https://reddit.com/r/AskReddit/comments/gqo600/how_would_you_guys_rank_the_the_order_of_blame/frtv8l4/" },
      { label: { zh: "Conundrum 视频样本 · Narthana · Martian", en: "Conundrum video sample · Narthana · Martian" }, url: "https://youtu.be/iaCdzPTIKaM" },
      { label: { zh: "Conundrum 视频样本 · Narthana · Chocolate", en: "Conundrum video sample · Narthana · Chocolate" }, url: "https://youtu.be/b9mq1QocaXM" },
      { label: { zh: "Conundrum 视频样本 · Narthana · Pizza", en: "Conundrum video sample · Narthana · Pizza" }, url: "https://youtu.be/h3-1djQfK0I" },
      { label: { zh: "Conundrum 视频样本 · Leonid · The Lake", en: "Conundrum video sample · Leonid · The Lake" }, url: "https://youtu.be/Mm_WO84swDk" },
      { label: { zh: "City Journal（2023-05）", en: "City Journal (2023-05)" }, url: "https://www.city-journal.org/article/the-next-frontier-in-stem-education" },
      { label: { zh: "Niche 档案页", en: "Niche profile page" }, url: "https://www.niche.com/k12/astra-nova-school-ca/" },
      { label: { zh: "阅读第一（腾讯新闻，2024-08-17）", en: "\"Read First\" (Tencent News, 2024-08-17)" }, url: "https://news.qq.com/rain/a/20240817A00V7J00" },
      { label: { zh: "谷雨星球 · Synthesis 三个中国孩子（2024-09-26）", en: "Valley Rain Institute · three Chinese kids at Synthesis (2024-09-26)" }, url: "https://news.qq.com/rain/a/20240926A04H1700" },
    ],
  },
];

export const methodologyNote: Bilingual[] = [
  {
    zh: "① adastraschool.org 内容基本停留在 2024–25 招生季，官方信息反映的是首届规划的当时状态；② nymag.com 与 NYT 原文有付费墙/抓取限制，相关细节经由转引核对；③ Wikipedia 上「Astra Nova 全日制学费 $32,500」为旧数据，本报告以官网 2026/27 分档学费为准。",
    en: "① adastraschool.org's content largely remains frozen at the 2024–25 admissions cycle — its official information reflects the first-cohort planning as it stood then. ② nymag.com and NYT originals sit behind paywalls/scraping limits; relevant details were cross-checked via secondary citations. ③ Wikipedia's \"$32,500 full-time Astra Nova tuition\" is outdated; this report uses the official site's 2026/27 tiered tuition instead.",
  },
  {
    zh: "方法说明：Reddit 官方接口屏蔽抓取，经 PullPush 存档库检索（覆盖约 2020–2025 年中，此后内容未能覆盖）；小红书不被外部索引，仅能通过公众号截图间接证实；X/Quora/Facebook 受访问限制。",
    en: "Methodology note: Reddit's official API blocks scraping, so this report used the PullPush archive (covering roughly 2020 through mid-2025; content after that wasn't reachable). Xiaohongshu isn't externally indexed and could only be confirmed indirectly via WeChat-article screenshots. X/Quora/Facebook access was limited.",
  },
];

/* ════════════════════════════════════════════════════════════════════════
   Aggregate export
   ════════════════════════════════════════════════════════════════════════ */

export const adAstra = {
  meta,
  genealogy,
  admissions,
  ageStages,
  highSchool,
  cases,
  mythbusting,
  sourcesIntro,
  sourceGroups,
  methodologyNote,
};

// NOTE for Task D2: add a sibling `export const schools = { ... }` below,
// sourced from ad-astra-调研报告.html §3 「同类高端创新学校对比」 — keep it
// independent of `adAstra` above (no shared mutable state) so this file can
// grow to serve /institute/schools without touching this export.
