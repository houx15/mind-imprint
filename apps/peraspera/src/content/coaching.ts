// Per Aspera — 申请辅导 (application coaching) page (/coaching, /en/coaching)
// content. zh is the source of truth; en is an idiomatic (not literal)
// translation.
//
// This replaces the old "冲刺营" (sprint camp) framing entirely — we call
// this application coaching (申请辅导), full stop. No deadlines, no info
// sessions (说明会), no antithesis ("不是…而是" / not-X-but-Y), no jargon.
// Plain, warm, concrete wording throughout, written the way a normal parent
// talks and a normal parent understands on first read.
//
// Facts about Astra Nova below are drawn from the research long-read at
// src/content/research.ts (an online, WASC-accredited nonprofit school
// founded by the original Ad Astra teaching team; admission is based on a
// recorded reasoning response, a parent letter, and a group interview with
// live follow-up questioning — not grades or standardized tests). This page
// only states the plain, durable facts; the deep detail with citations lives
// at /institute/ad-astra and /institute/schools, which this page links to.

import type { Bilingual } from "./site";

/* ---- Page hero -------------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "申请辅导", en: "Application coaching" },
  title: {
    zh: "帮孩子准备好，去申请一所看重思考方式的学校。",
    en: "We help your child get ready to apply to a school that cares how they think.",
  },
  sub: {
    zh: "以马斯克教育谱系里的 Astra Nova 为例——两个月的时间，我们陪孩子把真实的能力练出来，也陪家长一起准备。",
    en: "Astra Nova, from Elon Musk's education lineage, is our example. Over two months we help your child build real ability — and help you prepare alongside them.",
  },
};

/* ---- 1 · What Astra Nova is ------------------------------------------------- */
export const introSection: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  links: { label: Bilingual; href: string }[];
} = {
  eyebrow: { zh: "认识 Astra Nova", en: "Meet Astra Nova" },
  title: {
    zh: "一所在线学校，看重孩子怎么想问题的方式。",
    en: "An online school that cares how a child thinks.",
  },
  body: [
    {
      zh: "Astra Nova 是一所在线学校，由马斯克教育谱系里的原班教师团队独立创办。它不看成绩单，也不用标准化考试挑学生：孩子提交一段录像，讲清楚自己怎么想一道没有标准答案的问题；家长写一封信，说说家庭在找什么样的教育；接下来是一场小组面试，老师会当场追问,看孩子是不是真的在思考。",
      en: "Astra Nova is an online school, founded by the original teaching team from Elon Musk's education lineage after they went independent. It doesn't ask for a transcript or a standardized test score. Instead, a child records a short video walking through how they reason about a question with no single right answer; a parent writes a letter about what kind of education the family is looking for; then comes a group interview, where teachers ask follow-up questions on the spot to see whether the child is really thinking it through.",
    },
    {
      zh: "学校规模很小,学生来自世界各地,横跨不同时区,面向的是那些愿意自己动脑筋、也敢在别人追问下调整看法的孩子。",
      en: "The school is small, with students joining from around the world across different time zones. It looks for children who are willing to think for themselves — and brave enough to change their mind when someone pushes back.",
    },
  ],
  links: [
    {
      label: { zh: "深入了解 Astra Nova", en: "Learn more about Astra Nova" },
      href: "/institute/ad-astra",
    },
    {
      label: { zh: "同类学校对比", en: "See how similar schools compare" },
      href: "/institute/schools",
    },
  ],
};

/* ---- 2 · Why it's valuable, and why it's hard ------------------------------- */
export const whySection: {
  eyebrow: Bilingual;
  title: Bilingual;
  valuable: { title: Bilingual; body: Bilingual };
  hard: { title: Bilingual; body: Bilingual[] };
} = {
  eyebrow: { zh: "为什么值得，也为什么难", en: "Why it's worth it — and why it's hard" },
  title: {
    zh: "这类学校练的,正是 AI 时代最重要的能力。",
    en: "This kind of school builds exactly the abilities that matter in an AI world.",
  },
  valuable: {
    title: { zh: "为什么值得", en: "Why it's worth it" },
    body: {
      zh: "当 AI 已经能回答几乎所有问题,真正稀缺的是独立思考、清楚表达、和别人一起把问题解决掉的能力,以及愿意跟 AI 一起想、又不把判断力交出去的分寸感。Astra Nova 这类学校的招生标准和日常训练,恰好就是在培养和考察这些能力——所以就算不申请这类学校,这套能力本身也值得孩子拥有。",
      en: "Now that AI can answer almost anything, what's genuinely scarce is the ability to think independently, express yourself clearly, work with others to solve real problems, and collaborate with AI without handing over your own judgment. The admissions bar and everyday training at a school like Astra Nova are built around exactly these abilities — which is why they're worth having whether or not your child ends up applying.",
    },
  },
  hard: {
    title: { zh: "为什么难", en: "Why it's hard" },
    body: [
      {
        zh: "Astra Nova 找的是这样的孩子:面对一道没有标准答案的问题,能讲出自己完整的推理过程;被人当场追问,甚至被反问“如果答案反过来呢”,也愿意认真重新想一遍。",
        en: "Astra Nova looks for a specific kind of child: one who can walk through their own full reasoning on a question with no model answer, and who — when pressed on the spot, or even asked \"what if the answer were the opposite?\" — is willing to genuinely think it through again.",
      },
      {
        zh: "面试是一场真实的对话:老师会顺着孩子的回答继续追问,也会故意抛出反转,看孩子当场是真实的思考,还是背好的台词。",
        en: "The interview is a live conversation: teachers follow up on what a child says, and sometimes deliberately introduce a reversal, to see whether the reaction is genuine reasoning or a memorized line.",
      },
      {
        zh: "这也是为什么背答案、套模板这条路走不通——在真实的追问面前,任何背下来的东西都很容易露出破绽。真正管用的,是让孩子的思考能力本身变强。",
        en: "That's exactly why memorizing answers or following a script doesn't work — real follow-up questioning exposes anything that's rehearsed. What actually helps is making the child's own thinking genuinely stronger.",
      },
    ],
  },
};

