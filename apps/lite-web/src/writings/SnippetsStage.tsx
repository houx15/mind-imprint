import { useEffect, useRef, useState } from "react";
import { Plus, HelpCircle, Eye, LayoutGrid } from "lucide-react";
import { Button, EmptyState, Icon } from "@/ui";
import { countWords } from "@/workspace/blocks/wordcount";
import { ApiError } from "../api/client";
import { useAlive } from "../shared/useAlive";
import { GuideBox } from "./GuideBox";
import { CommentPanel } from "./CommentPanel";
import { DeepenDrawer } from "./DeepenDrawer";
import { RoleBoard } from "./RoleBoard";
import { splitSentences, ROLE_BOARD_MIN } from "./sentences";
import { apiErrorText } from "../api/errorText";
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

type Slot = {
  position: number;
  outlineId: string | null;
  heading: string;
  /** The generic block label from the skeleton ("反方最强的说法"), when this
   *  slot comes from one. Free paragraphs have none. */
  role: string;
  snippet: WritingSnippet | null;
};

/**
 * Two rules, both learned from bugs:
 *
 * 1. EVERY persisted snippet gets a slot, whether or not it maps to a current
 *    outline point. Slicing to outline-derived slots ONLY used to silently
 *    drop any snippet that wasn't one of them — a free paragraph added via
 *    加一段, or one written before a structure was ever chosen. Its text still
 *    lands in the composed draft either way, so hiding it left her unable to
 *    see or edit part of her own finished piece.
 * 2. A snippet is matched to a block strictly by `outlineId` (the server's own
 *    by-text-repaired link), never by array position. Position matching can
 *    point at the WRONG block the moment the outline is reordered — "specific,
 *    confident and wrong", exactly what the backend refuses to do.
 */
function buildSlots(outline: WritingOutlineItem[], snippets: WritingSnippet[]): Slot[] {
  const sortedOutline = outline.slice().sort((a, b) => a.position - b.position);
  const outlineIds = new Set(sortedOutline.map((o) => o.id));

  const outlineSlots: Slot[] = sortedOutline.map((o) => ({
    position: o.position,
    outlineId: o.id,
    // Her own sentence is the heading when she has written one; the generic
    // role is the fallback, so a block she hasn't summarised yet still says
    // what it is for rather than showing an empty strip.
    heading: o.text.trim() || o.role,
    role: o.role,
    snippet: snippets.find((s) => s.outlineId === o.id) ?? null,
  }));

  const freeSlots: Slot[] = snippets
    .filter((s) => !s.outlineId || !outlineIds.has(s.outlineId))
    .slice()
    .sort((a, b) => a.position - b.position)
    .map((s) => ({
      position: s.position,
      outlineId: s.outlineId,
      heading: s.outlineHeading,
      role: "",
      snippet: s,
    }));

  return [...outlineSlots, ...freeSlots];
}

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
  onGoToStructure,
  onSay,
}: {
  writingId: string;
  outline: WritingOutlineItem[];
  snippets: WritingSnippet[];
  onSnippetsChange: (next: WritingSnippet[]) => void;
  /** 一块板摆完了：把结果当成她说的一句话发出去，印记 在右栏接住它。 */
  onSay: (text: string, board?: WritingBoardKind) => Promise<void>;
  /** Sends her to 结构 from the empty state — naming the step she needs is
   *  not the same as getting her there. */
  onGoToStructure: () => void;
}) {
  const slots = buildSlots(outline, snippets);

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
        if (alive.current) setBatchError(apiErrorText(err));
      })
      .finally(() => {
        if (alive.current) setBatching(false);
      });
  }, [needsBatch, writingId, alive]);

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

  async function addFreeParagraph() {
    const saved = await putWritingSnippet(writingId, { position: nextFreePosition, text: "" }).catch(() => null);
    if (saved) onSnippetsChange(saved);
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <h2 className="text-mk-h2 text-mk-ink">段落</h2>
        <p className="text-mk-body text-mk-muted">一块一块来。每一块上面都写着它要做的事，照着想就行。</p>
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
            />
          );
        })}
      </div>

      <button
        type="button"
        onClick={() => void addFreeParagraph()}
        className="flex w-fit items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-accent-700 hover:bg-mk-accent-50"
      >
        <Icon icon={Plus} size={14} /> 加一段
      </button>

      {deepen && (
        <DeepenDrawer
          writingId={writingId}
          outlineId={deepen.outlineId}
          heading={deepen.heading}
          onClose={() => setDeepen(null)}
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
}) {
  const [text, setText] = useState(slot.snippet?.text ?? "");
  const [saving, setSaving] = useState(false);
  const [guiding, setGuiding] = useState(false);
  const [commenting, setCommenting] = useState(false);
  const [boardOpen, setBoardOpen] = useState(false);
  const [boardBusy, setBoardBusy] = useState(false);
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
   */
  const [collapsed, setCollapsed] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setText(slot.snippet?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slot.snippet?.id]);

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
      setError(apiErrorText(err));
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
      setError(apiErrorText(err));
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
      setError(apiErrorText(err));
    } finally {
      setGuiding(false);
    }
  }

  const showGuide = guide !== null && !collapsed;

  return (
    <div className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-4">
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

      {showGuide && <GuideBox guide={guide} onDismiss={() => setCollapsed(true)} onDeepen={onDeepen} />}

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
      <div className="flex items-center gap-2 text-mk-small text-mk-faint">
        {saving ? (
          <span>处理中…</span>
        ) : saved ? (
          <span>已保存 · {countWords(text)} 字</span>
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
      {comment && <CommentPanel comment={comment} onTrace={trace} />}
    </div>
  );
}
