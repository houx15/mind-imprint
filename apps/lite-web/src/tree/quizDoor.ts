/**
 * 页眉上那颗兴趣测试入口，出现还是不出现。
 *
 * # 为什么把它从 JSX 里挪出来（2026-09-22）
 *
 * 它原来是一行内联的条件，改过两次，第二次把一类学生锁在门外：
 *
 *	改前：quizTaken !== null
 *	改后：quizTaken !== null && (树不空 || 她做过一趟)
 *
 * 改的本意是对的 —— 空树上本来就有一条邀请（「这棵树还没有关键词」底下那颗
 * 「开始兴趣测试」），页眉再摆一颗就是同一件事说两遍。
 *
 * 🚨 但**做到一半离开**的那个学生两条都不满足：树还是空的，`quizTaken` 也
 * 还是 false（她没走完过一趟）。于是她回来时页眉那颗没了，屏幕上只剩空状态
 * 那颗「开始兴趣测试」—— 按下去是从头再来，她上一趟说过的话没有任何一条路
 * 通回去。线上两条走查（awakening-reentry）当场红了，报的就是这件事。
 *
 * 空状态那条邀请写死了「开始」，所以「继续」这句话只能由页眉这颗说。
 * 第三个出现理由因此是必需的：**她手上有一趟没走完的**。
 *
 * 写成纯函数而不是继续留在 JSX 里，是为了能直接测它 —— 这正是
 * AGENTS.md 说的那一类「读代码看不出对错」的逻辑。
 */
export function shouldShowQuizDoor(opts: {
  /** 老师视角。整个入口收起 —— 这是学生自己的测试。 */
  readOnly: boolean;
  /** 她做过一趟没有。null = 还不知道（状态没读回来）。 */
  quizTaken: boolean | null;
  /** 这棵树上一个词都还没有。 */
  treeEmpty: boolean;
  /** 她手上有一趟没走完的。 */
  resumable: boolean;
}): boolean {
  const { readOnly, quizTaken, treeEmpty, resumable } = opts;
  if (readOnly) return false;
  // 状态未知时不显示：在一个上个月已经做过的学生面前每次都闪一下
  // 「来做个测试」，比不显示糟。
  if (quizTaken === null) return false;
  // 树不空：页眉这颗是常驻入口（重做是再长几个词，不是清空重来）。
  if (!treeEmpty) return true;
  // 树是空的 —— 空状态里已经有一条邀请了，所以只在它说不出口的时候才出现：
  // 她做过一趟（那条邀请只在没做过时出现），或者她有一趟没走完（那条邀请
  // 写死了「开始」，说不出「继续」）。
  return quizTaken || resumable;
}
