/**
 * 印记 在用的方法 — the guiding theory, made visible.
 *
 * ## Why this is a screen and not a system prompt
 * The product's promise is that a student leaves knowing HOW to work, not
 * just holding a finished thing. A method the AI follows silently teaches
 * nothing; the same method named, sourced, and readable is a thing she can
 * take to a project we were never part of. So every card that comes from a
 * named practice says which one, and this file is where the practice is
 * written down.
 *
 * ## 🚨 Sourcing discipline
 * Only practices we could actually verify are named here with a source. A
 * research pass on 2026-08-31 produced a much longer list — d.school's design
 * abilities, the Double Diamond, the MYP design cycle, High Tech High's
 * principles — and then RETRACTED most of it as unverified. Those are absent
 * on purpose. If you want to add one, verify it first: a product that teaches
 * students to check sources cannot cite a framework it did not open.
 */

export interface Method {
  id: string;
  /** Short name a student can repeat. */
  label: string;
  /** Where it comes from, honestly. */
  from: string;
  /** The actual content — the named stages or rules, not a summary of them. */
  points: string[];
  /** Why it is in this product. */
  why: string;
}

export const METHODS: Method[] = [
  {
    id: "gold-standard",
    label: "一个项目该有的七件事",
    from: "PBLWorks「Gold Standard PBL」的七个设计要素",
    points: [
      "一个真正难的问题（不是一个已知答案的题目）",
      "持续的追问：查、问、改，不是一次做完",
      "真实性：真实的人、真实的场景、真实的用途",
      "你的声音和选择：题目和做法是你定的",
      "反思：你做完要能说出你学到了什么",
      "批评与修改：至少改过一轮，且改的理由说得出来",
      "公开的成品：交给真实的读者，不是交给老师",
    ],
    why: "我们用它来判断一个项目是不是真的项目。少了第 6、7 条的，通常只是一次作业。",
  },
  {
    id: "critique",
    label: "批评的三条规矩",
    from: "Ron Berger / EL Education 的 critique protocol",
    points: [
      "善意（Be Kind）：对事不对人",
      "具体（Be Specific）：指出是哪一句、哪一块，不说「感觉怪怪的」",
      "有用（Be Helpful）：说完问题，给一个能动手的建议",
      "顺序：先说好的（warm feedback），再说不行的（cool feedback）",
      "改稿次数是作品的一部分，不是丢人的事",
    ],
    why: "印记给你意见的时候按这个顺序来；你给别人意见的时候也一样。",
  },
  {
    id: "survey-wording",
    label: "问卷最常见的几种问坏",
    from: "Pew Research Center《Writing Survey Questions》",
    points: [
      "双重问题：一句话里问了两件事，答案就没法解读",
      "引导性用词：换个词，结果能差二十个百分点",
      "「同意 / 不同意」式提问会让人倾向于同意——改成让他在两个说法之间选一个",
      "回忆负担：问「过去一年你有几次」，得到的是猜测",
      "选项顺序会影响结果，所以要打乱",
    ],
    why: "做调查那条赛道上，问卷体检卡逐条对着这几项检查你写的题。",
  },
  {
    id: "turn-discipline",
    label: "印记说话的纪律",
    from: "TeachLM（arXiv 2510.05087）测出的真人家教对话特征",
    points: [
      "一次只问一个问题",
      "每次说话尽量短——真人家教平均约 70 个词，AI 通常说 150–300",
      "你说的话应该比印记多",
      "不确定的时候给你选项，而不是猜一个答案",
    ],
    why: "这是我们给自己定的硬指标，不是风格偏好。说太多的老师，学生就不想了。",
  },
];

export function methodById(id: string): Method | undefined {
  return METHODS.find((m) => m.id === id);
}

/**
 * The PBL covenant — shown once when a project opens, and again on the method
 * panel.
 *
 * This is the place 铁律 gets its project-side reading. In WRITING, 印记 never
 * produces body text. In a PROJECT, it may absolutely do concrete production
 * work — write code, draft a layout, generate an image, build a form — because
 * refusing to would just teach a student to go use a different tool for that
 * part. What is non-negotiable is the other half: the judgement stays hers,
 * and she has to be able to say why.
 */
export const PBL_COVENANT = {
  title: "在项目里，我们的分工",
  can: [
    "写代码、出草图、排版、做表格、生成图片——动手的活印记可以做",
    "把你说的东西整理成结构，给你三个可选方案",
    "指出你哪里没想清楚，哪里的证据不够",
  ],
  must: [
    "做什么、为谁做、放弃什么——这些判断是你的",
    "每次采纳印记的东西，你要能说出你为什么选它",
    "作品里所有说给别人听的话，是你写的",
  ],
  line: "印记可以动手，但不能替你判断。这两件事分开，你才是在做项目，而不是在验收项目。",
};
