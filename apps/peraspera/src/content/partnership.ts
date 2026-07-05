// Per Aspera — 合作 (partnership) page (/partnership, /en/partnership) content.
// zh is the source of truth; en is an idiomatic (not literal) translation.
//
// This page is for schools. It moves 理念 → 方案 → details of the 方案:
//   理念   — why a school should bring AI into teaching thoughtfully (ANL).
//   方案   — two ways we help: build the school's AI teaching system, and run
//            AI winter/summer camps.
//   details — three components of "build the AI teaching system": the 思维印记
//            evaluation product (shown as a real dark interface mockup in the
//            page), the AI-native course system, and teacher training.
//
// 思维印记 is represented faithfully, matching how it describes itself on
// apps/site (HomePage / EvaluationPage): an AI tool (like an agent workspace)
// with two special layers — interactive 思维工具卡 (thinking tool-cards) and
// 过程评估 (process evaluation). It evaluates the thinking process; the
// finished work stays the student's own. The cognitive model splits "using AI
// well" into two faces (生成式驾驭 🚀 / 批判式防护 🛡️) across ten dimensions;
// growth is read on a four-level SOLO scale (L1–L4); the evaluation is private
// to the student and offered, never pushed.
//
// Plain, warm, concrete wording — no jargon, no deadlines, no info sessions
// (说明会), no antithesis ("不是…而是" / not-X-but-Y).

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "合作", en: "Partnership" },
  title: {
    zh: "为学校提供 AI 时代的转型方案。",
    en: "AI-native transformation solutions for schools.",
  },
  sub: {
    zh: "如果一所学校也认同这样的学习方式——重视孩子怎么想问题,也重视他给出什么答案——我们可以陪学校一起,把 AI 踏踏实实地带进日常教学里。",
    en: "If a school shares this vision of learning — caring how a child thinks as much as what answer they hand in — we work alongside the school to bring AI into everyday teaching in a grounded, thoughtful way.",
  },
};

/* ---- 1 · 理念 (our philosophy) ------------------------------------------ */
export const philosophy: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  beliefs: { title: Bilingual; body: Bilingual }[];
} = {
  eyebrow: { zh: "理念", en: "Our philosophy" },
  title: {
    zh: "把 AI 认真地带进教学,是这个时代绕不开的事。",
    en: "Bringing AI into teaching, thoughtfully, is something no school can sidestep now.",
  },
  body: [
    {
      zh: "AI 已经在孩子手里了。他们用它查资料、写初稿、找思路——问题不在于挡不挡得住,而在于学校能不能帮他们学会,用一种真正能长出思考力的方式去用它。",
      en: "AI is already in children's hands. They use it to look things up, draft, and find a way in — the question is whether school helps them learn to use it in a way that actually grows their thinking.",
    },
    {
      zh: "我们把这件事叫做「AI 原生学习」(AI-Native Learning)。在这样的学习里,孩子练的是带着 AI 一起解决真实问题的能力,同时守住自己的判断力。该被看重的东西,慢慢从「交出了什么答案」,挪向「他是怎么想到这里的」。",
      en: "We call this AI-Native Learning. In it, a child practices solving real problems alongside AI while keeping their own judgment — and what deserves attention slowly shifts from the answer handed in toward how the child got there.",
    },
    {
      zh: "这不是加一门课、买一套工具就能完成的转变。它关乎课堂怎么设计、老师怎么带、学生的成长怎么被看见。我们愿意陪着认同这件事的学校,一步步把它落到日常里。",
      en: "This isn't a change you finish by adding one class or buying one tool. It touches how lessons are designed, how teachers guide, and how a student's growth is seen. We walk this out, step by step, with schools who believe in it.",
    },
  ],
  beliefs: [
    {
      title: { zh: "AI 会放大思考,也会替代思考", en: "AI can amplify thinking, or replace it" },
      body: {
        zh: "同一个工具,可以让一个孩子想得更深,也可以让他停止思考。差别不在工具,在怎么用——这正是可以教、也值得教的。",
        en: "The same tool can make a child think harder or stop thinking. The difference lies in how it's used — and that can be taught, and is worth teaching.",
      },
    },
    {
      title: { zh: "判断力要在真实任务里练", en: "Judgment is built on real tasks" },
      body: {
        zh: "核实一个说法、接住一个反例、分清哪些是自己的判断——这些能力,只有在孩子做自己真正在乎的题目时,才练得出来。",
        en: "Checking a claim, facing a counter-example, telling your own judgment apart from the machine's — these grow only when a child works on a task they genuinely care about.",
      },
    },
    {
      title: { zh: "成长要被看见", en: "Growth has to be visible" },
      body: {
        zh: "AI 时代最难看清的,是孩子的思考力到底在不在长。学校需要一种方式,把这条看不见的曲线变得看得见。",
        en: "The hardest thing to see in the AI era is whether a child's thinking is really growing. A school needs a way to make that invisible curve visible.",
      },
    },
  ],
};

