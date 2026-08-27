import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import type { WritingOutlineItem } from "../api/writingRoom";

/**
 * MindMap — the canvas that grows on the right while she plans.
 *
 * It renders `writing_outline` directly. Those rows carry `depth` and
 * `position`, which is exactly a flattened tree: a node's parent is the
 * nearest preceding row with a smaller depth. So there is no second data
 * model for "the map" — **the map IS the outline**, which is what makes "the
 * graph becomes the outline" free rather than a conversion that could lose
 * something.
 *
 * ## Why the layout is done this way
 *
 * An indented list renders the same information and feels like an outline;
 * the point of this panel is that it should feel like a 画布 she is drawing
 * on. So:
 *
 *   - **Shape comes from flexbox, not from arithmetic.** Each branch is a row
 *     (`items-center`) of [node | children-column]. Centring the row against
 *     the children column is precisely the mind-map shape, and it costs no
 *     layout pass and no estimated text heights — which is where hand-rolled
 *     tree layouts go wrong the moment a node wraps to three lines.
 *   - **Connectors come from measurement.** Curved edges cannot be drawn with
 *     borders once nodes wrap, so after layout every node's box is measured
 *     against the canvas and one cubic bezier is drawn per parent→child into
 *     an SVG underlay. Measuring after the browser has already solved the
 *     layout is what keeps the two in agreement at any text length.
 *
 * `justAdded` is what makes the panel feel alive: new nodes animate in from
 * the direction their connector grows, their edges draw themselves, and an
 * accent ring fades out on its own. Without it every reply silently
 * re-renders the whole map and she cannot tell what her answer just did —
 * which is the entire feedback loop of this screen.
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
      if (parent) parent.children.push(node);
      else roots.push(node);
      openByDepth.length = depth;
      openByDepth[depth] = node;
    }
  }
  return roots;
}

type Edge = { id: string; d: string; isNew: boolean };

export function MindMap({
  items,
  justAdded,
  onRemove,
  onEdit,
}: {
  items: WritingOutlineItem[];
  justAdded: string[];
  /** Hers to delete — a planning turn can only ever add. */
  onRemove?: (id: string) => void;
  onEdit?: (id: string, text: string) => void;
}) {
  const roots = useMemo(() => buildMindMap(items), [items]);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const nodeRefs = useRef(new Map<string, HTMLDivElement>());
  const [edges, setEdges] = useState<Edge[]>([]);
  const [canvasSize, setCanvasSize] = useState({ w: 0, h: 0 });

  const registerNode = useCallback((id: string, el: HTMLDivElement | null) => {
    if (el) nodeRefs.current.set(id, el);
    else nodeRefs.current.delete(id);
  }, []);

  /**
   * Measure once the browser has settled the layout, then draw one bezier per
   * parent→child. useLayoutEffect (not useEffect) so the edges are painted in
   * the same frame as the nodes — otherwise every update shows a flash of
   * unconnected boxes.
   */
  const measure = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const origin = canvas.getBoundingClientRect();
    setCanvasSize({ w: canvas.scrollWidth, h: canvas.scrollHeight });

    const next: Edge[] = [];
    const walk = (node: MindMapNode) => {
      const from = nodeRefs.current.get(node.item.id);
      for (const child of node.children) {
        const to = nodeRefs.current.get(child.item.id);
        if (from && to) {
          const f = from.getBoundingClientRect();
          const t = to.getBoundingClientRect();
          const x1 = f.right - origin.left + canvas.scrollLeft;
          const y1 = f.top + f.height / 2 - origin.top + canvas.scrollTop;
          const x2 = t.left - origin.left + canvas.scrollLeft;
          const y2 = t.top + t.height / 2 - origin.top + canvas.scrollTop;
          // Control points pushed horizontally, so a branch leaves its parent
          // sideways and arrives sideways — the shape a hand-drawn mind map
          // makes, and the reason a straight elbow reads as an org chart.
          const dx = Math.max(18, (x2 - x1) * 0.5);
          next.push({
            id: `${node.item.id}->${child.item.id}`,
            d: `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`,
            isNew: justAdded.includes(child.item.id),
          });
        }
        walk(child);
      }
    };
    roots.forEach(walk);
    setEdges(next);
  }, [roots, justAdded]);

  useLayoutEffect(measure, [measure]);

  // Re-measure when the panel resizes — a narrower window rewraps node text,
  // which moves every box under it.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() => measure());
    ro.observe(canvas);
    return () => ro.disconnect();
  }, [measure]);

  if (items.length === 0) {
    return (
      <div className="mk-canvas flex h-full items-center justify-center p-8 text-center">
        <p className="text-mk-small text-mk-faint">
          你说的每一点都会长在这里。
          <br />
          先跟印记说说这篇最想讲什么。
        </p>
      </div>
    );
  }

  return (
    <div ref={canvasRef} className="mk-canvas mk-scroll relative h-full overflow-auto p-5">
      <svg
        aria-hidden="true"
        className="pointer-events-none absolute left-0 top-0"
        width={canvasSize.w}
        height={canvasSize.h}
      >
        {edges.map((edge) => (
          <path
            key={edge.id}
            d={edge.d}
            pathLength={1}
            fill="none"
            stroke="var(--mk-accent-300)"
            strokeWidth={1.5}
            strokeLinecap="round"
            className={edge.isNew ? "mk-edge-draw" : undefined}
          />
        ))}
      </svg>

      {/* min-h-full + items-center vertically centres the map on the canvas
          while still letting it grow past the panel and scroll. A short map
          pinned to the top of a tall panel reads as a list that happens to be
          drawn; centred, it reads as something sitting on a sheet. */}
      <div className="relative flex min-h-full w-max items-center">
        <ul className="flex flex-col gap-4">
          {roots.map((node) => (
            <Branch key={node.item.id} node={node} justAdded={justAdded} registerNode={registerNode} onRemove={onRemove} onEdit={onEdit} />
          ))}
        </ul>
      </div>
    </div>
  );
}

