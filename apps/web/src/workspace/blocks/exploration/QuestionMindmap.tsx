import { useCallback, useEffect, useMemo, useState } from "react";
import {
  applyNodeChanges,
  Background,
  BaseEdge,
  Controls,
  getBezierPath,
  Handle,
  Position,
  ReactFlow,
  ReactFlowProvider,
  type EdgeProps,
  type Node,
  type NodeChange,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { ExplorationLead } from "@mind-imprint/contracts";
import { NEUTRAL } from "@/ui/tokens";
import {
  buildMindmapEdges,
  buildMindmapNodes,
  mixToward,
  themeForRoot,
  type MindmapNodeKind,
  type NodeTheme,
} from "./warrenLayout";

// GVb · Level-2 "钻进一个洞": one question's subtree as a real React Flow
// mindmap. The focused question sits at the center; its adopted papers and
// sub-questions ring around it (radial layout in warrenLayout.ts), all tinted
// with the root's color family (root strongest, deeper nodes lighter). Clicking
// a node SELECTS it → the right sidebar (NodeSidebar) shows its metadata + the
// two search actions; nothing is added to this map until the student 采纳s a
// dig result (铁律①). × on a node → a confirm modal → deleteLead.
//
// Like WarrenMap, the heavy layout/theming is pure + unit-tested; this file is
// the thin React Flow shell. @xyflow/react is mocked in jsdom tests (it can't
// measure a container there), so the node/edge views lean only on trivially
// mockable primitives (Handle, BaseEdge, getBezierPath) and keep real DOM in JSX.

type MindmapNodeData = {
  text: string;
  kind: MindmapNodeKind;
  isRoot: boolean;
  theme: NodeTheme;
  selected: boolean;
  onRequestDelete: (id: string) => void;
};

// ---------- custom card node ----------

// `selected` is a manual per-node flag threaded through `data` (not React
// Flow's built-in NodeProps.selected) — the map already drives it from the
// sidebar's selectedId, so clicking a paper/question node persists a real
// selection (unlike Level-1, where a click immediately zooms away).
function MindmapNodeView({ id, data }: NodeProps) {
  const d = data as unknown as MindmapNodeData;
  const w = d.isRoot ? 216 : 176;
  const h = d.isRoot ? 108 : 88;
  return (
    <div
      data-theme={d.theme.key}
      data-selected={d.selected ? "true" : "false"}
      className="group relative flex flex-col justify-center overflow-hidden border border-mk-border bg-mk-surface py-2.5 pl-4 pr-3"
      style={{
        width: w,
        height: h,
        borderRadius: "var(--mk-radius-sm)", // spec §18 L2 card radius 8
        boxShadow: d.selected
          ? "0 0 0 3px var(--mk-accent), var(--mk-shadow-sm)" // selection ring — accent, reserved for this
          : "var(--mk-shadow-xs)",
        cursor: "pointer",
      }}
    >
      {/* Depth-tinted family color left bar — root strongest, deeper nodes lighter. */}
      <span aria-hidden className="absolute inset-y-0 left-0 w-1.5" style={{ background: d.theme.border }} />

      {/* Edges attach to these; connecting is disabled on Level-2 (provenance is
          server-defined), so the handles are invisible + non-connectable. */}
      <Handle type="target" position={Position.Left} className="nodrag" style={{ opacity: 0, width: 1, height: 1 }} isConnectable={false} />
      <Handle type="source" position={Position.Right} className="nodrag" style={{ opacity: 0, width: 1, height: 1 }} isConnectable={false} />

      {/* × → confirm modal. nodrag + stopPropagation so it neither drags nor
          triggers the node's select-on-click. */}
      <button
        type="button"
        aria-label="删除这个节点"
        title="删除"
        className="nodrag absolute right-2 top-2 flex h-5 w-5 items-center justify-center rounded-full bg-mk-paper text-[14px] font-bold leading-none opacity-0 shadow-mk-xs transition group-hover:opacity-100 hover:bg-mk-paper"
        style={{ color: d.theme.label }}
        onClick={(e) => {
          e.stopPropagation();
          d.onRequestDelete(id);
        }}
      >
        ×
      </button>

      {d.kind === "paper" && (
        <span
          className="mb-0.5 w-fit rounded-full px-1.5 py-0.5 text-[9px] font-bold text-white"
          style={{ background: d.theme.border }}
        >
          论文
        </span>
      )}
      <span
        className="font-bold leading-snug"
        style={{
          color: d.theme.label,
          fontSize: d.isRoot ? 12.5 : 11.5,
          display: "-webkit-box",
          WebkitLineClamp: d.isRoot ? 3 : 2,
          WebkitBoxOrient: "vertical",
          overflow: "hidden",
        }}
      >
        {d.text}
      </span>
    </div>
  );
}

// ---------- custom provenance edge (a soft tinted curve, no label) ----------

function MindmapEdgeView({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data }: EdgeProps) {
  const [edgePath] = getBezierPath({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition });
  const stroke = (data as { stroke?: string } | undefined)?.stroke ?? "var(--mk-faint)";
  return <BaseEdge id={id} path={edgePath} style={{ stroke, strokeWidth: 2 }} />;
}

const nodeTypes = { mindmap: MindmapNodeView };
const edgeTypes = { provenance: MindmapEdgeView };

type RFNode = Node<MindmapNodeData>;

function posStorageKey(projectId: string, rootId: string): string {
  return `mk-mindmap-pos:${projectId}:${rootId}`;
}

function readSavedPositions(projectId: string, rootId: string): Record<string, { x: number; y: number }> {
  try {
    const raw = localStorage.getItem(posStorageKey(projectId, rootId));
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    return parsed && typeof parsed === "object" ? (parsed as Record<string, { x: number; y: number }>) : {};
  } catch {
    return {};
  }
}

export type QuestionMindmapProps = {
  projectId: string;
  root: ExplorationLead;
  leads: ExplorationLead[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onDeleteLead: (id: string) => void;
};

function QuestionMindmapInner({ projectId, root, leads, selectedId, onSelect, onDeleteLead }: QuestionMindmapProps) {
  // Which node's × was pressed → confirm-delete modal (铁律②: an explicit
  // student confirm before a node — and, for the root, its whole subtree — is
  // removed).
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; text: string } | null>(null);
  const requestDelete = useCallback(
    (id: string) => {
      const l = leads.find((x) => x.id === id);
      setDeleteTarget({ id, text: l?.text ?? "" });
    },
    [leads],
  );

  const saved = useMemo(() => readSavedPositions(projectId, root.id), [projectId, root.id]);
  const nodeModels = useMemo(() => buildMindmapNodes(root.id, leads, saved), [root.id, leads, saved]);
  const edgeModels = useMemo(() => buildMindmapEdges(root.id, leads), [root.id, leads]);
  const edgeStroke = useMemo(() => mixToward(themeForRoot(root.id, leads).border, NEUTRAL.surface, 0.4), [root.id, leads]);

  // React Flow node state, reconciled from the models on every data refresh:
  // surviving nodes keep their (possibly-dragged) position; new nodes take their
  // radial slot; removed nodes drop out. Data (theme/kind/selected) refreshes.
  const [rfNodes, setRfNodes] = useState<RFNode[]>([]);
  useEffect(() => {
    setRfNodes((prev) => {
      const prevById = new Map(prev.map((n) => [n.id, n]));
      return nodeModels.map((m) => {
        const existing = prevById.get(m.id);
        return {
          id: m.id,
          type: "mindmap",
          position: existing?.position ?? m.position,
          data: {
            text: m.text,
            kind: m.kind,
            isRoot: m.isRoot,
            theme: m.theme,
            selected: m.id === selectedId,
            onRequestDelete: requestDelete,
          },
          draggable: true,
        } as RFNode;
      });
    });
  }, [nodeModels, selectedId, requestDelete]);

  const onNodesChange = useCallback((changes: NodeChange<RFNode>[]) => {
    setRfNodes((nds) => applyNodeChanges(changes, nds) as RFNode[]);
  }, []);

  const onNodeDragStop = useCallback(
    (_evt: unknown, node: RFNode) => {
      try {
        const cur = readSavedPositions(projectId, root.id);
        cur[node.id] = { x: Math.round(node.position.x), y: Math.round(node.position.y) };
        localStorage.setItem(posStorageKey(projectId, root.id), JSON.stringify(cur));
      } catch {
        /* best-effort; a full/blocked storage must never break the map */
      }
    },
    [projectId, root.id],
  );

  const rfEdges = useMemo(
    () =>
      edgeModels.map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        type: "provenance",
        data: { stroke: edgeStroke },
      })),
    [edgeModels, edgeStroke],
  );

  const deletingRoot = deleteTarget?.id === root.id;

  return (
    <div className="relative h-full w-full bg-mk-paper">
      <ReactFlow
        nodes={rfNodes}
        edges={rfEdges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={onNodesChange}
        onNodeDragStop={onNodeDragStop}
        onNodeClick={(_evt, node) => onSelect(node.id)}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        minZoom={0.3}
        maxZoom={1.6}
        proOptions={{ hideAttribution: true }}
        nodesConnectable={false}
      >
        {/* Warm paper dot grid (spec §18). React Flow's Background `color` prop
            takes a literal, not a CSS var — #E7DDD0 is the spec's own value. */}
        <Background gap={18} size={1.4} color="#E7DDD0" />
        <Controls showInteractive={false} />
      </ReactFlow>

      {deleteTarget && (
        <div
          className="absolute inset-0 z-50 flex items-center justify-center bg-black/30 px-4"
          onClick={() => setDeleteTarget(null)}
        >
          <div
            className="w-80 rounded-mk-lg border border-mk-border bg-mk-surface p-4 shadow-[0_18px_44px_rgba(28,35,51,0.24)]"
            onClick={(e) => e.stopPropagation()}
          >
            <p className="text-[14px] font-bold text-mk-ink">{deletingRoot ? "删除这个问题？" : "删除这一项？"}</p>
            {deleteTarget.text && <p className="mt-1 text-[14px] text-mk-muted line-clamp-2">「{deleteTarget.text}」</p>}
            <p className="mt-1.5 text-[12px] leading-relaxed text-mk-faint">
              {deletingRoot
                ? "下面挖到的论文也会一起移除，这一步不能撤销。"
                : "挂在它下面的项也会一起移除，这一步不能撤销。"}
            </p>
            <div className="mt-3 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setDeleteTarget(null)}
                className="rounded-mk border border-mk-border px-3 py-1.5 text-[12px] font-bold text-mk-faint hover:text-mk-ink"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => {
                  onDeleteLead(deleteTarget.id);
                  setDeleteTarget(null);
                }}
                className="rounded-mk bg-mk-accent px-3 py-1.5 text-[12px] font-bold text-white hover:bg-mk-accent-600"
              >
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// React Flow needs a provider in the tree for its hooks/measurement.
export function QuestionMindmap(props: QuestionMindmapProps) {
  return (
    <ReactFlowProvider>
      <QuestionMindmapInner {...props} />
    </ReactFlowProvider>
  );
}
