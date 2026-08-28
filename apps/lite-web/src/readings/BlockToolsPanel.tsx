import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { Icon } from "@/ui";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { ApiError } from "../api/client";
import { explainReadingBlock, type ReadingBlockNote, type ReadingBlockTool } from "../api/readingRoom";
import { BlockToolbar } from "./BlockToolbar";

/**
 * BlockToolsPanel — 点开一段，把它拆给她看。
 *
 * The product call: *"for lite-level students, they first need to be taught
 * about the paragraphs. for english reading materials — 翻译、关键单词讲解、
 * 语法讲解、写作解析; chinese paragraph — 成语/修辞运用、案例、结构解析. then
 * we can guide students to focus on information/subject lens then."*
 *
 * That **then** is the whole point: being able to read one paragraph is the
 * step below reading a whole article through a lens. The lens was always
 * here; this is the rung underneath it.
 *
 * ## Shape (2026-08-28)
 *
 * This used to be a card that opened under the paragraph carrying (a) a copy
 * of the paragraph, (b) the row of tool buttons, (c) the explanation. The
 * copy was the complaint:
 *
 *   > after clicking 详细带读, the paragraph appears again in that card, which
 *   > is really duplicated. so as above: click a paragraph, a row of clickable
 *   > operations appears near my mouse, with a AI mascot at the left.
 *
 * So the buttons left for a floating bar at her pointer (BlockToolbar), the
 * duplicated paragraph is gone — she is looking straight at the real one —
 * and what remains in the flow is **only the explanation**, hung under the
 * paragraph it explains.
 *
 * ## 铁律 check
 *
 * These are EXPLANATORY and that is why they are safe. 铁律① forbids the AI
 * writing the student's OWN prose; explaining someone else's published
 * paragraph is what a teacher does, and withholding it would make the room
 * less useful without making it more honest.
 *
 * The line that IS held, and held structurally: every tool is keyed on a
 * `blockId` of the ARTICLE, and nothing in this component can write to her
 * 摘要, her 批注 or her takeaway — it has no such prop and no such call. The
 * explanations also live in their own server-side table so a later report can
 * always tell them apart from what she wrote.
 *
 * Results are cached server-side by (blockId, tool), so re-opening one is
 * instant and free — which is most of why this reads as a set of tools rather
 * than as another chat.
 */
export function BlockToolsPanel({
  readingId,
  blockId,
  anchorEl,
  pointerX,
  tools,
  notes,
  autoTool,
  onAutoToolConsumed,
  onNote,
  onClose,
}: {
  readingId: string;
  blockId: string;
  /** The paragraph element the floating bar pins itself to. Null → no bar
   *  (the explanation still renders; this is the path a bare render takes). */
  anchorEl?: HTMLElement | null;
  pointerX?: number;
  tools: ReadingBlockTool[];
  notes: ReadingBlockNote[];
  /** A tool 印记 reached for. Runs itself once, so its teaching lands as
   *  teaching rather than as a button she has to find and press. */
  autoTool?: string | null;
  onAutoToolConsumed?: () => void;
  onNote: (note: ReadingBlockNote) => void;
  onClose: () => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const noteFor = (tool: string) => notes.find((n) => n.blockId === blockId && n.tool === tool);

  async function run(tool: ReadingBlockTool) {
    // Already open → collapse. Already fetched → just show it, no call.
    if (open === tool.id) {
      setOpen(null);
      return;
    }
    if (noteFor(tool.id)) {
      setOpen(tool.id);
      return;
    }
    setBusy(tool.id);
    setError(null);
    try {
      const note = await explainReadingBlock(readingId, blockId, tool.id);
      onNote({ blockId: note.blockId, tool: note.tool, body: note.body });
      setOpen(tool.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次没讲出来，再试一次。");
    } finally {
      setBusy(null);
    }
  }

  // Run the coach's chosen tool once. Keyed on blockId+tool so moving to a new
  // paragraph re-arms it, and guarded by a ref so a re-render never fires the
  // same call twice — this is a metered call, not a render effect.
  //
  // DO NOT pair this latch with a `let cancelled = false` cleanup flag. Under
  // StrictMode the cleanup cancels the closure that fired the call while the
  // latch skips the remount, so the one real reply is thrown away and the
  // busy state never clears — the bug that hung 段落's guide box (write-up in
  // `src/shared/useAlive.ts`). `run()` sets state unconditionally,
  // which is why this site is correct as written; if it ever needs an unmount
  // guard, use `useAlive()`.
  const autoFired = useRef<string | null>(null);
  useEffect(() => {
    if (!autoTool) return;
    const key = `${blockId}:${autoTool}`;
    if (autoFired.current === key) return;
    const tool = tools.find((t) => t.id === autoTool);
    if (!tool) {
      onAutoToolConsumed?.();
      return;
    }
    autoFired.current = key;
    void run(tool).finally(() => onAutoToolConsumed?.());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoTool, blockId, tools]);

  const shown = open ? noteFor(open) : undefined;
  const shownLabel = tools.find((t) => t.id === open)?.label ?? "";
  const opened = new Set(notes.filter((n) => n.blockId === blockId).map((n) => n.tool));

  return (
    <>
      {anchorEl && (
        <BlockToolbar
          anchorEl={anchorEl}
          pointerX={pointerX ?? 0}
          tools={tools}
          openedTools={opened}
          busyTool={busy}
          activeTool={open}
          onPick={(tool) => void run(tool)}
          onClose={onClose}
        />
      )}

      {error && (
        <p role="alert" className="mt-2 text-mk-small text-mk-danger">
          {error}
        </p>
      )}

      {shown && (
        <div
          className="mt-2 rounded-mk-md border p-3.5"
          style={{
            borderColor: "var(--mk-accent-200)",
            background: "color-mix(in srgb, var(--mk-accent-500) 4%, var(--mk-paper))",
          }}
        >
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="text-mk-caption text-mk-accent-700">{shownLabel}</span>
            <button
              type="button"
              aria-label="收起这段讲解"
              onClick={() => setOpen(null)}
              className="shrink-0 rounded-mk-xs p-0.5 text-mk-faint hover:text-mk-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              <Icon icon={X} size={14} />
            </button>
          </div>
          <div className="text-mk-body leading-relaxed text-mk-ink">
            <ChatMarkdown text={shown.body} />
          </div>
        </div>
      )}
    </>
  );
}
