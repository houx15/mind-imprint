// tree/treeCopy.ts — the interest tree's second-person lines, and what the
// teacher's read-only view shows in their place.
//
// The student strings are addressed to her (「你的…」「我的…」). On a teacher
// screen those words point at the teacher, so readOnly swaps each for a
// neutral line, or drops it (`null`). readOnly=false must return the student
// strings byte-for-byte; treeCopy.test.ts pins that.

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
  intro: "你的树刚开始长。每读完一篇、写完一篇、做完一个项目，它就会多一个词。",
  keywordsHint: "根据你读过、收藏过、写过、做过的东西自动生成的兴趣关键词。每一个都可以点开，看它到底是从哪几件事来的。",
  outputsHint: "你已经完成的阅读、写作和已发布项目的总数。没做完的不算——这个数字只数你真的做出来的东西。",
  loadingTitle: "正在读取你的兴趣树",
  loadingDetail: "正在从你最近完成的阅读与写作里提取关键词，需要几秒。",
  // The JSX this replaced spanned two source lines, which JSX joins with one space.
  emptyDetail: "关键词由你完成的阅读、写作与项目自动生成。完成一篇后回到这里， 它会长出来。",
  evidenceLabel: "你自己写的",
};

const TEACHER: TreeCopy = {
  header: "兴趣树 · INTEREST TREE",
  intro: null,
  keywordsHint: "根据学生完成的阅读、收藏、写作和项目自动生成的兴趣关键词。点开可查看来源。",
  outputsHint: "学生已完成的阅读、写作和已发布项目的总数。未完成的不计入。",
  loadingTitle: "正在读取兴趣树",
  loadingDetail: "正在从最近完成的阅读与写作中提取关键词，需要几秒。",
  emptyDetail: "关键词由学生完成的阅读、写作与项目自动生成。",
  evidenceLabel: "学生原话",
};

export function treeCopy(readOnly: boolean): TreeCopy {
  return readOnly ? TEACHER : STUDENT;
}
