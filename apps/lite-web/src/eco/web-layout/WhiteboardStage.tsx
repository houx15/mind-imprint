import { useRef, useState } from "react";
import {
  Image as ImageIcon,
  Minus,
  MousePointer2,
  Pencil,
  RectangleHorizontal,
  Redo2,
  Square,
  Trash2,
  Type,
  Undo2,
} from "lucide-react";
import { Badge, Button, Icon, Segmented } from "@/ui";
import { serializeMindPage } from "./mindPageDsl";
import type { WireframeDocument, WireframeKind, WireframeNode, WireframePoint } from "./types";

const BOARD_WIDTH = 960;
const BOARD_HEIGHT = 720;
const MIN_SIZE = 28;

type Tool = "select" | WireframeKind;
type Gesture = {
  mode: "move" | "resize" | "pen";
  id: string;
  start: WireframePoint;
  original: WireframeNode;
  before: WireframeDocument;
};

const TOOL_ITEMS: Array<{ kind: Tool; label: string; icon: typeof MousePointer2 }> = [
  { kind: "select", label: "选择", icon: MousePointer2 },
  { kind: "section", label: "区块", icon: Square },
  { kind: "text", label: "文本", icon: Type },
  { kind: "image", label: "图片", icon: ImageIcon },
  { kind: "button", label: "按钮", icon: RectangleHorizontal },
  { kind: "divider", label: "分隔线", icon: Minus },
  { kind: "pen", label: "画笔", icon: Pencil },
];

const DEFAULT_TEXT: Partial<Record<WireframeKind, string>> = {
  section: "页面区块",
  text: "标题或正文",
  image: "图片",
  button: "按钮",
};

function newNode(kind: WireframeKind, point: WireframePoint): WireframeNode {
  const sizes: Record<WireframeKind, readonly [number, number]> = {
    section: [520, 180],
    text: [260, 54],
    image: [230, 160],
    button: [140, 48],
    divider: [340, 12],
    pen: [1, 1],
  };
  const [nodeWidth, nodeHeight] = sizes[kind];
  return {
    id: `wire.${kind}.${Math.random().toString(36).slice(2, 8)}`,
    kind,
    x: Math.max(0, Math.min(BOARD_WIDTH - nodeWidth, point.x - nodeWidth / 2)),
    y: Math.max(0, Math.min(BOARD_HEIGHT - nodeHeight, point.y - nodeHeight / 2)),
    width: nodeWidth,
    height: nodeHeight,
    text: DEFAULT_TEXT[kind],
  };
}

function sameDocument(a: WireframeDocument, b: WireframeDocument) {
  return JSON.stringify(a) === JSON.stringify(b);
}

function boardPoint(event: React.PointerEvent<SVGSVGElement>): WireframePoint {
  const rect = event.currentTarget.getBoundingClientRect();
  return {
    x: ((event.clientX - rect.left) / rect.width) * BOARD_WIDTH,
    y: ((event.clientY - rect.top) / rect.height) * BOARD_HEIGHT,
  };
}

export function summarizeWireframe(document: WireframeDocument): string {
  return document.nodes.length ? serializeMindPage(document) : "";
}

