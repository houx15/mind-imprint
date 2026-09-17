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

/**
 * 每个格子底下那一句白话。
 *
 * 🚨 **这一份必须是写作室自己的，不能用阅读室那一份。**
 *
 * `CoachBoard` 是两个房间共用的组件，而它自带的 BIN_HINT 是阅读室的：
 * 「主张 = **作者**要你接受的那句话」。那句话站在「读别人写的东西」这一侧说，
 * 搬到这里就是错的 —— **这一侧作者就是她自己**。而且「解释」「让步」两格在
 * 那张表里根本没有，直接用会让这块板半边有字半边没字。
 *
 * 上面那段已经写了「不要把这两张表合并」。格子不合并，格子底下这句话同样不能。
 *
 * 写的是**这一句在这一段里干什么**，不是给这个词下定义
 *（同阅读室那份的理由）。「让步」用的是学生看得懂的说法 ——
 * methods.json 里它的 name 就是「先承认，再反驳」，「让步」是正式名称。
 */
export const ROLE_BIN_HINT: Record<string, string> = {
  主张: "你要读者接受的那句话",
  证据: "你拿来撑住它的那件事、那个数字",
  解释: "说清这件事凭什么能支持上面那句",
  让步: "先承认对方有道理的那一句",
  背景: "交代情况，不参与说服",
};

/** 她摆完之后，这块板变成一条什么话——回灌给 印记 的就是这条。 */
export function composeRoleBoardAnswer(sentences: Sentence[], placement: BoardPlacement, heading = ""): string {
  // 🚨 带上这一段的标题（2026-09-18）。没有它，印记 不知道她标的是开头段
  // 还是主体段，于是对着开头段说「缺证据，把例子挪进来」—— 而那个例子
  // 在提纲里本来就安排在下一段。
  const which = heading.trim() ? `「${heading.trim()}」这一段` : "这一段";
  const lines: string[] = [`我给${which}的每一句标了它在干什么：`];
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
  heading = "",
  busy,
  onSubmit,
  onCancel,
}: {
  /** 她写的这一段。 */
  text: string;
  /** 这一段在提纲里的标题，写进回灌的第一行。 */
  heading?: string;
  /** 这一段是哪一条 snippet——只用来给卡片一个稳定的来源标记。 */
  snippetId: string;
  busy?: boolean;
  /** 摆完了。调用方负责把这条消息发出去，并且收起这块板。 */
  onSubmit: (message: string) => void;
  /** 先不标了。板自己要有退出的路——它现在是被递过来的，不是一颗开关。 */
  onCancel?: () => void;
}) {
  const { sentences, truncated } = sentencesForBoard(text);
  const items: BoardItem[] = sentences.map((s) => ({
    id: s.id,
    text: s.text,
    blockId: snippetId,
  }));

  return (
    <div className="flex flex-col gap-2">
      <p className="flex flex-wrap items-center gap-2 text-mk-body text-mk-muted">
        <span>这一段有 {sentences.length} 句。把每一句拖到它在做的那件事下面。</span>
        {onCancel && (
          <button type="button" onClick={onCancel} className="underline hover:text-mk-accent-700">
            先不标
          </button>
        )}
        {/* 少几句必须说出来 —— 悄悄截断会让她以为自己写的东西丢了。 */}
        {truncated && <span>（这一段比板装得下的更长，先标前 {sentences.length} 句。）</span>}
      </p>
      <CoachBoard
        items={items}
        bins={[...ROLE_BINS]}
        binHints={ROLE_BIN_HINT}
        itemLabel="把每一句拖到下面某一格里，或者点一句再点一格。"
        submitLabel="标好了"
        busy={busy}
        onSubmit={(placement) => onSubmit(composeRoleBoardAnswer(sentences, placement, heading))}
      />
    </div>
  );
}
