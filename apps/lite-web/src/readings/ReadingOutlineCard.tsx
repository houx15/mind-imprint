import type { ReadingOutline } from "../api/readings";

/**
 * ReadingOutlineCard —— 这篇文章的导读，读完之后当全文总结摆出来。
 *
 * 🚨 2026-09-18 从对话最上面挪到了**清单走完之后**。同事（《敬业与乐业》）：
 * 「the conclusion card - 核心问题，关键结论，结构 etc. we suggest this to
 * appear after the steps finished, appear at the end to work as a summary.
 * and can also appear in the reading report?」一开读就摆出关键结论，后面
 * 「总结论点」那一步她只要照抄；读完再摆，它是她拿来对照自己总结的那一份。
 * 通读时的分段地图没有丢：每一部分开读时正文里有一行说明（ReadingRoom 的
 * activePart）。
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
 * # 卡片上的几样东西
 *
 *   核心问题   这篇在**问**什么。
 *   关键结论   作者**主张**什么。
 *   结构       四到六个词。她带着一张地图读，比闷头逐段啃有效。
 *   文章概览   每一部分叫什么、第几段到第几段、它在干什么。段号可以点。
 *   重点段落   共几段、核心几段、分别是第几段。段号**可以点**，点了滚过去。
 *
 * 🚨 这五个名字 2026-09-17 全改过一遍，产品负责人逐条给的：
 * 在问→核心问题、中心思想→关键结论、分几部分→文章概览、承重→重点段落。
 * 前两个原来是**半句话**（「在问」「中心思想」），后两个是我们自己造的词
 * （「分几部分」「承重」）—— 按 AGENTS.md §界面文案怎么写 的规矩 1
 * 「标签是名词，不是句子」和规矩 6「用真正的专业词」。
 *              这是整份调研里唯一真的回答了「该在哪儿停」的东西：按论证承重，
 *              不按生词多少。
 *
 * # 「中心思想」是 2026-09-16 加的，而它推翻了这张卡片原来的一条设计
 *
 * 原来整份导读刻意不说结论（「结论摆出来她就不用读了」）。产品负责人走查之后
 * 判的是另一头：
 *
 *   > currently, the 通读部分 is too general. and one student, if they haven't
 *   > read the article before, they would feel that ai's guidance is not easy
 *   > to understand. maybe we should let ai give more scaffolding … 导读 -
 *   > 文章的中心思想、主旨、行文结构. then ask students to read part by part.
 *
 * 两者并存的办法是分工：「在问什么」是她带着去读的那个问题，「中心思想」是她
 * 拿来对照的那张地图。要她做的事没有被拿走 —— 一篇文章的价值不在那个结论，
 * 而在它凭什么这么说，而「凭什么」每一步都还在等她。
 *
 * 承重的判断是模型做的，所以卡片上明说它是系统判断 —— 她可以不同意。
 * （不同意这件事本身值得记下来，但那是下一步，这一版还没有那个入口。）
 */

export function ReadingOutlineCard({
  outline,
  ordinalOf,
  onLocate,
  heading,
}: {
  outline: ReadingOutline;
  /** 段 id → 第几段。段号是她屏幕上唯一认得的坐标，b3 不是。 */
  ordinalOf: (blockId: string) => number;
  onLocate: (blockId: string) => void;
  /** 卡片顶上的标题。读完之后当总结摆出来时给「全文总结」。 */
  heading?: string;
}) {
  const core = outline.core.filter((id) => ordinalOf(id) > 0);
  // 段号由服务端给（fromOrd/toOrd）。这里只挡掉正文换过之后对不上的那些 ——
  // 一个指着不存在的段落的按钮点下去什么都不会发生，而她不知道为什么。
  const parts = (outline.parts ?? []).filter((p) => ordinalOf(p.from) > 0);
  // 🚨 一个字段都没有就整个不渲染，而不是一个空框。
  //
  // 服务端 2026-09-18 起会在「导读整份作废」的时候单独留下体裁那一个词
  // （reading_plan.go）—— 那是给系统看的，她屏幕上没有它的位置。
  if (!outline.oneLine && !outline.gist && !outline.shape && parts.length === 0 && core.length === 0) {
    return null;
  }
  return (
    <section
      className="mk-reading-outline"
      aria-label={heading ?? "导读"}
    >
      {heading && <p className="mk-reading-outline__heading">{heading}</p>}
      {outline.oneLine && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">核心问题</span>
          {outline.oneLine}
        </p>
      )}
      {outline.gist && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">关键结论</span>
          {outline.gist}
        </p>
      )}
      {outline.shape && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">结构</span>
          {outline.shape}
        </p>
      )}
      {parts.length > 0 && (
        <div className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">文章概览</span>
          <ol className="mk-outline-parts">
            {parts.map((p) => (
              <li key={p.from}>
                <button
                  type="button"
                  className="mk-reading-outline__jump"
                  onClick={() => onLocate(p.from)}
                >
                  第 {p.fromOrd}
                  {p.toOrd > p.fromOrd ? `–${p.toOrd}` : ""} 段
                </button>
                <span className="mk-outline-parts__title">{p.title}</span>
                {p.does && <span className="mk-outline-parts__does">{p.does}</span>}
              </li>
            ))}
          </ol>
        </div>
      )}
      {core.length > 0 && (
        <p className="mk-reading-outline__line">
          <span className="mk-reading-outline__key">重点段落</span>
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
      <p className="mk-reading-outline__note">重点段落由系统判断，供参考。</p>
    </section>
  );
}
