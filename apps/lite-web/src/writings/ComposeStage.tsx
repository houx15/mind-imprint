import { useEffect, useRef, useState } from "react";
import { Layers, MessageSquareText, Check } from "lucide-react";
import { Button, Icon, Modal } from "@/ui";
import { countWords } from "@/workspace/blocks/wordcount";
import { ApiError } from "../api/client";
import { ProseSurface } from "./ProseSurface";
import { CommentPanel } from "./CommentPanel";
import { NamePieceModal } from "./NamePieceModal";
import {
  composeWritingDraft,
  putWritingDraft,
  reviewWritingDraft,
  listWritingComments,
  finishWriting,
  suggestWritingTitles,
  type Comment,
  type WritingDraft,
  type WritingSnippet,
} from "../api/writingRoom";
import { renameWriting, type Writing } from "../api/writings";

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
  writingId,
  draft,
  snippets,
  onDraftChange,
  onFinished,
  onRenamed,
}: {
  writingId: string;
  draft: WritingDraft;
  /** Her 段落 blocks, shown in the rail beside the page — there by default,
   *  not summoned. */
  snippets: WritingSnippet[];
  onDraftChange: (next: WritingDraft) => void;
  onFinished: (writing: Writing) => void;
  /** Lifted so the room header's `EditableTitle` shows the new name the
   *  moment she settles on one at 完成这篇, rather than at the next load. */
  onRenamed?: (writing: Writing) => void;
}) {
  const [body, setBody] = useState(draft.body);
  const [composing, setComposing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [comment, setComment] = useState<Comment | null>(null);
  const [highlight, setHighlight] = useState<string | null>(null);
  const [confirmingReassemble, setConfirmingReassemble] = useState(false);
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
      setError(err instanceof ApiError ? err.message : "保存失败，请重试。");
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
      setError(err instanceof ApiError ? err.message : "拼合失败，请重试。");
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

  async function review() {
    setReviewing(true);
    setError(null);
    try {
      // Reviewing text the server does not have would produce a critique of an
      // older draft, quoting sentences she can no longer see.
      if (!(await flush())) return;
      setComment(await reviewWritingDraft(writingId));
      setHighlight(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次体检没成功，请重试。");
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
      setError(err instanceof ApiError ? err.message : "完成失败，请重试。");
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
      setError(err instanceof ApiError ? err.message : "完成失败，请重试。");
      setFinishing(false);
    }
  }

  /** 就叫这个，完成 — save the name she settled on, then finish.
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
      setNameError(err instanceof ApiError ? err.message : "名字没存上，请重试。");
      setNaming(false);
      return;
    }
    setNaming(false);
    setTitleIdeas(null);
    await doFinish();
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-mk-border px-4 py-2.5">
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

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="mk-scroll min-h-0 overflow-y-auto bg-mk-paper">
          <ProseSurface
            value={body}
            onChange={onBodyChange}
            onBlur={() => void flush()}
            highlight={highlight}
            placeholder="从哪儿开始都行。先把你最想说的那句话写下来，剩下的会跟着它长出来。"
          />
        </div>

        <aside className="mk-scroll flex min-h-0 flex-col gap-4 overflow-y-auto border-mk-border bg-mk-surface p-4 lg:border-l">
          {comment && <CommentPanel comment={comment} onTrace={setHighlight} />}
          <SnippetRail snippets={snippets} />
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
            现在这篇成稿有 {countWords(body)} 字。重新拼会用「段落」里的 {snippets.length} 段整个替换它，你在这一页上写的、改的都会没有，也找不回来。
          </p>
          <p className="text-mk-small text-mk-muted">
            如果只是想补上刚改过的某一段，回「段落」改完再回来，把那几句自己贴进去，比整篇重拼稳妥。
          </p>
        </div>
      </Modal>

      {titleIdeas !== null && (
        <NamePieceModal
          ideas={titleIdeas}
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
 */
function SnippetRail({ snippets }: { snippets: WritingSnippet[] }) {
  const written = snippets.filter((s) => s.text.trim() !== "").slice().sort((a, b) => a.position - b.position);

  return (
    <section className="flex flex-col gap-2">
      <h3 className="text-mk-small font-semibold text-mk-secondary">你的段落</h3>
      {written.length === 0 ? (
        <p className="text-mk-body text-mk-muted">「段落」那一步还没有写好的段。写了以后会出现在这里，方便你对着改。</p>
      ) : (
        <ul className="flex list-none flex-col gap-3">
          {written.map((s) => (
            <li key={s.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-3">
              <p className="text-mk-small text-mk-muted">{s.outlineHeading || "自由段落"}</p>
              <p className="mt-1 whitespace-pre-wrap text-mk-body text-mk-ink">{s.text}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
