// Per Aspera — 思维印记 (Mind Imprint) product page (/mind-imprint,
// /en/mind-imprint) content.
//
// zh is the source of truth; en is an idiomatic (not literal) translation.
//
// This is the product-first story: 思维印记 is our ONE core AI-education
// product, carrying a whole way of learning through three components —
//   1 · AI-assisted interactive courses  (思辨 + 产品思维 threads)
//   2 · two AI-guided workbenches         (批判性思维 / 产品思维)
//   3 · process assessment                (turns the reasoning process into a
//                                          record only the student sees)
// Three services are built on top of the product and only linked from here:
// 申请辅导 /coaching · 课程 /academy · 学校合作 /partnership.
//
// The assessment content and the dark interface mockup are the faithful
// 思维印记 representation (two faces 生成式驾驭 🚀 / 批判式防护 🛡️ across ten
// dimensions D1–D10; growth on a four-level SOLO scale L1–L4; private to the
// student, always the strongest model, offered and never pushed).
//
// Plain, warm, parent-facing wording — no jargon, no deadlines, no info
// sessions, no antithesis ("不是…而是" / not-X-but-Y).

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "思维印记", en: "Mind Imprint" },
  title: {
    zh: "我们的核心产品：一款把整套学习方式装进去的 AI 教育产品——课程、工作台，和一份能看见思考过程的评估。",
    en: "Our core product: one AI education product that carries a whole way of learning — courses, workbenches, and an assessment that sees the thinking process.",
  },
  sub: {
    zh: "孩子带着自己真实的任务进来，和 AI 一起把它想清楚、做出来；这一路他是怎么想的，都被记录下来，慢慢长成一份只给他自己看的成长记录。",
    en: "A child brings in a real task and works it through with AI — thinking it out, building it up — and the whole way they got there is recorded, growing into a record made just for them.",
  },
};

/* ---- 1 · Why we made it a product -------------------------------------- */
export const whyProduct: { eyebrow: Bilingual; title: Bilingual; body: Bilingual[] } = {
  eyebrow: { zh: "为什么把它做成一个产品", en: "Why we made it a product" },
  title: {
    zh: "AI 时代，真实的能力既要被养出来，也要被看见。",
    en: "In the AI era, real ability has to be built — and made visible.",
  },
  body: [
    {
      zh: "AI 已经在每个孩子手边了。真正的本事——会提问、会核实一个说法、会自己拿判断——需要有人在旁边一步步带着练，才长得出来。",
      en: "AI is already within every child's reach. The real skills — asking good questions, checking a claim, holding your own judgment — grow only when someone guides the practice, step by step.",
    },
    {
      zh: "所以我们把这套学习方式做成了一个产品：让每个孩子都能得到稳定、到位的 AI 引导，也让他思考力怎么一点点长起来，变成一份看得见、留得下的记录。",
      en: "So we turned this way of learning into a product: every child gets steady, capable AI guidance, and the way their thinking grows becomes a record you can see and keep.",
    },
  ],
};

/* ---- 2 · Component 1 · AI-assisted interactive courses ----------------- */
export interface Thread {
  name: Bilingual;
  body: Bilingual;
}

export const courses: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  threads: Thread[];
  linkLabel: Bilingual;
  linkHref: string;
} = {
  eyebrow: { zh: "组成一 · AI 互动课", en: "Component 1 · AI-assisted courses" },
  title: {
    zh: "真人小班、AI 全程陪着上的互动课。",
    en: "Live, small-group courses, taught with AI alongside.",
  },
  body: [
    {
      zh: "我们的课都是真人小班、实时在线上的，AI 全程在旁边搭把手。这里没有录播视频——光看视频是 AI 出现之前的老办法；在我们的课上，孩子从头到尾都在动手参与。",
      en: "Our classes are live, small-group, and taught in real time, with AI helping throughout. There are no recorded video lectures — passive video is a pre-AI format; here, a child takes part hands-on from start to finish.",
    },
    {
      zh: "课程分两条线，一条练思辨，一条练产品思维。孩子可以按自己的兴趣和节奏，两条线一起走。",
      en: "The courses run on two threads — one for critical thinking, one for product thinking. A child can follow both, at their own interest and pace.",
    },
  ],
  threads: [
    {
      name: { zh: "思辨", en: "Critical thinking" },
      body: {
        zh: "练怎么提问、怎么核实一个说法、怎么把论证搭起来再改扎实。这条线从三十多门在真实国际课堂里带过的课里，一点点长出来。",
        en: "Practicing how to ask, how to check a claim, and how to build an argument and revise it until it holds. This thread grew out of 30-plus courses proven in real international classrooms.",
      },
    },
    {
      name: { zh: "产品思维", en: "Product thinking" },
      body: {
        zh: "带孩子和 AI 一起动手，把一个真实问题一路做到一个能用的原型。",
        en: "Working alongside AI to take a real problem all the way to a working prototype.",
      },
    },
  ],
  linkLabel: { zh: "看看我们的课程", en: "See our courses" },
  linkHref: "/academy",
};

