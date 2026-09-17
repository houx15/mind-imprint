import { useEffect, useId, useRef, useState } from "react";
import {
  CoachBoard,
  CoachBoardRecap,
  type BoardItem,
  type BoardPlacement,
} from "./CoachBoards";

/**
 * CoachCard — 印记 把这一步递到她手上，让她点。
 *
 * A card is a question 印记 wrote into its own reply, answered by tapping
 * instead of by typing a paragraph. The product ruling it exists to serve:
 *
 *   > the questions are ladders, we use these to support students to dive to
 *   > think deep to connect his own interests. we do use them as gate
 *   > sometimes to avoid students' 胡乱阅读, but we do not serve as an exam,
 *   > 不要变成标准化考试, we think each child's thoughts are precious.
 *
 * 🚨 **这张卡片上永远不会出现对错。** 没有 ✓、没有 ✗、没有分数、没有连胜
 * （铁律②）。这不是「暂时没做」——服务端根本不发 answer key，因为**不存在**
 * 一个正确答案：`validateCoachCard`（`reading_coach_card.go`）校验的是
 * 「选项是不是文章里真有的一句话」，不是「哪一句对」，而 prompt 里写死了
 * 「问她的判断，不能有唯一正解」。她点下去的是一个**选择**，不是一次作答尝试。
 * 所以这里没有任何地方需要「判分」的信息，也不许有人后来补上。
 *
 * ## 为什么不用 `@/cards/CardRenderer`
 *
 * 那是工具卡的渲染器，`CardSpec` 的 Zod 要求
 * `id/category/name/purpose/trigger_condition/steps/rubric_tags` 全齐，
 * 并且 `card_id` 必须在 `CARD_REGISTRY` 里解析得到——模型现写的一张三选一
 * 卡片永远满足不了，为它去编这些字段等于把契约弄假。lite 自己写一个窄组件。
 *
 * ## 三种形状
 *
 * - `choose_span`     文章里 2–4 句原话，点一句。
 * - `pick_in_article` 不给选项：这一步要她自己回文章里去指。卡片只说话，
 *                     指的动作发生在文章上（房间的引用条），Task 8 接线。
 * - `short_text`      她用自己的话写一句。
 *
 * 已作答的卡片仍然留在对话里显示她选了什么——那是她说过的话，不该在刷新之后
 * 消失——但**不再可点**：第二次点击只会变成第二条学生消息。
 *
 * ## 它必须看起来不是一句话（2026-08-29，对着真实截图）
 *
 * 产品负责人用真文章走了一遍之后：*"currently in the box ai's chat box and
 * task card is not very clear. task card. AI avatar is necessary."*
 * 卡片和 印记 的气泡曾经是同一列里两个几乎一样浅的框，她读不出「这是在跟我
 * 说话」和「这是要我动手的东西」的差别。现在卡片有左边一条实心竖杠、更实的
 * 边框底色，和一枚 「动手」 小标签；头像那一半在 `ReadingCoachPanel` 的
 * `CoachLog` 里（挂在气泡外面）。
 *
 * 🚨 `mk-*` 是**裸 CSS 变量**，所以这里每一处半透明都是 `color-mix()`：
 * `bg-mk-accent-500/30` 这类 Tailwind alpha 语法对它们一个字节的 CSS 都不生成，
 * 静默变成透明。
 *
 * ## 🚨 卡片不许说屏幕的方位
 *
 * 这里曾经写死「在**左边**文章里点出那一句」，而文章在桌面端排在**右边**、
 * 手机上排在**下面**。两次真实走查开头的第一张卡都是 `pick_in_article`，
 * 所以那是学生看到的**第一件事**，而它把她指去了空白的那一边——她找不到，
 * 打字说「我读完了」，hunt 判定正确地拒绝推进，印记 连着训了她两次。
 * 布局本来就随视口变，换一个方位词只是把错误推迟：**一个都不提**。
 *
 * ## `stale`：一次只问一个（铁律③）
 *
 * 印记 给一张卡 → 她不理，直接打字 → 印记 又给一张。两张都敞开的话，她得先猜
 * 房间到底要哪一个——这正是铁律③ 要消除的认知负担。旧的那张收成一行问题。
 *
 * 🚨 **不是置灰。** 一排死掉的灰色 UI 读起来就是「这是你没做完的所有事」，
 * 计分板的情绪从后门溜进来了，而这个房间恰恰最不能有计分板。折叠保留了记录
 * （印记 确实问过），同时只有一张可操作，读起来是「我们往下走了」。
 *
 * 🚨 **必须能点开。** 配对循环（ReadingCoachPanel `cards`）特意支持「新卡出现
 * 之后仍然回去答旧卡」，折叠不许把它弄丢；而且**由她选择重新打开，房间不替她
 * 关死**——这条是铁律②。
 *
 * `stale` 由 `ReadingCoachPanel` 按「后面还有没有更新的卡片」算，而不是按
 * 「它是不是最新的那张未答卡片」——后者会让一张早就折起来的旧卡在她答掉当前
 * 这张的瞬间**自己弹开**，下一条回复到达时又折回去（`05-turn1-card-AFTER-tap.png`
 * 拍到的那一下抖动）。一旦折起来就保持折着，直到她**主动点开**。
 */

