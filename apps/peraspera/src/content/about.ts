// Per Aspera — about page (/about, /en/about) content. zh is the source of
// truth; en is an idiomatic (not literal) translation.
//
// Rewritten 2026-07-05 per user feedback (docs/superpowers/plans/
// 2026-07-05-peraspera-refactor.md, task R8):
//   - The hero used to be a riddle sentence ("两个人，一个执念：能力无法代办。").
//     Replaced with a plain, real heading: "关于我们" / "About us", plus a
//     short warm subhead that plainly says what we're building and why.
//   - Founder roles corrected: 陈玉洁 is CEO · co-founder · head of the
//     education research institute; 侯煜欣 is co-founder · head of product.
//   - The founders' letter is trimmed to four short, genuine paragraphs —
//     no riddles, no invented business terms (success fee / refund policy /
//     14-week guarantee) that aren't backed up anywhere else on the site.
//   - FAQ reviewed against the current site (coaching.ts / academy.ts /
//     research.ts): dropped the old "冲刺营" naming, the time-zone item
//     (academy.ts explicitly avoids that topic), the invented refund policy,
//     and the old literal "record a video, write a letter" enrollment steps
//     that mirrored Astra Nova's own process. The "how to start" FAQ now
//     reframes to what's actually true: leave your contact and we'll reach
//     out (links to /contact) — no 说明会, no deadlines.
//
// Plain, warm, concrete wording throughout — no jargon, no lorem, and no
// antithesis ("不是…而是" / not-X-but-Y).

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "Per Aspera", en: "Per Aspera" },
  title: { zh: "关于我们", en: "About us" },
  sub: {
    zh: "我们是两位教育者，一起做 Per Aspera——帮孩子准备申请像 Astra Nova 这样看重思考方式的学校，也为认同这套学习方式的家庭开设课程。",
    en: "We're two educators building Per Aspera together — helping children get ready to apply to schools like Astra Nova that care how they think, and offering courses for families who share this way of learning.",
  },
};

/* ---- Founders ----------------------------------------------------------- */
export const foundersIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "团队", en: "Team" },
  title: { zh: "创始人", en: "Founders" },
};

export interface Founder {
  name: Bilingual;
  role: Bilingual;
  bio: Bilingual;
  photoSlot: Bilingual;
}

export const founders: Founder[] = [
  {
    name: { zh: "陈玉洁", en: "Yujie Chen" },
    role: {
      zh: "CEO · 联合创始人 · 教育研究院负责人",
      en: "CEO · Co-founder · Head of the Education Research Institute",
    },
    bio: {
      zh: "多年联合国 ESG 与气候课程、IB 课程辅导经验。她创立了「思维印记」这套方法——在真实国际课堂里长出来的三十多门批判性思维课，以及产品背后的过程评估标准。",
      en: "Years of experience with UN ESG and climate programs, and with IB coaching. She created the Mind Imprint approach — 30-plus critical-thinking courses grown in real international classrooms, and the process-assessment rubric behind the product.",
    },
    photoSlot: { zh: "照片位", en: "Photo placeholder" },
  },
  {
    name: { zh: "侯煜欣", en: "Yuxin Hou" },
    role: {
      zh: "联合创始人 · 产品负责人",
      en: "Co-founder · Head of Product",
    },
    bio: {
      zh: "连续创业者。AI 思维、产品思维、计算思维教育者。带领「思维印记」的产品化——工作台的交互设计、评估工程，以及对 AI 的调校。",
      en: "A serial founder, and an educator in AI, product, and computational thinking. He leads the productization of Mind Imprint — the workbench interaction, the assessment engineering, and the tuning of the AI.",
    },
    photoSlot: { zh: "照片位", en: "Photo placeholder" },
  },
];

/* ---- Founders' letter ----------------------------------------------------
   Trimmed to four short, plain paragraphs — no riddles, no business terms
   (success fee, refund policy, guaranteed timelines) that aren't stated
   anywhere else on the site. */