export function WhiteboardStage({
  value,
  onChange,
  embedded = false,
}: {
  value: WireframeDocument;
  onChange: (document: WireframeDocument) => void;
  embedded?: boolean;
}) {
  const [tool, setTool] = useState<Tool>("select");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [past, setPast] = useState<WireframeDocument[]>([]);
  const [future, setFuture] = useState<WireframeDocument[]>([]);
  const gesture = useRef<Gesture | null>(null);
  const textBefore = useRef<WireframeDocument | null>(null);
  const selected = value.nodes.find((node) => node.id === selectedId) ?? null;

  function commit(next: WireframeDocument, before = value) {
    if (sameDocument(next, before)) return;
    setPast((items) => [...items.slice(-39), before]);
    setFuture([]);
    onChange(next);
  }

  function undo() {
    const previous = past[past.length - 1];
    if (!previous) return;
    setPast((items) => items.slice(0, -1));
    setFuture((items) => [value, ...items].slice(0, 40));
    onChange(previous);
    setSelectedId(null);
  }

  function redo() {
    const next = future[0];
    if (!next) return;
    setFuture((items) => items.slice(1));
    setPast((items) => [...items, value].slice(-40));
    onChange(next);
    setSelectedId(null);
  }

  function removeSelected() {
    if (!selectedId) return;
    commit({ ...value, nodes: value.nodes.filter((node) => node.id !== selectedId) });
    setSelectedId(null);
  }

  function updateSelectedText(text: string) {
    if (!selected) return;
    onChange({ ...value, nodes: value.nodes.map((node) => node.id === selected.id ? { ...node, text } : node) });
  }

  function beginTextEdit() {
    textBefore.current ??= value;
  }

  function finishTextEdit() {
    const before = textBefore.current;
    textBefore.current = null;
    if (before && !sameDocument(before, value)) {
      setPast((items) => [...items.slice(-39), before]);
      setFuture([]);
    }
  }

  function pointerDown(event: React.PointerEvent<SVGSVGElement>) {
    const point = boardPoint(event);
    if (tool === "select") {
      if (event.target === event.currentTarget) setSelectedId(null);
      return;
    }
    if (tool === "pen") {
      const node = newNode("pen", point);
      node.x = 0;
      node.y = 0;
      node.points = [point];
      const next = { ...value, nodes: [...value.nodes, node] };
      onChange(next);
      setSelectedId(node.id);
      gesture.current = { mode: "pen", id: node.id, start: point, original: node, before: value };
      event.currentTarget.setPointerCapture(event.pointerId);
      return;
    }
    const node = newNode(tool, point);
    commit({ ...value, nodes: [...value.nodes, node] });
    setSelectedId(node.id);
    setTool("select");
  }

  function startGesture(event: React.PointerEvent<SVGGElement>, node: WireframeNode, mode: "move" | "resize") {
    if (tool !== "select" || node.kind === "pen") return;
    event.stopPropagation();
    const svg = event.currentTarget.ownerSVGElement;
    if (!svg) return;
    const rect = svg.getBoundingClientRect();
    const point = { x: ((event.clientX - rect.left) / rect.width) * BOARD_WIDTH, y: ((event.clientY - rect.top) / rect.height) * BOARD_HEIGHT };
    setSelectedId(node.id);
    gesture.current = { mode, id: node.id, start: point, original: node, before: value };
    svg.setPointerCapture(event.pointerId);
  }

  function pointerMove(event: React.PointerEvent<SVGSVGElement>) {
    const active = gesture.current;
    if (!active) return;
    const point = boardPoint(event);
    if (active.mode === "pen") {
      onChange({ ...value, nodes: value.nodes.map((node) => node.id === active.id ? { ...node, points: [...(node.points ?? []), point] } : node) });
      return;
    }
    const dx = point.x - active.start.x;
    const dy = point.y - active.start.y;
    onChange({
      ...value,
      nodes: value.nodes.map((node) => {
        if (node.id !== active.id) return node;
        if (active.mode === "move") return {
          ...node,
          x: Math.max(0, Math.min(BOARD_WIDTH - node.width, active.original.x + dx)),
          y: Math.max(0, Math.min(BOARD_HEIGHT - node.height, active.original.y + dy)),
        };
        return {
          ...node,
          width: Math.max(MIN_SIZE, Math.min(BOARD_WIDTH - node.x, active.original.width + dx)),
          height: Math.max(MIN_SIZE, Math.min(BOARD_HEIGHT - node.y, active.original.height + dy)),
        };
      }),
    });
  }

  function pointerUp() {
    const active = gesture.current;
    if (!active) return;
    gesture.current = null;
    if (!sameDocument(value, active.before)) {
      setPast((items) => [...items.slice(-39), active.before]);
      setFuture([]);
    }
  }

  return (
    <div className={`eco-whiteboard-stage${embedded ? " eco-whiteboard-stage--embedded" : ""}`}>
      <div className="eco-whiteboard-heading">
        <div>
          <Badge tone="draft">可选步骤</Badge>
          {embedded ? <h2 className="mt-2 text-mk-h2 text-mk-ink">页面白板</h2> : <h1 className="mt-3 text-mk-display text-mk-ink">绘制页面白板</h1>}
          <p className="mt-2 max-w-[720px] text-mk-body leading-relaxed text-mk-secondary">放置页面区块和组件，表达大致顺序与比例。AI 会结合主题、参考资料和白板推荐页面方向。</p>
        </div>
        <Segmented
          value={value.viewport}
          onChange={(viewport) => commit({ ...value, viewport: viewport as "desktop" | "phone" })}
          options={[{ value: "desktop", label: "宽屏" }, { value: "phone", label: "手机" }]}
        />
      </div>

      <div className="eco-whiteboard-shell">
        <div className="eco-whiteboard-toolbar" aria-label="白板工具">
          {TOOL_ITEMS.map((item) => (
            <button key={item.kind} type="button" aria-pressed={tool === item.kind} title={item.label} onClick={() => setTool(item.kind)}>
              <Icon icon={item.icon} size={17} />
              <span>{item.label}</span>
            </button>
          ))}
          <span className="eco-whiteboard-tool-separator" />
          <button type="button" disabled={!past.length} title="撤销" onClick={undo}><Icon icon={Undo2} size={17} /><span>撤销</span></button>
          <button type="button" disabled={!future.length} title="重做" onClick={redo}><Icon icon={Redo2} size={17} /><span>重做</span></button>
          <button type="button" disabled={!selectedId} title="删除所选" onClick={removeSelected}><Icon icon={Trash2} size={17} /><span>删除</span></button>
        </div>

        <div className={`eco-whiteboard-viewport eco-whiteboard-viewport--${value.viewport}`}>
          <svg
            className="eco-whiteboard-canvas"
            viewBox={`0 0 ${BOARD_WIDTH} ${BOARD_HEIGHT}`}
            onPointerDown={pointerDown}
            onPointerMove={pointerMove}
            onPointerUp={pointerUp}
            onPointerCancel={pointerUp}
          >
            <defs>
              <pattern id="eco-whiteboard-grid" width="24" height="24" patternUnits="userSpaceOnUse">
                <circle cx="1" cy="1" r="1" fill="var(--mk-border)" />
              </pattern>
            </defs>
            <rect width="100%" height="100%" fill="url(#eco-whiteboard-grid)" pointerEvents="none" />
            {value.nodes.map((node) => node.kind === "pen" ? (
              <polyline key={node.id} points={(node.points ?? []).map((point) => `${point.x},${point.y}`).join(" ")} fill="none" stroke="var(--mk-accent)" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" pointerEvents="none" />
            ) : (
              <g key={node.id} className={`eco-wire-node eco-wire-node--${node.kind}${selectedId === node.id ? " is-selected" : ""}`} onPointerDown={(event) => startGesture(event, node, "move")}>
                {node.kind === "divider" ? (
                  <line x1={node.x} y1={node.y + node.height / 2} x2={node.x + node.width} y2={node.y + node.height / 2} />
                ) : (
                  <rect x={node.x} y={node.y} width={node.width} height={node.height} rx={node.kind === "button" ? 22 : 7} />
                )}
                {node.kind === "image" && <path d={`M ${node.x + 16} ${node.y + node.height - 18} L ${node.x + node.width * .42} ${node.y + node.height * .48} L ${node.x + node.width * .62} ${node.y + node.height * .68} L ${node.x + node.width - 16} ${node.y + 24}`} />}
                {node.text && node.kind !== "divider" && (selectedId === node.id ? (
                  <foreignObject x={node.x + 7} y={node.y + Math.max(5, Math.min(14, node.height / 2 - 18))} width={Math.max(24, node.width - 14)} height={38}>
                    <input
                      className="eco-wire-inline-input"
                      aria-label={`直接编辑${DEFAULT_TEXT[node.kind] ?? "白板组件"}`}
                      value={node.text}
                      onPointerDown={(event) => event.stopPropagation()}
                      onFocus={beginTextEdit}
                      onChange={(event) => updateSelectedText(event.target.value)}
                      onBlur={finishTextEdit}
                      onKeyDown={(event) => {
                        if (event.key === "Enter") event.currentTarget.blur();
                      }}
                    />
                  </foreignObject>
                ) : (
                  <text x={node.x + (node.kind === "button" ? node.width / 2 : 16)} y={node.y + Math.min(32, node.height / 2 + 6)} textAnchor={node.kind === "button" ? "middle" : "start"}>{node.text}</text>
                ))}
                {selectedId === node.id && (
                  <g className="eco-wire-resize" onPointerDown={(event) => startGesture(event, node, "resize")}>
                    <rect x={node.x + node.width - 8} y={node.y + node.height - 8} width="16" height="16" rx="3" />
                  </g>
                )}
              </g>
            ))}
          </svg>
        </div>

        <div className="eco-whiteboard-inspector">
          {selected && selected.kind !== "pen" ? (
            <>
              <label className="text-mk-label text-mk-secondary" htmlFor="eco-wire-label">内容标记</label>
              <input id="eco-wire-label" value={selected.text ?? ""} onChange={(event) => updateSelectedText(event.target.value)} className="eco-whiteboard-label-input" />
              <p className="mt-2 text-mk-small text-mk-muted">可以在画布或此处直接修改文字。拖动组件移动位置，拖动右下角改变大小。</p>
            </>
          ) : (
            <p className="text-mk-small text-mk-muted">选择组件后可以修改内容标记。白板不要求精确，主要用于表达结构。</p>
          )}
          {value.nodes.length > 0 && (
            <details className="eco-whiteboard-language">
              <summary>查看 MindPage DSL</summary>
              <pre>{serializeMindPage(value)}</pre>
            </details>
          )}
        </div>
      </div>
    </div>
  );
}
