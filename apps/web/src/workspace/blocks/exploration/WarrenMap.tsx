import { useCallback, useEffect, useMemo, useState } from "react";
import {
  applyNodeChanges,
  Background,
  BaseEdge,
  Controls,
  EdgeLabelRenderer,
  getBezierPath,
  Handle,
  Position,
  ReactFlow,
  ReactFlowProvider,
  type Connection,
  type EdgeProps,
  type Node,
  type NodeChange,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { ExplorationLead, QuestionEdge, QuestionEdgeLabel } from "@mind-imprint/contracts";
import {
  buildWarrenEdges,
  buildWarrenNodes,
  EDGE_LABELS,
  type NodeTheme,
} from "./warrenLayout";

// GVa · the Level-1 "兔子洞地图": a real, interactive React Flow graph of the
// project's ROOT question nodes. Each root is a white card with a macaron
// ordinal left bar (per-root theme via warrenLayout), draggable, with zoom/pan
// and a fit-view. Clicking a
// node zooms into that question's Level-2 view (the caller's job). Relations are
// labeled edges — dragged node-to-node by the student, or proposed by 印记 and
// confirmed by the student (铁律②: nothing auto-confirms).
//
// The heavy layout / theming / counting logic lives in warrenLayout.ts (pure,
// unit-tested); this file is the thin React Flow shell over it. In tests
// @xyflow/react is mocked (it can't render in jsdom), which is why the node /
// edge views only lean on trivially-mockable primitives (Handle, BaseEdge,
// EdgeLabelRenderer, getBezierPath) and keep all real DOM in plain JSX.

// 铁律②: confirmed relations draw ink-solid; proposed (印记-suggested, not yet
// confirmed) draw muted-dashed (spec §18). React Flow's edge `style` prop is
// applied as a real inline `style` attribute (not an SVG presentation attr), so
// `var(--mk-ink)` resolves fine at runtime — EDGE_DASHED is spec's own literal
// `#C9BCAD` (no existing named token matches it exactly), kept as a documented
// literal per the "restyle only, minimize hex" rule.
const EDGE_SOLID = "var(--mk-ink)"; // confirmed relation
const EDGE_DASHED = "#C9BCAD"; // proposed / unconfirmed — spec §18 literal

// The 未归类 system node's fixed id — a sentinel, never a real ExplorationLead
// id, so it never collides with a question node and can be cheaply guarded
// against everywhere (click routing, drag persistence).
export const UNFILED_NODE_ID = "__unfiled__";

// Pure click router, unit-tested without rendering React Flow (jsdom can't
// render the canvas): the sentinel opens the unfiled panel, any real root
// zooms in as before.
export function routeNodeClick(nodeId: string, onZoom: (id: string) => void, onOpenUnfiled: () => void): void {
  if (nodeId === UNFILED_NODE_ID) onOpenUnfiled();
  else onZoom(nodeId);
}

// ---------- data threaded onto each React Flow node / edge ----------

type WarrenNodeData = {
  text: string;
  paperCount: number;
  theme: NodeTheme;
  onRequestDelete: (id: string) => void;
};

type QuestionEdgeData = {
  label: QuestionEdgeLabel;
  status: "proposed" | "confirmed";
  busy: boolean;
  onConfirm: (id: string) => void;
  onDismiss: (id: string) => void;
  onRelabel: (id: string, label: QuestionEdgeLabel) => void;
};

// ---------- custom card node ----------

// selected comes straight off React Flow's NodeProps — it is the sole place a
// question node reads accent (spec §18: accent is reserved for selection; the
// per-root color is the macaron ordinal theme, never accent).
function WarrenNodeView({ id, data, selected }: NodeProps) {
  const d = data as unknown as WarrenNodeData;
  const { text, paperCount, theme, onRequestDelete } = d;
  return (
    <div
      data-theme={theme.key}
      data-selected={selected ? "true" : "false"}
      data-tour="warren-question"
      className="group relative flex flex-col justify-center overflow-hidden border border-mk-border bg-mk-surface py-3 pl-5 pr-3"
      style={{
        width: 208,
        height: 104,
        borderRadius: "var(--mk-radius-md)", // spec §18 L1 card radius 10
        boxShadow: selected
          ? "0 0 0 3px var(--mk-accent), var(--mk-shadow-sm)" // selection ring — accent, reserved for this
          : "var(--mk-shadow-xs)",
        cursor: "pointer",
      }}
    >
      {/* Macaron ordinal left bar — the only per-root color on the card. */}
      <span aria-hidden className="absolute inset-y-0 left-0 w-1.5" style={{ background: theme.border }} />

      {/* Drag-connect handles: near-invisible until you hover the node, then a
          small colored dot you can grab and pull to another node. */}
      <Handle
        type="target"
        position={Position.Left}
        className="nodrag"
        style={{ width: 9, height: 9, background: theme.border, border: "2px solid var(--mk-surface)", opacity: 0.35 }}
      />
      <Handle
        type="source"
        position={Position.Right}
        className="nodrag"
        style={{ width: 9, height: 9, background: theme.border, border: "2px solid var(--mk-surface)", opacity: 0.35 }}
      />

      {/* × delete → confirm modal. nodrag + stopPropagation so it neither starts
          a drag nor triggers the node's zoom-on-click. */}
      <button
        type="button"
        aria-label="删除这个问题"
        title="删除这个问题"
        className="nodrag absolute right-2 top-2 flex h-5 w-5 items-center justify-center rounded-full bg-mk-paper text-[14px] font-bold leading-none opacity-0 shadow-mk-xs transition group-hover:opacity-100 hover:bg-mk-paper"
        style={{ color: theme.label }}
        onClick={(e) => {
          e.stopPropagation();
          onRequestDelete(id);
        }}
      >
        ×
      </button>

      <span
        className="text-[14px] font-semibold leading-snug"
        style={{
          color: theme.label,
          display: "-webkit-box",
          WebkitLineClamp: 2,
          WebkitBoxOrient: "vertical",
          overflow: "hidden",
        }}
      >
        {text}
      </span>
      <span className="mt-1 text-[12px] font-bold text-mk-muted">文献 {paperCount} 篇</span>
    </div>
  );
}

// Neutral system node for references that hang under no question yet. Not a
// real ExplorationLead — no theme, no delete, no drag-connect handles; a
// click routes to `onOpenUnfiled` (see routeNodeClick), never `onZoom`.
function UnfiledNodeView({ data }: NodeProps) {
  const d = data as unknown as { count: number };
  return (
    <div
      data-tour="warren-unfiled"
      className="flex flex-col justify-center overflow-hidden border border-dashed border-mk-border bg-mk-paper py-3 pl-5 pr-4"
      style={{ width: 208, height: 104, borderRadius: "var(--mk-radius-md)", boxShadow: "var(--mk-shadow-xs)", cursor: "pointer" }}
    >
      <span className="text-[14px] font-bold text-mk-muted">未归类</span>
      <span className="mt-1 text-[12px] font-bold text-mk-faint">还没挂到问题下 · {d.count} 篇</span>
      <span className="mt-1 text-[11px] text-mk-faint">点开，把它们挂到问题下</span>
    </div>
  );
}

// ---------- custom edge (label chip + controls) ----------

function QuestionEdgeView({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data }: EdgeProps) {
  const d = data as unknown as QuestionEdgeData;
  const { label, status, busy, onConfirm, onDismiss, onRelabel } = d;
  const [pickerOpen, setPickerOpen] = useState(false);
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });
  const confirmed = status === "confirmed";

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={{
          stroke: confirmed ? EDGE_SOLID : EDGE_DASHED,
          strokeWidth: 2,
          strokeDasharray: confirmed ? undefined : "6 5",
        }}
      />
      <EdgeLabelRenderer>
        <div
          data-testid={confirmed ? "edge-confirmed" : "edge-proposed"}
          className="nodrag nopan absolute"
          style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`, pointerEvents: "all" }}
        >
          {confirmed ? (
            // Confirmed: the label chip is a button → a relabel picker (closed
            // set) + a delete row.
            <div className="relative flex -translate-y-1/2 flex-col items-center">
              <button
                type="button"
                onClick={() => setPickerOpen((o) => !o)}
                disabled={busy}
                title="换一个关系词，或删除这条连线"
                className="whitespace-nowrap rounded-full border border-mk-accent/40 bg-mk-surface px-2 py-0.5 text-[12px] font-bold text-mk-accent shadow-sm hover:border-mk-accent disabled:opacity-50"
              >
                {label}
              </button>
              {pickerOpen && (
                <div className="absolute left-1/2 top-full z-30 mt-1 w-28 -translate-x-1/2 rounded-mk border border-mk-border bg-mk-surface p-1 shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
                  {EDGE_LABELS.map((l) => (
                    <button
                      key={l}
                      type="button"
                      onClick={() => {
                        setPickerOpen(false);
                        onRelabel(id, l);
                      }}
                      className={`block w-full rounded px-2 py-1 text-left text-[12px] font-semibold hover:bg-mk-accent-50 ${
                        l === label ? "text-mk-accent" : "text-mk-ink"
                      }`}
                    >
                      {l}
                    </button>
                  ))}
                  <button
                    type="button"
                    onClick={() => {
                      setPickerOpen(false);
                      onDismiss(id);
                    }}
                    className="mt-0.5 block w-full rounded border-t border-mk-border px-2 py-1 text-left text-[12px] font-semibold text-mk-muted hover:bg-mk-paper hover:text-mk-accent"
                  >
                    删除
                  </button>
                </div>
              )}
            </div>
          ) : (
            // Proposed (dashed): the label chip + 确认 / 忽略 (铁律②: student decides).
            // Taro-colored (spec §18) — visually marks "印记提议" without a
            // separate copy string, distinct from a confirmed relation's chip.
            <div className="flex flex-col items-center gap-1">
              <span className="whitespace-nowrap rounded-full border border-dashed border-mk-taro/50 bg-mk-taro-bg px-1.5 py-0.5 text-[12px] font-bold text-mk-taro-fg">
                {label}
              </span>
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => onConfirm(id)}
                  disabled={busy}
                  className="rounded-full bg-mk-success px-2 py-0.5 text-[12px] font-bold text-white shadow-sm hover:opacity-90 disabled:opacity-50"
                >
                  确认
                </button>
                <button
                  type="button"
                  onClick={() => onDismiss(id)}
                  disabled={busy}
                  className="rounded-full border border-mk-border bg-mk-surface px-2 py-0.5 text-[12px] font-bold text-mk-faint shadow-sm hover:text-mk-accent disabled:opacity-50"
                >
                  忽略
                </button>
              </div>
            </div>
          )}
        </div>
      </EdgeLabelRenderer>
    </>
  );
}

// Stable references (React Flow requires nodeTypes/edgeTypes not be re-created).
const nodeTypes = { warren: WarrenNodeView, unfiled: UnfiledNodeView };
const edgeTypes = { question: QuestionEdgeView };

export type WarrenMapProps = {
  projectId: string;
  roots: ExplorationLead[];
  // "文献 x 篇" per root — descendant papers (connectedReferenceId), from warrenLayout.
  countByRoot: Map<string, number>;
  edges: QuestionEdge[];
  onZoom: (rootId: string) => void;
  // Edge lifecycle (铁律②: 印记 proposes, student decides).
  busyEdgeIds: Set<string>;
  onConfirmEdge: (eid: string) => void;
  onDismissEdge: (eid: string) => void;
  onRelabelEdge: (eid: string, label: QuestionEdgeLabel) => void;
  // Student-drawn relation (drag one question onto another) + deleting a question.
  onCreateEdge: (fromLeadId: string, toLeadId: string, label: QuestionEdgeLabel) => void;
  onDeleteLead: (leadId: string) => void;
  // The 未归类 system node: how many references hang under no question (read or
  // unread), and what to do when the student opens it. count 0 → node hidden.
  unfiledCount: number;
  onOpenUnfiled: () => void;
};

type RFNode = Node<WarrenNodeData>;

function posStorageKey(projectId: string): string {
  return `mk-warren-pos:${projectId}`;
}

function readSavedPositions(projectId: string): Record<string, { x: number; y: number }> {
  try {
    const raw = localStorage.getItem(posStorageKey(projectId));
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    return parsed && typeof parsed === "object" ? (parsed as Record<string, { x: number; y: number }>) : {};
  } catch {
    return {};
  }
}

function WarrenMapInner({
  projectId,
  roots,
  countByRoot,
  edges,
  onZoom,
  busyEdgeIds,
  onConfirmEdge,
  onDismissEdge,
  onRelabelEdge,
  onCreateEdge,
  onDeleteLead,
  unfiledCount,
  onOpenUnfiled,
}: WarrenMapProps) {
  const [helpOpen, setHelpOpen] = useState(false);
  // Which question's × was pressed → confirm-delete modal (铁律②: an explicit
  // student confirm before a question and its papers are removed).
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; text: string } | null>(null);
  // A pending drag-connect awaiting its relation label (closed set).
  const [connectPending, setConnectPending] = useState<{ from: string; to: string } | null>(null);
  // A gentle note when a drag-connect is a no-op (the pair is already linked).
  const [dupNote, setDupNote] = useState<string | null>(null);

  const requestDelete = useCallback((id: string) => {
    setDeleteTarget((cur) => cur ?? { id, text: "" });
  }, []);
  // Fill in the question text for the modal from the current roots.
  const deleteText = useMemo(
    () => (deleteTarget ? roots.find((r) => r.id === deleteTarget.id)?.text ?? "" : ""),
    [deleteTarget, roots],
  );

  // Node models (positions restored from localStorage where dragged before).
  const saved = useMemo(() => readSavedPositions(projectId), [projectId]);
  const nodeModels = useMemo(
    () => buildWarrenNodes(roots, countByRoot, saved),
    [roots, countByRoot, saved],
  );

  // React Flow node state. Reconciled from the models on every data refresh:
  // surviving nodes keep their (possibly-dragged) position; new nodes get their
  // circle slot; removed nodes drop out. Data (count/theme) always refreshes.
  const [rfNodes, setRfNodes] = useState<RFNode[]>([]);
  useEffect(() => {
    setRfNodes((prev) => {
      const prevById = new Map(prev.map((n) => [n.id, n]));
      const rootNodes = nodeModels.map((m) => {
        const existing = prevById.get(m.id);
        return {
          id: m.id,
          type: "warren",
          position: existing?.position ?? m.position,
          data: { text: m.text, paperCount: m.paperCount, theme: m.theme, onRequestDelete: requestDelete },
          draggable: true,
        } as RFNode;
      });
      if (unfiledCount > 0) {
        const existing = prevById.get(UNFILED_NODE_ID);
        rootNodes.push({
          id: UNFILED_NODE_ID,
          type: "unfiled",
          position: existing?.position ?? { x: 0, y: 320 },
          data: { count: unfiledCount },
          draggable: false,
        } as unknown as RFNode);
      }
      return rootNodes;
    });
  }, [nodeModels, requestDelete, unfiledCount]);

  const onNodesChange = useCallback((changes: NodeChange<RFNode>[]) => {
    setRfNodes((nds) => applyNodeChanges(changes, nds) as RFNode[]);
  }, []);

  // Persist a node's resting position after a drag (best-effort localStorage).
  const onNodeDragStop = useCallback(
    (_evt: unknown, node: RFNode) => {
      if (node.id === UNFILED_NODE_ID) return;
      try {
        const cur = readSavedPositions(projectId);
        cur[node.id] = { x: Math.round(node.position.x), y: Math.round(node.position.y) };
        localStorage.setItem(posStorageKey(projectId), JSON.stringify(cur));
      } catch {
        /* best-effort; a full/blocked storage must never break the map */
      }
    },
    [projectId],
  );

  // Drag one question onto another → ask for the relation label (no auto-label).
  // Guards: no self-loops, and no DUPLICATE of an existing (from→to) edge — the
  // question_edge table is UNIQUE(from_lead_id, to_lead_id) and the server does a
  // plain insert, so a repeat would 500. On a duplicate we no-op with a gentle
  // inline note instead of firing a request that can't succeed.
  const onConnect = useCallback(
    (c: Connection) => {
      if (!c.source || !c.target || c.source === c.target) return;
      const dup = edges.some((e) => e.fromLeadId === c.source && e.toLeadId === c.target);
      if (dup) {
        setDupNote("这两个问题已经连过了。");
        return;
      }
      setDupNote(null);
      setConnectPending({ from: c.source, to: c.target });
    },
    [edges],
  );

  const rootIds = useMemo(() => new Set(roots.map((r) => r.id)), [roots]);
  const rfEdges = useMemo(
    () =>
      buildWarrenEdges(edges, rootIds).map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        type: "question",
        data: {
          label: e.label,
          status: e.status,
          busy: busyEdgeIds.has(e.id),
          onConfirm: onConfirmEdge,
          onDismiss: onDismissEdge,
          onRelabel: onRelabelEdge,
        } satisfies QuestionEdgeData,
      })),
    [edges, rootIds, busyEdgeIds, onConfirmEdge, onDismissEdge, onRelabelEdge],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Header · the explained title. 「兔子洞」is welcome ONLY here, as a title
          with a "?" that explains the metaphor in plain words. */}
      <div className="relative mb-2 flex flex-none items-center gap-2">
        <h3 className="font-sans text-[14px] font-bold text-mk-ink">兔子洞地图</h3>
        <button
          type="button"
          aria-label="这是什么"
          onClick={() => setHelpOpen((o) => !o)}
          className="flex h-5 w-5 items-center justify-center rounded-full border border-mk-border bg-mk-surface text-[12px] font-bold text-mk-faint hover:border-mk-accent hover:text-mk-accent"
        >
          ?
        </button>
        {dupNote ? (
          <span className="ml-auto text-[12px] font-semibold text-mk-accent">{dupNote}</span>
        ) : (
          <span className="ml-auto text-[12px] text-mk-faint">拖动问题排布 · 拖一个问题到另一个上，连出它们的关系</span>
        )}
        {helpOpen && (
          <div className="absolute left-0 top-full z-30 mt-1 w-72 rounded-mk border border-mk-border bg-mk-surface p-3 text-[12px] leading-relaxed text-mk-ink shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
            「兔子洞」= 你顺着一个问题往下追的过程。每个问题就是一个洞口，钻进去能看到你为它挖到的文献。点开一个问题往下挖，这张图就会慢慢长出来。
          </div>
        )}
      </div>

      <div className="relative min-h-[320px] w-full flex-1 overflow-hidden rounded-mk-lg border border-mk-border bg-mk-paper">
        <ReactFlow
          nodes={rfNodes}
          edges={rfEdges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onNodesChange={onNodesChange}
          onNodeDragStop={onNodeDragStop}
          onNodeClick={(_evt, node) => routeNodeClick(node.id, onZoom, onOpenUnfiled)}
          onConnect={onConnect}
          fitView
          fitViewOptions={{ padding: 0.25 }}
          minZoom={0.3}
          maxZoom={1.6}
          proOptions={{ hideAttribution: true }}
          nodesConnectable
        >
          {/* Warm paper dot grid (spec §18). React Flow's Background `color` prop
              takes a literal, not a CSS var — #E7DDD0 is the spec's own value. */}
          <Background gap={18} size={1.4} color="#E7DDD0" />
          <Controls showInteractive={false} />
        </ReactFlow>

        {/* Drag-connect → relation label picker (closed vocabulary, no freeform). */}
        {connectPending && (
          <div className="absolute inset-0 z-40 flex items-center justify-center bg-black/30" onClick={() => setConnectPending(null)}>
            <div
              className="w-64 rounded-mk-lg border border-mk-border bg-mk-surface p-3 shadow-[0_18px_44px_rgba(28,35,51,0.24)]"
              onClick={(e) => e.stopPropagation()}
            >
              <p className="mb-2 text-[14px] font-bold text-mk-ink">这两个问题是什么关系？</p>
              <div className="flex flex-col gap-1">
                {EDGE_LABELS.map((l) => (
                  <button
                    key={l}
                    type="button"
                    onClick={() => {
                      onCreateEdge(connectPending.from, connectPending.to, l);
                      setConnectPending(null);
                    }}
                    className="rounded-mk px-2.5 py-1.5 text-left text-[14px] font-semibold text-mk-ink hover:bg-mk-accent-50 hover:text-mk-accent"
                  >
                    {l}
                  </button>
                ))}
              </div>
              <button
                type="button"
                onClick={() => setConnectPending(null)}
                className="mt-2 w-full rounded-mk border border-mk-border px-2.5 py-1.5 text-[12px] font-bold text-mk-faint hover:text-mk-ink"
              >
                取消
              </button>
            </div>
          </div>
        )}
      </div>

      {/* × on a node → confirm before removing the question (and its papers). */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" onClick={() => setDeleteTarget(null)}>
          <div
            className="w-80 rounded-mk-lg border border-mk-border bg-mk-surface p-4 shadow-[0_18px_44px_rgba(28,35,51,0.24)]"
            onClick={(e) => e.stopPropagation()}
          >
            <p className="text-[14px] font-bold text-mk-ink">删除这个问题？</p>
            {deleteText && <p className="mt-1 text-[14px] text-mk-muted line-clamp-2">「{deleteText}」</p>}
            <p className="mt-1.5 text-[12px] leading-relaxed text-mk-faint">下面挖到的文献也会一起移除，这一步不能撤销。</p>
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
export function WarrenMap(props: WarrenMapProps) {
  return (
    <ReactFlowProvider>
      <WarrenMapInner {...props} />
    </ReactFlowProvider>
  );
}