/* ---- 3 · What we offer: a two-month, three-stage journey -------------------- */
export const offerIntro: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "我们提供什么", en: "What we offer" },
  title: { zh: "两个月，三个阶段的旅程。", en: "A two-month journey, in three stages." },
  sub: {
    zh: "我们扎扎实实地把能力练出来,再帮孩子把这些能力讲清楚。",
    en: "We build the ability first, genuinely and step by step, then help your child express it clearly.",
  },
};

export interface Ability {
  title: Bilingual;
  body: Bilingual;
}

export const stage1: {
  tag: Bilingual;
  title: Bilingual;
  body: Bilingual;
  abilities: Ability[];
} = {
  tag: { zh: "阶段一 · 打好能力基础", en: "Stage 1 · Build the abilities" },
  title: { zh: "把 Astra Nova 看重的能力,一项一项练出来。", en: "Grow the abilities Astra Nova looks for, one at a time." },
  body: {
    zh: "前几周,我们不碰申请材料,先带孩子练五项能力。每一项都对应着 Astra Nova 招生时真正想看到的东西。",
    en: "In the first few weeks, we don't touch the application at all — we work through five abilities with your child. Each one maps directly onto something Astra Nova is actually trying to see in an applicant.",
  },
  abilities: [
    {
      title: { zh: "信息素养与思辨", en: "Information literacy & critical thinking" },
      body: {
        zh: "练习分辨信息来源、识别夸大和似是而非的说法,把一个观点的论证过程拆开来看。Astra Nova 的思辨题没有标准答案,评的正是孩子怎么想问题的过程。",
        en: "Learning to check where information comes from, spot exaggerated or misleading claims, and take an argument apart to see how it's built. Astra Nova's reasoning prompts have no model answer — what's graded is the reasoning process itself.",
      },
    },
    {
      title: { zh: "问题解决", en: "Problem solving" },
      body: {
        zh: "带孩子把一个复杂问题拆成能一步步动手的小问题,再一步步找出路。这正是 Astra Nova 那类“没有说明书”的思辨题在考察的东西。",
        en: "Helping a child break a complicated problem into smaller pieces they can actually work through, one step at a time. This is exactly what Astra Nova's \"no manual\" reasoning prompts are testing for.",
      },
    },
    {
      title: { zh: "AI 使用", en: "Using AI well" },
      body: {
        zh: "练习怎么和 AI 一起想问题——用它拓展思路、核对事实,但自己保留最后的判断,敢对 AI 说“这里我不同意”。这是面试官很想在孩子身上看到的分寸感。",
        en: "Practicing how to think alongside AI — using it to widen ideas and check facts, while keeping the final judgment for yourself, and being willing to tell it \"I don't think that's right.\" This is exactly the kind of judgment interviewers want to see.",
      },
    },
    {
      title: { zh: "自信表达", en: "Confident expression" },
      body: {
        zh: "练习把自己的想法讲清楚、讲完整,包括在全英文的环境里表达和讨论。面试是当场的对话,讲不清楚,再好的想法也传达不出来。",
        en: "Practicing how to say what you think, clearly and completely — including in English. The interview is a live conversation; even a great idea doesn't land if it can't be expressed.",
      },
    },
    {
      title: { zh: "项目能力", en: "Project ability" },
      body: {
        zh: "带孩子完成一个真实的小项目,从一个想法开始,做出一个真正的成果。这练的是把事情坚持做完、并对结果负责的能力。",
        en: "Guiding a child through one real, small project — from an idea to something they've actually made. This builds the ability to see something through and stand behind the result.",
      },
    },
  ],
};

