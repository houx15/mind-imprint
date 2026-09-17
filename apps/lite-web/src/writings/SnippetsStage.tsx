import { useEffect, useRef, useState } from "react";
import { Plus, HelpCircle, Eye, LayoutGrid } from "lucide-react";
import { Button, EmptyState, Icon } from "@/ui";
import { countWords } from "@/workspace/blocks/wordcount";
import { wordUnit } from "./wordUnit";
import { useAlive } from "../shared/useAlive";
import { GuideBox } from "./GuideBox";
import { CommentPanel } from "./CommentPanel";
import { DeepenDrawer } from "./DeepenDrawer";
import { RoleBoard } from "./RoleBoard";
import { buildSlots, type Slot } from "./slots";
import { splitSentences, ROLE_BOARD_MIN } from "./sentences";
import { registerPendingSave } from "./pendingSaves";
import { handleWriteError } from "./writeErrors";
import {
  putWritingSnippet,
  guideWritingBlock,
  guideWritingBlocks,
  commentOnWritingSnippet,
  listWritingComments,
  type Comment,
  type WritingOutlineItem,
  type WritingSnippet,
  type WritingBlockGuide,
  type WritingBoardKind,
} from "../api/writingRoom";

/**
 * SnippetsStage — 段落.
 *
 * The 2026-08-27 note that reshaped this file: *"snippets is important, the
 * key is the AI-generated guiding box, instead of letting students write
 * paragraph by paragraph."* The old shape was the second thing — a bare
 * textarea under a heading, and a student staring at a cursor. What was
 * missing wasn't a bigger box; it was something to think *about*.
 *
 * ## Guidance is PRESENT ON ARRIVAL (Task 11)
 *
 * It used to be that the only way to see a guide was to find and press
 * 「卡住了？」 — which meant the student who most needed it (the one who does
 * not know what she is allowed to ask for) was the one least likely to get
 * it. Now:
 *
 *   - every block's guide is **stored** server-side and arrives on
 *     `GET /outline` (`WritingOutlineItem.guide`), so on any later visit it
 *     is simply painted, with no call and no click;
 *   - the first time a piece reaches 段落 with nothing stored yet, the BATCH
 *     route (`POST /writings/{id}/guide`, one model call for the whole
 *     outline) runs once by itself. She should not have to ask to be taught.
 *   - 「卡住了？」 survives as **regenerate this one block** — a second opinion
 *     when the first set of questions didn't land — not as the way in.
 *
 * ## 请印记看看这一段 (B4)
 *
 * The same structured critique 成稿 gets on the whole piece, at paragraph
 * zoom — one summary line plus points, each anchored to a sentence she
 * actually wrote (the server drops any point whose quote is not a literal
 * substring). It renders through the SAME `CommentPanel` 成稿 uses; only the
 * trace differs, because there is no `ProseSurface` here to highlight into.
 * Comments persist, so they are fetched on arrival rather than living only in
 * the seconds after she presses the button.
 *
 * 铁律① IS ENFORCED IN THIS FILE: GuideBox renders `guide.questions`, and the
 * server has already dropped anything that isn't a question
 * (writing_guide.go's parseWritingGuide). A question cannot be pasted into an
 * essay; a sentence can. The guarantee is the output TYPE, not a promise.
 *
 * There are no 工具卡 in this room at all (2026-08-27): pro's writing surface
 * barely used them, and a student stuck on a paragraph wants a question, not a
 * form to fill in.
 */

/** The guides the server already stored, keyed by outline row id. */
function storedGuides(outline: WritingOutlineItem[]): Record<string, WritingBlockGuide> {
  const out: Record<string, WritingBlockGuide> = {};
  for (const o of outline) {
    if (o.guide) out[o.id] = o.guide;
  }
  return out;
}

