import type { SubQuestion } from "@mind-imprint/contracts";

// sectionLabels — one place that turns a guided-writing snippet's MACHINE section
// key into the human part name the student recognises (the relevant outline /
// part name). Guided cards store each written part as a snippet keyed by an
// internal section string — `prop:thesis`, `prop:subq:<uuid>`, `claim:<uuid>`,
// `sub:intro`, `synthesis`… These keys must NEVER reach the surface: a folded
// finished card, the 片段 board, the 归到 dropdown all show the friendly name.
//
// `guidedSectionLabel` returns the friendly label for a guided section, or null
// when the section is a genuine student-made snippet label (a stale outline
// heading / 线索) — those keep their own text as-is.

// Fixed (non-parameterised) guided keys → their part name. Proposal parts are
// prefixed `prop:`; essay statement uses bare keys; submission uses `sub:`.
const FIXED_LABELS: Record<string, string> = {
  // proposal guide parts (prop:<key>)
  "prop:understanding": "对题目的理解",
  "prop:question-scope": "研究问题与范围",
  "prop:thesis": "暂定论点",
  "prop:research-plan": "研究计划 · 定子问题",
  "prop:resources": "资源",
  "prop:challenges": "可能的挑战",
  "prop:method": "研究方法",
  "prop:feasibility": "可行性 / 限制 / 伦理",
  "prop:expected": "预期结果",
  "prop:polish": "通读与润色",
  // essay statement steps (bare keys)
  outline: "大纲",
  synthesis: "比较 / 综合",
  conclusion: "结论",
  structure: "论证结构",
  challenges: "面对反方观点",
  // essay submission steps (sub:<key>)
  "sub:intro": "引言",
  "sub:conclusion": "结论",
  "sub:compose": "成文",
  "sub:polish": "润色定稿",
};

// A sub-question / claim card shows the student's own sub-question text after the
// ordinal so the header reads e.g. 「论点 2：Does the canopy–mortality…」. The text
// is trimmed so a long question doesn't blow out the fold header.
function withSubq(prefix: string, id: string, subQuestions: SubQuestion[]): string {
  const i = subQuestions.findIndex((s) => s.id === id);
  const text = i >= 0 ? subQuestions[i]!.text.trim() : "";
  const ord = i >= 0 ? ` ${i + 1}` : "";
  const tail = text ? `：${clip(text)}` : "";
  return `${prefix}${ord}${tail}`;
}

function clip(s: string, max = 42): string {
  const t = s.replace(/\s+/g, " ").trim();
  return t.length > max ? `${t.slice(0, max)}…` : t;
}

// guidedSectionLabel — the friendly part name for a guided section, or null when
// `section` isn't a guided-writing key (a real student snippet label).
export function guidedSectionLabel(
  section: string | null | undefined,
  subQuestions: SubQuestion[] = [],
): string | null {
  if (!section) return null;
  if (FIXED_LABELS[section]) return FIXED_LABELS[section];
  // parameterised: the claim (essay) and the proposal sub-question card both key
  // off the SAME sub-question id (claim:<id> ⇄ prop:subq:<id>).
  if (section.startsWith("claim:")) return withSubq("论点", section.slice("claim:".length), subQuestions);
  if (section.startsWith("prop:subq:")) return withSubq("子问题", section.slice("prop:subq:".length), subQuestions);
  return null;
}

// isGuidedSection — true when a snippet section is a guided-writing part (any of
// the keys guidedSectionLabel names), i.e. NOT a free student snippet label.
export function isGuidedSection(section: string | null | undefined): boolean {
  return guidedSectionLabel(section, []) !== null;
}
