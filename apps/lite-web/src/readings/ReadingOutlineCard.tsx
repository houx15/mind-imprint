import type { ReadingOutline } from "../api/readings";

/**
 * ReadingOutlineCard —— 正文顶上的导读。
 *
 * # 它为什么存在
 *
 * 印记 开场那一轮原来的活儿是「介绍一下你排的读法」，同时又被要求「不要说出
 * 这篇文章的结论」。那是一道自相矛盾的题，真模型给出的答案是把文章讲了一遍：
 *
 *   > 我们先一起把整篇文章通读一遍。这篇文章讲的是冲突里援助组织在忙什么——
 *   > 以色列和哈马斯在打仗……读完告诉我一声。
 *   >
 *   > —— "I don't think this helps."
 *
 * 所以这三样东西改成**确定性地摆出来**：这篇在问什么、它怎么组织、哪几段承重。
 * 模型那一轮因此没有内容可讲，只剩下递第一张卡片这一件事。
 * 见 docs/2026-09-10-reading-guidance-redesign.md。
 *
 * # 三样东西，为什么是这三样
 *
 *   在问什么   **是问题，不是结论**。结论摆出来她就不用读了 —— 服务端的
 *              prompt 明写着这一条，这里只是渲染。
 *   结构       四到六个词。她带着一张地图读，比闷头逐段啃有效。
 *   承重       共几段、核心几段、分别是第几段。段号**可以点**，点了滚过去。
 *              这是整份调研里唯一真的回答了「该在哪儿停」的东西：按论证承重，
 *              不按生词多少。
 *
 * 承重的判断是模型做的，所以卡片上明说它是系统判断 —— 她可以不同意。
 * （不同意这件事本身值得记下来，但那是下一步，这一版还没有那个入口。）
 */

export function ReadingOutlineCard({
  outline,
  ordinalOf,
  onLocate,
}: {
  outline: ReadingOutline;
  /** 段 id → 第几段。段号是她屏幕上唯一认得的坐标，b3 不是。 */
  ordinalOf: (blockId: string) => number;
  onLocate: (blockId: string) => void;
}) {
  const core = outline.core.filter((id) => ordinalOf(id) > 0);
  return (
    <section
      className="mk-reading-outline"
      aria-label="导读"
    >
      {outline.oneLine && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">在问</span>
          {outline.oneLine}
        </p>
      )}
      {outline.shape && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">结构</span>
          {outline.shape}
        </p>
      )}
      {core.length > 0 && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">承重</span>
          共 {outline.blocks} 段，核心 {core.length} 段：
          {core.map((id, i) => (
            <span key={id}>
              {i > 0 && "、"}
              <button
                type="button"
                className="mk-reading-outline__jump"
                onClick={() => onLocate(id)}
              >
                第 {ordinalOf(id)} 段
              </button>
            </span>
          ))}
        </p>
      )}
      {/* 承重是模型判断的。说出来，她才知道这是可以不同意的东西。 */}
      <p className="mk-reading-outline__note">承重由系统判断，供参考。</p>
    </section>
  );
}
