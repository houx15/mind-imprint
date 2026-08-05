import type { ExplorationLead, LeadStatus, QuestionEdge, QuestionEdgeLabel, QuestionEdgeStatus } from "@mind-imprint/contracts";

// GVa · pure layout + theming helpers for the Level-1 question graph. Kept free
// of React / React Flow so they unit-test in plain jsdom (React Flow itself
// can't render there). WarrenMap.tsx is the thin React Flow shell over these.

// A node color theme: a soft two-stop fill + a stronger border/label so each
// root question reads as its own tactile "bubble" — the fix for the old
// all-white, flat look. Families are derived from the mk-* brand (indigo /
// terracotta / teal-green / amber) plus complementary plum / slate / rose so
// ~7 questions each get a distinct, cohesive color without clashing.
export type NodeTheme = {
  key: string;
  fillFrom: string;
  fillTo: string;
  border: string;
  label: string; // strong text/label color
  glow: string; // shadow rgba
};

export const NODE_THEMES: NodeTheme[] = [
  { key: "indigo", fillFrom: "#EEF0FA", fillTo: "#DBE1F5", border: "#2A3B7A", label: "#2A3B7A", glow: "rgba(42,59,122,0.20)" },
  { key: "terracotta", fillFrom: "#FBEEE7", fillTo: "#F6DBCB", border: "#C4643F", label: "#AC4E2C", glow: "rgba(196,100,63,0.20)" },
  { key: "teal", fillFrom: "#E7F3EE", fillTo: "#CFE8DD", border: "#3B8168", label: "#2E6C56", glow: "rgba(76,154,130,0.20)" },
  { key: "amber", fillFrom: "#FBF1DA", fillTo: "#F6E1B0", border: "#C9862A", label: "#A06B1D", glow: "rgba(232,163,61,0.22)" },
  { key: "plum", fillFrom: "#F4E9F5", fillTo: "#E6D0EA", border: "#834E8C", label: "#6C3E74", glow: "rgba(131,78,140,0.20)" },
  { key: "slate", fillFrom: "#EAEEF5", fillTo: "#D6E0EC", border: "#4A6079", label: "#3B4E64", glow: "rgba(74,96,121,0.20)" },
  { key: "rose", fillFrom: "#FBE9EE", fillTo: "#F4D1DC", border: "#B8506A", label: "#9C3F57", glow: "rgba(184,80,106,0.20)" },
];

// Stable per-question theme: a small string hash of the lead id → theme index.
// Hashing the id (not the array index) keeps a question's color fixed even when
// siblings are added/removed and the list reorders.
export function themeIndexForId(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return h % NODE_THEMES.length;
}

export function themeForId(id: string): NodeTheme {
  return NODE_THEMES[themeIndexForId(id)]!;
}

// "文献 x 篇" tally: how many paper descendants hang under each root — i.e.
// descendant leads (any depth) that carry a connectedReferenceId. Sub-questions
// without a paper don't count; only adopted papers do.
export function countPapersByRoot(leads: ExplorationLead[]): Map<string, number> {
  const childrenByParent = new Map<string, ExplorationLead[]>();
  for (const l of leads) {
    if (l.parentLeadId) {
      const arr = childrenByParent.get(l.parentLeadId) ?? [];
      arr.push(l);
      childrenByParent.set(l.parentLeadId, arr);
    }
  }
  const countPapers = (id: string): number => {
    let n = 0;
    for (const k of childrenByParent.get(id) ?? []) {
      if (k.connectedReferenceId) n += 1;
      n += countPapers(k.id);
    }
    return n;
  };
  const roots = leads.filter((l) => l.parentLeadId == null);
  return new Map(roots.map((r) => [r.id, countPapers(r.id)]));
}

// Deterministic circle layout in React Flow's pixel space (fitView re-centers).
// One node sits at the origin; the rest ring around it. Radius grows with count
// so nodes never pile up. No Math.random — stable across renders.
export function circlePositions(rootIds: string[]): Map<string, { x: number; y: number }> {
  const m = new Map<string, { x: number; y: number }>();
  const n = rootIds.length;
  if (n === 0) return m;
  if (n === 1) {
    m.set(rootIds[0]!, { x: 0, y: 0 });
    return m;
  }
  const spacing = 250; // px of circumference budgeted per node
  const radius = Math.max(230, (n * spacing) / (2 * Math.PI));
  const rx = radius * 1.35; // wider than tall — matches the ellipse nodes
  const ry = radius;
  rootIds.forEach((id, i) => {
    const angle = -Math.PI / 2 + (i * 2 * Math.PI) / n;
    m.set(id, { x: Math.round(rx * Math.cos(angle)), y: Math.round(ry * Math.sin(angle)) });
  });
  return m;
}

export type WarrenNode = {
  id: string;
  text: string;
  paperCount: number;
  theme: NodeTheme;
  status: LeadStatus;
  position: { x: number; y: number };
};

// leads (roots) → node models. `saved` (dragged positions from localStorage,
// keyed by lead id) wins over the circle fallback; unknown ids get the circle
// slot. Everything else (theme, count) is derived deterministically.
export function buildWarrenNodes(
  roots: ExplorationLead[],
  paperCounts: Map<string, number>,
  saved?: Record<string, { x: number; y: number }>,
): WarrenNode[] {
  const circle = circlePositions(roots.map((r) => r.id));
  return roots.map((r) => ({
    id: r.id,
    text: r.text,
    paperCount: paperCounts.get(r.id) ?? 0,
    theme: themeForId(r.id),
    status: r.status,
    position: saved?.[r.id] ?? circle.get(r.id) ?? { x: 0, y: 0 },
  }));
}

export type WarrenEdge = {
  id: string;
  source: string;
  target: string;
  label: QuestionEdgeLabel;
  status: QuestionEdgeStatus;
};

// question_edges → edge models, dropping any whose endpoints aren't both roots
// (Level-1 only draws relations between root questions).
export function buildWarrenEdges(edges: QuestionEdge[], rootIds: Set<string>): WarrenEdge[] {
  return edges
    .filter((e) => rootIds.has(e.fromLeadId) && rootIds.has(e.toLeadId))
    .map((e) => ({ id: e.id, source: e.fromLeadId, target: e.toLeadId, label: e.label, status: e.status }));
}

// The closed relation vocabulary (kept in sync with contracts' QuestionEdgeLabel
// enum) — the drag-connect + relabel pickers offer exactly these, no freeform.
export const EDGE_LABELS: QuestionEdgeLabel[] = ["子问题", "支持", "反驳/张力", "细化", "依赖/前提"];
