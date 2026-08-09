import type { StepRef } from "@mind-imprint/contracts";

// docSections — slice 4b · assemble the guided per-part text back into one
// document (the buffer stays the single source for export/finish; the guided
// cards write per-part snippets, which this joins in step order). Each written
// part becomes a "## <title>\n<text>" section; empty parts are skipped.

export const PART_SECTION_PREFIX = "prop:";

export function partSectionKey(stepKey: string): string {
  return PART_SECTION_PREFIX + stepKey;
}

export function assembleGuidedDoc(steps: StepRef[], textByKey: Record<string, string>): string {
  const blocks: string[] = [];
  for (const s of steps) {
    const text = (textByKey[s.key] ?? "").trim();
    if (text === "") continue;
    blocks.push(`## ${s.title}\n${text}`);
  }
  return blocks.join("\n\n");
}
