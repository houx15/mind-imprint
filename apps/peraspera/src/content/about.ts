// Per Aspera — about page (/about, /en/about) content. zh is the source of
// truth; en is an idiomatic (not literal) translation. Sources:
//   - docs/astranova/PerAspera官网文案v2-多页版.md §四「关于我们页 /about」
//     (页首 / 创始人两卡 / 创始人信 body-swap note / 名字的由来)
//   - docs/astranova/PerAspera官网文案v1.md §8 (founder letter full body,
//     verbatim except one rewrite below) and §9 (all 8 FAQ items, verbatim
//     except one rewrite below).
//
// Two deliberate rewrites, per the site's positive-declarative copy rule
// (see home.ts, programs.ts for precedent):
//   1. Letter, final paragraph: v1/v2 both read "申请是出口之一，不是目的。"
//      (an "A, not B" antithesis). Rewritten to "真正的目标是这些能力本身，
//      申请只是其中一个出口。" — same meaning, no negated alternative.
//   2. FAQ Q3 (test-prep question): v1 reads "但‘不应试’不等于‘不打磨’…
//      改的是表达质量，不替孩子思考。" Rewritten to drop the "not X"
//      clause: "…打磨的是表达质量，思考全程仍由孩子自己完成。"

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "关于我们", en: "About Us" },
  title: {
    zh: "两个人，一个执念：能力无法代办。",
    en: "Two people, one conviction: ability cannot be outsourced.",
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
    name: { zh: "陈玉洁", en: "Chen Yujie" },
    role: {
      zh: "联合创始人 · 研究院负责人",
      en: "Co-founder · Head of the Institute",
    },
    bio: {
      zh: "多年联合国 ESG 与气候课程、IB 课程辅导经验。把真实国际课堂里长出来的批判性思维与信息素养课，系统化为“思维印记”课程体系与评估标准。",
      en: "Years of experience with UN ESG and climate curricula, and with IB coursework. She has taken the critical-thinking and information-literacy teaching that grew out of real international classrooms and systematized it into Mind Imprint's course framework and assessment standards.",
    },
    photoSlot: { zh: "照片位", en: "Photo placeholder" },
  },
  {
    name: { zh: "侯煜欣", en: "Hou Yuxin" },
    role: {
      zh: "联合创始人 · 课程与产品负责人",
      en: "Co-founder · Head of Curriculum & Product",
    },
    bio: {
      zh: "连续创业者。AI 思维、产品思维、计算思维教育者——相信最好的思维课是“从真问题到真原型”的完整旅程。",
      en: "A serial founder. An educator in AI thinking, product thinking, and computational thinking, who believes the best thinking curriculum is the full journey from a real problem to a real prototype.",
    },
    photoSlot: { zh: "照片位", en: "Photo placeholder" },
  },
];

