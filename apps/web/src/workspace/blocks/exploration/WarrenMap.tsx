import { useMemo } from "react";
import type { ExplorationLead, QuestionEdge } from "@mind-imprint/contracts";

// B4a · the top-level "map" — the overview graph of ROOT question nodes.
// Each root question is a card; labeled question_edges connect them. Layout is
// HAND-LAID (no physics/graph library): nodes sit on a deterministic circle
// derived from index, edges are an SVG overlay drawn in a 0–100 percent
// coordinate space so lines track node centers at any container size.
//
// Zoom model: clicking a node → the caller zooms into that question's "inside"
// (its subtree). Plain copy only — no rabbit metaphor in any rendered string.

type Pt = { x: number; y: number };

// Confirmed edges draw solid; proposed edges draw dashed (铁律①: an AI proposal
// hasn't been confirmed onto the map yet — B4b adds its 确认/忽略 controls).
const EDGE_SOLID = "#2A3B7A"; // mk-primary
const EDGE_DASHED = "#9AA1B0"; // mk-muted-2
const HINT = "#C7CBD6"; // fainter than a real edge — the derived 同源 hint

export type WarrenMapProps = {
  roots: ExplorationLead[];
  // Total descendants (papers + sub-questions) hanging under each root.
  countByRoot: Map<string, number>;
  edges: QuestionEdge[];
  // Derived (NOT question_edges): pairs of roots whose subtrees adopted the same
  // paper — drawn as a faint dotted "同源" hint line.
  sameSourcePairs: Array<[string, string]>;
  onZoom: (rootId: string) => void;
};

export function WarrenMap({ roots, countByRoot, edges, sameSourcePairs, onZoom }: WarrenMapProps) {
  // Deterministic circle layout: stable across renders (position from index, no
  // Math.random). A single node sits dead-center; the rest ring around it.
  const positions = useMemo(() => {
    const m = new Map<string, Pt>();
    const n = roots.length;
    if (n === 1) {
      m.set(roots[0]!.id, { x: 50, y: 50 });
      return m;
    }
    const cx = 50;
    const cy = 50;
    const rx = 38;
    const ry = 34;
    roots.forEach((r, i) => {
      const angle = -Math.PI / 2 + (i * 2 * Math.PI) / n;
      m.set(r.id, { x: cx + rx * Math.cos(angle), y: cy + ry * Math.sin(angle) });
    });
    return m;
  }, [roots]);

  // Only edges whose endpoints are both laid-out roots can be drawn.
  const drawableEdges = useMemo(
    () =>
      edges
        .map((e) => {
          const a = positions.get(e.fromLeadId);
          const b = positions.get(e.toLeadId);
          if (!a || !b) return null;
          return { edge: e, a, b, mid: { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 } };
        })
        .filter((x): x is { edge: QuestionEdge; a: Pt; b: Pt; mid: Pt } => x != null),
    [edges, positions],
  );

  const drawableHints = useMemo(
    () =>
      sameSourcePairs
        .map(([from, to]) => {
          const a = positions.get(from);
          const b = positions.get(to);
          if (!a || !b) return null;
          return { key: `${from}~${to}`, a, b };
        })
        .filter((x): x is { key: string; a: Pt; b: Pt } => x != null),
    [sameSourcePairs, positions],
  );

  return (
    <div className="relative h-[440px] w-full rounded-mk-lg border border-mk-border bg-mk-surface">
      {/* SVG overlay — lines only. viewBox is the same 0–100 percent space the
          node cards are positioned in, so line ends land on node centers.
          preserveAspectRatio="none" stretches to the box; non-scaling-stroke
          keeps line weight uniform despite the stretch. */}
      <svg
        className="pointer-events-none absolute inset-0 h-full w-full"
        viewBox="0 0 100 100"
        preserveAspectRatio="none"
        aria-hidden="true"
      >
        {drawableHints.map((h) => (
          <line
            key={h.key}
            data-testid="samesource-hint"
            x1={h.a.x}
            y1={h.a.y}
            x2={h.b.x}
            y2={h.b.y}
            stroke={HINT}
            strokeWidth={1}
            strokeDasharray="1 3"
            vectorEffect="non-scaling-stroke"
          />
        ))}
        {drawableEdges.map(({ edge, a, b }) => (
          <line
            key={edge.id}
            data-testid={edge.status === "confirmed" ? "edge-confirmed" : "edge-proposed"}
            x1={a.x}
            y1={a.y}
            x2={b.x}
            y2={b.y}
            stroke={edge.status === "confirmed" ? EDGE_SOLID : EDGE_DASHED}
            strokeWidth={1.5}
            strokeDasharray={edge.status === "confirmed" ? undefined : "5 4"}
            vectorEffect="non-scaling-stroke"
          />
        ))}
      </svg>

      {/* Edge labels — HTML chips at the line midpoints (crisp text, unaffected
          by the SVG stretch). The label is the closed-vocabulary relation word. */}
      {drawableEdges.map(({ edge, mid }) => (
        <span
          key={`lbl-${edge.id}`}
          className={`absolute -translate-x-1/2 -translate-y-1/2 whitespace-nowrap rounded-full border px-1.5 py-0.5 text-[10px] font-bold ${
            edge.status === "confirmed"
              ? "border-mk-primary/30 bg-mk-surface text-mk-primary"
              : "border-dashed border-mk-muted-2/50 bg-mk-surface text-mk-muted-2"
          }`}
          style={{ left: `${mid.x}%`, top: `${mid.y}%` }}
        >
          {edge.label}
        </span>
      ))}

      {/* Root question nodes — absolutely positioned cards, click to zoom in. */}
      {roots.map((r) => {
        const p = positions.get(r.id)!;
        const count = countByRoot.get(r.id) ?? 0;
        const pruned = r.status === "pruned";
        return (
          <button
            key={r.id}
            type="button"
            onClick={() => onZoom(r.id)}
            title="点开看这条问题下面挖到了什么"
            style={{ left: `${p.x}%`, top: `${p.y}%` }}
            className={`absolute w-40 -translate-x-1/2 -translate-y-1/2 rounded-mk border px-3 py-2 text-left shadow-[0_6px_18px_rgba(28,35,51,0.10)] transition hover:border-mk-primary hover:shadow-[0_8px_22px_rgba(28,35,51,0.16)] ${
              pruned ? "border-mk-border bg-mk-bg" : "border-mk-primary/30 bg-mk-primary-tint/50"
            }`}
          >
            <span
              className={`block text-[12.5px] font-semibold leading-snug ${
                pruned ? "text-mk-muted-2 line-through" : "text-mk-ink"
              }`}
              style={{ display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}
            >
              {r.text}
            </span>
            <span className="mt-1 block text-[10.5px] font-bold text-mk-muted-2">
              {count > 0 ? `挂了 ${count} 项` : "还没往下挖"}
            </span>
          </button>
        );
      })}
    </div>
  );
}