export const letterIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "创始人的话", en: "In their own words" },
  title: { zh: "创始人信", en: "A Letter from the Founders" },
};

export const letter: {
  paragraphs: Bilingual[];
  signatureLine: Bilingual;
  signoff: Bilingual;
} = {
  paragraphs: [
    { zh: "你好，我们是陈玉洁和侯煜欣。", en: "Hello. We're Yujie Chen and Yuxin Hou." },
    {
      zh: "陈玉洁在国际课堂里教了多年批判性思维和信息素养，带孩子拆解论证、追问一个说法到底从哪里来。侯煜欣做过几次创业，也一直在教 AI 思维和产品思维，相信最好的学习是从一个真实问题动手，做出一个真正的东西。",
      en: "Yujie Chen has spent years teaching critical thinking and information literacy in international classrooms — helping kids take an argument apart and ask where a claim actually comes from. Yuxin Hou has founded a few companies, and has spent time teaching AI thinking and product thinking, believing the best kind of learning starts from a real problem and ends with something real, made by hand.",
    },
    {
      zh: "我们决定一起做 Per Aspera，是因为认真研究了 Elon Musk 教育谱系里的 Astra Nova 高中之后，越看越确认一件事：它看重的是孩子面对一道没有标准答案的问题时怎么推理、被追问时敢不敢改口。这恰好是 AI 时代最不容易贬值的能力。",
      en: "We decided to build Per Aspera together after digging deep into Astra Nova, the high school from Elon Musk's education lineage. The more we looked, the more we became sure of one thing: what it values is how a child reasons through a question with no model answer, and whether they're willing to change their mind when someone pushes back. That happens to be one of the abilities the AI era is least likely to devalue.",
    },
    {
      zh: "所以我们把这套学习方式做成了一个产品——「思维印记」。它装着我们的课程、两个工作台，还有一套过程评估。在它之上，我们做这些服务：帮准备申请的家庭做辅导，给认同这套学习方式的家庭和学校开课程。这些能力才是真正留下的东西——如果孩子最终被 Astra Nova 录取，太好了；如果没有，这些能力也已经留在他身上。",
      en: "So we turned this way of learning into a product — Mind Imprint. It carries our courses, two workbenches, and a process assessment. On top of it we build the services: coaching for families preparing to apply, and courses for families and schools who share this way of learning. The ability itself is what actually stays with a child. If they end up at Astra Nova, wonderful. If not, the ability is already theirs to keep.",
    },
  ],
  signatureLine: {
    zh: "有任何问题，随时留言给我们。",
    en: "If you have any questions, feel free to reach out any time.",
  },
  signoff: {
    zh: "—— 陈玉洁 · 侯煜欣　2026 年夏",
    en: "— Yujie Chen · Yuxin Hou, Summer 2026",
  },
};

/* ---- Name origin ---------------------------------------------------------- */
export const nameOrigin: {
  eyebrow: Bilingual;
  title: Bilingual;
  prefix: Bilingual;
  motto: string;
  body: Bilingual;
} = {
  eyebrow: { zh: "品牌", en: "The name" },
  title: { zh: "名字的由来", en: "Where the Name Comes From" },
  prefix: { zh: "拉丁格言", en: "The Latin motto" },
  motto: "ad astra per aspera",
  body: {
    zh: "——「循此苦旅，以达星辰」。Astra Nova 与 Ad Astra 取了「星辰」，我们取「苦旅」：能力没有捷径，思维的成长必须亲身穿越。",
    en: " means “through hardship, to the stars.” Astra Nova and Ad Astra both took the stars for their names. We take the hardship: there's no shortcut to real ability — the growth of a mind has to be walked in person.",
  },
};

/* ---- FAQ ------------------------------------------------------------------
   Reviewed against the current site: dropped the old "冲刺营" naming, the
   time-zone item, the invented refund policy, and the enrollment steps that
   literally mirrored Astra Nova's own application (video + letter). "How to
   start" now reframes to what's actually true — leave your contact info and
   we'll reach out. */
