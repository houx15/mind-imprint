import { useEffect, useRef, useState } from "react";
import { Sparkles, X } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { ApiError } from "../api/client";
import { explainReadingBlock, type ReadingBlockNote, type ReadingBlockTool } from "../api/readingRoom";

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
  blockText,
  tools,
  notes,
  autoTool,
  onAutoToolConsumed,
  onNote,
  onClose,
}: {
  readingId: string;
  blockId: string;
  blockText: string;
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

  return (
    <div className="flex flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-4 shadow-mk-sm">
      <div className="flex items-start justify-between gap-2">
        <span className="text-mk-label text-mk-faint">这一段</span>
        <button
          type="button"
          aria-label="关闭段落工具"
          onClick={onClose}
          className="shrink-0 text-mk-faint hover:text-mk-muted"
        >
          <Icon icon={X} size={15} />
        </button>
      </div>

      {/* The paragraph itself, so she can read it and the explanation side by
          side without scrolling back. Clamped: this is a reminder of which
          paragraph she opened, not a second copy of the article. */}
      <p className="line-clamp-3 text-mk-small leading-relaxed text-mk-secondary">{blockText}</p>

      <div className="flex flex-wrap gap-1.5">
        {tools.map((tool) => {
          const has = Boolean(noteFor(tool.id));
          const isOpen = open === tool.id;
          return (
            <Button
              key={tool.id}
              variant={isOpen ? "secondary" : "ghost"}
              size="sm"
              onClick={() => void run(tool)}
              loading={busy === tool.id}
              iconStart={has ? undefined : <Icon icon={Sparkles} size={13} />}
            >
              {tool.label}
            </Button>
          );
        })}
      </div>

      {error && <p className="text-mk-small text-mk-danger">{error}</p>}

      {shown && (
        <div
          className="rounded-mk-sm border p-3"
          style={{
            borderColor: "var(--mk-accent-200)",
            background: "color-mix(in srgb, var(--mk-accent-500) 4%, var(--mk-paper))",
          }}
        >
          <div className="text-mk-body leading-relaxed text-mk-ink">
            <ChatMarkdown text={shown.body} />
          </div>
        </div>
      )}
    </div>
  );
}
