import { useEffect, useRef, useState } from "react";
import { ImagePlus, MessageSquare, Monitor, MousePointer2, Send, Smartphone, Square, Trash2, Type } from "lucide-react";
import { Icon } from "@/ui";
import type { PatchSpecV1, VisualDesignSpecV1, VisualNodeV1 } from "@mind-imprint/contracts";
import { manualPagePatch, manualPatch } from "./visualPatch";

type Tool = "select" | "text" | "shape" | "image" | "button" | "comment";
type Box = Pick<VisualNodeV1["bounds"], "x" | "y" | "width" | "height">;
type Edge = "nw" | "n" | "ne" | "e" | "se" | "s" | "sw" | "w";
type Gesture = { mode: "move" | "resize"; edge?: Edge; node: VisualNodeV1; x: number; y: number };
const grid = 8; const snapDistance = 12;
const containers = new Set(["section", "header", "nav", "footer", "group", "grid"]);

function overlaps(a: Box, b: Box, gap = 0) { return a.x < b.x + b.width + gap && a.x + a.width + gap > b.x && a.y < b.y + b.height + gap && a.y + a.height + gap > b.y; }
function near(value: number, targets: number[]) { return targets.find((target) => Math.abs(value - target) <= snapDistance) ?? Math.round(value / grid) * grid; }
function nodeStyle(node: VisualNodeV1, selected: boolean): React.CSSProperties {
  const style = node.style; const text = style.typography;
  return { position: "absolute", left: node.bounds.x, top: node.bounds.y, width: node.bounds.width, height: node.bounds.height, zIndex: node.bounds.zIndex, background: style.background, color: style.color, opacity: style.opacity, borderRadius: style.borderRadius, border: style.border ? `${style.border.width}px ${style.border.style} ${style.border.color}` : selected ? "2px solid #2563eb" : "1px solid transparent", overflow: style.overflow, fontFamily: text?.fontFamily, fontSize: text?.fontSize, fontWeight: text?.fontWeight, lineHeight: text?.lineHeight, letterSpacing: text?.letterSpacing, textAlign: text?.textAlign, cursor: node.locked ? "default" : "move" };
}

