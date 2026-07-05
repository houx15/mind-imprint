// Per Aspera — programs page (/programs, /en/programs) content.
// zh is the source of truth, copied verbatim from
// docs/astranova/PerAspera官网文案v2-多页版.md §二「课程页 /programs」; en is an
// idiomatic (not literal) translation. All ¥____ price blanks in the source
// are intentionally unfilled — they render as a "待定/TBD" badge, never an
// invented number.
//
// One deliberate rewrite: the 立场声明's closing line in the source reads
// "但打磨的是表达，不是替你思考。" (an "A, not B" contrast). Per the site's
// positive-declarative copy rule (see home.ts), it's rephrased below without
// the negation-contrast while keeping the same meaning: the family owns the
// materials; we only help polish how they're expressed.

import type { Bilingual } from "./site";

/* ---- Page hero -------------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "课程", en: "Programs" },
  title: {
    zh: "两个产品，一条主线：真实的能力。",
    en: "Two programs, one throughline: real ability.",
  },
  sub: {
    zh: "短线，为 10 月 15 日而战；长线，为孩子的十年而建。",
    en: "Short-term, built for October 15. Long-term, built for the next ten years of your child's thinking.",
  },
};

/* ---- Sprint · #sprint -------------------------------------------------------- */

export interface Track {
  title: Bilingual;
  bullets: Bilingual[];
  /** Optional cross-link, used once to point the reasoning bullet at the Institute. */
  note?: { text: Bilingual; href: string };
}

export interface TimelineStep {
  label: Bilingual;
  date: Bilingual;
}

export interface FeeItem {
  name: Bilingual;
  /** Always the TBD placeholder in this launch — the source leaves every price blank. */
  price: Bilingual;
  desc: Bilingual;
}