export const faqIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "常见问题", en: "FAQ" },
  title: { zh: "常见问题", en: "Frequently Asked Questions" },
};

export interface Faq {
  q: Bilingual;
  a: Bilingual;
}

export const faq: Faq[] = [
  {
    q: {
      zh: "Per Aspera 与 Astra Nova School 是什么关系？",
      en: "How is Per Aspera related to Astra Nova School?",
    },
    a: {
      zh: "没有任何关系。我们是独立机构，未获 Astra Nova School 授权、认可或合作。我们研究它、认同它看重思考方式的理念，仅此而已。（这句话也出现在页脚。）",
      en: "None at all. We're an independent organization — not authorized, endorsed, or partnered with Astra Nova School in any way. We study it, and share its belief in valuing how a child thinks; that's all. (The same statement appears in the footer.)",
    },
  },
  {
    q: { zh: "你们能保证录取吗？", en: "Can you guarantee admission?" },
    a: {
      zh: "不能，任何人都不能——Astra Nova 一届只招几十名新生，最终决定权在学校。我们能保证的是：两个月的辅导里，孩子的思考、表达和协作能力会有看得见的成长。收费方式会在联系你之后当面说清楚。",
      en: "No — and no one can. Astra Nova admits only a few dozen new students a year, and the final call is the school's alone. What we can promise is that over two months of coaching, your child's reasoning, expression, and collaboration will show real, visible growth. We'll walk you through pricing directly once we're in touch.",
    },
  },
  {
    q: {
      zh: "为什么说应试没用？那你们最后帮不帮改材料？",
      en: "Why doesn't test prep work? Do you still help polish the materials?",
    },
    a: {
      zh: "Astra Nova 的思辨题没有标准答案，面试也会当场追问，任何背下来的东西都容易露馅。我们仍然会帮忙打磨材料：最后阶段会一起看视频表达和家长信写得清不清楚，打磨的是表达的清楚程度，思考的内容始终是孩子自己的。",
      en: "Astra Nova's reasoning prompts have no model answer, and the interview probes with live follow-up questions — anything rehearsed tends to show. We do still help with the materials: in the final stage, we go over how clearly the video and the parent letter come across. What we polish is how clearly it's expressed; the thinking inside stays the child's own, start to finish.",
    },
  },
  {
    q: {
      zh: "课程什么语言？孩子英语一般怎么办？",
      en: "What language is instruction in? What if my child's English is average?",
    },
    a: {
      zh: "思辨内容用中文教透，理解深度优先；每周会有固定的全英文小组讨论，面试模拟也全程英文。英语一般但思维强的孩子，恰恰是我们最想教的——表达可以练，好奇心和思考力更难教。",
      en: "We teach the reasoning content in Chinese, for depth of understanding first, with a fixed weekly all-English group discussion and fully English mock interviews. A child with average English but strong thinking is exactly who we want to teach — expression can be trained; curiosity and reasoning are much harder to teach.",
    },
  },
  {
    q: { zh: "孩子几岁可以来？", en: "What ages do you work with?" },
    a: {
      zh: "申请辅导跟着 Astra Nova 招生的年龄段来，大致是 11 到 18 岁。学院的课程项目不设年龄门槛，只要孩子和家庭认同这套学习方式，都欢迎来上课。",
      en: "Application coaching follows Astra Nova's own admission ages, roughly 11 to 18. Our academy courses don't have an age limit — as long as this way of learning resonates with your family, your child is welcome.",
    },
  },
  {
    q: { zh: "怎么开始？", en: "How do we get started?" },
    a: {
      zh: "先留下联系方式，告诉我们孩子的大致情况和你关心的问题，我们看到后会尽快联系你，一起聊聊适合的下一步。",
      en: "Leave your contact info and tell us a bit about your child and what's on your mind — we'll see it and get back to you soon to talk through what fits best.",
    },
  },
];
