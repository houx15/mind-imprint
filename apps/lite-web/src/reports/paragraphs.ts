// reports/paragraphs.ts — pure paragraph splitting, no React import.
//
// Kept separate from ArticleView.tsx (a component file) so a pure module
// like writings/versionDiff.ts can import it without pulling React into a
// logic-only file. ArticleView renders these as real `<p>`s; versionDiff
// diffs them paragraph by paragraph.

/** Blank-line-separated paragraphs, as plain strings. `\r` is stripped because
 *  a draft can arrive from a Windows paste. */
export function splitParagraphs(piece: string): string[] {
  return piece
    .split(/\n\s*\n/)
    .map((p) => p.replace(/\r/g, "").trim())
    .filter((p) => p !== "");
}