export const sprint: {
  eyebrow: Bilingual;
  title: Bilingual;
  sub: Bilingual;
  position: Bilingual;
  positionBody: Bilingual[];
  tracksIntro: { eyebrow: Bilingual; title: Bilingual };
  tracks: Track[];
  timelineIntro: { eyebrow: Bilingual; title: Bilingual };
  timeline: TimelineStep[];
  feesIntro: { eyebrow: Bilingual; title: Bilingual };
  fees: FeeItem[];
  feesDisclaimer: Bilingual;
  screeningTitle: Bilingual;
  screeningBody: Bilingual;
} = {
  eyebrow: { zh: "冲刺营", en: "The Sprint" },
  title: { zh: "Per Aspera 冲刺营", en: "Per Aspera Sprint" },
  sub: {
    zh: "面向 2026-10-15 Astra Nova 高中申请 · 14 周 · 双轨制 · 首届 6–10 组家庭",
    en: "Built for the Oct 15, 2026 Astra Nova high-school application · 14 weeks · two parallel tracks · a first cohort of 6–10 families",
  },

  position: {
    zh: "先说清楚：应试没有用。",
    en: "Let's say this upfront: cramming for the test doesn't work.",
  },
  positionBody: [
    {
      zh: "Astra Nova 的申请没有题库——思辨题没有标准答案，评的是推理过程；小组面试会当场追问反转（真实案例：老师展示一张照片问“是 PS 的吗”，多数孩子答是，老师追问——“如果我告诉你是真的呢？”）。被训练过痕迹的孩子，在这种时刻反而会露馅。",
      en: "Astra Nova's application has no question bank. Its reasoning prompts have no model answer — what's graded is the reasoning itself — and the group interview probes for a reversal in real time. (A real case: a teacher held up a photo and asked, “Is this photoshopped?” Most kids said yes — then came the follow-up: “What if I told you it's real?”) Kids who've been visibly coached tend to unravel in exactly that moment.",
    },
    {
      zh: "所以我们做的是另一件事：用 14 周，真实地提升孩子的思辨、表达与协作，同时提升家长的教育理念。最后两周，我们会帮孩子打磨材料的表达方式——思考本身，从头到尾都由孩子自己完成。",
      en: "So we do something else entirely: spend 14 weeks genuinely building your child's reasoning, expression, and collaboration, while sharpening your own thinking about education alongside them. In the final two weeks we help polish how the materials are expressed — the thinking inside them stays entirely your child's own, start to finish.",
    },
  ],

  tracksIntro: {
    eyebrow: { zh: "双轨设计", en: "Two tracks" },
    title: {
      zh: "孩子在训练，家长也在成长。",
      en: "The child trains. The parents grow alongside them.",
    },
  },
  tracks: [
    {
      title: { zh: "孩子轨 · 12 周 × 每周 2 次", en: "Student track · 12 weeks × 2 sessions/week" },
      bullets: [
        {
          zh: "W1–4 思维打底：论证与谬误、事实/观点/价值、伦理两难、计算思维拆问题",
          en: "Weeks 1–4 — Thinking foundations: argument and fallacy, fact vs. opinion vs. value, ethical dilemmas, breaking down problems with computational thinking",
        },
        {
          zh: "W5–8 思辨实战：每周一道真实思辨题，录视频 → 思维印记 AI 复盘（论证结构分析 + 思维等级周报）",
          en: "Weeks 5–8 — Reasoning in practice: one real reasoning prompt a week, recorded on video, then reviewed by Mind Imprint AI (argument-structure analysis plus a weekly thinking-level report)",
        },
        {
          zh: "W9–12 面试形态：全英文无领导小组讨论 ×4、反转追问训练、倾听与当众修正观点",
          en: "Weeks 9–12 — Interview form: four all-English leaderless group discussions, training for reversal follow-ups, and practice listening and revising your view in front of others",
        },
        {
          zh: "10 月：选题定稿、视频录制（严守 30 秒–2 分钟）、两次全真模拟",
          en: "October — finalizing the topic, recording the video (strictly 30 seconds to 2 minutes), and two full mock interviews",
        },
      ],
      note: {
        text: { zh: "了解思维印记如何评估这个过程", en: "See how Mind Imprint assesses this process" },
        href: "/institute",
      },
    },
    {
      title: { zh: "家长轨 · 6 次隔周工作坊", en: "Parent track · 6 biweekly workshops" },
      bullets: [
        {
          zh: "Astra Nova 到底要什么（一手案例讲透，含被拒案例）",
          en: "What Astra Nova is actually looking for — first-hand cases, including rejections",
        },
        {
          zh: "家长信写作：一页、像邮件、反包装",
          en: "Writing the parent letter — one page, written like an email, no packaging",
        },
        {
          zh: "饭桌上的思辨：怎么在家做思维对话",
          en: "Reasoning at the dinner table — how to run a real thinking conversation at home",
        },
        {
          zh: "非传统路径的代价：时差、社交、学历衔接，全部摊开谈",
          en: "The real cost of a nontraditional path — time zones, social life, credential transfer, all on the table",
        },
        {
          zh: "家庭面谈模拟",
          en: "A mock family interview",
        },
        {
          zh: "录取后决策：选档与资助申请实操",
          en: "Post-admission decisions — choosing a track and applying for financial aid",
        },
      ],
    },
  ],

  timelineIntro: {
    eyebrow: { zh: "时间线", en: "Timeline" },
    title: { zh: "从报名到放榜，五个半月。", en: "Five and a half months, enrollment to decision." },
  },
  timeline: [
    { label: { zh: "报名与家庭面试", en: "Enrollment & family interview" }, date: { zh: "7 月", en: "July" } },
    { label: { zh: "12 周训练", en: "12 weeks of training" }, date: { zh: "7 月中–10 月上", en: "Mid-Jul–early Oct" } },
    { label: { zh: "材料定稿与提交", en: "Finalize & submit materials" }, date: { zh: "10.15", en: "Oct 15" } },
    { label: { zh: "面试集训", en: "Interview bootcamp" }, date: { zh: "10.15–11.1", en: "Oct 15–Nov 1" } },
    { label: { zh: "小组面试", en: "Group interview" }, date: { zh: "11.1", en: "Nov 1" } },
    { label: { zh: "放榜", en: "Decisions released" }, date: { zh: "12.15", en: "Dec 15" } },
  ],

  feesIntro: {
    eyebrow: { zh: "收费", en: "Tuition & fees" },
    title: { zh: "透明的三笔费用。", en: "Three line items, fully transparent." },
  },
  fees: [
    {
      name: { zh: "基础费", en: "Base tuition" },
      price: { zh: "待定", en: "TBD" },
      desc: {
        zh: "覆盖 14 周全部训练 + 双轨工作坊 + 思维评估报告，无论申请结果",
        en: "Covers all 14 weeks of training, both tracks' workshops, and the thinking-assessment report — regardless of the application outcome",
      },
    },
    {
      name: { zh: "录取成功费", en: "Success fee" },
      price: { zh: "待定", en: "TBD" },
      desc: {
        zh: "仅在 12.15 获录取后支付",
        en: "Charged only after an offer on Dec 15",
      },
    },
    {
      name: { zh: "首届内测价", en: "Founding-cohort rate" },
      price: { zh: "待定", en: "TBD" },
      desc: {
        zh: "前 __ 组家庭 __ 折，以案例授权为条件",
        en: "A discount for the first __ families, conditional on case-study consent",
      },
    },
  ],
  feesDisclaimer: {
    zh: "我们不承诺录取。警惕任何承诺录取的机构——这所学校一届只收几十人，且材料必须由孩子本人完成。",
    en: "We do not promise admission. Be wary of any program that does — this school admits only a few dozen students a year, and the materials must be the child's own work.",
  },

  screeningTitle: { zh: "我们会筛选家庭。", en: "We screen families before enrollment." },
  screeningBody: {
    zh: "入营前一次家庭面试：孩子 30 分钟（一道思辨题看推理基线），家长 30 分钟（教育理念对谈）。我们只与认同“能力优先于结果”的家庭合作。",
    en: "Before enrollment, every family has one interview: 30 minutes with the child (one reasoning prompt, to read a baseline), and 30 minutes with the parents (a conversation about educational philosophy). We work only with families who agree that ability comes before outcome.",
  },
};

