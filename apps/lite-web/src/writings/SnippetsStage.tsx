import { useEffect, useState } from "react";
import { Sparkles, Plus, HelpCircle } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import { GuideBox } from "./GuideBox";
import {
  putWritingSnippet,
  generateWritingSnippetExemplar,
  guideWritingBlock,
  type WritingOutlineItem,
  type WritingSnippet,
  type WritingExemplar,
  type WritingBlockGuide,
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
 * So every block now carries 「卡住了？」 → GuideBox: two to four questions
 * about THIS block, grounded in what she has already said. Not suggestions,
 * not a model paragraph — questions.
 *
 * 铁律① IS ENFORCED IN THIS FILE, twice over:
 *
 *   - GuideBox renders `guide.questions`, and the server has already dropped
 *     anything that isn't a question (writing_guide.go's parseWritingGuide).
 *     A question cannot be pasted into an essay; a sentence can. The
 *     guarantee is the output TYPE, not a promise.
 *   - ExemplarBlock (English only) renders the model's demonstration
 *     paragraph as plain, non-editable text in its own bordered 「示范」 box,
 *     with nothing clickable inside it that touches the textarea above. The
 *     two pieces of state — `slotText` and `exemplar` — are never assigned to
 *     each other anywhere in this component. That is the whole guarantee.

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

export function SnippetsStage({
  writingId,
  lang,
  outline,
  snippets,
  onSnippetsChange,
}: {
  writingId: string;
  lang: string;
  outline: WritingOutlineItem[];
  snippets: WritingSnippet[];
  onSnippetsChange: (next: WritingSnippet[]) => void;
}) {
  const slots = buildSlots(outline, snippets);

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
        <p className="text-mk-body text-mk-muted">一块一块来。写不动了就点「卡住了？」。</p>
      </div>

      {slots.length === 0 && (
        <p className="rounded-mk-md border border-dashed border-mk-border p-4 text-mk-small text-mk-muted">
          还没挑结构，段落没有能跟着的块。先去「结构」挑一副，或者直接加一段。
        </p>
      )}

      <div className="flex flex-col gap-5">
        {slots.map((slot) => (
          <SnippetBlock
            key={`${slot.outlineId ?? "free"}-${slot.position}`}
            writingId={writingId}
            lang={lang}
            slot={slot}
            onSaved={onSnippetsChange}
          />
        ))}
      </div>

      <button
        type="button"
        onClick={() => void addFreeParagraph()}
        className="flex w-fit items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-accent-700 hover:bg-mk-accent-50"
      >
        <Icon icon={Plus} size={14} /> 加一段
      </button>
    </div>
  );
}

function SnippetBlock({
  writingId,
  lang,
  slot,
  onSaved,
}: {
  writingId: string;
  lang: string;
  slot: Slot;
  onSaved: (next: WritingSnippet[]) => void;
}) {
  const [text, setText] = useState(slot.snippet?.text ?? "");
  const [saving, setSaving] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [exemplar, setExemplar] = useState<WritingExemplar | null>(null);
  const [guide, setGuide] = useState<WritingBlockGuide | null>(null);
  const [guiding, setGuiding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setText(slot.snippet?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slot.snippet?.id]);

  async function save() {
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
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存这一段失败，请重试。");
    } finally {
      setSaving(false);
    }
  }

  async function askForGuide() {
    if (!slot.outlineId) {
      // A free paragraph has no block to reason about — the guide endpoint is
      // keyed on an outline row. Say so rather than firing a call that 404s.
      setError("这是一段自由写的段落，先把它挂到「结构」里的某一块上，印记才知道该往哪个方向问。");
      return;
    }
    setGuiding(true);
    setError(null);
    try {
      setGuide(await guideWritingBlock(writingId, slot.outlineId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次没问出问题来，再试一次。");
    } finally {
      setGuiding(false);
    }
  }

  async function requestExemplar() {
    setGenerating(true);
    setError(null);
    try {
      // The exemplar endpoint needs an existing snippet row — if this slot has
      // never been saved, save it first (even empty) so there is something to
      // attach the demonstration to.
      let sid = slot.snippet?.id;
      if (!sid) {
        const saved = await putWritingSnippet(writingId, { outlineId: slot.outlineId, position: slot.position, text });
        onSaved(saved);
        sid = saved.find((s) => s.position === slot.position)?.id;
      }
      if (!sid) throw new Error("missing snippet id");
      setExemplar(await generateWritingSnippetExemplar(writingId, sid));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "生成示范失败，请重试。");
    } finally {
      setGenerating(false);
    }
  }

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
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void askForGuide()}
            loading={guiding}
            iconStart={<Icon icon={HelpCircle} size={14} />}
          >
            卡住了？
          </Button>
          {lang === "en" && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void requestExemplar()}
              loading={generating}
              iconStart={<Icon icon={Sparkles} size={14} />}
            >
              示范段落
            </Button>
          )}
        </div>
      </div>

      {guide && <GuideBox guide={guide} onDismiss={() => setGuide(null)} />}

      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => void save()}
        placeholder="写这一段……"
        className="min-h-[100px] w-full resize-y rounded-mk-sm border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      />
      {saving && <span className="text-mk-label text-mk-faint">保存中…</span>}
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}

      {exemplar && <ExemplarBlock exemplar={exemplar} />}
    </div>
  );
}

/**
 * ExemplarBlock — the 铁律① pressure point. Visually separate container (its
 * own border/background, distinct from the textarea), labeled 「示范」, and
 * there is NOTHING clickable inside it that touches `text` in the sibling
 * component — no copy button, no "用这段" button, no drag handle. Read it,
 * then go write your own.
 */
function ExemplarBlock({ exemplar }: { exemplar: WritingExemplar }) {
  return (
    <div
      className="flex flex-col gap-2 rounded-mk-md border border-dashed p-3"
      style={{ borderColor: "var(--mk-accent-300)", background: "color-mix(in srgb, var(--mk-accent-500) 5%, var(--mk-paper))" }}
    >
      <div className="flex items-center gap-1.5">
        <span
          className="rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
          style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
        >
          示范
        </span>
        <span className="text-mk-label text-mk-faint">读一读别人会怎么写这一段，再回去写你自己的版本——不是给你抄的</span>
      </div>
      <p className="select-text whitespace-pre-wrap text-mk-body italic text-mk-secondary">{exemplar.exemplar}</p>
      {exemplar.prompts.length > 0 && (
        <div className="flex flex-col gap-1.5 pt-1">
          {exemplar.prompts.map((p, i) => (
            <div key={i} className="rounded-mk-sm bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-muted shadow-mk-xs">
              {p}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
