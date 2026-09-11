import { CoachBoard, type BoardItem, type BoardPlacement } from "../readings/CoachBoards";
import { sentencesForBoard, type Sentence } from "./sentences";

/**
 * RoleBoard —— 标注板：把她自己这一段里的每一句，拖进一个「这句在干什么」的格子。
 *
 * # 为什么写作房间需要一块板
 *
 * 2026-09-11 走查这个房间时最刺眼的一件事：`src/writings/` 里**一处 pointer
 * 事件、一处拖动、一处重新排序都没有**。思维导图是照着 `depth`+`position` 画
 * 出来的，她只能打字，画由系统替她画；段落是 textarea；成稿是 textarea；
 * 引导框是四段文字。而阅读室那边 2026-09-10 已经有两块能拖的板了。
 *
 * 产品负责人对「交互」有确定的意思（2026-09-04）：**一块能用手摆的板，
 * 拖是主要动词**。这就是写作面的第一块。
 *
 * 🚨 这不是把 2026-08-27 删掉的工具卡搬回来。那次删的是**卡片货架**——一排摆在
 * 那儿等她挑、挑了要填表的东西。这块板不是货架：它长在她已经写完的某一段上，
 * 和旁边那颗「请印记看看这一段」是同一类东西（一个针对这一段的动作），
 * 摆完就没了。
 *
 * # 卡片是**拆**出来的，不是模型挑的
 *
 * 板上每一张卡都是 `splitSentences` 从她自己那一段里切出来的一句话
 * （见 sentences.ts）。三个后果，都在承重：
 *
 *  1. **铁律① 结构性成立。** 板上不可能出现一个她没写过的字——拆分只会切。
 *  2. **不需要引文核对。** 阅读室那两块板必须逐字回文章里核对（模型会编一句
 *     读着很像的话），这里连编的机会都没有。
 *  3. **不花一次模型调用**，所以板是即时出现的，不会等、不会失败。
 *
 * # 它测的是什么
 *
 * qifeng 的诊断表里有一条叫「论证像观点清单」，它的信号是
 * **「每段都是一个判断，段落之间可以任意换序」**。这块板把那个信号变成她自己
 * 看得见的东西：**摆不出「证据」这一格，就是这一段没有证据**——
 * 而她一个字都不用写，也没有人对她说「你的论证不够充分」。
 *
 * 摆完之后，哪几格是空的会写进回灌给 印记 的那条消息里，下一轮它对着那件事说话。
 */

/**
 * 格子：这一句在这一段里**干什么**。
 *
 * 🚨 **闭表，而且和阅读室那块标注板的五个格子故意不一样。**
 * 阅读室标的是「作者这一句在论证里是什么角色」（主张/证据/限制/背景/对比），
 * 标的是别人的文章；这里标的是「我这一句在我这一段里做什么活」。
 * 两者最像的是「主张」和「证据」，但写作这一侧必须有「解释」——
 * 学生最常见的毛病正是举了例子却没有一句话说清它凭什么支持主张
 * （`evidence_not_explained`，writing_symptoms.go）；而阅读那一侧必须有「对比」，
 * 写一段话的时候那不是一个句子的角色。
 *
 * 所以**不要把这两张表合并**。它们看着像，问的是两件事。
 */
export const ROLE_BINS = ["主张", "证据", "解释", "让步", "背景"] as const;

/** 她摆完之后，这块板变成一条什么话——回灌给 印记 的就是这条。 */
export function composeRoleBoardAnswer(sentences: Sentence[], placement: BoardPlacement): string {
  const lines: string[] = ["我给这一段的每一句标了它在干什么："];
  for (const s of sentences) {
    const bin = placement[s.id];
    if (!bin) continue;
    lines.push(`${s.index} ${bin} —— ${s.text}`);
  }

  // 🚨 哪几格是空的，才是这块板真正产出的东西。不写进回灌里，印记 就只能看到
  // 她标对了什么，看不到这一段缺什么——而缺什么是下一轮该谈的事。
  const used = new Set(Object.values(placement));
  const missing = ROLE_BINS.filter((b) => !used.has(b));
  if (missing.length > 0) {
    lines.push(`这一段里没有：${missing.join("、")}`);
  }
  return lines.join("\n");
}

export function RoleBoard({
  text,
  snippetId,
  busy,
  onSubmit,
}: {
  /** 她写的这一段。 */
  text: string;
  /** 这一段是哪一条 snippet——只用来给卡片一个稳定的来源标记。 */
  snippetId: string;
  busy?: boolean;
  /** 摆完了。调用方负责把这条消息发出去，并且收起这块板。 */
  onSubmit: (message: string) => void;
}) {
  const { sentences, truncated } = sentencesForBoard(text);
  const items: BoardItem[] = sentences.map((s) => ({
    id: s.id,
    text: s.text,
    blockId: snippetId,
  }));

  return (
    <div className="flex flex-col gap-2">
      <p className="text-mk-body text-mk-muted">
        这一段有 {sentences.length} 句。把每一句拖到它在做的那件事下面。
        {/* 少几句必须说出来 —— 悄悄截断会让她以为自己写的东西丢了。 */}
        {truncated && `（这一段比板装得下的更长，先标前 ${sentences.length} 句。）`}
      </p>
      <CoachBoard
        items={items}
        bins={[...ROLE_BINS]}
        itemLabel="把每一句拖到下面某一格里，或者点一句再点一格。"
        submitLabel="标好了"
        busy={busy}
        onSubmit={(placement) => onSubmit(composeRoleBoardAnswer(sentences, placement))}
      />
    </div>
  );
}