/* ---- 2 · 方案 overview: two kinds of solutions -------------------------- */
export const solutionsIntro: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "方案", en: "Our solution" },
  title: { zh: "两种合作方式。", en: "Two ways we work with a school." },
  sub: {
    zh: "一种是长期的——陪学校把 AI 教学体系搭起来;一种是集中的——我们为学校的学生带一期沉浸式的 AI 营。",
    en: "One is long-term — helping the school build its AI teaching system; one is concentrated — an immersive AI camp we run for the school's students.",
  },
};

export interface SolutionItem {
  title: Bilingual;
  body: Bilingual;
}

export interface Solution {
  tag: string;
  title: Bilingual;
  body: Bilingual;
  /** Sub-items shown as small cards under the body. Empty for solutions that
   *  are better described in a single paragraph (e.g. the camps). */
  items: SolutionItem[];
}

export const solutions: Solution[] = [
  {
    tag: "①",
    title: { zh: "帮学校搭建 AI 教学体系", en: "Build the school's AI teaching system" },
    body: {
      zh: "从课程、工具、老师三个方面一起入手,帮学校把 AI 真正用进日常教学里。三块内容会在下面逐一展开。",
      en: "We work on the courses, the tools, and the teachers together, so AI genuinely becomes part of everyday teaching. Each of the three is laid out below.",
    },
    items: [
      {
        title: { zh: "思维印记 · AI 评估工具", en: "Mind imprint · an AI evaluation tool" },
        body: {
          zh: "我们自己的 AI 评估产品,持续看到学生真实的思考力是怎么成长的。",
          en: "Our own AI evaluation product, keeping track of how students' real thinking ability grows.",
        },
      },
      {
        title: { zh: "AI 原生课程体系", en: "AI-native course system" },
        body: {
          zh: "一套以真实思考为核心、可与学校共同设计的课程。",
          en: "A course system built around real thinking, co-designed with the school.",
        },
      },
      {
        title: { zh: "教师培训", en: "Teacher training" },
        body: {
          zh: "带老师用好 AI、上好这些课、看懂思维印记的评估。",
          en: "Helping teachers use AI well, teach these courses, and read the mind-imprint evaluations.",
        },
      },
    ],
  },
  {
    tag: "②",
    title: { zh: "AI 冬夏令营", en: "AI winter & summer camps" },
    body: {
      zh: "我们为学校的学生设计并带一期沉浸式的 AI 营——集中的几天里,孩子们围着一个真实问题,和 AI 一起动手想、动手做,练的是怎么提问、怎么核查、怎么把一个想法做成看得见的成果。营期结束时,每个孩子都能讲清楚自己是怎么想通这件事的。",
      en: "We design and run an immersive AI camp for a school's students. Over a few concentrated days, kids work through a real question together, thinking and building alongside AI — practicing how to ask good questions, check what AI tells them, and turn an idea into something real. By the end, every child can walk you through how they actually worked it out.",
    },
    items: [],
  },
];