/* ---- 3 · Component 2 · Two workbenches --------------------------------- */
export interface Workbench {
  name: Bilingual;
  desc: Bilingual;
  exampleLabel: Bilingual;
  example: Bilingual;
}

export const workbenches: {
  eyebrow: Bilingual;
  title: Bilingual;
  intro: Bilingual[];
  items: Workbench[];
} = {
  eyebrow: { zh: "组成二 · 两个工作台", en: "Component 2 · Two workbenches" },
  title: {
    zh: "两个和 AI 一起动手的工作台。",
    en: "Two workbenches for working alongside AI.",
  },
  intro: [
    {
      zh: "工作台是一个让孩子和 AI 一起完成真实任务的互动界面。在对的时刻，AI 会把该想的那一步交回给孩子——它会引导、追问、提出挑战，始终把结论留给孩子自己下。",
      en: "A workbench is an interactive space where a child works a real task together with AI. At the right moment, the AI hands the thinking step back to the child — it guides, probes, and challenges, and always leaves the conclusion for the child to reach.",
    },
    {
      zh: "这背后是「思维工具卡」：在合适的时刻，AI 会停下来，请孩子自己先想一步，再往下走。",
      en: "Behind this are thinking tool-cards: at the right moment, the AI pauses and asks the student to think a step through for themselves before going on.",
    },
  ],
  items: [
    {
      name: { zh: "批判性思维工作台", en: "Critical Thinking Workbench" },
      desc: {
        zh: "围绕提问、核实来源、搭建和修改论证。",
        en: "For questioning, verifying sources, and building and revising an argument.",
      },
      exampleLabel: { zh: "举个例子", en: "For example" },
      example: {
        zh: "手上有一篇文章想拿来当证据，AI 会先请孩子停一停，把来源一路溯清楚——查到独立的原始出处，再判断它到底能说明什么。",
        en: "A child wants to cite an article as evidence — the AI first asks them to pause and trace the source all the way back, finding the independent origin before judging what it really proves.",
      },
    },
    {
      name: { zh: "产品思维工作台", en: "Product Thinking Workbench" },
      desc: {
        zh: "从一个真实问题出发，一路做到一个能用的原型，难度按年龄来调。",
        en: "Starting from a real problem and working all the way to a working prototype, sized to the child's age.",
      },
      exampleLabel: { zh: "举个例子", en: "For example" },
      example: {
        zh: "任务是给学校食堂做一个减少浪费的小工具，AI 像一个同事一样帮着出主意、写代码，但它给的每一步，孩子都要自己核对过才算数。",
        en: "The task is a small tool to cut waste in the school canteen — the AI works like a colleague, suggesting ideas and writing code, and every step it hands over has to be checked by the child before it counts.",
      },
    },
  ],
};

/* ---- 4 · Component 3 · Process assessment ------------------------------ */
export const miIntro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual[] } = {
  eyebrow: { zh: "组成三 · 过程评估", en: "Component 3 · Process assessment" },
  title: {
    zh: "把思考的过程，变成一张成长记录。",
    en: "Turning the thinking process into a report card.",
  },
  body: [
    {
      zh: "在课程和工作台里，孩子每一次和 AI 的互动，都是评估的素材。过程评估看的是孩子这一路怎么和 AI 一起把问题想清楚的——那份最后交出的成品，它先放到一边。",
      en: "Inside the courses and the workbenches, every interaction a child has with AI is assessment data. Process assessment looks at how the child actually worked the problem through with AI — the finished piece itself, it sets aside for a moment.",
    },
    {
      zh: "它会看：有没有去核实一个说法的来源，撞到一个反例时是绕开还是认真接住，哪些地方是孩子自己的判断、哪些地方是直接照搬了 AI 的话。",
      en: "It watches for things like: whether they checked where a claim came from, whether they faced a counter-example head-on or dodged it, and which parts were the child's own judgment versus copied straight from the AI.",
    },
  ],
};