/**
 * One branch: the node, and its children stacked to the right. `items-center`
 * is what makes a parent sit level with the middle of its children — the
 * whole mind-map shape, in one CSS property.
 */
function Branch({
  node,
  justAdded,
  registerNode,
  onRemove,
  onEdit,
}: {
  node: MindMapNode;
  justAdded: string[];
  registerNode: (id: string, el: HTMLDivElement | null) => void;
  onRemove?: (id: string) => void;
  onEdit?: (id: string, text: string) => void;
}) {
  const isNew = justAdded.includes(node.item.id);
  const isRoot = node.item.depth === 0;

  return (
    <li className="flex list-none items-center gap-7">
      <div
        ref={(el) => registerNode(node.item.id, el)}
        className={[
          "group/node relative flex shrink-0 items-start gap-2 rounded-mk-md border px-3 py-2 shadow-mk-xs",
          // Narrower the deeper it goes: three 240px columns cannot fit a side
          // panel, and evidence nodes are short phrases anyway. Widths shrink
          // rather than the text truncating — a clipped sentence on her own
          // plan is worse than a scroll.
          isRoot ? "max-w-[190px]" : node.item.depth === 1 ? "max-w-[175px]" : "max-w-[165px]",
          isNew ? "mk-node-new mk-node-flash" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        style={
          isRoot
            ? { borderColor: "var(--mk-accent-500)", background: "var(--mk-accent-50)" }
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
              affordance. Editing goes through a prompt on click. */}
          <span
            role={onEdit ? "button" : undefined}
            tabIndex={onEdit ? 0 : undefined}
            onClick={() => {
              if (!onEdit) return;
              const next = window.prompt("改一下这一条", node.item.text);
              if (next !== null && next.trim() && next.trim() !== node.item.text) onEdit(node.item.id, next.trim());
            }}
            className={[isRoot ? "text-mk-body font-semibold text-mk-ink" : "text-mk-body text-mk-ink", onEdit ? "cursor-text" : ""]
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
        <ul className="flex flex-col gap-3">
          {node.children.map((child) => (
            <Branch
              key={child.item.id}
              node={child}
              justAdded={justAdded}
              registerNode={registerNode}
              onRemove={onRemove}
              onEdit={onEdit}
            />
          ))}
        </ul>
      )}
    </li>
  );
}