export function VisualDesignCanvas({ design, selectedId, onSelect, onPatch, onComment }: { design: VisualDesignSpecV1; selectedId: string | null; onSelect: (id: string | null) => void; onPatch: (patch: PatchSpecV1) => void; onComment: (nodeId: string, text: string) => void }) {
  const page = design.pages[0]; const pageRef = useRef<HTMLDivElement>(null); const viewportRef = useRef<HTMLDivElement>(null); const imageInput = useRef<HTMLInputElement>(null); const active = useRef<Gesture | null>(null);
  const [tool, setTool] = useState<Tool>("select"); const [draft, setDraft] = useState<Record<string, Box>>({}); const [note, setNote] = useState(""); const [preview, setPreview] = useState<"desktop" | "mobile">("desktop"); const [availableWidth, setAvailableWidth] = useState(920);
  if (!page) return null;
  useEffect(() => { const target = viewportRef.current; if (!target) return; const resize = () => setAvailableWidth(target.clientWidth); resize(); const observer = new ResizeObserver(resize); observer.observe(target); return () => observer.disconnect(); }, []);
  const zoom = preview === "mobile"
    ? Math.max(.18, Math.min(1, (Math.max(260, availableWidth) - 40) / 390) * 390 / page.viewport.width)
    : Math.min(1, Math.max(.18, (Math.max(260, availableWidth) - 40) / page.viewport.width));
  const selected = page.nodes.find((node) => node.id === selectedId) ?? null;
  const point = (event: { clientX: number; clientY: number }) => { const rect = pageRef.current!.getBoundingClientRect(); return { x: (event.clientX - rect.left) / zoom, y: (event.clientY - rect.top) / zoom }; };
  const place = (node: VisualNodeV1, proposed: Box) => {
    const parent = node.parentId ? page.nodes.find((item) => item.id === node.parentId) : null;
    const boundary = parent?.bounds ?? { x: 0, y: 0, width: page.viewport.width, height: page.viewport.height };
    const right = boundary.x + boundary.width; const bottom = boundary.y + boundary.height;
    const siblings = page.nodes.filter((item) => item.id !== node.id && item.parentId === node.parentId).map((item) => item.bounds);
    const width = Math.max(36, Math.min(proposed.width, boundary.width)); const height = Math.max(28, Math.min(proposed.height, boundary.height));
    const xTargets = [boundary.x, right - width, ...siblings.flatMap((item) => [item.x, item.x + item.width, item.x - width, item.x + item.width - width])];
    const yTargets = [boundary.y, bottom - height, ...siblings.flatMap((item) => [item.y, item.y + item.height, item.y - height, item.y + item.height - height])];
    const result: Box = { width, height, x: Math.max(boundary.x, Math.min(right - width, near(proposed.x, xTargets))), y: Math.max(boundary.y, Math.min(bottom - height, near(proposed.y, yTargets))) };
    for (let count = 0; count < 80; count += 1) {
      const hit = siblings.find((item) => overlaps(result, item, grid)); if (!hit) break;
      if (hit.y + hit.height + grid + result.height <= bottom) result.y = hit.y + hit.height + grid;
      else if (hit.x + hit.width + grid + result.width <= right) result.x = hit.x + hit.width + grid;
      else break;
    }
    return result;
  };
  const start = (event: React.PointerEvent, node: VisualNodeV1, mode: Gesture["mode"], edge?: Edge) => {
    if (tool !== "select" || node.locked) return;
    event.stopPropagation(); const p = point(event); active.current = { mode, edge, node, x: p.x, y: p.y }; onSelect(node.id); (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  };
  const move = (event: React.PointerEvent) => {
    const current = active.current; if (!current) return;
    const p = point(event); const dx = p.x - current.x; const dy = p.y - current.y;
    let raw: Box = { x: current.node.bounds.x, y: current.node.bounds.y, width: current.node.bounds.width, height: current.node.bounds.height };
    if (current.mode === "move") raw = { ...raw, x: raw.x + dx, y: raw.y + dy };
    else { const edge = current.edge ?? "se"; if (edge.includes("e")) raw.width += dx; if (edge.includes("s")) raw.height += dy; if (edge.includes("w")) { raw.x += dx; raw.width -= dx; } if (edge.includes("n")) { raw.y += dy; raw.height -= dy; } }
    setDraft((items) => ({ ...items, [current.node.id]: place(current.node, raw) }));
  };
  const finish = () => {
    const current = active.current; if (!current) return; active.current = null; const bounds = draft[current.node.id]; if (!bounds) return;
    setDraft((items) => { const next = { ...items }; delete next[current.node.id]; return next; });
    const operation = current.mode === "move" ? { operationId: `op.move.${crypto.randomUUID()}`, action: "move-node" as const, nodeId: current.node.id, targetPageId: page.id, targetParentId: current.node.parentId, index: current.node.order, position: { x: bounds.x, y: bounds.y } } : { operationId: `op.resize.${crypto.randomUUID()}`, action: "resize-node" as const, nodeId: current.node.id, bounds: { x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height } };
    onPatch(manualPatch(design, current.node.id, current.mode === "move" ? "磁吸拖动组件" : "磁吸缩放组件", operation));
  };
  const add = (kind: Exclude<Tool, "select" | "comment">, p: { x: number; y: number }, assetRef?: string) => {
    const parent = page.nodes.filter((node) => containers.has(node.type) && p.x >= node.bounds.x && p.x <= node.bounds.x + node.bounds.width && p.y >= node.bounds.y && p.y <= node.bounds.y + node.bounds.height).sort((a, b) => a.bounds.width * a.bounds.height - b.bounds.width * b.bounds.height)[0] ?? null;
    const preset = kind === "text" ? { name: "文本", width: 280, height: 64, content: { text: "直接点击编辑文字" }, style: { color: design.theme.palette.text[0], typography: { fontFamily: design.theme.bodyFont, fontSize: 18, fontWeight: 400, lineHeight: 1.5 } } } : kind === "button" ? { name: "按钮", width: 160, height: 48, content: { text: "行动按钮" }, style: { background: design.theme.palette.accent[0], color: "#FFFFFF", borderRadius: design.theme.defaultRadius, typography: { fontFamily: design.theme.bodyFont, fontSize: 16, fontWeight: 600, lineHeight: 1.2 } } } : kind === "image" ? { name: assetRef ? "上传图片" : "图片占位", width: 280, height: 180, content: assetRef ? { assetRef, alt: "用户上传图片" } : { text: "图片占位", alt: "待替换图片" }, style: { background: design.theme.palette.surface[0], borderRadius: design.theme.defaultRadius } } : { name: "色块", width: 220, height: 120, content: undefined, style: { background: design.theme.palette.accent[0], borderRadius: design.theme.defaultRadius } };
    const node: VisualNodeV1 = { id: `node.manual.${crypto.randomUUID()}`, type: kind, name: preset.name, parentId: parent?.id ?? null, order: page.nodes.length, bounds: { x: p.x, y: p.y, width: preset.width, height: preset.height, zIndex: 50 }, style: preset.style, ...(preset.content ? { content: preset.content } : {}), interactionIds: [], author: "student" };
    node.bounds = { ...node.bounds, ...place(node, node.bounds) };
    onPatch(manualPagePatch(design, page.id, `新增${preset.name}`, { operationId: `op.add.${crypto.randomUUID()}`, action: "add-node", pageId: page.id, parentId: node.parentId, index: page.nodes.length, node }));
    onSelect(node.id); setTool("select");
  };
  const canvasDown = (event: React.PointerEvent<HTMLDivElement>) => { if (event.target !== event.currentTarget) return; if (tool === "select") { onSelect(null); return; } if (tool !== "comment") add(tool, point(event)); };
  const controls: Array<[Tool, typeof MousePointer2, string]> = [["select", MousePointer2, "选择"], ["text", Type, "文本"], ["shape", Square, "色块"], ["image", ImagePlus, "图片"], ["button", Square, "按钮"], ["comment", MessageSquare, "批注"]];
  const uploadImage = (file: File | undefined) => {
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => add("image", { x: page.viewport.width * .5 - 140, y: Math.min(page.viewport.height - 220, 120) }, String(reader.result));
    reader.readAsDataURL(file);
  };

  return <div className="eco-vd-canvas-editor">
    <div className="eco-vd-canvas-tools"><div className="eco-vd-tool-group">{controls.map(([value, icon, label]) => <button key={value} className={tool === value ? "is-active" : ""} onClick={() => value === "image" ? imageInput.current?.click() : setTool(value)} title={label}><Icon icon={icon} size={15} /><span>{label}</span></button>)}</div><div className="eco-vd-preview-switch"><button className={preview === "desktop" ? "is-active" : ""} onClick={() => setPreview("desktop")}><Icon icon={Monitor} size={14} />桌面</button><button className={preview === "mobile" ? "is-active" : ""} onClick={() => setPreview("mobile")}><Icon icon={Smartphone} size={14} />手机</button></div><input ref={imageInput} className="eco-vd-image-input" type="file" accept="image/png,image/jpeg,image/webp,image/gif" onChange={(event) => { uploadImage(event.target.files?.[0]); event.currentTarget.value = ""; }} />{selected && <button className="is-danger" onClick={() => onPatch(manualPatch(design, selected.id, "删除组件", { operationId: `op.remove.${crypto.randomUUID()}`, action: "remove-node", nodeId: selected.id, expectedParentId: selected.parentId }))}><Icon icon={Trash2} size={15} /><span>删除</span></button>}</div>
    <div className="eco-vd-viewport" ref={viewportRef} onPointerMove={move} onPointerUp={finish} onPointerCancel={finish}>
      <div className={`eco-vd-page-wrap is-${preview}`} style={{ width: page.viewport.width * zoom, height: page.viewport.height * zoom }}>
      <div className={`eco-vd-page is-tool-${tool}`} ref={pageRef} style={{ width: page.viewport.width, height: page.viewport.height, transform: `scale(${zoom})`, transformOrigin: "top left", background: page.background || design.theme.palette.background[0] }} onPointerDown={canvasDown}>
        {page.nodes.map((original) => {
          const node = draft[original.id] ? { ...original, bounds: { ...original.bounds, ...draft[original.id] } } : original; const content = node.content;
          return <div key={node.id} data-node-id={node.id} style={nodeStyle(node, selectedId === node.id)} onPointerDown={(event) => { if (tool === "comment") { event.stopPropagation(); onSelect(node.id); } else start(event, original, "move"); }} onClick={(event) => { event.stopPropagation(); onSelect(node.id); }}>
            {node.type === "image" ? content?.assetRef ? <img src={content.assetRef} alt={content.alt || node.name} draggable={false} style={{ width: "100%", height: "100%", objectFit: node.style.objectFit || "cover", display: "block" }} /> : <div className="eco-vd-image-placeholder"><Icon icon={ImagePlus} size={22} /><span>{content?.text || "图片占位"}</span></div> : node.type === "divider" ? null : <div className="eco-vd-text" contentEditable={!node.locked && ["text", "button", "link", "input"].includes(node.type)} suppressContentEditableWarning onPointerDown={(event) => { if ((event.target as HTMLElement).isContentEditable) event.stopPropagation(); }} onBlur={(event) => { const text = event.currentTarget.textContent || ""; if (text !== (content?.text || "")) onPatch(manualPatch(design, node.id, "直接编辑文字", { operationId: `op.content.${crypto.randomUUID()}`, action: "replace-content", nodeId: node.id, content: { ...content, text } })); }}>{content?.text}</div>}
            {selectedId === node.id && tool === "select" && !node.locked && <>{(["nw", "n", "ne", "e", "se", "s", "sw", "w"] as Edge[]).map((edge) => <button key={edge} type="button" className={`eco-vd-resize eco-vd-resize--${edge}`} aria-label={`从${edge}方向缩放组件`} onPointerDown={(event) => start(event, original, "resize", edge)} />)}</>}
          </div>;
        })}
        {selected && <aside className="eco-vd-node-popover" style={{ left: Math.min(page.viewport.width - 242, selected.bounds.x + selected.bounds.width + 12), top: Math.max(8, selected.bounds.y) }} onPointerDown={(event) => event.stopPropagation()}>
          <header><strong>{selected.name}</strong><span>组件操作</span></header>
          <div className="eco-vd-inline-colors"><label>背景<input type="color" value={selected.style.background?.startsWith("#") ? selected.style.background.slice(0, 7) : "#ffffff"} onChange={(event) => onPatch(manualPatch(design, selected.id, "修改组件背景色", { operationId: `op.style.${crypto.randomUUID()}`, action: "set-style", nodeId: selected.id, style: { background: event.target.value } }))} /></label><label>文字<input type="color" value={selected.style.color?.startsWith("#") ? selected.style.color.slice(0, 7) : "#172033"} onChange={(event) => onPatch(manualPatch(design, selected.id, "修改组件文字色", { operationId: `op.style.${crypto.randomUUID()}`, action: "set-style", nodeId: selected.id, style: { color: event.target.value } }))} /></label></div>
          <textarea value={note} onChange={(event) => setNote(event.target.value)} rows={3} placeholder={`批注“${selected.name}”的修改意见`} />
          <button className="eco-vd-note-send" disabled={!note.trim()} onClick={() => { onComment(selected.id, note.trim()); setNote(""); }}><Icon icon={Send} size={13} />生成修改建议</button>
        </aside>}
      </div></div>
    </div>
  </div>;
}