/** Caption shown under the interface mockup. */
export const miShotCaption: Bilingual = {
  zh: "思维印记的工作区与评估界面——左边和 AI 一次一步地把问题想深，右边的过程树记录每一步，最后长成一份只给学生本人看的思维印记。",
  en: "The mind-imprint workspace and assessment screen — on the left, thinking a question through with AI one step at a time; on the right, a process tree recording every step; and finally an imprint seen only by the student.",
};

export const miAfterShot: Bilingual[] = [
  {
    zh: "一篇漂亮的作文，可能是 AI 一键生成的，也可能是孩子认真想了很久的；单看成品，分不出这两者的差别。过程能看出这个差别——它补上一份成绩单看不到的那部分，让家长和孩子都真正看清楚，AI 时代的孩子，思考能力到底在不在长。",
    en: "A polished essay could be one AI click, or it could be hours of a child's real thought — from the finished piece alone, you can't tell the two apart. The process shows the difference. It fills in the part a report card can't, so both parents and the child can see whether a child's thinking ability is genuinely growing in the AI era.",
  },
];

export interface Dimension {
  code: string;
  name: Bilingual;
}

export interface Face {
  face: "drive" | "guard";
  icon: string;
  title: Bilingual;
  sub: Bilingual;
  dims: Dimension[];
}

export const miModel: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual; faces: Face[] } = {
  eyebrow: { zh: "认知模型", en: "The cognitive model" },
  title: { zh: "两面、十个维度。", en: "Two faces, ten dimensions." },
  sub: {
    zh: "我们把「善用 AI」拆成一枚硬币的两面——主动驾驭 AI，与批判地守住自己的判断——再往下分成十个具体维度，逐一评估。",
    en: "We split \"using AI well\" into two sides of one coin — actively driving AI, and critically guarding your own judgment — and evaluate ten concrete dimensions underneath.",
  },
  faces: [
    {
      face: "drive",
      icon: "🚀",
      title: { zh: "生成式驾驭", en: "Generative drive" },
      sub: {
        zh: "主动驾驭 AI，把它变成放大思考的工具",
        en: "Actively driving AI — making it a tool that amplifies thinking",
      },
      dims: [
        { code: "D1", name: { zh: "提问清晰度", en: "Prompt clarity" } },
        { code: "D10", name: { zh: "协作编排", en: "Collaborative orchestration" } },
        { code: "D4", name: { zh: "多视角与让步", en: "Perspective & concession" } },
        { code: "D5", name: { zh: "论证拆解", en: "Argument analysis" } },
        { code: "D7", name: { zh: "论证质量", en: "Argument quality" } },
      ],
    },
    {
      face: "guard",
      icon: "🛡️",
      title: { zh: "批判式防护", en: "Critical guard" },
      sub: {
        zh: "不被 AI 俘获，守住判断与诚信",
        en: "Not getting captured by AI — guarding judgment and integrity",
      },
      dims: [
        { code: "D2", name: { zh: "信源辨识", en: "Source discernment" } },
        { code: "D3", name: { zh: "横向验证", en: "Lateral verification" } },
        { code: "D6", name: { zh: "反思与元认知", en: "Reflection & metacognition" } },
        { code: "D8", name: { zh: "信息再生产", en: "Information reproduction" } },
        { code: "D9", name: { zh: "AI 边界与伦理", en: "AI limits & ethics" } },
      ],
    },
  ],
};

export interface SoloLevel {
  code: string;
  name: Bilingual;
  body: Bilingual;
}