export type CoachCardType =
  | "choose_span"
  | "pick_in_article"
  | "short_text"
  // 两块板（2026-09-10）。前三种是「她说」，这两种是「她摆」——
  // 见 CoachBoards.tsx 的头注。
  | "label_roles"
  | "word_bank";

/**
 * 卡片上的一个选项：文章里某一段（blockId）的某一句原话（quote），以及它在
 * 第几段（where，「第4段」）。
 *
 * 🚨 `where` 是**服务端数的**（`stampOptionWhere`），不是前端从 blockId 推的，
 * 也不是模型写的 —— 它必须和正文旁边那个号码是同一个，不然她照着去找就找不到。
 * 老消息里的卡片没有这个字段，所以它是可选的：没有就不显示。
 */
export type CoachCardOption = { blockId: string; quote: string; where?: string };

/** 生词板上的一个词。**没有释义字段**，而且是故意的：板上不摆答案。 */
export type CoachCardWord = { blockId: string; term: string };

/** 服务端 `coachCard` 键上那张卡片（`reading_coach.go` 的响应）。 */
export type CoachCardSpec = {
  type: CoachCardType;
  prompt: string;
  /** `choose_span` 和 `label_roles` 有；其余服务端会清空。 */
  options?: CoachCardOption[];
  /** `word_bank` 专用。 */
  words?: CoachCardWord[];
  /** `label_roles` 那块板上的格子。**服务端填的闭表**，模型给不了。 */
  labels?: string[];
};

/**
 * 她的作答，原样发给 `POST /readings/{id}/coach` 的 `cardAnswer`。
 *
 * 🚨 `choice` 必须是她点中的那句**原文照抄**——不要 trim 掉标点、不要做任何
 * 改写。服务端拿它回文章里做字面子串核对来分辨「原文」和「她自己的话」
 * （`quoteIsArticleText`）；核不上就当成她的话处理，「她指了」这件事就没了，
 * hunt 步也就落不了地。
 */
export type CoachCardAnswer = {
  type: CoachCardType;
  prompt: string;
  choice: string;
  blockId?: string;
};

/** 生词板的三格。写死在这里而不是由服务端发：它们和这块板是同一件东西，
 *  换了格子就是换了一块板，而服务端那一侧没有任何东西需要知道它们。 */
export const WORD_BINS = ["认识", "不确定", "不认识"];

/** 一块板上待分类的那些东西，从卡片本身派生。 */
export function boardItems(card: CoachCardSpec): BoardItem[] {
  if (card.type === "word_bank") {
    // 词不带段号：一个词标上「第4段」没有帮助，她要分的是认不认识，不是它在哪儿。
    return (card.words ?? []).map((w, i) => ({
      id: `w${i}`,
      text: w.term,
      blockId: w.blockId,
    }));
  }
  return (card.options ?? []).map((o, i) => ({
    id: `o${i}`,
    text: o.quote,
    blockId: o.blockId,
    where: o.where,
  }));
}

/**
 * 她摆完的那块板，从她那条作答里读回来。
 *
 * 🚨 逐行对着 `composeBoardAnswer` 写的，而且**只认那一种格式** —— 认不出来的
 * 行就跳过。这段字是唯一的记录（它就是存进 atom_message 的那一份），所以读它
 * 比另存一份状态更诚实：刷新之后回来看到的，和服务端看到的是同一个东西。
 */