export const stage2: {
  tag: Bilingual;
  title: Bilingual;
  body: Bilingual;
  points: Bilingual[];
} = {
  tag: { zh: "阶段二 · 模拟与训练", en: "Stage 2 · Mock and train" },
  title: { zh: "在真实的追问里,练出真实的反应。", en: "Practice the real thing, with real follow-up questions." },
  body: {
    zh: "能力练得差不多了,我们开始做贴近真实场景的模拟训练,让孩子提前习惯被追问、被反问、和别人一起讨论的感觉。",
    en: "Once the abilities are in place, we move to realistic practice — so your child gets comfortable being questioned, being pushed back on, and thinking alongside other kids, before it happens for real.",
  },
  points: [
    {
      zh: "反转追问训练:孩子给出一个答案后,我们会像真实面试官一样继续追问,甚至反过来问“如果答案是反的呢”,练习当场把想法重新想一遍。",
      en: "Reversal and follow-up training: after your child gives an answer, we push further — sometimes asking \"what if the answer were the opposite?\" — so they practice genuinely rethinking on the spot.",
    },
    {
      zh: "无领导小组讨论:几个孩子一起讨论一个开放性问题,没有人指定谁来主持,练习怎么在群体里表达观点、倾听别人、也能在合适的时候改变想法。",
      en: "Leaderless group discussion: several kids discuss an open-ended question with no one assigned to lead, practicing how to speak up, listen to others, and change their mind when it makes sense to.",
    },
    {
      zh: "全英文表达:模拟面试和讨论都用英文进行,帮孩子习惯用英文清楚地讲道理,而不只是背句子。",
      en: "Expressing in English: mock interviews and discussions run in English, so a child gets used to reasoning clearly in English — not just reciting memorized sentences.",
    },
  ],
};

export const stage3: {
  tag: Bilingual;
  title: Bilingual;
  body: Bilingual[];
} = {
  tag: { zh: "阶段三 · 准备申请材料", en: "Stage 3 · Prepare the application" },
  title: { zh: "把练出来的思考,讲清楚、交出去。", en: "Put the thinking into a clear, ready application." },
  body: [
    {
      zh: "到这个阶段,孩子已经真正练出了这些能力。我们帮孩子把要提交的材料准备好——比如那段展示推理过程的录像,该怎么选题、怎么讲得清楚。",
      en: "By this stage, your child has genuinely built these abilities. We help put together what needs to be submitted — like choosing a topic and structuring the recorded reasoning response so it comes through clearly.",
    },
    {
      zh: "我们打磨的是表达的清楚程度,思考的内容始终是孩子自己的。",
      en: "What we help polish is how clearly it's expressed — the thinking inside stays the child's own throughout.",
    },
  ],
};

export const forParents: {
  title: Bilingual;
  intro: Bilingual;
  items: { month: Bilingual; title: Bilingual; body: Bilingual }[];
} = {
  title: { zh: "给家长的支持", en: "For parents" },
  intro: {
    zh: "这两个月,我们也陪着家长一起准备。",
    en: "Over these two months, we support parents too, every step of the way.",
  },
  items: [
    {
      month: { zh: "第一个月", en: "Month 1" },
      title: { zh: "每周一次一对一谈话", en: "A weekly one-on-one conversation" },
      body: {
        zh: "我们和家长每周聊一次,说说 Astra Nova 到底在找什么样的孩子、孩子这周练得怎么样、家里可以怎么配合。",
        en: "We talk with you once a week — about what Astra Nova is really looking for, how your child's practice is going, and how you can support it at home.",
      },
    },
    {
      month: { zh: "第二个月", en: "Month 2" },
      title: { zh: "手把手帮忙准备家长信", en: "Hands-on help with the parent letter" },
      body: {
        zh: "我们会一段一段地陪家长打磨这封信,帮家长把想说的话讲清楚。",
        en: "We work through the letter with you paragraph by paragraph, helping you say clearly what you want the school to know.",
      },
    },
  ],
};

/* ---- 4 · How to apply to work with us --------------------------------------- */
export const applySection: {
  eyebrow: Bilingual;
  title: Bilingual;
  sub: Bilingual;
  steps: { title: Bilingual; body: Bilingual }[];
  ctaLabel: Bilingual;
  ctaHref: string;
} = {
  eyebrow: { zh: "怎么开始", en: "How to get started" },
  title: { zh: "从留下联系方式开始。", en: "It starts with leaving your contact info." },
  sub: {
    zh: "不用一次说清楚所有细节,先让我们认识一下孩子和家庭。",
    en: "You don't need every detail figured out up front — let's just get to know your child and your family first.",
  },
  steps: [
    {
      title: { zh: "留下联系方式", en: "Leave your contact info" },
      body: {
        zh: "告诉我们孩子的大致情况和你关心的问题,我们看到后会尽快联系你。",
        en: "Tell us a bit about your child and what's on your mind — we'll see it and get back to you soon.",
      },
    },
    {
      title: { zh: "一次简短的沟通", en: "A short conversation" },
      body: {
        zh: "我们会安排一次沟通,进一步了解孩子的情况,也回答你的问题。",
        en: "We'll set up a short conversation to learn more about your child and answer your questions.",
      },
    },
    {
      title: { zh: "开始两个月的旅程", en: "Begin the two-month journey" },
      body: {
        zh: "双方都觉得合适,就正式开始上面说的三个阶段。",
        en: "Once it feels like a good fit on both sides, we begin the three stages above.",
      },
    },
  ],
  ctaLabel: { zh: "留下联系方式", en: "Leave your contact" },
  ctaHref: "/contact",
};