/* ---- 3a · details: 思维印记 (mind imprint) ------------------------------ */
export const miIntro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual[] } = {
  eyebrow: { zh: "方案详解 · 思维印记", en: "In detail · mind imprint" },
  title: {
    zh: "一款只评估思考过程的 AI 工具。",
    en: "An AI tool that evaluates only the thinking process.",
  },
  body: [
    {
      zh: "思维印记是一个 AI 工具,形态有点像一个带 AI 助理的工作台。学生带着自己真实的任务进来——一篇作文、一个课题、一份报告——在和 AI 协作的过程中,被引导做更结构化的思考;这整个过程会被完整记录下来,再被评估。",
      en: "Mind imprint is an AI tool that works something like a workspace with an AI assistant built in. A student brings in a real task — an essay, a project, a report — and while working with the AI, is guided toward more structured thinking. The whole process is recorded, and then evaluated.",
    },
    {
      zh: "它有两层独有的能力。第一层是互动的「思维工具卡」——在合适的时刻,AI 会请学生停下来,自己先想一步。第二层是「过程评估」——评估看的是学生这一路是怎么和 AI 一起把问题想清楚的:有没有去核实一个说法的来源,撞到一个反例时是绕开还是认真接住,哪些地方是自己的判断、哪些地方是直接照搬 AI 的话。",
      en: "It has two special layers. The first is interactive thinking tool-cards — at the right moment, the AI asks the student to pause and think for themselves. The second is process evaluation — it looks at how the student actually worked through the problem with AI: whether they checked where a claim came from, whether they faced a counter-example head-on or dodged it, and which parts were the student's own judgment versus copied straight from the AI.",
    },
  ],
};

/** Caption shown under the interface mockup. */
export const miShotCaption: Bilingual = {
  zh: "思维印记的工作区与评估界面——左边和 AI 一次一步地把问题想深,右边的过程树记录每一步,最后长成一份只给学生本人看的思维印记。",
  en: "The mind-imprint workspace and evaluation screen — on the left, thinking a question through with AI one step at a time; on the right, a process tree recording every step; and finally an imprint seen only by the student.",
};