export function parseBoardAnswer(card: CoachCardSpec, choice: string): BoardPlacement {
  const items = boardItems(card);
  const byText = new Map(items.map((it) => [it.text.trim(), it.id]));
  const out: BoardPlacement = {};
  const lines = choice.split("\n");
  if (card.type === "word_bank") {
    // 「词 — 格子」，一行一个。
    for (const line of lines) {
      const i = line.indexOf(" — ");
      if (i < 0) continue;
      const id = byText.get(line.slice(0, i).trim());
      const bin = line.slice(i + 3).trim();
      if (id && bin) out[id] = bin;
    }
    return out;
  }
  // 「格子：」一行，原文那一句在下一行。
  for (let i = 0; i + 1 < lines.length; i++) {
    const head = lines[i]!.trim();
    if (!head.endsWith("：")) continue;
    const id = byText.get(lines[i + 1]!.trim());
    if (id) out[id] = head.slice(0, -1);
  }
  return out;
}

/**
 * 上一块板上她摆好的那些，搬到这一块板上来。
 *
 * 🚨 同事 2026-09-17 报的第 3 条：一块板常常要两三轮才摆对，而 印记 每一轮
 * 重发的板是**空的** —— 上一轮她摆对的那几句，也要从头再摆一遍。
 *
 * 按**句子本身**认，不按 id 认：每张卡片的 id 是它自己那一份里的序号
 * （`o0` `o1`…），两张卡片上的 `o0` 常常不是同一句话。这和
 * `parseBoardAnswer` 认句子的办法是同一条。
 *
 * 两块板的格子不一样时（`label_roles` 换了一套 labels），落在这块板上没有的
 * 那一格里的那一句就不搬 —— 搬过去她会看到一张卡片卡在一个不存在的格子里。
 */
export function carryOverPlacement(
  prev: { card: CoachCardSpec; choice: string },
  next: CoachCardSpec,
  nextBins: string[],
): BoardPlacement {
  if (prev.card.type !== next.type) return {};
  const placed = parseBoardAnswer(prev.card, prev.choice);
  const prevText = new Map(boardItems(prev.card).map((it) => [it.id, it.text.trim()]));
  const binByText = new Map<string, string>();
  for (const [id, bin] of Object.entries(placed)) {
    const text = prevText.get(id);
    if (text && nextBins.includes(bin)) binByText.set(text, bin);
  }
  const out: BoardPlacement = {};
  for (const it of boardItems(next)) {
    const bin = binByText.get(it.text.trim());
    if (bin) out[it.id] = bin;
  }
  return out;
}

/**
 * 她摆完之后，这块板变成一段什么话。
 *
 * 🚨 这段话要同时被两个人读：印记（它得看懂她把什么放进了哪儿）和**她自己**
 * （它会原样留在对话记录里，刷新之后还在）。所以它不是 JSON，是人话。
 *
 * 🚨 而且每一行都得能被服务端逐行认出来。`composeCardAnswerMessage`
 * （reading_coach.go）会拿每一行回文章里做字面核对，是原文的那几行会带上
 * `> ` 前缀 —— 这是 R4 的第三道闸：文章的句子绝不能以「她说的话」的身份
 * 进语料。所以引文**单独成行**，标签写在它自己那一行上。
 */
export function composeBoardAnswer(
  card: CoachCardSpec,
  placement: BoardPlacement,
  items: BoardItem[],
): string {
  const lines: string[] = [];
  for (const it of items) {
    const bin = placement[it.id];
    if (!bin) continue;
    if (card.type === "word_bank") {
      // 词不是句子，它不会被误认成原文的一整行，所以一行放得下。
      lines.push(`${it.text} — ${bin}`);
      continue;
    }
    // 🚨 标签一行，引文一行。写成「证据：「原文」」的话，那一行既不是纯粹的
    // 原文（前面多了两个字），也就核对不上，于是文章的句子会以她的话的身份
    // 落进语料 —— 正是 R4 那条闸要拦的东西。
    lines.push(`${bin}：`);
    lines.push(it.text);
  }
  return lines.join("\n");
}

