// Per Aspera — home page (/) content. zh is the source of truth; en mirrors it.
// The page renders ONE language per locale via t(lang, zh, en) — nothing is shown
// in both languages at once (fixes the old belief-cards bug where zh text leaked
// onto the /en/ page). Plain, parent-legible wording throughout: no jargon, no
// deadlines, no antithesis (不是…而是 / not-X-but-Y).

import type { Bilingual } from "./site";

/* ---- 1 · Hero -------------------------------------------------------------- */
export const hero: {
  motto: string;
  headline: Bilingual;
  /** the accented fragment inside the headline, wrapped in .em */
  headlineEm: Bilingual;
  sub: Bilingual;
  ctaPrimary: { label: Bilingual; href: string };
  ctaSecondary: { label: Bilingual; href: string };
} = {
  motto: "Per Aspera",
  headline: {
    zh: "和 AI 一起，培养会独立思考、也能把问题做成真东西的年轻人。",
    en: "Alongside AI, we raise young people who think for themselves and turn problems into real things.",
  },
  headlineEm: { zh: "把问题做成真东西", en: "turn problems into real things" },
  sub: {
    zh: "教育正在走向 AI 原生学习。我们陪孩子练两样本事：遇到任何说法都会自己先想一想，也能和 AI 一起把一个真实的问题做出来。",
    en: "Education is moving toward AI-Native Learning. We help children build two abilities: thinking any claim through for themselves, and working with AI to turn a real problem into something that exists.",
  },
  ctaPrimary: {
    label: { zh: "了解更多", en: "Learn more" },
    href: "#mission",
  },
  ctaSecondary: {
    label: { zh: "联系我们", en: "Contact us" },
    href: "/contact",
  },
};

/* ---- 2 · Mission ----------------------------------------------------------- */
export const mission: {
  label: string;
  heading: Bilingual;
  body: Bilingual;
} = {
  label: "AI-Native Learning",
  heading: {
    zh: "教育正在走向 AI 原生学习。",
    en: "Education is moving toward AI-Native Learning.",
  },
  body: {
    zh: "我们把这次转变叫作 AI 原生学习（AI-Native Learning，简称 ANL）。越来越多的学生开始走进真实的公司里学习、甚至工作，也更早地动手做东西、把想法变成真实的作品。在一个个真实的问题里，他们长出和 AI 一起把事情解决掉的能力。我们相信这就是下一代教育该有的样子，这也是我们存在的理由。",
    en: "We call this shift AI-Native Learning (ANL). More and more students are starting to learn — and even work — inside real companies, and to build and create earlier, turning ideas into things that actually exist. Working on real problems, they grow the ability to solve them alongside AI. We believe this is what the next generation of education looks like, and it is why we exist.",
  },
};

/* ---- 3 · Our core product (思维印记) --------------------------------------- */
export interface ProductPart {
  name: Bilingual;
  body: Bilingual;
}

export const product: {
  label: Bilingual;
  heading: Bilingual;
  headingEm: Bilingual;
  body: Bilingual;
  parts: ProductPart[];
  cta: Bilingual;
  href: string;
} = {
  label: { zh: "核心产品", en: "Our core product" },
  heading: { zh: "思维印记，我们的核心产品。", en: "Mind Imprint, our core product." },
  headingEm: { zh: "思维印记", en: "Mind Imprint" },
  body: {
    zh: "这是一款 AI 教育产品，把我们整套学习方式都装了进去。它由三部分组成。",
    en: "It's one AI education product that carries our whole way of learning, and it's made of three parts.",
  },
  parts: [
    {
      name: { zh: "AI 互动课", en: "AI-assisted courses" },
      body: {
        zh: "真人小班、全程在线，AI 一路在旁边陪着上；分思辨和产品思维两条线，没有录播视频。",
        en: "Live, small-group, fully online, with AI alongside throughout — two threads, critical thinking and product thinking, and no recorded video.",
      },
    },
    {
      name: { zh: "两个工作台", en: "Two workbenches" },
      body: {
        zh: "一个让孩子和 AI 一起完成真实任务的互动界面；在关键的时刻，AI 会把该想的那一步交回给孩子。",
        en: "An interactive space where a child works a real task together with AI; at the key moment, the AI hands the thinking step back to the child.",
      },
    },
    {
      name: { zh: "过程评估", en: "Process assessment" },
      body: {
        zh: "把孩子这一路是怎么想的记录下来，慢慢长成一份只给他自己看的成长记录。",
        en: "It records how a child got there and grows it into a private record made just for them.",
      },
    },
  ],
  cta: { zh: "了解思维印记", en: "About Mind Imprint" },
  href: "/mind-imprint",
};

/* ---- 4 · Three services built on the product ------------------------------- */
export const offerIntro: { label: Bilingual; heading: Bilingual } = {
  label: { zh: "我们的服务", en: "Our services" },
  heading: {
    zh: "三种服务，都建立在思维印记之上。",
    en: "Three services, all built on Mind Imprint.",
  },
};

export interface Offer {
  name: Bilingual;
  body: Bilingual;
  cta: Bilingual;
  href: string;
}

