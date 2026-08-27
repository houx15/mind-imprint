import { useEffect, useState } from "react";
import { Sparkles, Plus } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  putWritingSnippet,
  generateWritingSnippetExemplar,
  type WritingOutlineItem,
  type WritingSnippet,
  type WritingExemplar,
} from "../api/writingRoom";

/**
 * SnippetsStage — 段落: "write paragraph by paragraph following the
 * outline, with guiding questions appearing as blocks; the English exemplar
 * sits in a visually separate container, labeled 「示范」, with no insert
 * button of any kind."
 *
 * 铁律① IS ENFORCED IN THIS FILE. `ExemplarBlock` below renders the model's
 * English paragraph as plain, non-editable text inside its own bordered
 * `示范` box — there is no button, icon, or click target anywhere near it
 * that could move that text into her paragraph textarea. The two pieces of
 * state (`slotText`, the textarea she types into, and `exemplar`, what the
 * box below it shows) are never assigned to each other anywhere in this
 * component — that is the whole guarantee, not a comment promising it.
 *
 * Exemplar generation is English-only server-side (writing_snippets.go 400s
 * `exemplar_not_available` for `lang !== "en"`), so the "示范" affordance is
 * only offered at all when `lang === "en"` — a Chinese writing simply has no
 * demonstration block, rather than a button that always fails.
 */

type Slot = {
  position: number;
  outlineId: string | null;
  heading: string;
  snippet: WritingSnippet | null;
};

/**
 * B2/H1 fix. Two rules:
 *
 * 1. EVERY persisted snippet gets a slot, whether or not it maps to a
 *    current outline point. Slicing to outline-derived slots ONLY
 *    (whenever an outline existed) used to silently drop any snippet that
 *    wasn't one of them — a free paragraph added via 加一段, or one
 *    written before the outline was ever confirmed. Its text still lands
 *    in the composed draft either way, so hiding it left her unable to see
 *    or edit part of her own finished piece (B2).
 * 2. A snippet is matched to an outline point strictly by `outlineId`
 *    (the server's own by-text-repaired link — see writing_snippets.go's
 *    `writingSnippetHeadingByID`), never by array position. Position
 *    matching can point at the WRONG outline point the moment the outline
 *    is reordered or a point is removed — "specific, confident and
 *    wrong", exactly what the backend refuses to do (H1).
 */
function buildSlots(outline: WritingOutlineItem[], snippets: WritingSnippet[]): Slot[] {
  const sortedOutline = outline.slice().sort((a, b) => a.position - b.position);
  const outlineIds = new Set(sortedOutline.map((o) => o.id));

  const outlineSlots: Slot[] = sortedOutline.map((o) => ({
    position: o.position,
    outlineId: o.id,
    heading: o.text,
    snippet: snippets.find((s) => s.outlineId === o.id) ?? null,
  }));

  // Every snippet not claimed by a current outline point — genuinely free
  // (no outlineId), or its link went stale (outline point renamed/removed,
  // in which case `outlineHeading` is the server's own honest last-known
  // heading, never guessed here).
  const freeSlots: Slot[] = snippets
    .filter((s) => !s.outlineId || !outlineIds.has(s.outlineId))
    .slice()
    .sort((a, b) => a.position - b.position)
    .map((s) => ({ position: s.position, outlineId: s.outlineId, heading: s.outlineHeading, snippet: s }));

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
  // Avoid colliding with an outline point's own position too — position is
  // writing_snippet's upsert key, so a free paragraph minted at a position
  // an (as yet unfilled) outline slot will later use would silently
  // overwrite that outline point's paragraph and detach it, the moment she
  // saves it.
  const usedPositions = [...snippets.map((s) => s.position), ...outline.map((o) => o.position)];
  const nextFreePosition = usedPositions.length === 0 ? 0 : Math.max(...usedPositions) + 1;

  async function addFreeParagraph() {
    const saved = await putWritingSnippet(writingId, { position: nextFreePosition, text: "" }).catch(() => null);
    if (saved) onSnippetsChange(saved);
  }

  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-mk-h2 text-mk-ink">段落</h2>

      {slots.length === 0 && (
        <p className="rounded-mk-md border border-dashed border-mk-border p-4 text-mk-small text-mk-muted">
          还没有提纲，段落就没有跟着的地方——先去「大纲」列几个要点，或者直接加一段自由写。
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
        // Only send outlineId to ESTABLISH a link, on this slot's very
        // first save (no persisted row yet). Once a snippet row exists,
        // omit it — the PUT's own "absent outlineId = preserve whatever
        // link is already there" semantics then apply, so an ordinary
        // text save can never clobber a link the server holds (including
        // one it just repaired by heading text after an outline edit) with
        // a value merely inferred client-side (H1).
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

  async function requestExemplar() {
    setGenerating(true);
    setError(null);
    try {
      // The exemplar endpoint needs an existing snippet row (`{sid}`) — if
      // this slot has never been saved yet, save it first (even if still
      // empty) so there is something to attach the demonstration to.
      let sid = slot.snippet?.id;
      if (!sid) {
        const saved = await putWritingSnippet(writingId, { outlineId: slot.outlineId, position: slot.position, text });
        onSaved(saved);
        sid = saved.find((s) => s.position === slot.position)?.id;
      }
      if (!sid) throw new Error("missing snippet id");
      const result = await generateWritingSnippetExemplar(writingId, sid);
      setExemplar(result);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "生成示范失败，请重试。");
    } finally {
      setGenerating(false);
    }
  }

  return (
    <div className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-4">
      <div className="flex items-center justify-between gap-2">
        <span className="text-mk-small font-semibold text-mk-ink">{slot.heading || "自由段落"}</span>
        {lang === "en" && (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void requestExemplar()}
            loading={generating}
            iconStart={<Icon icon={Sparkles} size={14} />}
          >
            示范段落
          </Button>
        )}
      </div>

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
 * ExemplarBlock — the 铁律① pressure point. Visually separate container
 * (its own border/background, distinct from the textarea above), labeled
 * 「示范」, and there is NOTHING clickable inside it that touches `text` in
 * the sibling component — no copy button, no "用这段" button, no drag
 * handle. Read it, then go write your own.
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
