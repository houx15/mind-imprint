import type { CardSpec } from "@mind-imprint/contracts";

// Turn a completed card's raw field_values into a readable text block a student
// can keep — the artifact a tool card leaves behind once its sheet closes (#7).
// Before this, a finished card stored its values only into the read-only process
// tree and vanished from the surface; now the writing room compiles it into a
// 片段 (snippet) the student can see, edit, and pull into the draft.
//
// It's a plain projection, not authorship (铁律①): it only reformats what the
// STUDENT typed into the card — it never adds sentences of its own.

function valueToText(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v.trim();
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  if (Array.isArray(v)) return v.map(valueToText).filter(Boolean).join("、");
  if (typeof v === "object") {
    // repeatable_group items / {text, source}-shaped custom-renderer values:
    // flatten one level so a nested answer still reads.
    return Object.values(v as Record<string, unknown>).map(valueToText).filter(Boolean).join(" · ");
  }
  return "";
}

// compileCardEnvelope walks the card's declared fields (in field order) and
// joins the student's own answers into ONE flowing paragraph — a 片段 that
// reads like her own writing, not a "**label**\ntext" labeled dump (item C /
// 铁律①: never authorship, only a projection of what she typed). The card's
// name is kept only as a subtle provenance prefix line, never woven into the
// paragraph itself. Custom-renderer cards whose field_values keys don't line
// up with declared field keys still contribute (so no answer is silently
// dropped). Returns "" when the student filled nothing.
export function compileCardEnvelope(spec: CardSpec, fieldValues: Record<string, unknown>): string {
  const parts: string[] = [];
  const seen = new Set<string>();
  for (const step of spec.steps) {
    for (const field of step.fields) {
      const text = valueToText(fieldValues[field.key]);
      if (!text) continue;
      seen.add(field.key);
      parts.push(text);
    }
  }
  // Fallback: surface any non-empty values a custom renderer stored under keys
  // the declarative walk didn't cover.
  for (const [k, v] of Object.entries(fieldValues)) {
    if (seen.has(k)) continue;
    const text = valueToText(v);
    if (text) parts.push(text);
  }
  if (parts.length === 0) return "";
  // Each answer is already a sentence/clause in her own words; just make sure
  // it ends with terminal punctuation before running the next one on, so
  // consecutive answers don't fuse mid-sentence. No reordering, no relabeling.
  const prose = parts.map((p) => (/[。！？.!?…”】)）]$/.test(p) ? p : `${p}。`)).join("");
  return `【${spec.name}】\n${prose}`;
}

// compileCardForCoach compiles a completed card into a short student-turn text
// the coach responds to — the SAME algorithm as the Go CompileCardForCoach
// (apps/api/internal/agent/compile_card.go), so the locally-shown student turn
// and the server-persisted one read identically on reload. Emits
// `我刚填完《<name>》：` then one `- <label>：<value>` line per non-empty field,
// with a generic key-dump fallback for custom-renderer keys. Returns "" when the
// student filled nothing (an empty card is a no-op). Still a projection, not
// authorship (铁律①): only the student's own words are reformatted.
export function compileCardForCoach(spec: CardSpec, fieldValues: Record<string, unknown>): string {
  const lines: string[] = [];
  const seen = new Set<string>();
  for (const step of spec.steps) {
    for (const field of step.fields) {
      const text = valueToText(fieldValues[field.key]);
      if (!text) continue;
      seen.add(field.key);
      lines.push(`- ${field.label}：${text}`);
    }
  }
  for (const k of Object.keys(fieldValues).sort()) {
    if (seen.has(k)) continue;
    const text = valueToText(fieldValues[k]);
    if (text) lines.push(`- ${k}：${text}`);
  }
  if (lines.length === 0) return "";
  return `我刚填完《${spec.name}》：\n${lines.join("\n")}`;
}