export const offers: Offer[] = [
  {
    name: { zh: "申请辅导", en: "Application coaching" },
    body: {
      zh: "一对一陪学生和家长，准备申请那些为 AI 时代而建的顶尖学校，比如马斯克创办的 Astra Nova；平时的练习就在思维印记里做。",
      en: "One-on-one help for students and parents preparing to apply to top schools built for the AI era — like Elon Musk's Astra Nova — with the practice happening inside Mind Imprint.",
    },
    cta: { zh: "了解申请辅导", en: "Learn about coaching" },
    href: "/coaching",
  },
  {
    name: { zh: "课程项目", en: "Courses" },
    body: {
      zh: "业余时间、全程在线的课程项目，孩子就在思维印记里和 AI 一起上课、动手；还没进入这些学校的家庭也能来学。",
      en: "Part-time, fully online courses where a child learns and builds with AI right inside Mind Imprint — open to families who haven't enrolled in those schools too.",
    },
    cta: { zh: "了解课程项目", en: "See our courses" },
    href: "/academy",
  },
  {
    name: { zh: "学校合作", en: "School partnership" },
    body: {
      zh: "陪认同这个方向的学校，把 AI 踏实地带进日常教学，用的正是思维印记，还有配套的教师培训和课程。",
      en: "We help schools that share this direction bring AI into everyday teaching — with Mind Imprint itself, plus teacher training and courses.",
    },
    cta: { zh: "了解学校合作", en: "Explore partnership" },
    href: "/partnership",
  },
];

/* ---- 5 · What we believe (single language per locale) ---------------------- */
export const beliefsIntro: { label: Bilingual; heading: Bilingual } = {
  label: { zh: "我们相信什么", en: "What we believe" },
  heading: { zh: "我们相信的几件事。", en: "A few things we believe." },
};

export interface Belief {
  /** short thematic tag beside the star mark */
  tag: Bilingual;
  head: Bilingual;
  body?: Bilingual;
}

export const beliefs: Belief[] = [
  {
    tag: { zh: "思考", en: "Thinking" },
    head: {
      zh: "在 AI 什么都能答的时代，怎么想，才是真正要学的东西。",
      en: "When AI can answer anything, how you think is what's really worth learning.",
    },
  },
  {
    tag: { zh: "能力", en: "Ability" },
    head: {
      zh: "真正的能力，得自己长出来，也得看得见。",
      en: "Real ability has to grow from within — and be seen.",
    },
    body: {
      zh: "没有人能替孩子完成一次真实的思考。我们陪着一起练，也把这一路怎么想的记录下来，让成长看得见。",
      en: "No one can think for your child. We practise alongside them and record how they got there, so the growth is visible.",
    },
  },
  {
    tag: { zh: "家庭", en: "Family" },
    head: {
      zh: "家长是教育的另一半。",
      en: "Parents are the other half of education.",
    },
    body: {
      zh: "很多真实的学习，就发生在家里的饭桌上。",
      en: "A lot of real learning happens at the family dinner table.",
    },
  },
  {
    tag: { zh: "主动权", en: "Agency" },
    head: {
      zh: "让孩子握住和 AI 相处的主动权。",
      en: "Keep your child in charge of how they work with AI.",
    },
    body: {
      zh: "会用 AI 的孩子很多，敢对 AI 说“这里不对”的孩子很少，我们想培养后者。",
      en: "Many kids can use AI; few dare to tell it “this is wrong.” We raise the few.",
    },
  },
];

/* ---- 6 · Who we are -------------------------------------------------------- */
export const team: {
  label: Bilingual;
  heading: Bilingual;
  intro: Bilingual;
  members: { name: Bilingual; role: Bilingual }[];
  cta: Bilingual;
  href: string;
} = {
  label: { zh: "团队", en: "Team" },
  heading: { zh: "我们是谁", en: "Who we are" },
  intro: {
    zh: "一位多年深耕国际课程里的思辨教学，一位是连续创业者、常年做 AI 与产品。我们把研究和方法都公开出来。",
    en: "One of us has spent years teaching critical thinking in international classrooms; the other is a serial founder working in AI and product. We publish our research and methods openly.",
  },
  members: [
    {
      name: { zh: "陈玉洁", en: "Yujie Chen" },
      role: {
        zh: "CEO · 联合创始人 · 教育研究院负责人",
        en: "CEO · Co-founder · Head of the education research institute",
      },
    },
    {
      name: { zh: "侯煜欣", en: "Yuxin Hou" },
      role: {
        zh: "联合创始人 · 产品负责人",
        en: "Co-founder · Head of product",
      },
    },
  ],
  cta: { zh: "了解我们", en: "Meet the team" },
  href: "/about",
};

/* ---- 7 · Questions --------------------------------------------------------- */
export const faqIntro: { label: Bilingual; heading: Bilingual } = {
  label: { zh: "常见问题", en: "FAQ" },
  heading: { zh: "几个常见问题。", en: "A few common questions." },
};

export interface Faq {
  q: Bilingual;
  a: Bilingual;
}

export const faqs: Faq[] = [
  {
    q: {
      zh: "Per Aspera 和 Astra Nova 是什么关系？",
      en: "How is Per Aspera related to Astra Nova?",
    },
    a: {
      zh: "我们是一家独立机构，和 Astra Nova 没有任何关联，也不代表它。",
      en: "We are an independent organization. We are not affiliated with Astra Nova and do not represent it.",
    },
  },
  {
    q: { zh: "你们能保证录取吗？", en: "Can you guarantee admission?" },
    a: {
      zh: "不能，任何人都不能。我们能做的，是帮孩子真正把能力练出来。",
      en: "No — and no one can. What we can do is help your child genuinely build their abilities.",
    },
  },
  {
    q: { zh: "孩子多大可以参加？", en: "What ages do you work with?" },
    a: {
      zh: "申请辅导主要面向准备申请这类学校的中学生；课程项目也欢迎更小的孩子，我们会为每个孩子设计合适的计划。",
      en: "Application coaching is mainly for secondary-school students preparing to apply; our courses also welcome younger children, and we design a plan that fits each child.",
    },
  },
];

export const faqMore: { label: Bilingual; contactLabel: Bilingual } = {
  label: { zh: "更多问题看关于页", en: "More questions on the About page" },
  contactLabel: { zh: "联系我们", en: "Contact us" },
};
