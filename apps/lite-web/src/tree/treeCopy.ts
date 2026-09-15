// tree/treeCopy.ts — the interest tree's second-person lines, and what the
// teacher's read-only view shows in their place.
//
// The student strings are addressed to her (「你的…」「我的…」). On a teacher
// screen those words point at the teacher, so readOnly swaps each for a
// neutral line, or drops it (`null`). Tests preserve this audience distinction.

export type TreeCopy = {
  /** Header label above the name. */
  header: string;
  /** Line under the name when there is no growth axis to show; null = omit. */
  intro: string | null;
  keywordsHint: string;
  outputsHint: string;
  loadingTitle: string;
  loadingDetail: string;
  emptyDetail: string;
  /** Label under a keyword source's quoted evidence (KeywordDrawer). */
  evidenceLabel: string;
};

const STUDENT: TreeCopy = {
  header: "我的兴趣树 · INTEREST TREE",
  intro: "兴趣树会根据已完成的阅读、写作和项目更新。",
  keywordsHint: "根据阅读、收藏、写作和项目生成兴趣关键词。点击关键词可查看来源。",
  outputsHint: "已完成的阅读、写作和已发布项目的总数。",
  loadingTitle: "正在读取你的兴趣树",
  loadingDetail: "正在根据已完成的学习记录整理兴趣关键词。",
  emptyDetail: "完成阅读、写作或项目后，可在这里查看相关兴趣关键词。",
  evidenceLabel: "你的原话",
};

const TEACHER: TreeCopy = {
  header: "兴趣树 · INTEREST TREE",
  intro: null,
  keywordsHint: "根据学生完成的阅读、收藏、写作和项目自动生成的兴趣关键词。点开可查看来源。",
  outputsHint: "学生已完成的阅读、写作和已发布项目的总数。未完成的不计入。",
  loadingTitle: "正在读取兴趣树",
  loadingDetail: "正在根据已完成的学习记录整理兴趣关键词。",
  emptyDetail: "关键词由学生完成的阅读、写作与项目自动生成。",
  evidenceLabel: "学生原话",
};

export function treeCopy(readOnly: boolean): TreeCopy {
  return readOnly ? TEACHER : STUDENT;
}
