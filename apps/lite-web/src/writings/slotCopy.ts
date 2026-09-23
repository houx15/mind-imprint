import { outlineKindLabel } from "./outlineKind";
import { slotBodyKind, type Slot } from "./slots";

/**
 * slotCopy.ts —— 段落那一步，每张卡上印的字。
 *
 * # 为什么这个文件存在（2026-09-23）
 *
 * 产品负责人逐字报的那个断点：
 *
 *   「记叙文到了「段落」这一步，就没有记叙文的对应模版。
 *     我用含英咀华这道题写记叙文，同样的三条内容：
 *     在「结构」这一步，它们标的是 场景、转折、感悟。
 *     在「行文」这一步，还是 场景、转折、感悟。
 *     到了「段落」这一步，全部变成 分论点 1、分论点 2、分论点 3。
 *     开头那张卡的引导语也变成「提出这篇要证明的中心论点，让读者知道你要
 *     说什么、为什么值得读下去」。」
 *
 * 前两步是对的，第三步掉了 —— 所以这不是「记叙文没做」，是**一条轴在最后
 * 一段路上断了**。断点很具体：`Slot` 本来就带着 `outlineKind`（结构图节点的
 * 闭表取值），而 SnippetsStage 里那两个小标题和 slotJob 一个都没读它，
 * 全是写死的议论文措辞。
 *
 * 🚨 同一个断点还吃掉了**语言**这条轴：一篇英文议论文在图上印的是
 * "Thesis statement / Topic sentence"（outlineKind.ts 早就分好了），
 * 到了段落这一步又变回「中心论点 / 分论点」。
 *
 * # 词从哪儿来
 *
 * 小标题**不在这里另起一套名字**，一律走 `outlineKindLabel(kind, lang)` ——
 * 图上、行文那一步、段落这一步印的必须是同一个词，否则就是三个名字。
 * 这个文件只负责「这一张卡要做的事」那一句（slotJob），以及挑哪个 kind 去
 * 问 outlineKindLabel。
 *
 * # 这一句是骨架的说明，不是印记写的字
 *
 * 和原来那版一样：这几句话讲的是「这一段在文章里干什么」，和她写什么无关，
 * 所以是写死的常量，不过模型。铁律①（AI 不替学生撰写）管的是正文，
 * 不管这块牌子上写的是「这一段要说清例子和观点的关系」。
 */

/** 卡片底下那个节点是记叙文的哪一种。空串 = 不是记叙文的块。 */
const NARRATIVE_KINDS = new Set(["scene", "detail", "turn", "feeling"]);

export function slotIsNarrative(s: Slot): boolean {
  return NARRATIVE_KINDS.has(s.outlineKind);
}

/**
 * slotJob —— 这一张卡要做的事。
 *
 * 按**节点的 kind**分，不按卡片那一层的 kind：卡片那一层只有
 * opening/point/closing/free 四种，而「这一段干什么」在 场景 和 分论点 之间
 * 是两件完全不同的事。
 *
 * `genre` 是整篇的文体，只用来决定开头和结尾这两张卡怎么说 —— 它们没有
 * 自己的节点 kind（虚拟卡），所以 outlineKind 上读不出文体。
 */
