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

// ============================================================================
// GVb · Level-2 (inside one question) — a radial mindmap of that question's
// subtree. Same pure-helper discipline as Level-1: layout + theming compute
// here (unit-tested in plain jsdom); QuestionMindmap.tsx is the React Flow shell.
// ============================================================================

// Blend a hex color toward a target (used to LIGHTEN a root's theme for its
// deeper descendants — so a question's whole subtree reads as one color family,
// root strongest, papers lighter tints).
function clamp255(n: number): number {
  return Math.max(0, Math.min(255, Math.round(n)));
}
function parseHex(hex: string): [number, number, number] {
  const s = hex.replace("#", "");
  return [parseInt(s.slice(0, 2), 16), parseInt(s.slice(2, 4), 16), parseInt(s.slice(4, 6), 16)];
}
export function mixToward(hex: string, target: string, t: number): string {
  const a = parseHex(hex);
  const b = parseHex(target);
  const out = a.map((c, i) => clamp255(c + (b[i]! - c) * t));
  return "#" + out.map((n) => n.toString(16).padStart(2, "0")).join("");
}

// A depth-tinted theme: depth 0 = the root's own (strongest) theme; deeper nodes
// keep the same border/label hue but get a progressively lighter fill.
export function depthTint(base: NodeTheme, depth: number): NodeTheme {
  if (depth <= 0) return base;
  const t = Math.min(0.5, 0.26 * depth);
  return {
    ...base,
    fillFrom: mixToward(base.fillFrom, "#FFFFFF", t),
    fillTo: mixToward(base.fillTo, "#FFFFFF", t),
  };
}

// Live (non-pruned) children indexed by parent id — the backbone of the subtree.
function indexLiveChildren(leads: ExplorationLead[]): Map<string, ExplorationLead[]> {
  const m = new Map<string, ExplorationLead[]>();
  for (const l of leads) {
    if (l.status === "pruned") continue;
    if (l.parentLeadId) {
      const arr = m.get(l.parentLeadId) ?? [];
      arr.push(l);
      m.set(l.parentLeadId, arr);
    }
  }
  return m;
}

export type PosD = { x: number; y: number; depth: number };

// Radial tree layout for one question's subtree: the root at the origin, its
// descendants on concentric rings by depth. Leaves get evenly-spread angular
// slots; each internal node is centered over its children's angles, so siblings
// fan out and a chain stays roughly collinear. Deterministic (no Math.random),
// fitView re-centers. Cycle-guarded so malformed data can't loop forever.
export function radialLayout(rootId: string, childrenByParent: Map<string, ExplorationLead[]>): Map<string, PosD> {
  const depthOf = new Map<string, number>();
  const order: string[] = [];
  const leaves: string[] = [];
  // Pre-order DFS (guarding cycles) to establish depth + leaf set.
  const dfs = (id: string, depth: number) => {
    if (depthOf.has(id)) return;
    depthOf.set(id, depth);
    order.push(id);
    const kids = childrenByParent.get(id) ?? [];
    if (kids.length === 0) leaves.push(id);
    for (const k of kids) dfs(k.id, depth + 1);
  };
  dfs(rootId, 0);

  const n = leaves.length;
  const leafAngle = new Map<string, number>();
  leaves.forEach((id, i) => leafAngle.set(id, n <= 1 ? -Math.PI / 2 : -Math.PI / 2 + (i * 2 * Math.PI) / n));

  const angleOf = new Map<string, number>();
  const computeAngle = (id: string): number => {
    const kids = (childrenByParent.get(id) ?? []).filter((k) => depthOf.has(k.id));
    if (kids.length === 0) {
      const a = leafAngle.get(id) ?? -Math.PI / 2;
      angleOf.set(id, a);
      return a;
    }
    const a = kids.reduce((s, k) => s + computeAngle(k.id), 0) / kids.length;
    angleOf.set(id, a);
    return a;
  };
  computeAngle(rootId);

  const ringStep = 260;
  const out = new Map<string, PosD>();
  for (const id of order) {
    const depth = depthOf.get(id)!;
    if (depth === 0) {
      out.set(id, { x: 0, y: 0, depth });
      continue;
    }
    const r = depth * ringStep;
    const a = angleOf.get(id) ?? -Math.PI / 2;
    out.set(id, { x: Math.round(r * Math.cos(a) * 1.25), y: Math.round(r * Math.sin(a)), depth });
  }
  return out;
}

export type MindmapNodeKind = "question" | "paper";

export type MindmapNode = {
  id: string;
  kind: MindmapNodeKind;
  text: string;
  isRoot: boolean;
  depth: number;
  theme: NodeTheme;
  connectedReferenceId: string | null;
  position: { x: number; y: number };
};

// One question's subtree → mindmap node models. A lead carrying a
// connectedReferenceId is a "paper"; everything else is a "question" (the root,
// or a sub-question). `saved` (dragged positions) wins over the radial slot.
export function buildMindmapNodes(
  rootId: string,
  leads: ExplorationLead[],
  saved?: Record<string, { x: number; y: number }>,
): MindmapNode[] {
  const children = indexLiveChildren(leads);
  const pos = radialLayout(rootId, children);
  const base = themeForId(rootId);
  const byId = new Map(leads.map((l) => [l.id, l]));
  const nodes: MindmapNode[] = [];
  for (const [id, p] of pos) {
    const lead = byId.get(id);
    if (!lead) continue;
    nodes.push({
      id,
      kind: lead.connectedReferenceId ? "paper" : "question",
      text: lead.text,
      isRoot: id === rootId,
      depth: p.depth,
      theme: depthTint(base, p.depth),
      connectedReferenceId: lead.connectedReferenceId,
      position: saved?.[id] ?? { x: p.x, y: p.y },
    });
  }
  return nodes;
}

export type MindmapEdge = { id: string; source: string; target: string };

// Provenance edges (parentLeadId → child) within the focused subtree only.
export function buildMindmapEdges(rootId: string, leads: ExplorationLead[]): MindmapEdge[] {
  const children = indexLiveChildren(leads);
  const ids = new Set(radialLayout(rootId, children).keys());
  const byId = new Map(leads.map((l) => [l.id, l]));
  const edges: MindmapEdge[] = [];
  for (const id of ids) {
    const lead = byId.get(id);
    if (lead?.parentLeadId && ids.has(lead.parentLeadId)) {
      edges.push({ id: `${lead.parentLeadId}->${id}`, source: lead.parentLeadId, target: id });
    }
  }
  return edges;
}
