import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { Layers, MessageSquareText, Check, FileUp } from "lucide-react";
import { Button, Icon, Modal } from "@/ui";
import { countWords } from "@/workspace/blocks/wordcount";
import { wordUnit } from "./wordUnit";
import { ProseSurface } from "./ProseSurface";
import { CommentPanel } from "./CommentPanel";
import { registerPendingSave } from "./pendingSaves";
import { NamePieceModal } from "./NamePieceModal";
import { studentArtwork } from "../learning/StudentArtwork";
import {
  composeWritingDraft,
  putWritingDraft,
  reviewWritingDraft,
  listWritingComments,
  finishWriting,
  suggestWritingTitles,
  suggestWritingTitleKeywords,
  type Comment,
  type WritingDraft,
  type WritingSnippet,
} from "../api/writingRoom";
import { extractDocument, renameWriting, type Writing } from "../api/writings";
import { apiErrorText } from "../api/errorText";
import { handleWriteError } from "./writeErrors";
import { splitBroughtFile } from "./broughtFile";

/**
 * ComposeStage — 成稿, as a page rather than a box.
 *
 * What was here was a bordered `mk-body` textarea at 14px: chrome type doing a
 * page's job, with a 从段落拼出初稿 button she had to find and press before
 * there was anything to write on at all. Three things change.
 *
 * **The draft is simply there when she arrives.** `composeWritingDraft` is a
 * deterministic concatenation of her own snippets — no model call, nothing of
 * 印记's in it (writing_compose.go's `composeSnippetsIntoDraft`) — so running
 * it for her the moment she opens 成稿 is a system step, not the AI writing
 * anything (铁律①'s scope: it never authors HER prose, and this authors none).
 * The button stays, demoted to 从段落重新拼一次, for after she has gone back
 * and changed 段落.
 *
 * **Re-assembling asks first.** Pulling from 段落 over a draft she has edited
 * HERE would destroy that editing with no trace and no undo — the same
 * silent-data-loss shape as the free-paragraph position collision that once
 * overwrote a snippet's text and relinked it under a heading she never wrote
 * it under. So: free when the draft is untouched since assembly, and a dialog
 * naming exactly what goes otherwise. `window.confirm` would technically ask,
 * but it cannot say what is at stake and cannot be styled or tested as part of
 * the room.
 *
 * **Comments live in the rail and persist.** `reviewWritingDraft` returns a
 * stored `Comment` (summary + points, each anchored to a sentence she actually
 * wrote), and `listWritingComments` brings back the last one on arrival — the
 * critique used to render once and evaporate on navigation. Clicking a point
 * lifts its quote into `ProseSurface`'s `highlight`. Nothing a comment carries
 * is ever written into the draft.
 */

/** Idle time before the draft saves itself. Blur, 请印记看看 and 完成这篇 all
 *  flush immediately, so this only covers the case that used to lose work: a
 *  long stretch of typing with no blur in it. */
const AUTOSAVE_MS = 1500;