/* ---- Founders' letter ---------------------------------------------------
   Full body from v1 §8, with v2's adjusted final paragraph swapped in for
   v1's "所以 Per Aspera 只做一件事…" paragraph (see v2 §四 note). */
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
    { zh: "你好。", en: "Hello." },
    {
      zh: "我们是两位教育者：一位在国际课堂里教了多年批判性思维——教孩子拆穿漂绿广告、给气候纪录片的信息手法分类、追问“我们能多确定”；一位创过业、做过产品——习惯把一切大问题拆小，再动手把答案做出来。",
      en: "We're two educators. One has spent years teaching critical thinking in international classrooms — showing kids how to spot a greenwashed ad, sorting the persuasion tactics in climate documentaries, and asking, again and again, “how sure can we really be?” The other has founded companies and built products — trained to take any big problem apart, then go build the answer.",
    },
    {
      zh: "我们决定一起做 Per Aspera，起点是一次深入的研究：我们把 Elon Musk 教育谱系的公开资料全部读完——从 SpaceX 园区里的第一间教室，到今天的 Astra Nova 高中。看得越深，我们越确认两件事。",
      en: "We decided to build Per Aspera together, and it started with deep research: we read everything public about Elon Musk's education lineage — from the first classroom on the SpaceX campus to today's Astra Nova high school. The deeper we looked, the more two things became clear.",
    },
    {
      zh: "第一，这套教育抓住了对的东西。没有题库、没有标化、不在乎 IQ，只看孩子面对一道两难问题时怎么推理、被追问时敢不敢改口。这恰好是 AI 时代唯一不会贬值的能力。",
      en: "First, this approach to education has hold of the right thing. No question bank, no standardized tests, no interest in IQ — only how a child reasons through a genuine dilemma, and whether they're brave enough to change their mind when pushed. That happens to be the one ability the AI era can't devalue.",
    },
    {
      zh: "第二，这套教育无法被应试攻克——这正是它最好的地方。我们见过太多“攻略”：背题、代写、视频脚本。它们不但没用（面试的当场反转会让一切包装露馅），而且有害——它们偷走了孩子真正成长的机会。",
      en: "Second, this kind of education resists being cracked by test prep — and that's exactly what makes it good. We've seen every playbook: memorized answers, ghostwritten letters, scripted videos. They don't work (a live reversal in the interview exposes the packaging instantly), and worse, they cause harm — they rob a child of the chance to actually grow.",
    },
    {
      zh: "所以 Per Aspera 是两件事：一所训练真实能力的学院，和一个测量真实能力的研究院。孩子的思辨、表达与协作；家长的理念、对话方式与判断力；以及一把能看见“怎么想”的尺子。真正的目标是这些能力本身，申请只是其中一个出口。如果你的孩子最终去了 Astra Nova，太好了；如果没去，这些能力也已经长在他身上——这是我们敢把研究全部公开、敢在合同里写明“不承诺录取”的原因。",
      en: "So Per Aspera is two things: an academy that trains real ability, and an institute that measures it. Your child's reasoning, expression, and collaboration. Your own philosophy, way of talking with your child, and judgment as a parent. And a ruler that can actually see how someone thinks. Real growth in ability is the real goal; an offer of admission is just one of the outcomes along the way. If your child ends up at Astra Nova, wonderful. If not, the ability is already theirs to keep — which is why we're willing to publish all of our research, and to write “no guaranteed admission” into the contract itself.",
    },
  ],
  signatureLine: {
    zh: "穿越荆棘，以达星辰。荆棘没有捷径，但可以有同行的人。",
    en: "Through hardship, to the stars. There's no shortcut through the thorns — but you don't have to walk them alone.",
  },
  signoff: {
    zh: "—— 陈玉洁 · 侯煜欣　2026 年夏",
    en: "— Chen Yujie · Hou Yuxin, Summer 2026",
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
    zh: "——“循此苦旅，以达星辰”。Astra Nova 与 Ad Astra 取了“星辰”，我们取“苦旅”：能力没有捷径，思维的成长必须亲身穿越。",
    en: " means “through hardship, to the stars.” Astra Nova and Ad Astra both took the stars for their names. We take the hardship: there's no shortcut to real ability — the growth of a mind has to be walked in person.",
  },
};

