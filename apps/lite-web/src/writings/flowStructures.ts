/**
 * 行文那一步的闭表 —— 服务端 `writing_flow.go` + `vocab.Structures()` 的 TS 侧。
 *
 * 🚨 名字和定义**不在这里写第二份**：它们由 `GET /flow/structures` 从 vocab
 * 取回来。这个文件只放那些前端自己要判断的东西（是不是合法的 id、
 * 一块上能不能标某个方法），免得两份词表分岔 —— 分岔的那天，她在板上选的
 * 结构服务端不认，而屏幕上什么都不说。
 */

/** 服务端 `writingFlowStructureDTO`。 */
export type FlowStructure = {
  id: string;
  name: string;
  definition: string;
  /** 这种结构摆出来长什么样 —— 一句借来的示范（讲的是别的题目）。 */
  example: string;
};

/**
 * 🚨 2026-09-22 这里原来有 `FLOW_METHOD_KINDS` / `flowCanCarryMethod`，
 * 服务的是「每一条打算怎么证明」那个下拉。那一块整个拿掉了（见 FlowStage
 * 顶上那段）：它存下来的 `method` 没有任何一条提示词读过，而产品负责人
 * 2026-09-22 的原话是「they would hate this」。
 *
 * 排顺序的那一层现在由 `OUTLINE_KIND_DEPTH[kind] === 1` 直接算
 *（FlowOrderList.isFlowBlock）—— 深度表本来就是这件事的真相源，
 * 再写一份 id 名单只是在等它和闭表分岔。
 */

/**
 * 这一步算不算做完了。
 *
 * 判据只有一条：**整篇的结构选了**。每块的方法不强求 ——
 * 她可以只想清楚「这几条是并列的」就去写，方法在写的时候再定也来得及。
 *
 * 🚨 这不是一道关卡。顶上那条导航一直点得动（writing_stage.go 的既有裁定：
 * 往前往后跳都是 200），这个函数只决定底栏那句话说什么。
 */
export function flowLooksDone(structureKey: string): boolean {
  return structureKey.trim() !== "";
}
