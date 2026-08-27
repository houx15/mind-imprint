import { useMemo } from "react";
import { Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import type { WritingOutlineItem } from "../api/writingRoom";

/**
 * MindMap — the graph that grows on the right while she plans.
 *
 * It renders `writing_outline` directly. Those rows carry `depth` and
 * `position`, which is exactly a flattened tree: a node's parent is the
 * nearest preceding row with a smaller depth. So there is no second data
 * model for "the map" — **the map IS the outline**, which is what makes
 * "the graph becomes the outline" free rather than a conversion step that
 * could lose something.
 *
 * Drawn as a left-to-right tree in plain DOM rather than SVG. Two reasons:
 * a real radial map is prettier but unreadable at three levels of Chinese
 * text, and DOM nodes are focusable, selectable and screen-readable, which
 * an SVG canvas is not without a lot of work. The connectors are borders.
 *
 * `justAdded` highlights what the last turn grew. Without it every reply
 * silently re-renders the whole map and she cannot tell what her answer
 * actually did — which is the entire feedback loop of this screen.
 */

export type MindMapNode = {
  item: WritingOutlineItem;
  children: MindMapNode[];
};

/**
 * Rebuild the tree from the flattened rows. A stack of open ancestors keyed
 * by depth: each row attaches to the last node shallower than itself, and a
 * row deeper than its predecessor by more than one level is clamped rather
 * than dropped — a gap in depth is a data oddity, not a reason to hide her
 * sentence.
 */
export function buildMindMap(items: WritingOutlineItem[]): MindMapNode[] {
  const sorted = items.slice().sort((a, b) => a.position - b.position);
  const roots: MindMapNode[] = [];
  const openByDepth: MindMapNode[] = [];

  for (const item of sorted) {
    const node: MindMapNode = { item, children: [] };
    const depth = Math.max(0, Math.min(item.depth, openByDepth.length));
    if (depth === 0) {
      roots.push(node);
      openByDepth.length = 0;
      openByDepth[0] = node;
    } else {
      const parent = openByDepth[depth - 1];
      if (parent) {
        parent.children.push(node);
      } else {
        roots.push(node);
      }
      openByDepth.length = depth;
      openByDepth[depth] = node;
    }
  }
  return roots;
}

export function MindMap({
  items,
  justAdded,
  onRemove,
  onEdit,
}: {
  items: WritingOutlineItem[];
  justAdded: string[];
  /** Hers to delete — the planning turn can only ever add. */
  onRemove?: (id: string) => void;
  onEdit?: (id: string, text: string) => void;
}) {
  const roots = useMemo(() => buildMindMap(items), [items]);

  if (items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-8 text-center">
        <p className="text-mk-small text-mk-faint">
          你说的每一点都会长在这里。
          <br />
          先跟印记说说这篇最想讲什么。
        </p>
      </div>
    );
  }

  return (
    <div className="mk-scroll h-full overflow-auto p-4">
      <ul className="flex flex-col gap-2">
        {roots.map((node) => (
          <MindMapBranch key={node.item.id} node={node} justAdded={justAdded} onRemove={onRemove} onEdit={onEdit} />
        ))}
      </ul>
    </div>
  );
}

function MindMapBranch({
  node,
  justAdded,
  onRemove,
  onEdit,
}: {
  node: MindMapNode;
  justAdded: string[];
  onRemove?: (id: string) => void;
  onEdit?: (id: string, text: string) => void;
}) {
  const isNew = justAdded.includes(node.item.id);
  const depth = node.item.depth;

  return (
    <li className="flex flex-col gap-2">
      <div
        className="group/node flex items-start gap-2 rounded-mk-sm border px-3 py-2 transition-colors duration-300 ease-mk"
        style={
          isNew
            ? { borderColor: "var(--mk-accent-500)", background: "var(--mk-accent-50)" }
            : depth === 0
              ? { borderColor: "var(--mk-accent-200)", background: "var(--mk-surface)" }
              : { borderColor: "var(--mk-border)", background: "var(--mk-surface)" }
        }
      >
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          {node.item.role && (
            <span className="text-mk-label" style={{ color: "var(--mk-accent-700)" }}>
              {node.item.role}
            </span>
          )}
          {/* contentEditable is deliberately NOT used: a stray keystroke on a
              contentEditable div silently mutates her plan with no save
              affordance. Editing goes through a real input on click. */}
          <span
            role={onEdit ? "button" : undefined}
            tabIndex={onEdit ? 0 : undefined}
            onClick={() => {
              if (!onEdit) return;
              const next = window.prompt("改一下这一条", node.item.text);
              if (next !== null && next.trim() && next.trim() !== node.item.text) onEdit(node.item.id, next.trim());
            }}
            className={[
              depth === 0 ? "text-mk-body font-semibold text-mk-ink" : "text-mk-body text-mk-ink",
              onEdit ? "cursor-text" : "",
            ]
              .filter(Boolean)
              .join(" ")}
          >
            {node.item.text}
          </span>
        </div>
        {onRemove && (
          <button
            type="button"
            aria-label={`删掉「${node.item.text}」`}
            onClick={() => onRemove(node.item.id)}
            className="shrink-0 text-mk-faint opacity-0 transition-opacity duration-[120ms] ease-mk hover:text-mk-danger focus-visible:opacity-100 group-hover/node:opacity-100"
          >
            <Icon icon={Trash2} size={14} />
          </button>
        )}
      </div>

      {node.children.length > 0 && (
        <ul className="ml-4 flex flex-col gap-2 border-l border-mk-border pl-4">
          {node.children.map((child) => (
            <MindMapBranch key={child.item.id} node={child} justAdded={justAdded} onRemove={onRemove} onEdit={onEdit} />
          ))}
        </ul>
      )}
    </li>
  );
}