/* ---- Full FAQ (verbatim v1 §9, all 8 items) --------------------------------- */
export const faqIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "常见问题", en: "FAQ" },
  title: { zh: "完整 FAQ", en: "The Full FAQ" },
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
      zh: "没有任何关系。我们是独立机构，未获 Astra Nova School 授权、认可或合作。我们研究它、认同它的理念方向，仅此而已。（此声明同时出现在页脚。）",
      en: "None at all. We're an independent organization — not authorized, endorsed, or partnered with Astra Nova School in any way. We study it, and share its philosophical direction; that's all. (This same statement appears in the footer.)",
    },
  },
  {
    q: { zh: "你们能保证录取吗？", en: "Can you guarantee admission?" },
    a: {
      zh: "不能，任何人都不能。Astra Nova 高中一届只收几十人，且申请材料必须由孩子本人完成。我们承诺的是 14 周内可见的能力提升（有评估报告为证），以及只在录取后才收取的成功费——我们把商业利益和你的目标绑在一起，但不出售幻觉。",
      en: "No — and no one can. Astra Nova admits only a few dozen students a year, and every application must be the child's own work. What we promise is visible growth in ability within 14 weeks, backed by an assessment report, plus a success fee charged only after an offer — our commercial interest is tied to your goal, without selling you an illusion.",
    },
  },
  {
    q: {
      zh: "为什么说应试没用？那你们最后帮不帮改材料？",
      en: "Why doesn't test prep work? Do you still help polish the materials?",
    },
    a: {
      zh: "思辨题没有标准答案，小组面试会有当场追问，包装终究会露馅。我们仍然打磨材料：最后两周会 review 视频表达与家长信的清晰度，打磨的是表达质量，思考全程仍由孩子自己完成。",
      en: "Reasoning prompts have no model answer, and the group interview probes with live follow-up questions — packaging always shows eventually. We still polish the materials: in the final two weeks, we review how the video reads and how clear the parent letter is. What we polish is the quality of expression; the thinking inside stays entirely the child's own, start to finish.",
    },
  },
  {
    q: {
      zh: "课程什么语言？我孩子英语一般怎么办？",
      en: "What language is instruction in? What if my child's English is average?",
    },
    a: {
      zh: "思辨内容用中文教透（理解深度优先），每周固定全英文小组讨论，面试模拟全英文。英语弱但思维强的孩子恰恰是我们最想收的——表达可以练，好奇心很难教。",
      en: "We teach the reasoning content in Chinese, for depth of understanding first, with a fixed weekly all-English group discussion and fully English interview mock sessions. A child with strong thinking and average English is exactly who we want — expression can be trained; curiosity is much harder to teach.",
    },
  },
  {
    q: { zh: "孩子几岁可以来？", en: "What ages do you work with?" },
    a: {
      zh: "冲刺营面向 13–17 岁（对应 Astra Nova 高中 14–18 岁入学）。长线学院面向 9–14 岁。更小的孩子建议先从家庭对话开始——家长轨工作坊单独开放报名。",
      en: "The Sprint is for ages 13–17 (matching Astra Nova's high-school admission at 14–18). The Academy is for ages 9–14. For younger children, we suggest starting with conversation at home — the parent-track workshops are open for enrollment on their own.",
    },
  },
  {
    q: { zh: "时差怎么办？", en: "What about time zones?" },
    a: {
      zh: "Per Aspera 全部课程在亚洲友好时段。这正是我们存在的理由之一：Astra Nova 的课在美西时段，亚洲孩子只能半日制参与。",
      en: "Every Per Aspera class runs in Asia-friendly hours. That's part of why we exist: Astra Nova's classes run on U.S. West Coast time, so kids in Asia can only join part-time.",
    },
  },
  {
    q: { zh: "退费规则？", en: "What's the refund policy?" },
    a: {
      zh: "细则仍在确定中，目前的草案是：入营后前两周内可无理由全额退费；成功费仅在孩子获得录取后收取，本身不会产生退费纠纷。最终版本以正式合同为准。",
      en: "The details are still being finalized. The current draft: a full, no-questions-asked refund within the first two weeks after enrollment; the success fee is charged only after an offer, so it carries no refund dispute by design. The final version will be set out in the enrollment contract.",
    },
  },
  {
    q: { zh: "报名流程是什么样的？", en: "What does the enrollment process look like?" },
    a: {
      zh: "提交申请（孩子选一道思辨题录 2 分钟视频 + 家长写一页信）→ 家庭面试（孩子 30 分钟 + 家长 30 分钟）→ 双向确认入营。是的，我们的申请方式和 Astra Nova 一样——这是你们家的第一次实战演练。",
      en: "Submit an application (your child records a 2-minute video answering one of three reasoning prompts, and you write a one-page letter) → a family interview (30 minutes with the child, 30 minutes with the parents) → mutual confirmation of enrollment. Yes, our process mirrors Astra Nova's own — think of it as your family's first live rehearsal.",
    },
  },
];
