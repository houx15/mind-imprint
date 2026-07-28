import { CARD_REGISTRY } from "@mind-imprint/contracts";

// readingDeck.ts — the lens-library catalog: the fixed set of reading-room
// cards the student may browse and summon herself, mirroring
// apps/api/internal/agent/reading_deck.go's ReadingDeckIDs (single source of
// truth lives server-side; this list is kept in lockstep by hand, same as
// every other cross-runtime card id list in this codebase). Source-check
// (craap, sift) + deep reading. Growing the deck later = adding an id here
// AND to ReadingDeckIDs; no page or mechanic change either side.
export const READING_DECK_IDS = [
  "craap", "sift",
  "fact-opinion-value", "argument-map", "toulmin", "steelman", "concession",
  "data-literacy", "opcvl", "framing", "spin-detector", "cda",
  "perspective-matrix", "certainty-spectrum", "science-knowing",
] as const;

// Source-check cards get their own group in the library — they gate the
// deep-reading cards (SIFT needs a completed CRAAP first, server-enforced)
// and read as a distinct "first pass" step to the student.
const SOURCE_CHECK_IDS = new Set<string>(["craap", "sift"]);

export type ReadingDeckEntry = {
  id: string;
  name: string;
  purpose: string;
  category: string;
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
    out.push({ id, name: spec.name, purpose: spec.purpose, category: spec.category });
  }
  return out;
}

export type ReadingDeckGroups = {
  sourceCheck: ReadingDeckEntry[];
  deepReading: ReadingDeckEntry[];
};

// groupReadingDeck splits the deck into the two display groups LensLibrary
// renders: 信源体检 (source-check) and 深读 (deep reading).
export function groupReadingDeck(deck: ReadingDeckEntry[] = readingDeck()): ReadingDeckGroups {
  const sourceCheck: ReadingDeckEntry[] = [];
  const deepReading: ReadingDeckEntry[] = [];
  for (const entry of deck) {
    (SOURCE_CHECK_IDS.has(entry.id) ? sourceCheck : deepReading).push(entry);
  }
  return { sourceCheck, deepReading };
}