/* ---- Academy · #academy ------------------------------------------------------ */

export interface CourseLine {
  title: Bilingual;
  desc: Bilingual;
}

export interface TierDef {
  name: Bilingual;
  hours: Bilingual;
  /** Always the TBD placeholder in this launch — the source leaves every price blank. */
  price: Bilingual;
  features: Bilingual[];
}

export interface CatalogItem {
  name: Bilingual;
  age: Bilingual;
  hours: Bilingual;
  instructor: Bilingual;
  seats: Bilingual;
}

export const academy: {
  eyebrow: Bilingual;
  title: Bilingual;
  sub: Bilingual;
  whyIntro: { eyebrow: Bilingual; title: Bilingual };
  why: Bilingual;
  linesIntro: { eyebrow: Bilingual; title: Bilingual };
  lines: CourseLine[];
  tiersIntro: { eyebrow: Bilingual; title: Bilingual };
  tiers: TierDef[];
  catalogIntro: { eyebrow: Bilingual; title: Bilingual };
  catalog: CatalogItem[];
} = {
  eyebrow: { zh: "长线学院", en: "The Academy" },
  title: { zh: "Per Aspera 学院", en: "Per Aspera Academy" },
  sub: {
    zh: "亚洲时区的问题解决者教育 · 一年 3 学期 × 10 周 · 小班 6–12 人 · 双语教学 + 每周全英文讨论 · 面向 9–14 岁",
    en: "Problem-solver education built for Asian time zones · 3 terms a year × 10 weeks · small classes of 6–12 · bilingual teaching with weekly all-English discussion · ages 9–14",
  },

  whyIntro: {
    eyebrow: { zh: "为什么存在", en: "Why we exist" },
    title: { zh: "同一套培养逻辑，搬进亚洲时区。", en: "The same training logic, built for Asian time zones." },
  },
  why: {
    zh: "Astra Nova 的课程全部排在美西时段——亚洲的孩子只能半日制参与。Per Aspera 学院把同一套培养逻辑（思辨、建造、模拟、表达）搬进亚洲时区，服务三种家庭：几年后想申请 Astra Nova 的、因时差或预算不去的、以及单纯认同这套培养方式的。",
    en: "Astra Nova's classes all run on U.S. West Coast hours, so kids in Asia can only join part-time. Per Aspera Academy brings the same training logic — reasoning, building, simulation, expression — into Asian time zones, for three kinds of families: those aiming at Astra Nova a few years from now, those for whom the time difference or the tuition doesn't work, and those who simply believe in this way of learning.",
  },

  linesIntro: {
    eyebrow: { zh: "四条课程线", en: "Four course lines" },
    title: { zh: "思辨、建造、模拟、表达。", en: "Reasoning, building, simulation, expression." },
  },
  lines: [
    {
      title: { zh: "思辨线", en: "Reasoning" },
      desc: {
        zh: "两难思辨、信息素养（过度旅游、资金链溯源、企业漂绿…）、五大知识领域“怎么算知道”——每学期换新课。",
        en: "Ethical dilemmas, information literacy (overtourism, following the money in fossil-fuel financing, corporate greenwashing...), and “how do we know what we know” across five areas of knowledge — new material every term.",
      },
    },
    {
      title: { zh: "建造线", en: "Building" },
      desc: {
        zh: "计算思维、与 AI 协作而不失判断、产品思维（从问题到原型）、数据与统计素养、编程入门。",
        en: "Computational thinking, collaborating with AI without losing your own judgment, product thinking (from problem to prototype), data and statistical literacy, and an introduction to programming.",
      },
    },
    {
      title: { zh: "模拟线", en: "Simulation" },
      desc: {
        zh: "团队策略模拟——资源分配、市场博弈、危机决策。没有说明书，规则靠孩子们自己摸出来。",
        en: "Team strategy simulations — resource allocation, market competition, crisis decisions. There's no manual; the kids work the rules out for themselves.",
      },
    },
    {
      title: { zh: "表达线", en: "Expression" },
      desc: {
        zh: "每周全英文小组讨论；学期末 Demo Day——当众展示与答辩，家长做观众，企业家做评委。",
        en: "Weekly all-English group discussion, capped each term by Demo Day — a public presentation and defense, with parents in the audience and entrepreneurs as judges.",
      },
    },
  ],

  tiersIntro: {
    eyebrow: { zh: "三档", en: "Three tiers" },
    title: { zh: "按投入程度选一档。", en: "Choose a tier by time commitment." },
  },
  tiers: [
    {
      name: { zh: "轻量", en: "Light" },
      hours: { zh: "每周 2 小时", en: "2 hrs/week" },
      price: { zh: "待定/学期", en: "TBD/term" },
      features: [
        { zh: "选修一条课程线（思辨 / 建造 / 模拟 / 表达任选其一）", en: "Pick one course line — reasoning, building, simulation, or expression" },
        { zh: "低门槛入口，适合先体验", en: "A low-commitment way to try the academy first" },
      ],
    },
    {
      name: { zh: "标准", en: "Standard" },
      hours: { zh: "每周 4–6 小时", en: "4–6 hrs/week" },
      price: { zh: "待定/学期", en: "TBD/term" },
      features: [
        { zh: "选修两条课程线", en: "Two course lines of your choosing" },
        { zh: "每周全英文小组讨论", en: "Weekly all-English group discussion" },
      ],
    },
    {
      name: { zh: "完整", en: "Full" },
      hours: { zh: "每周 8+ 小时", en: "8+ hrs/week" },
      price: { zh: "待定/学期", en: "TBD/term" },
      features: [
        { zh: "全部四条课程线", en: "All four course lines" },
        { zh: "团队策略模拟", en: "Team strategy simulation" },
        { zh: "学期末 Demo Day 项目答辩", en: "Demo Day presentation & defense at term's end" },
        { zh: "学期思维评估报告", en: "A term-end thinking-assessment report" },
      ],
    },
  ],

  catalogIntro: {
    eyebrow: { zh: "本学期课程单", en: "This term's course list" },
    title: { zh: "首批开设的五门课。", en: "The first five courses on offer." },
  },
  catalog: [
    { name: { zh: "两难思辨入门", en: "Intro to Ethical Dilemmas" }, age: { zh: "待定", en: "TBD" }, hours: { zh: "待定", en: "TBD" }, instructor: { zh: "待定", en: "TBD" }, seats: { zh: "待定", en: "TBD" } },
    { name: { zh: "信息侦探（识别漂绿）", en: "Information Detective (Spotting Greenwashing)" }, age: { zh: "待定", en: "TBD" }, hours: { zh: "待定", en: "TBD" }, instructor: { zh: "待定", en: "TBD" }, seats: { zh: "待定", en: "TBD" } },
    { name: { zh: "计算思维拆问题", en: "Computational Thinking for Problem-Breakdown" }, age: { zh: "待定", en: "TBD" }, hours: { zh: "待定", en: "TBD" }, instructor: { zh: "待定", en: "TBD" }, seats: { zh: "待定", en: "TBD" } },
    { name: { zh: "与 AI 协作保持判断", en: "Collaborating with AI Without Losing Judgment" }, age: { zh: "待定", en: "TBD" }, hours: { zh: "待定", en: "TBD" }, instructor: { zh: "待定", en: "TBD" }, seats: { zh: "待定", en: "TBD" } },
    { name: { zh: "全英文讨论工作坊", en: "All-English Discussion Workshop" }, age: { zh: "待定", en: "TBD" }, hours: { zh: "待定", en: "TBD" }, instructor: { zh: "待定", en: "TBD" }, seats: { zh: "待定", en: "TBD" } },
  ],
};