export function ComposeStage({
  origin,
  lang,
  writingId,
  draft,
  snippets,
  onDraftChange,
  onFinished,
  onRenamed,
  onLocked,
  pendingHighlight,
  railTop,
}: {
  /** Shown at the top of the rail: the room's 老师批改 panel. */
  railTop?: ReactNode;
  /** `"brought"` = 她带进来的成稿（0146）。这一页据此说明结构和段落两步没有
   *  走过 —— 她看见那两步是空的，得知道为什么。 */
  origin?: string;
  writingId: string;
  draft: WritingDraft;
  /** Her 段落 blocks, shown in the rail beside the page — there by default,
   *  not summoned. */
  snippets: WritingSnippet[];
  /** 中文还是英文 —— 篇幅按「字」还是「词」说。见 wordUnit.ts。 */
  lang: string;
  onDraftChange: (next: WritingDraft) => void;
  onFinished: (writing: Writing) => void;
  /** Lifted so the room header's `EditableTitle` shows the new name the
   *  moment she settles on one at 完成这篇, rather than at the next load. */
  onRenamed?: (writing: Writing) => void;
  /** The deadline passed while she was mid-edit: every write below reloads
   *  the room into the locked finished page instead of showing a raw error
   *  on a page that can no longer save anything. */
  onLocked?: () => void;
  /**
   * A teacher grading's quote clicked in the room's 老师批改 panel
   * (`RoomTeacherFeedback`, 2026-09-17): highlight it here exactly the way
   * `CommentPanel`'s `onTrace` already does, via the same `highlight` state
   * and `ProseSurface`. The host passes a fresh object on every click, so
   * the effect below re-fires on a repeat click on the identical quote.
   */
  pendingHighlight?: { text: string } | null;
}) {
  const [body, setBody] = useState(draft.body);
  const [composing, setComposing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [comment, setComment] = useState<Comment | null>(null);
  /**
   * The highlighted quote plus a click counter. The text alone is not enough:
   * a second click on the same quote sets the same string, React skips the
   * re-render, and `ProseSurface` never scrolls back to it. The counter
   * changes on every click, and `ProseSurface` keys its scroll on it.
   */
  const [highlight, setHighlight] = useState<{ text: string; nonce: number } | null>(null);
  const highlightNonce = useRef(0);
  const showHighlight = useCallback((text: string) => {
    highlightNonce.current += 1;
    setHighlight({ text, nonce: highlightNonce.current });
  }, []);
  const [confirmingReassemble, setConfirmingReassemble] = useState(false);
  /** 右栏和意见那一块 —— 意见回来时要把她带过去，见 review()。 */
  const railRef = useRef<HTMLElement | null>(null);
  const commentRef = useRef<HTMLDivElement | null>(null);
  const [importing, setImporting] = useState(false);
  /** Text read from an uploaded file, waiting for her 替换 / 接在后面. */
  const [pendingImport, setPendingImport] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  /**
   * 印记's title candidates, or `null` when the naming dialog is closed.
   *
   * `null` vs `[]` carries real meaning here and the two must not be
   * collapsed: `null` is "not asking" (she already named the piece, or the
   * suggestion call failed and we went straight through), while `[]` is
   * "asking, with nothing to suggest" — she pressed 完成这篇 on an empty
   * draft, so there was nothing to draw a name from and the dialog shows a
   * plain box.
   */
  const [titleIdeas, setTitleIdeas] = useState<string[] | null>(null);
  const [naming, setNaming] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);

  /**
   * The body as of the last assembly — the ONLY thing that makes re-pulling
   * from 段落 safe. `null` means "this text did not come from an assembly this
   * session", which is deliberately the cautious answer: a draft loaded from
   * the server may or may not carry her edits, and we cannot tell, so we ask.
   */
  const [assembledBody, setAssembledBody] = useState<string | null>(null);

  /** What the server is known to hold. Not `draft.body`: a save in flight
   *  while she keeps typing would otherwise let the resolved prop overwrite
   *  the characters she added in the meantime. */
  const savedRef = useRef(draft.body);
  const bodyRef = useRef(draft.body);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Adopt an externally-changed draft (nothing does this today except the
  // first load), but never one that is merely our own save echoing back.
  useEffect(() => {
    if (draft.body === savedRef.current) return;
    savedRef.current = draft.body;
    bodyRef.current = draft.body;
    setBody(draft.body);
  }, [draft.body]);

  useEffect(() => () => clearPending(), []);

  useEffect(() => {
    if (pendingHighlight) showHighlight(pendingHighlight.text);
  }, [pendingHighlight, showHighlight]);

  // The last stored critique, so a comment survives navigating away and back.
  // Newest first, and only the whole-draft scope — per-paragraph comments
  // belong to 段落, not to this page.
  useEffect(() => {
    let cancelled = false;
    void listWritingComments(writingId)
      .then((all) => {
        if (cancelled) return;
        const latest = all.find((c) => c.scope === "draft");
        if (latest) setComment(latest);
      })
      .catch(() => {
        // A missing comment history is not worth an error banner on arrival —
        // the page is fully usable without it, and 请印记看看 still reports
        // its own failures.
      });
    return () => {
      cancelled = true;
    };
  }, [writingId]);

  /**
   * Assemble on arrival, so she lands on her own text instead of on a button.
   * Only when there is nothing here yet AND she has written paragraphs — an
   * empty 段落 would just produce an empty draft and a wasted round trip, and
   * an existing body is hers to keep.
   *
   * DO NOT add a `let cancelled = false` cleanup flag to this effect. A
   * once-latch plus a per-invocation cancel flag is the combination that hung
   * 段落's guide box (write-up in `shared/useAlive.ts`): StrictMode's cleanup
   * cancels the closure that fired the call, the latch skips the remount, and
   * the reply is dropped. `assemble()` sets state unconditionally here, which
   * is why this site is already correct — if it ever needs an unmount guard,
   * use `useAlive()`.
   */
  const arrived = useRef(false);
  useEffect(() => {
    if (arrived.current) return;
    arrived.current = true;
    if (draft.body.trim() !== "") return;
    if (!snippets.some((s) => s.text.trim() !== "")) return;
    void assemble();
    // Runs once on entry to 成稿; re-running it on every snippet change is
    // exactly the overwrite the guard below exists to prevent.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function clearPending() {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }

  /** Answers whether the server now holds `next` — 请印记看看 and 完成这篇
   *  both stop on `false` rather than acting on text that never landed. */
  async function save(next: string): Promise<boolean> {
    if (next === savedRef.current) return true;
    const previous = savedRef.current;
    savedRef.current = next;
    setSaving(true);
    setError(null);
    try {
      const saved = await putWritingDraft(writingId, next);
      savedRef.current = saved.body;
      onDraftChange(saved);
      return true;
    } catch (err) {
      savedRef.current = previous;
      handleWriteError(err, onLocked, setError);
      return false;
    } finally {
      setSaving(false);
    }
  }

  function onBodyChange(next: string) {
    bodyRef.current = next;
    setBody(next);
    clearPending();
    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      void save(next);
    }, AUTOSAVE_MS);
  }

  async function flush(): Promise<boolean> {
    clearPending();
    return save(bodyRef.current);
  }

  /**
   * 🚨 成稿这一页也有那个抢跑：她在正文里敲完就去问印记，防抖还没到，
   * 陪练读的还是服务端那一版。
   *
   * 段落那一步上一轮已经堵上了（pendingSaves.ts），成稿这一页当时漏了 ——
   * 第三十九轮中文那一路还在报同一件事：「成稿第三段明明已经有解释了，
   * 印记还说我缺解释，不知道是不是它看的是旧版本。」
   * flush 本来就有（失焦时用的那一个），差的只是把它登记出去。
   */
  useEffect(() => {
    return registerPendingSave(async () => {
      if (bodyRef.current === savedRef.current) return;
      await flush();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function assemble() {
    // The textarea stays enabled while this is in flight (a disabled surface
    // that steals focus mid-keystroke is worse), so she CAN type into a blank
    // arrival page during the round trip. Capture what was there when we
    // asked, and refuse to paint over anything she added since: the old
    // unconditional write dropped those keystrokes AND then let the next
    // autosave push the clobbered text to the server — the same silent loss
    // `wouldOverwrite` guards the manual 重新拼合 against.
    const startBody = bodyRef.current;
    setComposing(true);
    setError(null);
    try {
      const next = await composeWritingDraft(writingId);
      if (bodyRef.current !== "" && bodyRef.current !== startBody) return;
      clearPending();
      savedRef.current = next.body;
      bodyRef.current = next.body;
      setBody(next.body);
      setAssembledBody(next.body);
      setHighlight(null);
      onDraftChange(next);
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setComposing(false);
    }
  }

  /** Nothing to lose when the page is empty, or when every character on it
   *  came out of the last assembly. Anything else, ask. */
  const wouldOverwrite = body.trim() !== "" && body !== assembledBody;

  function requestAssemble() {
    if (wouldOverwrite) {
      setConfirmingReassemble(true);
      return;
    }
    void assemble();
  }

  /**
   * 上传一份写好的文件，文字落进这一页。
   *
   * 🚨 2026-09-18 写作入口走查：作业只能从作业条「开始」进来，而这个房间里
   * 没有上传 —— 在别处写完作业的学生只能把全文复制粘贴进来。落地页那个
   * 「带一篇写好的进来」建的是另一篇、不挂在作业上，老师那边看不到。
   * 所以上传放在成稿这一页：哪一篇都能用，作业也就能交上传的那一份。
   * 页面上已经有字时先问她是替换还是接在后面，不静默覆盖。
   */
  async function importFile(file: File | undefined) {
    if (!file || importing) return;
    setImporting(true);
    setError(null);
    try {
      const { body: text } = splitBroughtFile(await extractDocument(file), file.name);
      if (bodyRef.current.trim() === "") applyImport(text);
      else setPendingImport(text);
    } catch (err) {
      setError(`上传失败：${apiErrorText(err)}`);
    } finally {
      setImporting(false);
    }
  }

  function applyImport(next: string) {
    clearPending();
    bodyRef.current = next;
    setBody(next);
    setAssembledBody(null);
    setHighlight(null);
    void save(next);
  }

  async function review() {
    setReviewing(true);
    setError(null);
    try {
      // Reviewing text the server does not have would produce a critique of an
      // older draft, quoting sentences she can no longer see.
      if (!(await flush())) return;
      setComment(await reviewWritingDraft(writingId));
      setHighlight(null);
      // 🚨 2026-09-18 产品负责人：「请印记看看点击了以后，没有返回结果」。
      // 结果其实回来了：意见放在右栏**最上面**，而她点按钮的时候右栏正滚在下面
      // 看段落原文 —— 屏幕上什么都没变。审阅要花半分钟以上，她看到的只有按钮
      // 上那个小转圈。所以：审阅中在右栏顶上摆一行状态，意见一到就把右栏滚上去、
      // 把意见带到眼前。见 [[walk-the-loop-and-show-the-result-2026-09-17]]。
      requestAnimationFrame(() => {
        railRef.current?.scrollTo?.({ top: 0, behavior: "smooth" });
        commentRef.current?.scrollIntoView?.({ behavior: "smooth", block: "start" });
      });
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setReviewing(false);
    }
  }

  /**
   * 完成这篇, in two steps — because finishing is the moment the title stops
   * being private.
   *
   * Up to here the title has very likely still been her raw 「我想写：…」
   * sentence (createWriting stores it as the initial title, up to 200
   * characters of it). The instant she finishes, that string is the report's
   * hero, the exported poster's headline and what a stranger reads through
   * the share link. So we ask her to name the piece first — ONCE, and only
   * if she never renamed it herself.
   *
   * `suggestWritingTitles` is what decides, on the server, by comparing the
   * title against her stored opening turn. When she has already named it,
   * that call makes NO model call and returns immediately, so this detour is
   * invisible to anyone who used the header's editable title.
   *
   * A failure here must never cost her the finish: if asking for names blows
   * up (provider down, timeout), we go straight through to `doFinish` rather
   * than showing an error over a piece she just completed. A missing title
   * prompt is a small loss; a 完成这篇 that refuses to work is a real one.
   */
  async function finish() {
    setFinishing(true);
    setError(null);
    try {
      // Finishing on an unsaved body would freeze the piece — and its report —
      // on the version before her last edits, with nothing on screen saying so.
      // Also true of the naming step below: the titles are drawn from the
      // draft the SERVER holds.
      if (!(await flush())) return;
      let ideas: string[] | null = null;
      try {
        const res = await suggestWritingTitles(writingId);
        if (res.needsName) ideas = res.ideas;
      } catch {
        // See the doc comment: never block finishing on this.
      }
      if (ideas !== null) {
        setTitleIdeas(ideas);
        setFinishing(false);
        return;
      }
      await doFinish();
    } catch (err) {
      handleWriteError(err, onLocked, setError);
      setFinishing(false);
    }
  }

  /** The finish itself, after the naming question is settled one way or the
   *  other. Separate from `finish` so the dialog's two buttons can both reach
   *  it without re-running the flush and the naming check. */
  async function doFinish() {
    setFinishing(true);
    setError(null);
    try {
      onFinished(await finishWriting(writingId));
    } catch (err) {
      handleWriteError(err, onLocked, setError, "提交");
      setFinishing(false);
    }
  }

  /** 确认并完成 — save the name she settled on, then finish.
   *
   *  If the rename fails the piece is NOT finished: unlike the suggestion
   *  call above, this is her own words being dropped, and silently finishing
   *  under the old placeholder title would be the exact outcome this whole
   *  flow exists to prevent. She sees why and can try again. */
  async function nameThenFinish(title: string) {
    if (!title) return;
    setNaming(true);
    setNameError(null);
    try {
      onRenamed?.(await renameWriting(writingId, title));
    } catch (err) {
      handleWriteError(err, onLocked, setNameError);
      setNaming(false);
      return;
    }
    setNaming(false);
    setTitleIdeas(null);
    await doFinish();
  }

  return (
    <div className="writing-compose flex h-full min-h-0 flex-col">
      <div className="writing-compose__toolbar flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-mk-border px-4 py-2.5">
        <div className="flex items-center gap-2">
          <h2 className="text-mk-body font-semibold text-mk-ink">成稿</h2>
          <Button
            variant="ghost"
            size="sm"
            onClick={requestAssemble}
            loading={composing}
            iconStart={<Icon icon={Layers} size={14} />}
          >
            从段落重新拼一次
          </Button>
          <label
            className={`inline-flex cursor-pointer items-center gap-1.5 rounded-mk-sm px-2 py-1 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700 focus-within:ring-2 focus-within:ring-mk-accent-200 ${
              importing ? "pointer-events-none opacity-60" : ""
            }`}
            title="PDF / DOCX / TXT"
          >
            <Icon icon={FileUp} size={14} />
            {importing ? "正在读取文件…" : "上传文件"}
            <input
              type="file"
              accept=".pdf,.docx,.txt,.md"
              className="sr-only"
              disabled={importing}
              onChange={(e) => {
                void importFile(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void review()}
            loading={reviewing}
            disabled={!body.trim()}
            iconStart={<Icon icon={MessageSquareText} size={14} />}
          >
            请印记看看
          </Button>
          <Button
            size="sm"
            onClick={() => void finish()}
            loading={finishing}
            disabled={!body.trim()}
            iconStart={<Icon icon={Check} size={14} />}
          >
            完成这篇
          </Button>
        </div>
      </div>

      {error && (
        <p role="alert" className="shrink-0 px-4 py-2 text-mk-small text-mk-danger">
          {error}
        </p>
      )}

      {/* 🚨 如实说这一篇是带进来的。
          不是元数据洁癖：她会看到结构和段落两步是空的，得知道为什么；
          而过程评估的全部意义就是分清哪些是她在这儿想出来的
          —— 铁律① 最后半句说的就是「诚实介绍 AI 和人的分工」。 */}
      {origin === "brought" && (
        <p className="shrink-0 px-4 py-2 text-mk-small text-mk-muted">
          这一篇是你带进来的。结构和段落两步没有走过，印记 只看这一份成稿。
        </p>
      )}

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="writing-compose__desk mk-scroll min-h-0 overflow-y-auto bg-mk-paper">
          <ProseSurface
            value={body}
            onChange={onBodyChange}
            onBlur={() => void flush()}
            highlight={highlight?.text ?? null}
            highlightNonce={highlight?.nonce}
            placeholder="请先写下你最想说的那句话，再围绕它展开。"
          />
        </div>

        <aside
          ref={railRef}
          className="writing-compose__feedback mk-scroll flex min-h-0 flex-col gap-4 overflow-y-auto border-mk-border bg-mk-surface p-4 lg:border-l"
        >
          {reviewing && (
            <p
              role="status"
              className="rounded-mk-sm border px-3 py-2 text-mk-small text-mk-accent-700"
              style={{
                background: "color-mix(in srgb, var(--mk-accent-50) 80%, var(--mk-surface))",
                borderColor: "color-mix(in srgb, var(--mk-accent-500) 30%, transparent)",
              }}
            >
              印记正在通读全文，意见会显示在这里，通常需要半分钟左右。
            </p>
          )}
          {railTop}
          {/* 🚨 这一块本来只传了 comment 和 onTrace —— 段落那一步早就会说
              「这条是上一版」了，成稿这一步一直没接上，而**整稿意见更容易过期**：
              她照着改的正是被引的那几句。2026-09-11 第八轮走查，两个学生一共
              五步在说同一件事：

                「印记的建议里还引用着『我站在收残台旁边数了一下』这些旧句子，
                  但我正文里已经没有这些了」
                「下面的材料卡片显示的还是我改之前的旧句子…我不知道该点哪个按钮
                  把这些卡片消掉或者更新」

              判据和存的那一版都是现成的，只差把它们接上。 */}
          {comment && (
            <div ref={commentRef}>
              <CommentPanel
                comment={comment}
                onTrace={showHighlight}
                currentText={body}
                onRecheck={() => void review()}
                rechecking={reviewing}
              />
            </div>
          )}
          {/* 带进来的一篇没有段落原文；「暂无段落原文」只会让她以为漏了一步。 */}
          {/* 2026-09-18 走查：到了成稿，学生没有任何提示去请印记通读，
              「教到了吗」一栏在这一步掉到 1 —— 她直接按了「完成这篇」。
              没有审阅过的时候，在右栏把这一步摆出来。 */}
          {!comment && body.trim() !== "" && (
            <div className="writing-review-invitation">
              <img src={studentArtwork.writing} alt="" />
              <p className="text-mk-body font-semibold text-mk-ink">全文审阅</p>
              <p className="mt-1 text-mk-small text-mk-muted">
                印记会通读全文，先指出最需要修改的一两处，并说明怎么改；改完后可以再请印记看。
              </p>
              <Button
                className="mt-2"
                variant="secondary"
                size="sm"
                onClick={() => void review()}
                loading={reviewing}
                iconStart={<Icon icon={MessageSquareText} size={14} />}
              >
                请印记通读
              </Button>
            </div>
          )}
          {!(origin === "brought" && !snippets.some((s) => s.text.trim() !== "")) && (
            <SnippetRail snippets={snippets} edited={wouldOverwrite} lang={lang} />
          )}
        </aside>
      </div>

      {/* Fixed, so it can never reflow the page under her hands mid-sentence —
          which is exactly what the old inline 保存中… did. `color-mix` rather
          than `bg-mk-ink/6`: mk-* are bare CSS vars, and any /NN suffix on one
          emits no CSS at all. */}
      {saving && (
        <span
          aria-live="polite"
          className="pointer-events-none fixed bottom-5 right-5 z-40 rounded-mk-full px-2.5 py-1 text-mk-small text-mk-faint"
          style={{ background: "color-mix(in srgb, var(--mk-ink) 7%, transparent)" }}
        >
          保存中…
        </span>
      )}

      <Modal
        open={confirmingReassemble}
        onClose={() => setConfirmingReassemble(false)}
        title="重新拼一次会覆盖你改过的字"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmingReassemble(false)}>
              先不拼
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                setConfirmingReassemble(false);
                void assemble();
              }}
            >
              覆盖，重新拼
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-2">
          <p className="text-mk-body text-mk-ink">
            现在这篇成稿有 {countWords(body)} {wordUnit(lang)}。重新拼会用「段落」里的 {snippets.length} 段整个替换它，你在这一页上写的、改的都会没有，也找不回来。
          </p>
          <p className="text-mk-small text-mk-muted">
            如果只是想补上刚改过的某一段，回「段落」改完再回来，把那几句自己贴进去，比整篇重拼稳妥。
          </p>
        </div>
      </Modal>

      <Modal
        open={pendingImport !== null}
        onClose={() => setPendingImport(null)}
        title="这一页已经有正文"
        footer={
          <>
            <Button variant="ghost" onClick={() => setPendingImport(null)}>
              取消
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                const text = pendingImport ?? "";
                setPendingImport(null);
                applyImport(bodyRef.current.trimEnd() + "\n\n" + text);
              }}
            >
              接在后面
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                const text = pendingImport ?? "";
                setPendingImport(null);
                applyImport(text);
              }}
            >
              替换
            </Button>
          </>
        }
      >
        <p className="text-mk-body text-mk-ink">
          文件里读到 {countWords(pendingImport ?? "")} {wordUnit(lang)}。替换会用文件内容覆盖现在的 {countWords(body)} {wordUnit(lang)}；接在后面会把文件内容加到末尾。
        </p>
      </Modal>

      {titleIdeas !== null && (
        <NamePieceModal
          onHint={() => suggestWritingTitleKeywords(writingId)}
          saving={naming || finishing}
          error={nameError}
          onName={(t) => void nameThenFinish(t)}
          onKeep={() => {
            setTitleIdeas(null);
            void doFinish();
          }}
          // Closing the dialog is NOT the same as 用原来的: she asked to back
          // out of finishing altogether, so nothing is saved and nothing is
          // finished. 完成这篇 is still there when she wants it.
          onClose={() => {
            setTitleIdeas(null);
            setNameError(null);
          }}
        />
      )}
    </div>
  );
}

