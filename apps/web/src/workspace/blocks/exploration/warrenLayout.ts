import dagre from "@dagrejs/dagre";
import type { ExplorationLead, LeadStatus, QuestionEdge, QuestionEdgeLabel, QuestionEdgeStatus } from "@mind-imprint/contracts";
import { MACARONS, NEUTRAL, type MacaronName } from "@/ui/tokens";

// GVa/GVc · pure layout + theming helpers for the question graph. Kept free of
// React / React Flow so they unit-test in plain jsdom (React Flow itself can't
// render there). WarrenMap.tsx / QuestionMindmap.tsx are thin shells over these.

// A node color theme: a per-root accent (border/left-bar), a WCAG-legible label
// color, and a pale wash (fillFrom/fillTo) for chips/badges — so each root
// question reads as its own calm, tactile family without ever touching the
// product's accent color (accent is reserved for selection, spec §18).
export type NodeTheme = {
  key: string;
  fillFrom: string;
  fillTo: string;
  border: string;
  label: string; // strong text/label color
  glow: string; // shadow rgba
};

// Design-system §18: L1 question nodes get MACARON ordinal colors — the same 7
// macaron tokens used everywhere else (ui/tokens.ts MACARONS), walked in one
// fixed hue order so adjacent roots always read as clearly different families.
// (This replaces the old hand-rolled 6-hue palette that lived only here.)
const MACARON_ORDER: MacaronName[] = ["peach", "butter", "matcha", "lake", "mist", "taro", "berry"];

function macaronTheme(name: MacaronName): NodeTheme {
  const m = MACARONS[name];
  const [r, g, b] = parseHex(m.base);
  return { key: name, fillFrom: m.bg, fillTo: m.bg, border: m.base, label: m.fg, glow: `rgba(${r},${g},${b},0.22)` };
}

export const NODE_THEMES: NodeTheme[] = MACARON_ORDER.map(macaronTheme);

// Theme by ORDINAL (a root's index in the project's stable root ordering),
// modulo the palette length. Assigning by position — not an id hash — guarantees
// adjacent roots always land on different, visibly-spread hues (the id hash used
// before could collide neighbouring questions into the same family). Handles
// negatives defensively so a not-found ordinal still maps in-range.
export function themeForOrdinal(ordinal: number): NodeTheme {
  const n = NODE_THEMES.length;
  return NODE_THEMES[((ordinal % n) + n) % n]!;
}

// A root's ordinal = its index among the top-level (parentLeadId == null) leads,
// in their given array order — the same ordering WarrenMap lays roots out in, so
// a question keeps one consistent color across Level-1 and Level-2.
export function rootOrdinal(rootId: string, leads: ExplorationLead[]): number {
  const i = leads.filter((l) => l.parentLeadId == null).findIndex((r) => r.id === rootId);
  return i < 0 ? 0 : i;
}

// The theme for a root question, resolved from its ordinal within `leads`.
export function themeForRoot(rootId: string, leads: ExplorationLead[]): NodeTheme {
  return themeForOrdinal(rootOrdinal(rootId, leads));
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
  // "已读" badge: at least one reference "contained" by this root (its own
  // connectedReferenceId, or any live descendant lead's) is readingStatus
  // "done" — see anyReferenceDoneByRoot below.
  hasReadReference: boolean;
  position: { x: number; y: number };
};

// leads (roots) → node models. `saved` (dragged positions from localStorage,
// keyed by lead id) wins over the circle fallback; unknown ids get the circle
// slot. Everything else (theme, count) is derived deterministically.
export function buildWarrenNodes(
  roots: ExplorationLead[],
  paperCounts: Map<string, number>,
  saved?: Record<string, { x: number; y: number }>,
  readRootIds?: Set<string>,
): WarrenNode[] {
  const circle = circlePositions(roots.map((r) => r.id));
  return roots.map((r, i) => ({
    id: r.id,
    text: r.text,
    paperCount: paperCounts.get(r.id) ?? 0,
    // theme by ordinal (index) → adjacent roots always differ in hue
    theme: themeForOrdinal(i),
    status: r.status,
    hasReadReference: readRootIds?.has(r.id) ?? false,
    position: saved?.[r.id] ?? circle.get(r.id) ?? { x: 0, y: 0 },
  }));
}