export function CoachCard({
  card,
  onAnswer,
  answered,
  busy = false,
  stale = false,
  prefill,
}: {
  card: CoachCardSpec;
  /** 她点了/写了。调用方负责把它发出去并把 `answered` 传回来。 */
  onAnswer: (answer: CoachCardAnswer) => void;
  /** 已经答过了：显示她的选择，整张卡片停止响应。 */
  answered?: CoachCardAnswer | null;
  /** 这一轮还在飞——不接第二次作答。 */
  busy?: boolean;
  /** 她上一次在同一块板上摆好的那些，开局就摆着。见 carryOverPlacement。 */
  prefill?: BoardPlacement;
  /** 后面又来了一张还没答的卡：这张收起来，点一下能重新展开。 */
  stale?: boolean;
}) {
  const [draft, setDraft] = useState("");
  const [reopened, setReopened] = useState(false);
  const promptId = useId();
  const options = card.options ?? [];
  const done = Boolean(answered);
  // 已作答的卡片永远不折叠：她说过的话不该缩回去。
  const collapsed = stale && !done && !reopened;
  // 挂载时就已经答过 → 这是刷新回来的旧卡，别抢焦点（她刚进房间，不该被拽到
  // 某张旧卡上）。只有「在她眼前从未答变成已答」才需要有人接住焦点。
  const answeredAtMount = useRef(done);

  function answer(choice: string, blockId?: string) {
    if (done || busy || collapsed) return;
    onAnswer({ type: card.type, prompt: card.prompt, choice, ...(blockId ? { blockId } : {}) });
  }

  if (collapsed) {
    // 一行问题 + 一句邀请。没有「未完成」、没有计数、没有任何在记账的字眼——
    // 这是一扇还开着的门，不是一条对她的记账。
    return (
      <button
        type="button"
        data-coach-card="collapsed"
        aria-expanded={false}
        onClick={() => setReopened(true)}
        className="mk-coachcard my-2 flex w-full flex-col gap-0.5 rounded-mk-lg border border-mk-border px-3 py-2 text-left transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        style={{
          // mk-* 是裸 CSS 变量，alpha 语法一个字节都不生成：只能 color-mix。
          background: "color-mix(in srgb, var(--mk-accent-50) 35%, var(--mk-surface))",
          // 收起来的卡片保留那条竖杠，只是调轻：她一眼还认得出这是同一族东西
          // （一张卡片），而不是日志里又一句话。
          borderLeftWidth: "3px",
          borderLeftColor: "color-mix(in srgb, var(--mk-accent-500) 35%, transparent)",
        }}
      >
        <span className="text-mk-small leading-relaxed text-mk-muted">{card.prompt}</span>
        <span className="text-mk-small text-mk-faint">想回来答这个，点一下就行。</span>
      </button>
    );
  }

  return (
    <div
      role="group"
      data-coach-card={done ? "answered" : "open"}
      aria-labelledby={promptId}
      className="mk-coachcard my-2 flex flex-col gap-2 rounded-mk-lg border p-3"
      style={{
        // mk-* 是裸 CSS 变量：`bg-mk-accent-50/60` 这类 alpha 语法一个字节的
        // CSS 都不会生成（静默透明）。半透明只能走 color-mix。
        //
        // 这一层比气泡重一档，是有意的：真实截图里（`02-first-reply-with-card.png`）
        // 印记 的话是一个浅色气泡，卡片是**另一个**几乎一样浅的框，两者在同一列
        // 里分不出轻重，她读不出「这是在跟我说话」和「这是要我动手的东西」的差别。
        // 答过之后这张卡片退一档：它已经不再要她做什么了，屏幕上最重的那一张
        // 永远该是**现在轮到的**那一张。
        background: done
          ? "color-mix(in srgb, var(--mk-accent-50) 45%, var(--mk-surface))"
          : "color-mix(in srgb, var(--mk-accent-50) 88%, var(--mk-surface))",
        borderColor: `color-mix(in srgb, var(--mk-accent-500) ${done ? 20 : 34}%, transparent)`,
        // 左边那条实心竖杠是这张卡片最便宜、也最管用的身份标记：气泡永远不会有。
        borderLeftWidth: "3px",
        borderLeftColor: done
          ? "color-mix(in srgb, var(--mk-accent-500) 55%, transparent)"
          : "var(--mk-accent-500)",
        boxShadow: done ? "none" : "0 1px 3px color-mix(in srgb, var(--mk-accent-700) 10%, transparent)",
      }}
    >
      {/* 一眼可辨的小标签。它说的是「这一格要你动手」，不是一道题的题号——
          所以没有编号、没有计数，也永远不会有对错。答完就撤掉：它是一句邀请，
          不是一枚一直挂在那儿的印章。 */}
      {!done && (
        <span
          className="inline-flex w-fit items-center gap-1 rounded-mk-full px-2 py-0.5 text-mk-caption"
          style={{
            background: "color-mix(in srgb, var(--mk-accent-500) 16%, transparent)",
            color: "var(--mk-accent-700)",
          }}
        >
          动手
        </span>
      )}
      <p id={promptId} className="text-mk-body leading-relaxed text-mk-ink">
        {card.prompt}
      </p>

      {done && card.type === "choose_span" ? (
        <AnsweredOptions
          options={options}
          choice={answered!.choice}
          takeFocus={!answeredAtMount.current}
        />
      ) : done && (card.type === "label_roles" || card.type === "word_bank") ? (
        <AnsweredBoard card={card} choice={answered!.choice} takeFocus={!answeredAtMount.current} />
      ) : done ? (
        <HerAnswer answer={answered!} takeFocus={!answeredAtMount.current} />
      ) : card.type === "choose_span" ? (
        <>
          <ul className="flex flex-col gap-1.5">
            {options.map((o, i) => (
              <li key={`${o.blockId}-${i}`}>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => answer(o.quote, o.blockId)}
                  className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-left text-mk-small leading-relaxed text-mk-ink transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <OptionWhere where={o.where} />
                  {o.quote}
                </button>
              </li>
            ))}
          </ul>
          {/* 「点一句就行，怎么想都算你的。」→「请选择一句。」
              两条规矩同时指向这个改动（AGENTS.md §界面文案怎么写）：
              规矩 7「不要铺垫、不要撒娇、不要替她减压」——「就行」「都算你的」
              预设了她怕，而这个预设本身不尊重人；规矩 3「要她做事就用请+
              祈使句」。卡片上没有对错这件事由卡片自己保证（没有 ✓ / ✗ /
              分数，服务端也不发答案），不需要再用一句话去安抚。 */}
          <p className="text-mk-small text-mk-faint">请选择一句。</p>
        </>
      ) : card.type === "label_roles" || card.type === "word_bank" ? (
        <CoachBoard
          items={boardItems(card)}
          bins={card.type === "label_roles" ? card.labels ?? [] : WORD_BINS}
          itemLabel={
            card.type === "label_roles"
              ? "把每一句拖到它的角色下面。也可以先点一句，再点一个格子。"
              : "把每个词拖到你现在的状态下面。也可以先点一个词，再点一个格子。"
          }
          submitLabel={card.type === "label_roles" ? "摆好了" : "分好了"}
          busy={busy}
          prefill={prefill}
          onSubmit={(placement) =>
            answer(composeBoardAnswer(card, placement, boardItems(card)))
          }
        />
      ) : card.type === "pick_in_article" ? (
        // 🚨 一个方位词都不许有。这里曾经写着「在**左边**文章里点出那一句」，
        // 而文章在桌面端排在**右边**、手机上排在**下面**——两次真实走查开头的
        // 第一张卡都是 pick_in_article，所以那是学生看到的第一件事，而它把她
        // 指去了空白的那一边。她找不到 → 打字说「我读完了」→ hunt 判定正确地
        // 拒绝推进 → 印记 连着训她两次。
        // 布局本来就会随视口变，任何写死的方位词迟早都是错的：只说做什么。
        //
        // 🚨 而且必须说**对**那个动作。同事试用报的「选句子的指引不好」不是
        // 措辞问题——是这句话让她做的事和文章上真正发生的事**不是同一件**：
        //
        // 这张卡片出现在 `loop.status === "idle"` 的时候（它是 印记 写在回复里
        // 的卡片，不是一副敞开的透镜）。而在 idle 下，文章上「点一下某一段」
        // 走的是 `onReferenceBlock` → 弹出**段落工具条**；真正能把一个句子变成
        // 引用的手势是**划选**（`onReferenceSelection`，靠 mouseup 时的真实
        // 文本选区）。所以旧文案「点出那一句，点完它就会出现在对话里」照着做
        // 的结果是：跳出一排段落工具，对话里什么都没有出现。
        //
        // （透镜敞开时是另一回事：`selectMode` 下一次点击确实会取走光标所在的
        // 那一句——见 Annotate 的 `pickSentence`。两种状态两种手势，所以这句
        // 话只能说 idle 下的那一种。）
        <p className="text-mk-small text-mk-faint">
          请在文章里划选那一句：按住并拖动选中文字，它会成为引用，随你的下一句话一起发出。
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          <textarea
            // placeholder 不是可及名称的归宿：读屏用户要听见的是 印记 问的那
            // 个问题，不是「写一句你自己的话就够了」。
            aria-label={card.prompt}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            // 🚨 回车发送，Shift+回车换行 —— 和输入框（Composer）同一套手势。
            // 产品负责人 2026-09-17：「动手部分，按回车键无法发送，需要点击
            // 发送按钮才能发送。」这个框和下面那个输入框长得一样、挨在一起，
            // 一个认回车一个不认，她只会以为发送坏了。
            //
            // 🚨 `isComposing`：中文输入法用回车上屏候选词。不挡的话，她打
            // 「礼貌」按回车选词，选的那一下就把半句话发出去了。
            onKeyDown={(e) => {
              if (e.key !== "Enter" || e.shiftKey) return;
              if (e.nativeEvent.isComposing) return;
              e.preventDefault();
              if (busy) return;
              const text = draft.trim();
              if (!text) return;
              setDraft("");
              answer(text);
            }}
            rows={2}
            disabled={busy}
            placeholder="用中文写一句你自己的话（回车发送，Shift+回车换行）"
            className="mk-scroll w-full resize-none rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small leading-relaxed text-mk-ink placeholder:text-mk-faint focus:border-mk-accent-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:opacity-60"
          />
          <div className="flex justify-end">
            <button
              type="button"
              disabled={busy}
              onClick={() => {
                const text = draft.trim();
                // 一句空白不是作答：发出去只会变成一条空的学生消息。
                if (!text) return;
                setDraft("");
                answer(text);
              }}
              className="rounded-mk-full px-3 py-1 text-mk-small text-white transition-opacity duration-[120ms] ease-mk hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed disabled:opacity-60"
              style={{ background: "var(--mk-accent-500)" }}
            >
              说说看
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/** 选项前面那个段号。没有（老消息、生词板）就整个不渲染。 */
function OptionWhere({ where }: { where?: string }) {
  if (!where) return null;
  return (
    <span
      className="mr-1.5 inline-block rounded-mk-full px-1.5 py-px align-[1px] text-mk-caption"
      style={{
        background: "color-mix(in srgb, var(--mk-accent-500) 12%, transparent)",
        color: "var(--mk-accent-700)",
      }}
    >
      {where}
    </span>
  );
}

/**
 * 她答过之后的那张 choose_span：**几个选项全部留着**，她点的那一个标出来。
 *
 * 🚨 这推翻了 2026-08 的那条设计。原来这里只留她选的那一句，理由写着：
 * 「把它们并排摆着，中间还有一个被标出来的，屏幕上读起来就是『答案对照表』。」
 * 真用下来是反的 —— 产品负责人 2026-09-17 逐字报的：「阅读卡片选择以后无法
 * 看到其他选项（贴句子的卡片也是），无法回退。」
 *
 * 代价有两处，都在她那边：
 *
 *  - 印记 下一轮讲的往往就是**几个选项之间的差别**（「古训、俗话，跟你选的
 *    第 3 句，在让人相信这件事上不一样」）。选项一收走，她手上只剩自己那一句，
 *    只能往回翻 —— 而截图里 印记 索性把三个选项在对话里重抄了一遍，那正是
 *    屏幕上缺了东西的症状。
 *  - 她想回头看看自己当时在什么里面选的，屏幕上没有那个东西了。
 *
 * 「答案对照表」那个担心靠别的东西挡住，而且比收走选项挡得更准：**这里仍然
 * 没有 ✓、没有 ✗、没有分数**（铁律②），没被选的几条也不带任何「错」的记号，
 * 只是退到背景里。标出来的是「你选的」，不是「对的」。
 */
function AnsweredOptions({
  options,
  choice,
  takeFocus = false,
}: {
  options: CoachCardOption[];
  choice: string;
  takeFocus?: boolean;
}) {
  // 焦点：同 HerAnswer —— 她按的那个按钮在作答之后被卸载了，没人接住就掉回
  // <body>，键盘和读屏用户答完一题会回到文档顶部。
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (takeFocus) ref.current?.focus?.();
  }, [takeFocus]);
  const picked = choice.trim();
  // 她选的那一句不在选项里（兜底卡、或者正文换过了）→ 退回只显示她那一句，
  // 而不是把一组和她无关的选项摆在那儿。
  const matched = options.some((o) => o.quote.trim() === picked);
  return (
    <div
      ref={ref}
      role="status"
      tabIndex={-1}
      className="flex flex-col gap-1.5 rounded-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      {!matched && <span className="text-mk-small text-mk-faint">你选的</span>}
      {(matched ? options : [{ blockId: "", quote: picked } as CoachCardOption]).map((o, i) => {
        const mine = o.quote.trim() === picked;
        return (
          <div
            key={`${o.blockId}-${i}`}
            className="rounded-mk-md border-l-2 px-3 py-2 text-mk-small leading-relaxed"
            style={
              mine
                ? {
                    borderLeftColor: "var(--mk-accent-400)",
                    background: "color-mix(in srgb, var(--mk-surface) 88%, transparent)",
                    color: "var(--mk-ink)",
                  }
                : {
                    // 没被选的那几条：退到背景里，但仍然读得清。它们不是错的，
                    // 所以没有任何表示「错」的记号 —— 只是不是她挑的那一句。
                    borderLeftColor: "transparent",
                    background: "transparent",
                    color: "var(--mk-muted)",
                  }
            }
          >
            {mine && <span className="mr-1.5 text-mk-caption text-mk-faint">你选的</span>}
            <OptionWhere where={o.where} />
            {o.quote}
          </div>
        );
      })}
    </div>
  );
}

