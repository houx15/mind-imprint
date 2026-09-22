import { useEffect, useId, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Icon } from "@/ui";
import { Maximize2, X } from "lucide-react";
import { buildMindMap, MindMap, type MindMapNode } from "./MindMap";
import { outlineKindLabel, outlineKindOf } from "./outlineKind";
import type { WritingOutlineItem } from "../api/writingRoom";
import "./mini-map.css";

type PreviewNode = { node: MindMapNode; x: number; y: number; root: boolean };

/** A shared SVG coordinate space keeps thumbnail edges aligned at every size. */
function layoutPreview(items: WritingOutlineItem[]) {
  const nodes: PreviewNode[] = [];
  const edges: { from: PreviewNode; to: PreviewNode }[] = [];
  let row = 0;
  let maxDepth = 0;
  const walk = (node: MindMapNode, depth: number): PreviewNode => {
    maxDepth = Math.max(maxDepth, depth);
    const children = node.children.map((child) => walk(child, depth + 1));
    const y = children.length
      ? (children[0]!.y + children[children.length - 1]!.y) / 2
      : 24 + row++ * 48;
    const placed = { node, x: 12 + depth * 134, y, root: depth === 0 };
    nodes.push(placed);
    for (const child of children) edges.push({ from: placed, to: child });
    return placed;
  };
  buildMindMap(items).forEach((root) => walk(root, 0));
  return { nodes, edges, width: maxDepth * 134 + 132, height: Math.max(48, row * 48) };
}

/** Read-only structure reference during paragraph writing; edit in the structure stage. */
export function MiniMap({ outline }: { outline: WritingOutlineItem[] }) {
  const [open, setOpen] = useState(false);
  const preview = useMemo(() => layoutPreview(outline), [outline]);
  if (!outline.length) return null;

  return (
    <>
      <section className="writing-mini-map">
        <div className="writing-mini-map-heading">
          <h2>这一篇的结构</h2>
          <span>{outline.length} 个节点</span>
        </div>
        <button type="button" className="writing-mini-map-preview" onClick={() => setOpen(true)} aria-label="放大查看这一篇的结构">
          <svg viewBox={`0 0 ${preview.width} ${preview.height}`} aria-hidden="true" preserveAspectRatio="xMidYMid meet">
            {preview.edges.map(({ from, to }) => (
              <path key={`${from.node.item.id}-${to.node.item.id}`} d={`M ${from.x + 108} ${from.y} C ${from.x + 123} ${from.y}, ${to.x - 15} ${to.y}, ${to.x} ${to.y}`} fill="none" stroke="var(--mk-accent-300)" strokeWidth="1.5" />
            ))}
            {preview.nodes.map(({ node, x, y, root }) => {
              const label = outlineKindLabel(outlineKindOf(node.item));
              const text = Array.from(node.item.text);
              return (
                <g key={node.item.id}>
                  <rect x={x} y={y - 19} width="108" height="38" rx="8" fill={root ? "var(--mk-accent-50)" : "var(--mk-paper)"} stroke={root ? "var(--mk-accent-300)" : "var(--mk-border)"} />
                  <text x={x + 9} y={y - 4} fontSize="9" fontWeight="600" fill="var(--mk-accent-700)">{label}</text>
                  <text x={x + 9} y={y + 10} fontSize="10" fill="var(--mk-ink)">{text.slice(0, 8).join("")}{text.length > 8 ? "…" : ""}</text>
                </g>
              );
            })}
          </svg>
          <span className="writing-mini-map-expand"><Icon icon={Maximize2} size={13} />放大查看</span>
        </button>
        <p className="writing-mini-map-hint">只读预览 · 修改结构请返回「结构」</p>
      </section>
      {open && createPortal(<StructureDialog outline={outline} onClose={() => setOpen(false)} />, document.body)}
    </>
  );
}

function StructureDialog({ outline, onClose }: { outline: WritingOutlineItem[]; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descriptionId = useId();
  useEffect(() => {
    const element = dialog.current;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    element?.showModal();
    return () => {
      element?.close();
      opener?.focus();
    };
  }, []);

  return (
    <dialog ref={dialog} className="writing-structure-dialog" aria-labelledby={titleId} aria-describedby={descriptionId} onCancel={onClose} onKeyDown={(event) => {
      if (event.key !== "Tab") return;
      const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button:not([disabled]), [tabindex="0"], a[href], input:not([disabled]), textarea:not([disabled])'));
      const first = controls[0];
      const last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    }} onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <div className="writing-structure-dialog-content">
        <header>
          <div><h2 id={titleId}>这一篇的结构</h2><p id={descriptionId}>只读预览 · 修改结构请返回「结构」</p></div>
          <button type="button" onClick={onClose} aria-label="关闭结构预览" autoFocus><Icon icon={X} size={20} /></button>
        </header>
        <div className="min-h-0 flex-1"><MindMap items={outline} justAdded={[]} /></div>
      </div>
    </dialog>
  );
}