// "已读" badge data: which roots have AT LEAST ONE reference "contained" by
// them — the root's own connectedReferenceId (a root can itself be born from
// an adopted paper, carrying connectedReferenceId directly), or any live
// descendant lead's connectedReferenceId at any depth — whose readingStatus
// is "done". ANY-done, not all-done: the affordance is meant as "you've read
// something here already", so it lights up as soon as one source under a
// question is finished rather than gating on every source in a branch.
// referenceStatus is keyed by reference id → its readingStatus string (kept
// as a plain string, not the narrower literal union, so callers can build it
// straight off a Reference[] without importing that contract type here).
export function anyReferenceDoneByRoot(
  leads: ExplorationLead[],
  referenceStatus: Map<string, string>,
): Set<string> {
  const childrenByParent = new Map<string, ExplorationLead[]>();
  for (const l of leads) {
    if (l.parentLeadId) {
      const arr = childrenByParent.get(l.parentLeadId) ?? [];
      arr.push(l);
      childrenByParent.set(l.parentLeadId, arr);
    }
  }
  const isDone = (refId: string | null): boolean => refId != null && referenceStatus.get(refId) === "done";
  const subtreeHasDone = (id: string): boolean => {
    for (const k of childrenByParent.get(id) ?? []) {
      if (isDone(k.connectedReferenceId) || subtreeHasDone(k.id)) return true;
    }
    return false;
  };
  const roots = leads.filter((l) => l.parentLeadId == null);
  const out = new Set<string>();
  for (const r of roots) {
    if (isDone(r.connectedReferenceId) || subtreeHasDone(r.id)) out.add(r.id);
  }
  return out;
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
    fillFrom: mixToward(base.fillFrom, NEUTRAL.surface, t),
    fillTo: mixToward(base.fillTo, NEUTRAL.surface, t),
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

// ---------------------------------------------------------------------------
// GVc · dagre mindmap layout — the tidy, well-organized replacement for the
// loose radial scatter. Lays a question's subtree out as a clean left-to-right
// tree (root on the left, children flowing right) with even spacing and NO node
// overlaps. Pure + deterministic (dagre is DOM-free), so it unit-tests without
// React Flow. QuestionMindmap.tsx renders bezier edges + fitView over these.
// ---------------------------------------------------------------------------

// Rendered node footprints (must match MindmapNodeView in QuestionMindmap.tsx)
// so dagre spaces nodes by their true size and nothing overlaps.
const MM_ROOT_W = 216;
const MM_ROOT_H = 108;
const MM_NODE_W = 176;
const MM_NODE_H = 88;
const MM_RANK_GAP = 96; // horizontal gap between depth levels
const MM_NODE_GAP = 30; // vertical gap between siblings

// Subtree depth-by-id via a cycle-guarded DFS from the root — the shared notion
// of "which leads belong to this question's map" (dagre + radial agree on it).
function subtreeDepths(rootId: string, childrenByParent: Map<string, ExplorationLead[]>): Map<string, number> {
  const depthOf = new Map<string, number>();
  const dfs = (id: string, depth: number) => {
    if (depthOf.has(id)) return;
    depthOf.set(id, depth);
    for (const k of childrenByParent.get(id) ?? []) dfs(k.id, depth + 1);
  };
  dfs(rootId, 0);
  return depthOf;
}

// Tidy left-to-right tree layout for one question's subtree. Runs dagre with
// rankdir "LR", then converts dagre's node CENTERS to React Flow top-left
// positions and translates everything so the ROOT sits at the origin (0,0).
// Deterministic: same input → same positions. Cycle-guarded upstream.
export function dagreMindmapLayout(rootId: string, childrenByParent: Map<string, ExplorationLead[]>): Map<string, PosD> {
  const depthOf = subtreeDepths(rootId, childrenByParent);

  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "LR", nodesep: MM_NODE_GAP, ranksep: MM_RANK_GAP, marginx: 0, marginy: 0 });
  g.setDefaultEdgeLabel(() => ({}));

  const sizeOf = (id: string) =>
    id === rootId ? { width: MM_ROOT_W, height: MM_ROOT_H } : { width: MM_NODE_W, height: MM_NODE_H };
  for (const id of depthOf.keys()) g.setNode(id, sizeOf(id));
  for (const id of depthOf.keys()) {
    for (const k of childrenByParent.get(id) ?? []) {
      if (depthOf.has(k.id)) g.setEdge(id, k.id);
    }
  }
  dagre.layout(g);

  // dagre reports node centers; React Flow positions are top-left. Convert, then
  // anchor the root's top-left at (0,0) so the layout is translation-stable.
  const topLeft = (id: string): { x: number; y: number } => {
    const n = g.node(id) as { x?: number; y?: number } | undefined;
    const s = sizeOf(id);
    return { x: (n?.x ?? 0) - s.width / 2, y: (n?.y ?? 0) - s.height / 2 };
  };
  const rootTL = topLeft(rootId);

  const out = new Map<string, PosD>();
  for (const [id, depth] of depthOf) {
    const p = topLeft(id);
    out.set(id, { x: Math.round(p.x - rootTL.x), y: Math.round(p.y - rootTL.y), depth });
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
// or a sub-question). Positions come from the tidy dagre left-to-right layout;
// `saved` (dragged positions) wins over the dagre slot for that node.
export function buildMindmapNodes(
  rootId: string,
  leads: ExplorationLead[],
  saved?: Record<string, { x: number; y: number }>,
): MindmapNode[] {
  const children = indexLiveChildren(leads);
  const pos = dagreMindmapLayout(rootId, children);
  const base = themeForRoot(rootId, leads);
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
  const ids = new Set(subtreeDepths(rootId, children).keys());
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