/**
 * Her paragraphs, beside the page. Read-only on purpose: 段落 is where they
 * are written, and a second editable copy of the same text is how the two
 * quietly disagree about which one is current.
 *
 * 🚨 只读拦不住那场误会 —— 它只是拦住了「两边都能改」。2026-09-11 第六轮
 * 线上走查，一个学生六步都在问同一件事：
 *
 *   「成稿框里已经是改好的了，但下面段落卡片还是旧的错句子」
 *   「有点搞不清到底以哪个为准」
 *   「不知道要不要重新覆盖写一遍还是点哪个按钮提交」
 *
 * 她在成稿里改了语法，下面这些卡片当然还是原样 —— 它们本来就是「段落」那一步
 * 的原文。**只读是对的，没说出口才是错的**：屏幕上摆着她同一段文字的两个版本，
 * 而没有一个字讲过哪个算数。
 *
 * 所以这里现在直说：上面那块是这一篇的正文，下面这些是原文、不会跟着变。
 * 她改过之后再加一句，免得她以为是自己哪一步没保存。
 */
function SnippetRail({
  snippets,
  edited,
  lang,
}: {
  snippets: WritingSnippet[];
  edited: boolean;
  lang: string;
}) {
  const written = snippets.filter((s) => s.text.trim() !== "").slice().sort((a, b) => a.position - b.position);

  return (
    <details className="writing-reference">
      <summary>段落原文 <span>{written.length} 段 · 展开对照</span></summary>
      {written.length > 0 && (
        <p className="text-mk-small text-mk-muted">
          这里保留「段落」阶段的原文，只读，不会随成稿修改。
          {edited && "成稿已有修改，此处仍为原文。"}
        </p>
      )}
      {written.length === 0 ? (
        <div className="py-5 text-center">
          <img src={studentArtwork.writing} alt="" className="mx-auto mb-4 h-28 w-40 object-contain" />
          <p className="text-mk-body text-mk-secondary">暂无段落原文</p>
          <p className="mt-2 text-mk-small leading-relaxed text-mk-muted">在「段落」阶段写下的内容会显示在这里，供成稿时对照。</p>
        </div>
      ) : (
        <ul className="flex list-none flex-col gap-3">
          {written.map((s) => (
            <li key={s.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-3">
              {/* 🚨 每一段各自多少字/词。她要按老师给的篇幅删，得知道删哪一段
                  才有用 —— 2026-09-12 走查里她的原话是「我找不到每一段分别
                  多少词的显示」，而那时她正对着一个 150 词的上限。
                  上面那个总数只告诉她超了，没告诉她超在哪。 */}
              <p className="flex items-baseline justify-between gap-2 text-mk-small text-mk-muted">
                <span className="min-w-0 truncate">{s.outlineHeading || "自由段落"}</span>
                <span className="shrink-0">
                  {countWords(s.text)} {wordUnit(lang)}
                </span>
              </p>
              <p className="mt-1 whitespace-pre-wrap text-mk-body text-mk-ink">{s.text}</p>
            </li>
          ))}
        </ul>
      )}
    </details>
  );
}