export const miAfterShot: Bilingual[] = [
  {
    zh: "一篇漂亮的作文,可能是 AI 一键生成的,也可能是学生认真想了很久的,单看成品,分数分不出这两者的差别。过程能看出这个差别——这也是为什么它对学校有用:它能补上一份成绩单看不到的那部分,让老师和学校真正看清楚,AI 时代的孩子,思考能力到底在不在长。",
    en: "A polished essay could be one AI click, or it could be hours of real thought — looking at the finished piece alone, a grade can't tell the two apart. The process shows the difference. That's exactly why it's useful for a school: it fills in the part a report card alone can't show, so teachers and the school can actually see whether a child's thinking ability is genuinely growing in the AI era.",
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
    zh: "我们把「善用 AI」拆成一枚硬币的两面——主动驾驭 AI,与批判地守住自己的判断——再往下分成十个具体维度,逐一评估。",
    en: "We split \"using AI well\" into two sides of one coin — actively driving AI, and critically guarding your own judgment — and evaluate ten concrete dimensions underneath.",
  },
  faces: [
    {
      face: "drive",
      icon: "🚀",
      title: { zh: "生成式驾驭", en: "Generative drive" },
      sub: {
        zh: "主动驾驭 AI,把它变成放大思考的工具",
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
        zh: "不被 AI 俘获,守住判断与诚信",
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
    zh: "这四个阶段来自 SOLO 分类学,衡量的是理解的深浅——从零散的单点,一直到能融会贯通、迁移应用,和「对错」无关。",
    en: "The four stages come from the SOLO taxonomy. They measure how deep a student's understanding goes — from scattered single points to connected, transferable mastery — not right or wrong.",
  },
  levels: [
    { code: "L1", name: { zh: "萌芽", en: "Emerging" }, body: { zh: "只做到一个孤立的点,还没有联系起来。", en: "One isolated point, not yet connected to anything else." } },
    { code: "L2", name: { zh: "发展中", en: "Developing" }, body: { zh: "做到了几个点,但还没有连成一片。", en: "Several points, but not yet joined up." } },
    { code: "L3", name: { zh: "熟练", en: "Proficient" }, body: { zh: "把这些要素连成了结构,能主动运用。", en: "The pieces are linked into a working structure the student applies actively." } },
    { code: "L4", name: { zh: "卓越", en: "Advanced" }, body: { zh: "能迁移、能反思,超出了当下这一次任务。", en: "The student transfers and reflects on it, going beyond this one task." } },
  ],
};

export interface Principle {
  title: Bilingual;
  body: Bilingual;
}

export const miPrinciples: { eyebrow: Bilingual; title: Bilingual; items: Principle[] } = {
  eyebrow: { zh: "评估的原则", en: "Principles behind the evaluation" },
  title: { zh: "评估本身,也守着几件坚持。", en: "The evaluation keeps its own set of principles." },
  items: [
    {
      title: { zh: "只给学生本人看", en: "Private to the student" },
      body: {
        zh: "思维印记是一面私密的成长镜子,只照给学生自己看,不用来排名或比较。",
        en: "The mind imprint is a private mirror for growth, seen only by the student — never used for ranking or comparison.",
      },
    },
    {
      title: { zh: "评估永远用最强的模型", en: "Always the strongest model" },
      body: {
        zh: "日常陪练可以用更轻量的模型,但一到评估这一步,我们始终用最强的那一个,保证判断可靠。",
        en: "Everyday coaching can run on a lighter model, but for the evaluation itself, we always use the very best one, so the judgment holds up.",
      },
    },
    {
      title: { zh: "安静地提供,不会推着学生看", en: "Offered, never pushed" },
      body: {
        zh: "结果安静地出现——没有徽章、不会自动弹开、也没有排行榜,什么时候点开来看,由学生自己决定。",
        en: "Results appear quietly — no badges, no auto-open, no leaderboard. The student decides if and when to look.",
      },
    },
  ],
};

export const miWhySchool: { eyebrow: Bilingual; title: Bilingual; body: Bilingual } = {
  eyebrow: { zh: "对学校的价值", en: "Why it matters to a school" },
  title: { zh: "补上一块一直很难看到的东西。", en: "It fills in something that's always been hard to see." },
  body: {
    zh: "对一所学校来说,这套评估补上了一块一直很难看到的东西:学生在 AI 时代,思考能力到底是不是在真实地成长。它把过程变成可以看见、可以长期追踪的数据,给老师的判断提供更多依据,也帮学校更早发现哪些孩子已经在善用 AI、哪些孩子还需要多带一带。",
    en: "For a school, this evaluation fills in something that's always been hard to see: whether a student's thinking ability is genuinely growing in the AI era. It turns the process into something visible and trackable over time, giving teachers more to go on and helping a school notice earlier which students are already using AI well, and which ones need more guidance.",
  },
};

/* ---- 3b · details: 课程体系 (course system) ----------------------------- */
export interface CourseModule {
  code: string;
  name: Bilingual;
  body: Bilingual;
}

export const courseSystem: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  modules: CourseModule[];
  closing: Bilingual;
} = {
  eyebrow: { zh: "方案详解 · 课程体系", en: "In detail · course system" },
  title: {
    zh: "一套 AI 原生、以真实思考为核心的课程。",
    en: "An AI-native course system built around real thinking.",
  },
  body: [
    {
      zh: "我们带给学校的,是一套围绕真实问题、以讨论驱动的课程。课上不背标准答案,而是让学生带着 AI 去调查、去辩论、去动手做——把「和 AI 一起想清楚一件事」这件本事,拆成可以一节一节练的内容。",
      en: "What we bring is a course system built around real questions and driven by discussion. Instead of memorizing set answers, students investigate, debate, and build alongside AI — turning \"thinking something through with AI\" into a skill practiced lesson by lesson.",
    },
    {
      zh: "整套课程按几个模块组织,可以按学校的年级、学生和培养目标来取用、拼接。",
      en: "The whole system is organized into a few module families, which can be drawn from and combined to fit the school's grades, students, and goals.",
    },
  ],
  modules: [
    {
      code: "01",
      name: { zh: "信息素养", en: "Information literacy" },
      body: {
        zh: "溯源、横向核查、判断可信度——在 AI 什么都能生成的年代,先学会分辨真假。",
        en: "Sourcing, lateral reading, judging credibility — learning to tell true from false in an age when AI can generate anything.",
      },
    },
    {
      code: "02",
      name: { zh: "知识工具与思辨", en: "Thinking tools & reasoning" },
      body: {
        zh: "论证怎么拆、反例怎么接、让步段怎么写——一套可以反复用的思考工具。",
        en: "How to break down an argument, take on a counter-example, write a concession — a set of thinking tools students use again and again.",
      },
    },
    {
      code: "03",
      name: { zh: "五大知识领域 (TOK)", en: "Five domains of knowing (TOK)" },
      body: {
        zh: "从不同学科看同一个问题,理解知识是怎么来的、又有哪些边界。",
        en: "Looking at one question across disciplines — understanding where knowledge comes from and where its limits lie.",
      },
    },
    {
      code: "04",
      name: { zh: "AI 协作与伦理", en: "AI collaboration & ethics" },
      body: {
        zh: "怎么和 AI 分工、怎么署名、哪些事该由自己拿主意——用得好也用得正。",
        en: "How to divide the work with AI, how to attribute it, and what to decide for yourself — using it both well and honestly.",
      },
    },
    {
      code: "05",
      name: { zh: "元认知与反身", en: "Metacognition & reflection" },
      body: {
        zh: "回头看自己是怎么想的、哪里被 AI 带着走了——把反思变成习惯。",
        en: "Looking back at how you thought, and where AI carried you along — making reflection a habit.",
      },
    },
    {
      code: "06",
      name: { zh: "AI 与产品 · 建造", en: "AI & building" },
      body: {
        zh: "计算思维、和 AI 一起动手、从一个问题做到一个原型——把想法变成看得见的东西。",
        en: "Computational thinking, building with AI, going from a problem to a prototype — turning an idea into something you can see.",
      },
    },
  ],
  closing: {
    zh: "这套课程不是一份现成的教材直接搬进课堂。我们会和学校一起,按学生的实际情况共同设计,再落到每一节课里。",
    en: "This isn't a ready-made textbook dropped into the classroom. We co-design it with the school around the students' real situation, then bring it down into each lesson.",
  },
};

/* ---- 3c · details: 教师培训 (teacher training) -------------------------- */
export const teacherTraining: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  items: Principle[];
  closing: Bilingual;
} = {
  eyebrow: { zh: "方案详解 · 教师培训", en: "In detail · teacher training" },
  title: {
    zh: "带着老师,一起把这些课上起来。",
    en: "Getting teachers ready to teach this.",
  },
  body: [
    {
      zh: "再好的课程和工具,最后都要靠老师带进课堂。所以教师培训是这套方案里很实的一环——它是给学校老师的一套实操计划,帮他们在 AI 时代把课上得更有底气。",
      en: "Even the best course and the best tool still reach students through a teacher. So teacher training is a very practical part of this — a hands-on program for the school's staff, helping them teach with confidence in the AI era.",
    },
  ],
  items: [
    {
      title: { zh: "在教学里用好 AI", en: "Use AI well in teaching" },
      body: {
        zh: "怎么用 AI 备课、设计讨论、给反馈,又不让它替学生把该想的都想了——老师先自己走一遍。",
        en: "How to use AI to prepare, design discussion, and give feedback, without letting it do the thinking students should do — teachers walk it themselves first.",
      },
    },
    {
      title: { zh: "带得动这些课", en: "Run these courses" },
      body: {
        zh: "以真实问题、讨论驱动的课堂,和讲授式很不一样。我们带老师练怎么抛问题、怎么接学生的回应、怎么把讨论收回来。",
        en: "A question-driven, discussion-based classroom runs very differently from a lecture. We help teachers practice posing questions, taking up student responses, and bringing a discussion back together.",
      },
    },
    {
      title: { zh: "看懂思维印记", en: "Read the mind imprint" },
      body: {
        zh: "一份过程评估摊在面前,老师能读出学生强在哪、卡在哪,再决定下一步怎么带——把评估变成教学的依据。",
        en: "With a process evaluation in front of them, teachers can read where a student is strong and where they're stuck, then decide how to guide next — turning the evaluation into something teaching can act on.",
      },
    },
  ],
  closing: {
    zh: "培训会按学校的节奏来,可以是集中的工作坊,也可以是一学期里持续的陪伴。",
    en: "The training runs at the school's pace — as a concentrated workshop, or as ongoing support across a term.",
  },
};

/* ---- 4 · Closing CTA ---------------------------------------------------- */
export const closingCta: { title: Bilingual; sub: Bilingual; ctaLabel: Bilingual; ctaHref: string } = {
  title: { zh: "想了解合作，留下联系方式。", en: "Interested in partnering? Leave your contact." },
  sub: {
    zh: "告诉我们学校的大致情况和想合作的方向,我们会尽快联系你。",
    en: "Tell us a bit about your school and what kind of partnership you have in mind — we'll get back to you soon.",
  },
  ctaLabel: { zh: "留下联系方式", en: "Leave your contact" },
  ctaHref: "/contact",
};
