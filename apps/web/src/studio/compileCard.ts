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

// compileCardEnvelope walks the card's declared fields (label + the student's
// value) into a titled markdown block. Custom-renderer cards whose field_values
// keys don't line up with declared field keys fall back to a generic dump so no
// answer is silently dropped. Returns "" when the student filled nothing.
export function compileCardEnvelope(spec: CardSpec, fieldValues: Record<string, unknown>): string {
  const lines: string[] = [];
  const seen = new Set<string>();
  for (const step of spec.steps) {
    for (const field of step.fields) {
      const text = valueToText(fieldValues[field.key]);
      if (!text) continue;
      seen.add(field.key);
      lines.push(`**${field.label}**\n${text}`);
    }
  }
  // Fallback: surface any non-empty values a custom renderer stored under keys
  // the declarative walk didn't cover.
  for (const [k, v] of Object.entries(fieldValues)) {
    if (seen.has(k)) continue;
    const text = valueToText(v);
    if (text) lines.push(`**${k}**\n${text}`);
  }
  if (lines.length === 0) return "";
  return `【${spec.name}】\n\n${lines.join("\n\n")}`;
}