/** 她摆完之后的那块板：同样的格子、同样的卡片、同样的位置，只是不能再动。
 *  见 `CoachBoardRecap`。 */
function AnsweredBoard({
  card,
  choice,
  takeFocus = false,
}: {
  card: CoachCardSpec;
  choice: string;
  takeFocus?: boolean;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (takeFocus) ref.current?.focus?.();
  }, [takeFocus]);
  const items = boardItems(card);
  const placement = parseBoardAnswer(card, choice);
  const bins = card.type === "label_roles" ? card.labels ?? [] : WORD_BINS;
  // 一行都没认出来（格式变过、老数据）→ 退回原样显示她那段作答，而不是摆一块
  // 空板。空板读起来是「你什么都没摆」，那是假的。
  if (Object.keys(placement).length === 0 || bins.length === 0) {
    return <HerAnswer answer={{ type: card.type, prompt: card.prompt, choice }} takeFocus={takeFocus} />;
  }
  return (
    <div
      ref={ref}
      role="status"
      tabIndex={-1}
      className="flex flex-col gap-1 rounded-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span className="text-mk-small text-mk-faint">你摆的</span>
      <CoachBoardRecap items={items} bins={bins} placement={placement} />
    </div>
  );
}

/**
 * 她答过之后卡片剩下的东西：**只有她的选择**。
 *
 * 🚨 这一种现在只服务 `short_text` 和 `pick_in_article` —— 这两种本来就没有
 * 选项可留（一个是她自己写的，一个是她自己到正文里划的）。choose_span 和两块
 * 板走 AnsweredOptions / AnsweredBoard，见那两个函数上面的注释。
 */
