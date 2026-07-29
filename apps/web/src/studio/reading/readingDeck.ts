import { CARD_REGISTRY } from "@mind-imprint/contracts";

// readingDeck.ts — the lens-library catalog: the fixed set of reading-room
// cards the student may browse and summon herself, mirroring
// apps/api/internal/agent/reading_deck.go's ReadingDeckIDs (single source of
// truth lives server-side; this list is kept in lockstep by hand, same as
// every other cross-runtime card id list in this codebase). Source-check
// (craap, sift) + the 9 disciplinary reading lenses (学科透镜). The deep-reading
// slots used to be the WRITING tool cards, which couldn't ground a single
// illustrative sentence — see docs/2026-07-29-reading-lenses-adopt-demo.md.
// Growing the deck later = adding an id here AND to ReadingDeckIDs.
export const READING_DECK_IDS = [
  "craap", "sift",
  "lens-logic", "lens-methods",
  "lens-society", "lens-law", "lens-economics", "lens-ethics",
  "lens-history", "lens-communication", "lens-systems",
] as const;

// Source-check cards get their own group in the library — they gate the
// deep-reading cards (SIFT needs a completed CRAAP first, server-enforced)
// and read as a distinct "first pass" step to the student.
const SOURCE_CHECK_IDS = new Set<string>(["craap", "sift"]);

export type LensFamily = "reasoning" | "institutions" | "context";

export type ReadingDeckEntry = {
  id: string;
  name: string;
  purpose: string;
  category: string;
  family?: LensFamily;
};

// readingDeck() resolves READING_DECK_IDS against CARD_REGISTRY into the
// display shape LensLibrary renders. Skips any id missing from the registry
// defensively (never throws) — the library should degrade gracefully rather
// than blank the whole page over one drifted id.
export function readingDeck(): ReadingDeckEntry[] {
  const out: ReadingDeckEntry[] = [];
  for (const id of READING_DECK_IDS) {
    const spec = CARD_REGISTRY[id];
    if (!spec) continue;
    out.push({ id, name: spec.name, purpose: spec.purpose, category: spec.category, family: spec.reading_lens?.family });
  }
  return out;
}

// The three lens families, in display order, with their human group labels
// (mirrors the demo's LENS_FAMILIES). A lens with no family (shouldn't happen
// for a lens-* card) falls into "reasoning" so it is never dropped silently.
export const LENS_FAMILY_ORDER: { key: LensFamily; label: string }[] = [
  { key: "reasoning", label: "推理与证据" },
  { key: "institutions", label: "人与制度" },
  { key: "context", label: "语境与系统" },
];

export type LensFamilyGroup = { key: LensFamily; label: string; entries: ReadingDeckEntry[] };

export type ReadingDeckGroups = {
  sourceCheck: ReadingDeckEntry[];
  families: LensFamilyGroup[];
};

// groupReadingDeck splits the deck into the library's display groups: 信源体检
// (source-check) + the three lens families (推理与证据 / 人与制度 / 语境与系统).
// Empty family groups are dropped so the library never renders a bare heading.
export function groupReadingDeck(deck: ReadingDeckEntry[] = readingDeck()): ReadingDeckGroups {
  const sourceCheck: ReadingDeckEntry[] = [];
  const byFamily = new Map<LensFamily, ReadingDeckEntry[]>();
  for (const entry of deck) {
    if (SOURCE_CHECK_IDS.has(entry.id)) {
      sourceCheck.push(entry);
      continue;
    }
    const fam: LensFamily = entry.family ?? "reasoning";
    const list = byFamily.get(fam) ?? [];
    list.push(entry);
    byFamily.set(fam, list);
  }
  const families = LENS_FAMILY_ORDER.map(({ key, label }) => ({ key, label, entries: byFamily.get(key) ?? [] })).filter(
    (g) => g.entries.length > 0,
  );
  return { sourceCheck, families };
}
