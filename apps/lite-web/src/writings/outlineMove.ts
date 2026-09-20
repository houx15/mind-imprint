import type { WritingOutlineItem } from "../api/writingRoom";
import { OUTLINE_KIND_DEPTH, outlineKindOf, type OutlineKind } from "./outlineKind";

/**
 * 把图上的一个节点连着它底下的东西挪到别处。
 *
 * # 为什么这是个纯函数
 *
 * 这张思维导图不是一棵存下来的树 —— `writing_outline` 存的是一串扁平的行，
 * 每行带 `depth` 和 `position`，树是 `buildMindMap` 按「文档顺序 + 深度」当场
 * 拼出来的（见 MindMap.tsx 开头那段：没有第二份数据）。
 *
 * 所以「把这条理由挪到那个论点底下」这件事，本质上是一次**列表操作**：
 * 把一段连续的行（节点 + 它底下所有更深的行）剪下来，插到另一个位置，
 * 再把这一段整体的深度平移。它不碰 DOM、不碰指针事件，完全可以单独测 ——
 * 而这正是这个仓库要测的那一类东西（AGENTS.md：值得测的是「读代码看不出
 * 对错」的纯函数，不是渲染）。
 *
 * 服务端那一侧不用新接口：`PUT /outline` 本来就是「拿一整份新的扁平清单
 * 覆盖旧的」，而且覆盖之后会按标题文字把她已经写好的段落重新挂回去
 * （relinkWritingSnippetsToOutline）。所以只要这里算出来的清单是对的，
 * 她写过的字就不会掉。
 */

/**
 * 挪到哪儿。
 *
 * 🚨 `"after"` 从一开始就写在这里，而**没有任何调用点**：
 * `useMindMapDrag` 只会调 `"child"`。于是一条理由被挂进子层之后就再也出不来
 * —— 同事 2026-09-20 的意见 1：「论点被拖入到子论点后就没法拖出来了」。
 * `"root"` 是这次补的第三条路：落在空白画布上 = 升到最上层。
 */
export type OutlineMoveMode = "child" | "after" | "root";

/**
 * 深度上限，和服务端 writingPlanMaxDepth 是同一个数。
 *
 * 0 = 最上层的块（中心论点、开头、结尾），1 = 分论点，2 = 她自己的材料。
 * 再深就不是中学作文的提纲了，是一张组织结构图。
 */
export const OUTLINE_MAX_DEPTH = 2;

/**
 * 算出挪完之后那一整份扁平清单。
 *
 * 挪不了就返回 null —— 调用方据此什么都不做（不要把一次非法拖动变成一次
 * 「看起来成功了但图变形了」的写入）。三种挪不了：
 *
 *  1. 拖到它自己身上；
 *  2. 拖进它自己底下（那会把这一段从树上摘下来，谁也接不住）；
 *  3. 挪完之后它底下有东西会超过深度上限。
 *
 * 🚨 第 3 条是**拒绝**，不是悄悄压平。把她的三层结构压成两层而不告诉她，
 * 比不让她挪更糟 —— 那是在她没看见的时候改她的东西。
 */
export function moveOutlineNode(
  items: WritingOutlineItem[],
  draggedId: string,
  targetId: string,
  mode: OutlineMoveMode,
): WritingOutlineItem[] | null {
  if (draggedId === targetId) return null;

  const rows = items.slice().sort((a, b) => a.position - b.position);
  const from = rows.findIndex((r) => r.id === draggedId);
  if (from < 0) return null;
  const head = rows[from];
  if (!head) return null;

  // 这一段：节点本身，加上紧跟着它的每一行更深的（= 它的子孙）。
  let end = from + 1;
  while (end < rows.length && (rows[end]?.depth ?? -1) > head.depth) end++;
  const block = rows.slice(from, end);

  // 拖进自己底下：目标就在这一段里面。
  if (block.some((r) => r.id === targetId)) return null;

  const rest = [...rows.slice(0, from), ...rows.slice(end)];

  let newDepth: number;
  let insertAt: number;
  if (mode === "root") {
    // 落在空白画布上：升到最上层，排在最后。
    newDepth = 0;
    insertAt = rest.length;
  } else {
    const at = rest.findIndex((r) => r.id === targetId);
    if (at < 0) return null;
    const target = rest[at];
    if (!target) return null;
    newDepth = mode === "child" ? target.depth + 1 : target.depth;
    // 插在哪儿：
    //  - child：紧跟在目标后面。下一行比目标深一层，按「文档顺序 + 深度」
    //    的读法它就是目标的第一个孩子。
    //  - after：跳过目标底下整棵子树，落在它后面 —— 否则会插进目标的孩子中间，
    //    变成目标的孩子而不是兄弟。
    insertAt = at + 1;
    if (mode === "after") {
      while (insertAt < rest.length && (rest[insertAt]?.depth ?? -1) > target.depth) insertAt++;
    }
  }

  const delta = newDepth - head.depth;
  // 整段平移之后最深的那一行。超了就不挪 —— 见上面第 3 条。
  const deepest = Math.max(...block.map((r) => r.depth)) + delta;
  if (newDepth < 0 || deepest > OUTLINE_MAX_DEPTH) return null;

  // 图上已经有中心论点了吗 —— 决定被拖到最上层的那一条该变成什么。
  // 看的是**这一段之外**的行：把中心论点本身拖到最上层，它还是中心论点。
  const hasThesis = rest.some((r) => outlineKindOf(r) === "thesis");

  const moved = block.map((r) => {
    const depth = r.depth + delta;
    return { ...r, depth, kind: rekindMoved(r, depth, hasThesis) };
  });
  const next = [...rest.slice(0, insertAt), ...moved, ...rest.slice(insertAt)];

  // position 重排成 0..n-1。服务端按这个顺序存，树也按这个顺序读。
  return next.map((r, i) => ({ ...r, position: i }));
}

/**
 * 挪完之后，这个节点的 kind 该是什么。
 *
 * 深度对得上就不动 —— 把一条论据从一个分论点挪到另一个，它还是论据。
 * 对不上就换成那一层的默认种类：她做的那个动作的意思正是「这其实是一条理由」，
 * 标题当场跟着变，她才看得见自己刚才做成了什么。
 *
 * 🚨 深度和 kind 必须一致，否则服务端 PUT 会按 kind 把深度再算一遍
 *（buildWritingOutlineArrays），她拖的那一下就被静默撤销了。
 */
function rekindMoved(row: WritingOutlineItem, depth: number, hasThesis: boolean): OutlineKind {
  const kind = outlineKindOf(row);
  if (OUTLINE_KIND_DEPTH[kind] === depth) return kind;
  if (depth <= 0) return hasThesis ? "closing" : "thesis";
  if (depth === 1) return "point";
  return "evidence";
}