function HerAnswer({ answer, takeFocus = false }: { answer: CoachCardAnswer; takeFocus?: boolean }) {
  const hersInHerOwnWords = answer.type === "short_text";
  // 🚨 她按的那个按钮在作答之后被卸载了。什么都不接手的话，焦点掉回 `<body>`
  // ——用键盘或读屏的学生答完一题，回到文档顶部，而且没有任何提示告诉她刚才
  // 发生了什么。`role="status"` 让这块被念出来，`tabIndex={-1}` 让它能被聚焦
  // 而不进 tab 序列。
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (takeFocus) ref.current?.focus?.();
  }, [takeFocus]);
  return (
    <div
      ref={ref}
      role="status"
      tabIndex={-1}
      className="flex flex-col gap-1 rounded-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span className="text-mk-small text-mk-faint">{hersInHerOwnWords ? "你写的" : "你选的"}</span>
      <p
        className="rounded-mk-md border-l-2 px-3 py-2 text-mk-small leading-relaxed text-mk-ink"
        style={{
          borderLeftColor: "var(--mk-accent-400)",
          background: "color-mix(in srgb, var(--mk-surface) 80%, transparent)",
        }}
      >
        {hersInHerOwnWords ? answer.choice : `“${answer.choice}”`}
      </p>
    </div>
  );
}