export function slotJob(s: Slot, lang: string, genre: string): string {
  const en = lang === "en";
  const narrative = genre === "narrative";
  const letter = genre === "letter";

  if (s.kind === "free") {
    return en
      ? "This piece sits at the end of the draft; you can move it anywhere once the whole thing is assembled."
      : "放在全文最后，也可以在成稿里挪到合适的位置。";
  }

  if (s.kind === "opening") {
    // 🚨 书信的开头是**称呼加一句开门见山的话**，不是「提出中心论点」。
    // 同事那份应用文讲义里，升格的第一个动作就是删掉套话开场
    // （I want to share some ideas with you, wish you can be interested…）。
    if (letter) {
      return en
        ? "Write the salutation and get to the point in the first line: who you are writing to, and why you are writing. Cut any warm-up sentence that says nothing."
        : "写称呼，然后第一句就说清楚这封信是为什么写的。套话开场（「近来好吗，我有些想法想和你分享」）一句都不要。";
    }
    if (narrative) {
      return en
        ? "Open on the moment itself: when, where, who is there. Let the reader stand inside the scene before anything is explained."
        : "从这件事的现场写起：什么时候、在哪儿、谁在场。先让读者进到那个场面里，再谈别的。";
    }
    return en
      ? "State the thesis this piece will argue, so the reader knows what you are claiming and why it is worth reading on."
      : "提出这篇要证明的中心论点，让读者知道你要说什么、为什么值得读下去。";
  }

  if (s.kind === "closing") {
    // 书信的结尾是**给收信人的一句话**，不是观点总结。讲义把这一条单列为
    // 这一档最常见的失分：把给朋友的信写成了议论文的总结。
    if (letter) {
      return en
        ? "Close by speaking to the reader: what you hope they will do, or a line of thanks. End with a sign-off and your name. This is not the place to summarise your opinion."
        : "结尾写给收信人：希望他做什么，或者一句谢谢。最后写上结束语和署名。这里不是总结观点的地方。";
    }
    if (narrative) {
      return en
        ? "Say what you understand now that you did not understand before. Come back to something concrete from the scene rather than ending on a general lesson."
        : "写出这件事之后你明白了什么。回到前面写过的那个具体画面，不要只落一句道理。";
    }
    return en
      ? "Return to the thesis and state it more precisely than the opening did; you can write what the reader should take away."
      : "回到中心论点，把它说得比开头更准；可以写读者读完应该带走的判断。";
  }

  // —— 主体卡。这里才轮到节点的 kind 说话。——
  switch (s.outlineKind) {
    case "scene":
      return en
        ? "Write this scene out: fix the time and place first, then what actually happened, in the order it happened."
        : "把这个场景写出来：先交代时间和地点，再按事情发生的先后写出当时发生了什么。";
    case "detail":
      return en
        ? "Slow down on one action, one look, one line of speech, or one thing in the room. One detail written closely does more than three listed."
        : "停在一个动作、一个神态、一句话或者身边的一样东西上写细。一处写细，胜过三处带过。";
    case "turn":
      return en
        ? "Write the moment it changed: what you thought before, what you saw, and what you thought after. The change needs something you can point at."
        : "写出事情在这里变了：前面你是怎么想的，你看见了什么，之后又变成怎么想。那个变化要有一处看得见的依据。";
    case "feeling":
      return en
        ? "Write what this left you with. Tie it back to the detail you already wrote rather than reaching for a general lesson."
        : "写这件事在你心里留下了什么。接住前面写过的那处细节，不要另起一句大道理。";
    case "purpose":
      return en
        ? "Say in one or two sentences what this letter is for — what you want the reader to know or do."
        : "用一两句写清楚这封信要办成的那件事：你希望收信人知道什么、或者做什么。";
    case "matter":
      return en
        ? "Write this point so the reader can act on it: the time, the place, what to bring, how to reply — whatever this particular point needs."
        : "把这一件事写到收信人能照着做：时间、地点、要带什么、怎么回复 —— 这一条需要哪样就写哪样。「希望你能来」是一个愿望，不是一个要点。";
    case "courtesy":
      return en
        ? "One line to the reader: looking forward to a reply, a thank-you, or a wish. Keep the tone matched to who they are to you."
        : "给收信人的一句话：期待回复、道谢，或者一句祝愿。语气按你和他的关系来定。";
    case "counter":
      return en
        ? "State the strongest version of the opposing view, then say what you still hold and why."
        : "先把对方最有力的那个说法写出来，再说明你仍然保留的看法和理由。";
    case "rebuttal":
      return en
        ? "Answer the opposing view with something specific — what it leaves out, or where the evidence does not reach."
        : "针对上面那条反方观点作答：它漏掉了什么，或者它的材料到不了哪一步。";
  }

  if (s.needsPoint) {
    if (narrative) {
      return en
        ? "The material below has no scene to sit in yet. Say in one line what happened here, then write it out."
        : "下面的材料还没有对应的场景。请先用一句话写出这里发生了什么，再展开写。";
    }
    return en
      ? "The evidence below has no topic sentence yet. Write in one line what it proves, then develop it."
      : "下面的例子还没有对应的分论点。请先用一句话写出这些例子证明了什么，再展开例子。";
  }

  return en
    ? "Write the topic sentence first, then the evidence below it, and finally the line that explains how the evidence supports the point."
    : "先写出这条分论点，再用下面的例子证明它，最后说明例子和论点的关系。";
}

/**
 * slotHeadingLabel —— 卡片里那个小标题（原来写死的「分论点」）。
 *
 * 走 outlineKindLabel，所以图上印「场景」的时候这里也印「场景」，
 * 英文那边印 "Scene"。节点 kind 不认识时退回 point —— 和
 * writingKindDepth 的兜底方向一致（中间那一层错了代价最小）。
 */
export function slotHeadingLabel(s: Slot, lang: string, genre: string): string {
  return outlineKindLabel(slotBodyKind(s, genre), lang);
}

/**
 * slotClaimLabel —— 开头 / 结尾卡上那句「整篇要证明的话」的小标题。
 *
 * 只有议论文有中心论点，所以 claim 非空本身就说明这是议论文那一支
 * （slots.ts 只从 thesis 节点取 claim）。仍然按语言走，一篇英文议论文
 * 这里该印 "Thesis statement"。
 */
export function slotClaimLabel(lang: string): string {
  return outlineKindLabel("thesis", lang);
}
