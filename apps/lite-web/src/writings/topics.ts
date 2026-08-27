/**
 * topics.ts — 「不知道写什么？」's seed data, the writing-room sibling of
 * readings/recommendations.ts.
 *
 * 铁律②: a student who opens 写作 with nothing in mind has nothing to type
 * into the one box. These entries are the answer — a short, FIXED shelf,
 * never a feed (no paging, no "more like this", no personalization). Picking
 * one seeds `createWriting`'s `idea` field with a real opening sentence (not
 * just a bare title), because idea does double duty server-side: truncated it
 * becomes the writing's title, and verbatim it becomes the first turn of the
 * transcript — so a topic tile has to hand over a sentence she could
 * plausibly have typed herself, not a category label.
 */

export interface WritingTopic {
  id: string;
  /** The short label on the tile. */
  title: string;
  /** One line: why this is worth fifteen minutes. */
  reason: string;
  /** The only label on the tile, same role as recommendations.ts's `genre`. */
  genre: string;
  lang: "zh" | "en";
  /** Macaron token name — same shelf-of-different-books treatment as
   *  recommendations.ts's `tone`. */
  tone: "peach" | "matcha" | "lake" | "taro";
  /** Seeds `createWriting({ idea })` — a real opening sentence, not a title. */
  idea: string;
}

export const WRITING_TOPICS: WritingTopic[] = [
  {
    id: "school-schedule",
    title: "该不该把上学时间往后推？",
    reason: "一个你自己就有立场、也天天在经历的政策问题。",
    genre: "议论",
    lang: "zh",
    tone: "lake",
    idea: "我想写一篇论证文，说说该不该把中学的上学时间往后推一个小时。",
  },
  {
    id: "short-video",
    title: "短视频有没有让我们变笨？",
    reason: "把一个流行说法拆开，看看证据到底站不站得住。",
    genre: "议论",
    lang: "zh",
    tone: "peach",
    idea: "我想写一篇文章，讨论短视频到底有没有让人的专注力变差，不想只停在「大家都这么说」。",
  },
  {
    id: "campus-jobs",
    title: "学生该不该在学期中打工？",
    reason: "两边都有真实的代价，是练习「让步段」的好题目。",
    genre: "议论",
    lang: "zh",
    tone: "taro",
    idea: "我想论证一下高中生在学期中打工到底值不值得，正反两边我都能想到理由。",
  },
  {
    id: "personal-essay",
    title: "A Moment That Changed How I See Something",
    reason: "Practice one true story, told with enough specificity to argue a point.",
    genre: "英文写作",
    lang: "en",
    tone: "matcha",
    idea: "I want to write a personal essay about a small moment that changed how I see something.",
  },
];