export const miSolo: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual; levels: SoloLevel[] } = {
  eyebrow: { zh: "四级成长量表", en: "The four growth levels" },
  title: { zh: "每个维度，看到四个成长阶段。", en: "Each dimension, across four stages of growth." },
  sub: {
    zh: "这四个阶段来自 SOLO 分类学，衡量的是理解的深浅——从零散的单点，一直到能融会贯通、迁移应用，和「对错」无关。",
    en: "The four stages come from the SOLO taxonomy. They measure how deep a student's understanding goes — from scattered single points to connected, transferable mastery — apart from right or wrong.",
  },
  levels: [
    { code: "L1", name: { zh: "萌芽", en: "Emerging" }, body: { zh: "只做到一个孤立的点，还没有联系起来。", en: "One isolated point, not yet connected to anything else." } },
    { code: "L2", name: { zh: "发展中", en: "Developing" }, body: { zh: "做到了几个点，但还没有连成一片。", en: "Several points, but not yet joined up." } },
    { code: "L3", name: { zh: "熟练", en: "Proficient" }, body: { zh: "把这些要素连成了结构，能主动运用。", en: "The pieces are linked into a working structure the student applies actively." } },
    { code: "L4", name: { zh: "卓越", en: "Advanced" }, body: { zh: "能迁移、能反思，超出了当下这一次任务。", en: "The student transfers and reflects on it, going beyond this one task." } },
  ],
};

/** The initiative gradient — the process also records HOW a step was reached. */
export const miInitiative: { eyebrow: Bilingual; title: Bilingual; body: Bilingual } = {
  eyebrow: { zh: "主动性梯度", en: "The initiative gradient" },
  title: { zh: "怎么做到的，也一起记下来。", en: "How a step was reached is recorded too." },
  body: {
    zh: "过程还记着另一件事：一个该想的步骤，孩子是自己主动伸手去做的，还是被提醒了才做的，又或者跳过没做——这些都算数。孩子把一张工具卡放到一边，也一样被记下来。",
    en: "The process also keeps track of one more thing: whether a child reached for a thinking step on their own, only after a nudge, or skipped it altogether — all of it counts. Setting a tool-card aside is recorded too.",
  },
};

export interface Principle {
  title: Bilingual;
  body: Bilingual;
}

export const miPrinciples: { eyebrow: Bilingual; title: Bilingual; items: Principle[] } = {
  eyebrow: { zh: "评估的原则", en: "Principles behind the assessment" },
  title: { zh: "评估本身，也守着几件坚持。", en: "The assessment keeps its own set of principles." },
  items: [
    {
      title: { zh: "只给学生本人看", en: "Private to the student" },
      body: {
        zh: "思维印记是一面私密的成长镜子，只照给学生自己看，不用来排名或比较。",
        en: "The mind imprint is a private mirror for growth, seen only by the student — never used for ranking or comparison.",
      },
    },
    {
      title: { zh: "评估永远用最强的模型", en: "Always the strongest model" },
      body: {
        zh: "日常陪练可以用更轻量的模型，但一到评估这一步，我们始终用最强的那一个，保证判断可靠。",
        en: "Everyday coaching can run on a lighter model, but for the assessment itself, we always use the very best one, so the judgment holds up.",
      },
    },
    {
      title: { zh: "安静地提供，不会推着学生看", en: "Offered, never pushed" },
      body: {
        zh: "结果安静地出现——没有徽章、不会自动弹开、也没有排行榜，什么时候点开来看，由学生自己决定。",
        en: "Results appear quietly — no badges, no auto-open, no leaderboard. The student decides if and when to look.",
      },
    },
  ],
};

/* ---- 5 · Closing: one product, three services -------------------------- */
export interface ServiceLink {
  name: Bilingual;
  body: Bilingual;
  href: string;
}

export const closing: {
  title: Bilingual;
  sub: Bilingual;
  services: ServiceLink[];
  ctaLabel: Bilingual;
  ctaHref: string;
} = {
  title: { zh: "一个产品，支撑起我们做的每一件事。", en: "One product powers everything we do." },
  sub: {
    zh: "思维印记是核心。围着它，我们提供三种服务。",
    en: "Mind imprint is the core. Around it, we offer three services.",
  },
  services: [
    {
      name: { zh: "申请辅导", en: "Application coaching" },
      body: { zh: "一对一陪孩子走完申请这段路。", en: "One-on-one guidance through the whole application journey." },
      href: "/coaching",
    },
    {
      name: { zh: "课程", en: "Courses" },
      body: { zh: "业余时间、全程在线的课程项目。", en: "Part-time, fully online course programs." },
      href: "/academy",
    },
    {
      name: { zh: "学校合作", en: "Partnership" },
      body: { zh: "陪学校把 AI 踏实地带进日常教学。", en: "Bringing AI into a school's everyday teaching, thoughtfully." },
      href: "/partnership",
    },
  ],
  ctaLabel: { zh: "联系我们", en: "Contact us" },
  ctaHref: "/contact",
};
