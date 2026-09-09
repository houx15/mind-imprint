import { useRef, useState } from "react";
import type { PatchSpecV1, VisualDesignSpecV1, VisualNodeV1 } from "@mind-imprint/contracts";
import { manualPatch } from "./visualPatch";

type Gesture = { mode: "move" | "resize"; node: VisualNodeV1; startX: number; startY: number };

function cssStyle(node: VisualNodeV1, selected: boolean): React.CSSProperties {
  const style = node.style;
  const typography = style.typography;
  return {
    position: "absolute", left: `${node.bounds.x}px`, top: `${node.bounds.y}px`, width: `${node.bounds.width}px`, height: `${node.bounds.height}px`,
    zIndex: node.bounds.zIndex, transform: node.bounds.rotation ? `rotate(${node.bounds.rotation}deg)` : undefined,
    background: style.background, color: style.color, opacity: style.opacity, borderRadius: style.borderRadius,
    border: style.border ? `${style.border.width}px ${style.border.style} ${style.border.color}` : selected ? "2px solid #2563eb" : "1px solid transparent",
    boxShadow: style.shadow ? `${style.shadow.x}px ${style.shadow.y}px ${style.shadow.blur}px ${style.shadow.spread}px ${style.shadow.color}` : undefined,
    overflow: style.overflow, fontFamily: typography?.fontFamily, fontSize: typography?.fontSize, fontWeight: typography?.fontWeight,
    lineHeight: typography?.lineHeight, letterSpacing: typography?.letterSpacing, textAlign: typography?.textAlign, textTransform: typography?.textTransform,
    display: node.layout?.mode === "grid" ? "grid" : node.layout?.mode === "flex" ? "flex" : "block",
    gridTemplateColumns: node.layout?.mode === "grid" ? `repeat(${node.layout.columns}, minmax(0, 1fr))` : undefined,
    flexDirection: node.layout?.mode === "flex" ? node.layout.direction : undefined, gap: node.layout?.gap,
    justifyContent: node.layout?.justify, alignItems: node.layout?.align, flexWrap: node.layout?.wrap ? "wrap" : undefined,
    padding: node.layout?.padding ? `${node.layout.padding.top}px ${node.layout.padding.right}px ${node.layout.padding.bottom}px ${node.layout.padding.left}px` : undefined,
    cursor: node.locked ? "default" : "move", outline: selected ? "2px solid rgba(37,99,235,.35)" : undefined, outlineOffset: selected ? 2 : undefined,
  };
}

export function VisualDesignCanvas({ design, selectedId, onSelect, onPatch }: { design: VisualDesignSpecV1; selectedId: string | null; onSelect: (id: string | null) => void; onPatch: (patch: PatchSpecV1) => void }) {
  const page = design.pages[0];
  const shellRef = useRef<HTMLDivElement>(null);
  const gesture = useRef<Gesture | null>(null);
  const [draftBounds, setDraftBounds] = useState<Record<string, Partial<VisualNodeV1["bounds"]>>>({});
  if (!page) return null;
  const scalePoint = (clientX: number, clientY: number) => {
    const rect = shellRef.current!.getBoundingClientRect();
    const scale = rect.width / page.viewport.width;
    return { x: clientX / scale, y: clientY / scale };
  };
  const start = (event: React.PointerEvent, node: VisualNodeV1, mode: Gesture["mode"]) => {
    if (node.locked) return;
    event.stopPropagation(); onSelect(node.id);
    const point = scalePoint(event.clientX, event.clientY);
    gesture.current = { mode, node, startX: point.x, startY: point.y };
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  };
  const move = (event: React.PointerEvent) => {
    const active = gesture.current; if (!active) return;
    const point = scalePoint(event.clientX, event.clientY); const dx = point.x - active.startX; const dy = point.y - active.startY;
    const bounds = active.mode === "move"
      ? { x: Math.max(0, Math.min(page.viewport.width - active.node.bounds.width, active.node.bounds.x + dx)), y: Math.max(0, Math.min(page.viewport.height - active.node.bounds.height, active.node.bounds.y + dy)) }
      : { width: Math.max(36, Math.min(page.viewport.width - active.node.bounds.x, active.node.bounds.width + dx)), height: Math.max(28, Math.min(page.viewport.height - active.node.bounds.y, active.node.bounds.height + dy)) };
    setDraftBounds((items) => ({ ...items, [active.node.id]: bounds }));
  };
  const end = () => {
    const active = gesture.current; if (!active) return;
    const bounds = draftBounds[active.node.id]; gesture.current = null;
    if (!bounds) return;
    const operation = active.mode === "move"
      ? { operationId: `op.move.${crypto.randomUUID()}`, action: "move-node" as const, nodeId: active.node.id, targetPageId: page.id, targetParentId: active.node.parentId, index: active.node.order, position: { x: bounds.x!, y: bounds.y! } }
      : { operationId: `op.resize.${crypto.randomUUID()}`, action: "resize-node" as const, nodeId: active.node.id, bounds: { width: bounds.width!, height: bounds.height! } };
    setDraftBounds((items) => { const next = { ...items }; delete next[active.node.id]; return next; });
    onPatch(manualPatch(design, active.node.id, active.mode === "move" ? "拖动组件" : "缩放组件", operation));
  };
  return (
    <div className="eco-vd-viewport" ref={shellRef} onPointerMove={move} onPointerUp={end} onPointerCancel={end} onPointerDown={(event) => { if (event.target === event.currentTarget) onSelect(null); }}>
      <div className="eco-vd-page" style={{ width: page.viewport.width, height: page.viewport.height, background: page.background || design.theme.palette.background[0], transformOrigin: "top left" }}>
        {page.nodes.map((original) => {
          const node = draftBounds[original.id] ? { ...original, bounds: { ...original.bounds, ...draftBounds[original.id] } } : original;
          const selected = selectedId === node.id;
          const content = node.content;
          return (
            <div key={node.id} data-node-id={node.id} title={`${node.name} · ${node.id}`} style={cssStyle(node, selected)} onMouseDown={(event) => { event.stopPropagation(); onSelect(node.id); }} onPointerDown={(event) => start(event, original, "move")} onClick={(event) => { event.stopPropagation(); onSelect(node.id); }}>
              {node.type === "image" ? <img src={content?.assetRef} alt={content?.alt || node.name} draggable={false} style={{ width: "100%", height: "100%", objectFit: node.style.objectFit || "cover", display: "block" }} /> : node.type === "divider" ? null : (
                <div
                  className="eco-vd-text"
                  contentEditable={!node.locked && ["text", "button", "link", "input"].includes(node.type)}
                  suppressContentEditableWarning
                  onPointerDown={(event) => { if ((event.target as HTMLElement).isContentEditable) event.stopPropagation(); }}
                  onClick={(event) => { event.stopPropagation(); onSelect(node.id); }}
                  onFocus={() => onSelect(node.id)}
                  onBlur={(event) => {
                    const text = event.currentTarget.textContent || "";
                    if (text !== (content?.text || "")) onPatch(manualPatch(design, node.id, "直接编辑文字", { operationId: `op.content.${crypto.randomUUID()}`, action: "replace-content", nodeId: node.id, content: { ...content, text } }));
                  }}
                >{content?.text}</div>
              )}
              {selected && !node.locked && <button type="button" className="eco-vd-resize" aria-label="缩放组件" onPointerDown={(event) => start(event, original, "resize")} />}
            </div>
          );
        })}
      </div>
    </div>
  );
}