export function SnippetsStage({
  writingId,
  outline,
  snippets,
  onSnippetsChange,
  lang,
  onGoToStructure,
  onGoToDraft,
  onSay,
  onLocked,
}: {
  writingId: string;
  outline: WritingOutlineItem[];
  snippets: WritingSnippet[];
  onSnippetsChange: (next: WritingSnippet[]) => void;
  /** 一块板摆完了：把结果当成她说的一句话发出去，印记 在右栏接住它。 */
  onSay: (text: string, board?: WritingBoardKind) => Promise<void>;
  /** Sends her to 结构 from the empty state — naming the step she needs is
   *  not the same as getting her there. */
  /** 这一篇是中文还是英文 —— 决定篇幅按「字」还是「词」说。见 wordUnit.ts。 */
  lang: string;
  onGoToStructure: () => void;
  /** 去成稿。不是关卡 —— 顶上那条导航一直都能点。 */
  onGoToDraft: () => void;
  /** The deadline passed while she was mid-edit: every write below (and in
   *  each `SnippetBlock`) reloads the room into the locked finished page
   *  instead of showing a raw error on a page that can no longer save. */
  onLocked?: () => void;
}) {
  const slots = buildSlots(outline, snippets);

  /**
   * 现在开着的是**哪一块**的标注板。一次只开一块。
   *
   * 🚨 走查里她同时开了两块板（截图上下各一块）。两个后果：
   * 屏幕上一次摆着十个格子，她不知道哪一组对应哪一段；
   * 而且「摆完这一块」这件事没有终点 —— 她在两块之间来回点，
   * 一块都没交上去。板是被递过来的一件事，不是一排可以同时摊开的抽屉。
   */
  const [openBoardFor, setOpenBoardFor] = useState<string | null>(null);

  /**
   * Guides live here rather than inside each block, because the batch call
   * answers for the WHOLE outline at once and every block has to be able to
   * receive its share. Seeded from what the server already stored; a locally
   * regenerated guide wins over the stored one it replaced.
   */
  const [guides, setGuides] = useState<Record<string, WritingBlockGuide>>(() => storedGuides(outline));
  useEffect(() => {
    setGuides((prev) => ({ ...storedGuides(outline), ...prev }));
  }, [outline]);

  const [batching, setBatching] = useState(false);
  const [batchError, setBatchError] = useState<string | null>(null);
  // One attempt per mount. A failed batch must not turn into a retry loop
  // that bills a model call every render, and a piece whose outline genuinely
  // produced nothing must not be asked again on every keystroke.
  const batchTried = useRef(false);
  /**
   * Deliberately NOT a per-invocation `let cancelled = false` cleanup flag.
   *
   * THE TRAP (it hung this exact box for the whole 180s of the writing walk,
   * 2026-08-28): `batchTried` and a `cancelled` closure disagree under
   * StrictMode's mount → cleanup → remount. Pass 1 sets the latch and fires
   * the one real `/guide` call; the cleanup marks pass 1's closure cancelled;
   * pass 2 is skipped *because the latch is already set*. The single in-flight
   * request then lands in the only closure watching it — the cancelled one —
   * so `setGuides`/`setBatching(false)` are both dropped and the room sits on
   * 「印记正在把每一块都先想一遍」 forever, on a 200 the server answered
   * perfectly. `useAlive` is restored to true by the remount and only goes
   * false on a real unmount, so the latch and the guard can no longer
   * contradict each other. Full write-up in `shared/useAlive.ts`.
   */
  const alive = useAlive();

  const anyGuide = outline.some((o) => guides[o.id]);
  const needsBatch = outline.length > 0 && !anyGuide;

  useEffect(() => {
    if (!needsBatch || batchTried.current) return;
    batchTried.current = true;
    setBatching(true);
    void guideWritingBlocks(writingId)
      .then((next) => {
        if (alive.current) setGuides((prev) => ({ ...next, ...prev }));
      })
      .catch((err: unknown) => {
        // Surfaced, never masked: 「卡住了？」 still works per block, and
        // saying so is more useful than a page that silently teaches nothing.
        if (alive.current) handleWriteError(err, onLocked, setBatchError);
      })
      .finally(() => {
        if (alive.current) setBatching(false);
      });
  }, [needsBatch, writingId, alive, onLocked]);

  /** Which block, if any, has 深入一层 open. */
  const [deepen, setDeepen] = useState<{ outlineId: string; heading: string } | null>(null);

  /**
   * 印记's comments on individual paragraphs, keyed by snippet id — the
   * newest one per block.
   *
   * Fetched on arrival rather than only held from the moment she presses the
   * button: a comment is PERSISTED (migration 0102), and feedback that
   * silently disappears when she comes back tomorrow is the exact failure
   * `POST /review`'s old `{"feedback": "<prose>"}` had. `GET /comments`
   * returns both zoom levels newest-first, so the first row seen for a
   * snippet is the one to keep and the draft-scope rows are skipped here —
   * they belong to 成稿.
   */
  const [comments, setComments] = useState<Record<string, Comment>>({});
  useEffect(() => {
    let cancelled = false;
    void listWritingComments(writingId)
      .then((rows) => {
        if (cancelled) return;
        const byBlock: Record<string, Comment> = {};
        for (const c of rows) {
          if (c.scope !== "block" || !c.snippetId) continue;
          if (!byBlock[c.snippetId]) byBlock[c.snippetId] = c;
        }
        setComments(byBlock);
      })
      // Not worth an error banner: nothing she did failed, and 请印记看看这一段
      // still works. Silence here beats an alarm about a page she never asked
      // to load.
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [writingId]);

  // Free paragraphs live in a position range an outline can never reach.
  //
  // position is writing_snippet's upsert key, and outline positions are just
  // array indices 0..N-1 reassigned on every outline save. So "one past the
  // current maximum" is not safe: a free paragraph minted at position 1 while
  // the outline has one block sits exactly where a SECOND block will land the
  // next time the structure grows — and the first save of that new slot then
  // upserts onto her free paragraph's row, destroying its text and relinking
  // it to a heading she never wrote it under. Silent, and the kind of loss
  // she would only notice much later.
  const FREE_POSITION_BASE = 1000;
  const freePositions = snippets.map((s) => s.position).filter((p) => p >= FREE_POSITION_BASE);
  const nextFreePosition = freePositions.length === 0 ? FREE_POSITION_BASE : Math.max(...freePositions) + 1;

  const [addError, setAddError] = useState<string | null>(null);
  async function addFreeParagraph() {
    setAddError(null);
    try {
      onSnippetsChange(await putWritingSnippet(writingId, { position: nextFreePosition, text: "" }));
    } catch (err) {
      handleWriteError(err, onLocked, (message) => setAddError(`添加失败：${message}`));
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <h2 className="text-mk-h2 text-mk-ink">段落</h2>
        {/* 🚨 她一进来撞见的是八个空框。2026-09-13 第十三轮走查里三步在问
            同两件事：「不知道是不是要按顺序全写完才算一段」「不确定应该按顺序
            从第一块开始写还是先写中心论点那块」。
            原来这句写的是「一块一块来…照着想就行」—— 说了节奏，没说规则。
            她要的是两条事实：顺序归她，存盘各自独立。 */}
        <p className="text-mk-body text-mk-muted">一块一段，各自保存，先写哪一块都可以。每一块上面写着它要做的事。</p>
      </div>

      {batching && (
        <p className="text-mk-body text-mk-muted" role="status">
          印记正在把每一块都先想一遍…
        </p>
      )}
      {batchError && (
        <div role="alert" className="rounded-mk-sm px-3 py-2 text-mk-small text-mk-danger" style={{ background: "var(--mk-danger-bg)" }}>
          {batchError}
          <button type="button" className="ml-3 underline" onClick={() => setBatchError(null)}>
            知道了
          </button>
        </div>
      )}

      {/* The design system's own empty state, illustration and all — a bare
          dashed box with a sentence in it is the shape this page is supposed
          to avoid, and an empty stage is exactly where a student needs the
          most warmth rather than the least. `action` sends her to the step
          that actually unblocks her instead of only naming it. */}
      {slots.length === 0 && (
        <EmptyState
          illustration="writing"
          title="还没有可以写的块"
          body="段落是跟着结构里的每一块写的。先去把思路理一理，或者直接加一段自由写。"
          action={{ label: "去理思路", onClick: onGoToStructure }}
        />
      )}

      <div className="flex flex-col gap-5">
        {slots.map((slot) => {
          const oid = slot.outlineId;
          return (
            <SnippetBlock
              key={`${oid ?? "free"}-${slot.position}`}
              writingId={writingId}
              slot={slot}
              guide={oid ? (guides[oid] ?? null) : null}
              onGuide={(next) => {
                if (oid) setGuides((prev) => ({ ...prev, [oid]: next }));
              }}
              onDeepen={() => {
                if (oid) setDeepen({ outlineId: oid, heading: slot.heading });
              }}
              comment={slot.snippet ? (comments[slot.snippet.id] ?? null) : null}
              onCommented={(c) => {
                const sid = c.snippetId;
                if (sid) setComments((prev) => ({ ...prev, [sid]: c }));
              }}
              onSaved={onSnippetsChange}
              onSay={onSay}
              lang={lang}
              boardKey={`${oid ?? "free"}-${slot.position}`}
              openBoardFor={openBoardFor}
              onBoardOpen={setOpenBoardFor}
              onLocked={onLocked}
            />
          );
        })}
      </div>

      {/* 每一块都写好了 → 屏幕上出现一条真的邀请。
          🚨 这是结构那一步「去写」那条邀请的同一件事，在下一个接缝上。
          产品负责人当时的判断是：那颗按钮从第一秒就在，**但从来没有人提议过它**，
          于是一个已经做完的学生会继续在原地待着。段落这一步是同一个形状 ——
          2026-09-11 走查里她两次走到这儿停住：
          「每段都写完了，但没有一个按钮能把它们拼成整篇文章或者进入下一步」。
          顶上那排「结构 / 段落 / 成稿」一直可点，她只是没把它读成「下一步」。

          ⚠️ 它不替她走。按不按仍然是她的事，导航也照旧。 */}
      {slots.length > 0 && slots.every((s) => (s.snippet?.text ?? "").trim() !== "") && (
        <div
          className="flex flex-wrap items-center gap-3 rounded-mk-lg border p-3"
          style={{
            // mk-* 是裸 CSS 变量：Tailwind 的 alpha 语法对它们一个字节的 CSS
            // 都不生成，半透明只能走 color-mix。
            background: "color-mix(in srgb, var(--mk-accent-50) 80%, var(--mk-surface))",
            borderColor: "color-mix(in srgb, var(--mk-accent-500) 30%, transparent)",
          }}
        >
          <span className="text-mk-body text-mk-ink">每一块都写好了。下一步把它们拼成整篇。</span>
          <Button size="sm" onClick={onGoToDraft}>
            去成稿
          </Button>
        </div>
      )}
      {/* 2026-09-18 走查：写了三块、字数够了、剩两块空着 —— 她连着五步找不到
          「完成这篇」，印记还让她「删掉空白块」（这里删不了块）。完成在成稿那一步，
          空着的块拼成稿时自动跳过；这两件事屏幕上要直接说出来。 */}
      {slots.some((s) => (s.snippet?.text ?? "").trim() !== "") &&
        !slots.every((s) => (s.snippet?.text ?? "").trim() !== "") && (
          <div className="flex flex-wrap items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-3">
            <span className="text-mk-small text-mk-secondary">
              需要的段落写完后，请到「成稿」把它们拼成整篇，并在那里点「完成这篇」提交。空着的块不会进入成稿。
            </span>
            <Button size="sm" variant="secondary" onClick={onGoToDraft}>
              去成稿
            </Button>
          </div>
        )}

      <button
        type="button"
        onClick={() => void addFreeParagraph()}
        className="flex w-fit items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-accent-700 hover:bg-mk-accent-50"
      >
        <Icon icon={Plus} size={14} /> 加一段
      </button>
      {addError && (
        <p role="alert" className="break-words text-mk-small text-mk-danger">
          {addError}
        </p>
      )}

      {deepen && (
        <DeepenDrawer
          writingId={writingId}
          outlineId={deepen.outlineId}
          heading={deepen.heading}
          onClose={() => setDeepen(null)}
          onLocked={onLocked}
        />
      )}
    </div>
  );
}

function SnippetBlock({
  writingId,
  slot,
  guide,
  onGuide,
  onDeepen,
  comment,
  onCommented,
  onSaved,
  onSay,
  boardKey,
  openBoardFor,
  onBoardOpen,
  lang,
  onLocked,
}: {
  writingId: string;
  slot: Slot;
  /** Whatever guidance this block already has — stored from the server or
   *  just regenerated. Null only for a block that has never been guided (and
   *  for free paragraphs, which have no outline row to guide). */
  guide: WritingBlockGuide | null;
  onGuide: (next: WritingBlockGuide) => void;
  onDeepen: () => void;
  /** The newest stored comment on THIS paragraph, if 印记 has looked at it. */
  comment: Comment | null;
  onCommented: (next: Comment) => void;
  onSaved: (next: WritingSnippet[]) => void;
  onSay: (text: string, board?: WritingBoardKind) => Promise<void>;
  /** 这一块在「谁的板开着」里的名字。 */
  boardKey: string;
  lang: string;
  openBoardFor: string | null;
  onBoardOpen: (key: string | null) => void;
  /** The deadline passed while she was mid-edit: every write below reloads
   *  the room into the locked finished page. */
  onLocked?: () => void;
}) {
  const [text, setText] = useState(slot.snippet?.text ?? "");
  const [saving, setSaving] = useState(false);
  const [guiding, setGuiding] = useState(false);
  const [commenting, setCommenting] = useState(false);
  const [boardBusy, setBoardBusy] = useState(false);
  const boardOpen = openBoardFor === boardKey;
  const setBoardOpen = (on: boolean) => onBoardOpen(on ? boardKey : null);
  // 拆句是纯函数、很便宜，所以每次渲染算一遍就行——把它记忆化只会多一个
  // 会和 text 失去同步的地方。
  const sentenceCount = splitSentences(text).length;
  // 「已保存」＝ 服务端那一行的字和框里的字一模一样。比一个 savedAt 时间戳
  // 诚实：她改了一个字，这行就自己消失，不会留下一句过期的「已保存」。
  const saved = text.trim() !== "" && slot.snippet?.text === text;
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  /**
   * 收起 hides the box; it does not throw the guidance away. Regenerating on
   * the way back in would charge a model call to see something we already
   * have, so the button becomes 「打开引导」 instead.
   *
   * 🚨 **已经写过的那一块，进来时就折着。**
   *
   * 引导那几个问题是对着**一张白纸**写的（「你见过哪一次？」「那天几点？」）。
   * 她写完三百字之后，那几个问题还挂在正文框上面 —— 而它们看起来跟印记的
   * 反馈是一类东西。2026-09-13 第十三轮线上走查，两个学生一共三步在说这个：
   *
   *   「左侧的意见卡片[18][19][20]好像还是旧版的提示，没跟着我的正文一起更新，
   *     看着有点乱」
   *   「下面那个卡片18还显示旧的提示，看着有点乱」
   *
   * 她没说错：那几个问题**确实**不会跟着她的正文变，它们也不该变 ——
   * 它们不是反馈，是开工前的脚手架。脚手架该在她开工之后让开。
   *
   * ⚠️ 这不动「引导一进来就在」那条（Task 11：最需要它的那个学生，正是最不会
   * 主动去点的那个）—— 空白的块照旧摊开。折的只是**她已经写过的**那些块，
   * 而且只在进来那一刻决定一次：写到一半把她眼前的东西收走，比留着更糟。
   */
  const [collapsed, setCollapsed] = useState(() => (slot.snippet?.text ?? "").trim() !== "");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // 🚨 **她手正放在这个框里的时候，绝不把服务端那一份写回去。**
    //
    // 这一行原来是无条件 setText。在只有「失焦才存」的年代它是安全的：
    // 存的那一刻她已经不在框里了。下面加了打字停顿自动存之后，这一条就成了
    // 一个吞字的入口 —— 第一次存会把 snippet 行建出来，`slot.snippet?.id`
    // 从 undefined 变成一个 id，这个 effect 于是跑一次，把她**在那一次往返
    // 期间又敲的字**覆盖回存下去的那一版。
    //
    // 丢掉她写的字，是这一整轮里最不能犯的一类错（她跟印记说过三次「我的字
    // 被截断了」，那次只是没显示，这次会是真的没了）。所以宁可让框里那一份
    // 留着不同步：她在打字，框里那份就是最新的。
    if (textareaRef.current !== null && document.activeElement === textareaRef.current) return;
    setText(slot.snippet?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slot.snippet?.id]);

  /**
   * 打字停下来就存，不必等她离开这个框。
   *
   * 🚨 顶上那个「已写 N / 目标 M」数的是**服务端存着的**东西（草稿正文，
   * 没有就把各段加起来）。只在失焦时存，意味着她正在打的这一段服务端还没有，
   * 于是同一屏上两个数字互相打脸 —— 2026-09-12 第二十九轮她的原话：
   *
   *	「已写还是0，但我明明看到第二段有85字了」
   *
   * 这一块自己那行小字是当场算的（`countWords(text)`），所以她看见的是
   * 「这一段 85 字」和「已写 0」并排摆着。她的下一句是「不知道会不会影响保存」。
   *
   * 顺带把真的风险也堵上：在这之前，她写完一段却没点进别处，那段字**服务端
   * 一个字都没有**。
   *
   * 存下去会把 snippet 行建出来（`slot.snippet?.id` 从无到有），上面那个
   * effect 因此会跑一次 —— 那里的焦点判断就是为这一刻加的，别删。
   */
  useEffect(() => {
    if (text === (slot.snippet?.text ?? "")) return;
    const t = setTimeout(() => void save(), 1200);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, slot.snippet?.text]);

  /**
   * 🚨 **她一敲完就问印记的时候，防抖还没到。**
   *
   * 上面那个 1.2 秒是给「停下来」用的；她打完最后一个字直接去跟印记说话，
   * 这一轮的上文里就没有刚敲的那句。第三十八轮十六条卡壳里十条是这个 ——
   * 「框里明明已经有让步的句子了，印记还说我缺让步」。
   *
   * 所以把「存这一块」登记出去，`say()` 发消息之前会等它。
   * 卸载时注销，见 pendingSaves.ts 里那段。
   */
  useEffect(() => {
    return registerPendingSave(async () => {
      if (text === (slot.snippet?.text ?? "")) return;
      await save();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, slot.snippet?.text]);

  /**
   * Persist this block's text. Returns the saved row for THIS slot (or null
   * if the save failed), because 请印记看看这一段 needs the snippet id and the
   * comment endpoint is keyed on it — a block she has typed into but never
   * blurred has no row on the server at all.
   */
  async function save(): Promise<WritingSnippet | null> {
    setSaving(true);
    setError(null);
    try {
      const saved = await putWritingSnippet(writingId, {
        // Only send outlineId to ESTABLISH a link, on this slot's very first
        // save (no persisted row yet). Once a snippet row exists, omit it —
        // the PUT's "absent outlineId = preserve whatever link is already
        // there" semantics then apply, so an ordinary text save can never
        // clobber a link the server holds (including one it just repaired by
        // heading text after a structure change) with a value merely inferred
        // client-side.
        outlineId: slot.snippet ? undefined : slot.outlineId,
        position: slot.position,
        text,
      });
      onSaved(saved);
      // Same by-id-never-by-position discipline buildSlots uses: match on
      // outlineId when this slot has one, and only fall back to position for
      // a free paragraph, which has nothing else to be matched by.
      return (
        saved.find((s) => (slot.outlineId ? s.outlineId === slot.outlineId : s.position === slot.position)) ?? null
      );
    } catch (err) {
      handleWriteError(err, onLocked, setError);
      return null;
    } finally {
      setSaving(false);
    }
  }

  /**
   * 请印记看看这一段 — B4 at the paragraph zoom level.
   *
   * Saves first, deliberately: the endpoint needs a snippet row and judges
   * the text the SERVER holds, so commenting on a stale save would anchor
   * every point to sentences she has since rewritten. Empty text is answered
   * here rather than by burning a call the server will refuse anyway.
   */
  async function askForComment() {
    if (!text.trim()) {
      setError("这一段还没有内容，先写点什么再来看看。");
      return;
    }
    setCommenting(true);
    setError(null);
    try {
      const row = slot.snippet && slot.snippet.text === text ? slot.snippet : await save();
      if (!row) return; // save() already surfaced why.
      onCommented(await commentOnWritingSnippet(writingId, row.id));
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setCommenting(false);
    }
  }

  /**
   * Clicking a point traces it back to the sentence it is about.
   *
   * 成稿 hands the quote to `ProseSurface`; there is no prose surface here,
   * only a textarea, so the honest equivalent is to focus it and select the
   * quoted range. **A quote that cannot be located does nothing visible** —
   * no scroll, no approximate highlight. That is the same line the server
   * holds when it drops points whose quote is not a literal substring
   * (validateCommentPoints): a trace landing on the neighbouring sentence is
   * worse than no trace, because it teaches her something false about her own
   * paragraph. The miss is real — she may have edited the text since the
   * comment was generated — and silence is the correct answer to it.
   */
  function trace(quote: string) {
    const el = textareaRef.current;
    if (!el) return;
    const at = el.value.indexOf(quote);
    if (at < 0) return;
    el.focus();
    el.setSelectionRange(at, at + quote.length);
  }

  async function regenerate() {
    if (!slot.outlineId) {
      // A free paragraph has no block to reason about — the guide endpoint is
      // keyed on an outline row. Say so rather than firing a call that 404s.
      setError("这是一段自由写的段落，先把它挂到「结构」里的某一块上，印记才知道该往哪个方向问。");
      return;
    }
    setGuiding(true);
    setError(null);
    try {
      onGuide(await guideWritingBlock(writingId, slot.outlineId));
      setCollapsed(false);
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setGuiding(false);
    }
  }

  const showGuide = guide !== null && !collapsed;

  return (
    /* data-write-block 是给走查用的：**这一块的标题，和这一块里的框、按钮，
       是一组**。真人一眼就看见它们在同一张卡片里；模拟学生那只眼睛原来只拿到
       一串拉平的按钮和输入框，于是报「有三个『标一下这一段』按钮，不知道是不是
       都要点」「『中心论点』那个引导到底管哪一段」。屏幕上分了组，读屏的人
       没读到，记下来就成了产品的毛病。见 e2e/writewalk/screen.ts。 */
    <div
      data-write-block={slot.heading || "自由段落"}
      className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-baseline gap-2">
          {slot.role && (
            <span
              className="shrink-0 rounded-mk-xs px-1.5 py-0.5 text-mk-label font-semibold"
              style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-700)" }}
            >
              {slot.role}
            </span>
          )}
          {/* 🚨 号要摆出来 —— 印记 说的是「第 N 块」。
              第三十六轮中文那一路：「印记说的『第3段最后那两句』跟我现在看到的
              第一段最后一句有点像，不确定它到底在说哪一段，有点乱。」
              抬头上原来只有结构那一步的标题，一个数字都没有，于是她只能自己数；
              而空的块也占位置，数出来常常对不上。
              这个号是它在屏幕上的顺序，和服务端 writingBlockNumbers 同一条规则。 */}
          <span className="shrink-0 text-mk-small text-mk-faint">第 {slot.number} 块</span>
          <span className="truncate text-mk-small font-semibold text-mk-ink">{slot.heading || "自由段落"}</span>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {/*
            🚨 自由段落上**不摆**这颗按钮。

            同事试用：「snippet cards: 不是点了卡住了才开始引导的，太离谱了」。
            有提纲的块早就不是这样了——`guideWritingBlocks` 在进页面时就把
            整篇每一块的引导批量取回来，「卡住了？」只剩「换一组问题」的意思。
            但**自由段落**不在那一批里：它没有 outline 行，`guide` 永远是
            null，所以它一直显示「卡住了？」；而按下去 `regenerate()` 会直接
            拒绝（它需要 `slot.outlineId`），只回一句让她先去挂到「结构」上。

            也就是说这颗按钮在这里是一句**空头承诺**：长得像「点我给你引导」，
            点了却告诉她这儿要不到引导。她的读法只会是「引导要点了才来，而且
            点了还不来」。所以这里换成把条件直接说清楚的一行字——按
            AGENTS.md §界面文案怎么写：名词开头的状态，加一句「请+祈使」的
            出路，不留一颗会失败的按钮。
          */}
          {!slot.outlineId ? (
            <span className="text-mk-small text-mk-faint">
              自由段落 · 请挂到「结构」中的某一块后获取引导
            </span>
          ) : guide !== null && collapsed ? (
            <Button variant="secondary" size="sm" onClick={() => setCollapsed(false)}>
              打开引导
            </Button>
          ) : (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => void regenerate()}
              loading={guiding}
              iconStart={<Icon icon={HelpCircle} size={14} />}
            >
              {guide === null ? "获取引导" : "换一组问题"}
            </Button>
          )}
          {/* 请印记看看这一段 — the same critique 成稿 gets on the whole piece,
              at paragraph zoom. It comes AFTER the guide button on purpose:
              this one reads what she has written, so it only makes sense once
              there is something in the box. */}
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void askForComment()}
            loading={commenting}
            iconStart={<Icon icon={Eye} size={14} />}
          >
            请印记看看这一段
          </Button>
        </div>
      </div>

      {/* 结构图里挂在这一块下面的材料。它们写进这一段，不单独成段（slots.ts）。 */}
      {slot.materials.length > 0 && (
        <div className="rounded-mk-sm px-3 py-2 text-mk-small" style={{ background: "var(--mk-paper)" }}>
          <p className="font-semibold text-mk-secondary">这一段可用的材料</p>
          <ul className="mt-1 flex list-disc flex-col gap-0.5 pl-4 text-mk-ink">
            {slot.materials.map((m, i) => (
              <li key={i}>{m}</li>
            ))}
          </ul>
        </div>
      )}

      {showGuide &&<GuideBox guide={guide} onDismiss={() => setCollapsed(true)} onDeepen={onDeepen} />}

      <textarea
        ref={textareaRef}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => void save()}
        placeholder="写这一段……"
        className="min-h-[100px] w-full resize-y rounded-mk-sm border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      />
      {/* 🚨 这一段到底算不算交上去了。
          走查里她写完四段之后问的是：「发送按钮按不动，不知道怎么交」——
          这个房间是失焦自动存的，存完只闪一下「保存中…」就什么都不剩，
          于是她一直在找一颗并不存在的「发送」。
          存下来这件事本身要**留在屏幕上**，她才知道可以往下走。
          用 已/处理中 这对词（ui-copy-style 第 4 条），不写「未保存」吓她。 */}
      {/* 🚨 **一颗真的「保存」按钮。**
          上一版只把「已保存」这行字留在屏幕上，以为这样她就知道存过了。
          线上走查里她第二次报同一件事：「没有明显的写文章正文的按钮，
          不知道写完怎么提交这一段」—— 因为那行字**只在存过之后才出现**。
          她刚写完、还没失焦的那一刻，屏幕上关于「怎么交」一个字都没有，
          于是她继续找一颗并不存在的「发送」。
          状态看得见 ≠ 动作做得到：要交的那一下，得有个东西给她按。
          失焦自动存照旧，这颗按钮只是把那件事摆到手边。 */}
      <div className="flex items-center gap-2 text-mk-small text-mk-faint">
        {saving ? (
          <span>处理中…</span>
        ) : saved ? (
          <span>已保存 · {countWords(text)} {wordUnit(lang)}</span>
        ) : text.trim() !== "" ? (
          <>
            <Button variant="secondary" size="sm" onClick={() => void save()}>
              保存这一段
            </Button>
            <span>{countWords(text)} {wordUnit(lang)}</span>
          </>
        ) : null}
      </div>
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}

      {/* 🚨 **把板递到她手上，而不是把按钮摆在那儿等她发现。**
          第一版这颗按钮长在正文框**上面**那条工具条里，和「获取引导」
          「请印记看看这一段」挤在一起。模拟学生走查里它连着出现 22 步，
          她一次都没按过 —— 屏幕上有它，和她手上有它，是两回事。
          （这也正是 2026-08-27 删掉卡片货架的那条裁定在说的事。）
          现在它长在她刚写完的那一段**下面**，而且带一句话说清它是干嘛的：
          出现的时机是「她已经写出两句以上」，也就是真的有东西可标的那一刻。 */}
      {!boardOpen && sentenceCount >= ROLE_BOARD_MIN && (
        <div className="flex flex-wrap items-center gap-2 rounded-mk-sm border border-mk-border px-3 py-2">
          <span className="text-mk-body text-mk-muted">
            这一段有 {sentenceCount} 句。标一下每一句在做什么，就看得出缺了哪一种。
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setBoardOpen(true)}
            iconStart={<Icon icon={LayoutGrid} size={14} />}
          >
            标一下这一段
          </Button>
        </div>
      )}

      {/* 标注板 —— 这个房间里第一件她用手摆的东西。
          闭环和阅读室那两块板一模一样：她摆完 → 结果原样变成一条真的学生
          消息 → 印记 在右栏对着它说话。所以这里只负责把那条消息发出去，
          然后把板收起来。 */}
      {boardOpen && sentenceCount >= ROLE_BOARD_MIN && (
        <RoleBoard
          text={text}
          snippetId={slot.snippet?.id ?? `pos-${slot.position}`}
          heading={slot.heading}
          busy={boardBusy}
          onCancel={() => setBoardOpen(false)}
          onSubmit={(message) => {
            setBoardBusy(true);
            void onSay(message, "role")
              .then(() => setBoardOpen(false))
              .catch(() => {
                // 发不出去就把板留在原地：她摆的东西还在，可以再按一次。
                setError("发送失败，再试一次。");
              })
              .finally(() => setBoardBusy(false));
          }}
        />
      )}

      {/* The SAME renderer 成稿 uses — one comment shape, one component, two
          zoom levels. `onTrace` is what differs, because the surface differs. */}
      {comment && (
        <CommentPanel
          comment={comment}
          onTrace={trace}
          currentText={text}
          onRecheck={() => void askForComment()}
          rechecking={commenting}
        />
      )}
    </div>
  );
}
