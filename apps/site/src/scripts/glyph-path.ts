/**
 * The four figures again, as a static SVG path.
 *
 * The canvas版 grows them out of falling water and has to be animated. These
 * are the same masks standing still — used wherever one of the four needs to
 * appear as a mark rather than as weather: beside its ability, on the rail
 * next to the dial, in the closing row.
 *
 * Rows are run-length encoded into rectangles, so a figure is forty-odd
 * subpaths rather than two hundred <rect> elements.
 */
import { MASKS, GRID_COLS, GRID_ROWS, type GlyphKey } from "./glyph-masks";

/** A mono cell is this much wider than it is tall — the masks assume it. */
export const CELL_ASPECT = 0.6;

export const GLYPH_VIEWBOX = `0 0 ${(GRID_COLS * CELL_ASPECT).toFixed(2)} ${GRID_ROWS}`;

export function glyphPath(key: GlyphKey): string {
  const rows = MASKS[key];
  const out: string[] = [];
  for (let y = 0; y < rows.length; y++) {
    const row = rows[y];
    let x = 0;
    while (x < row.length) {
      if (row[x] !== "#") {
        x++;
        continue;
      }
      let n = 0;
      while (x + n < row.length && row[x + n] === "#") n++;
      const px = (x * CELL_ASPECT).toFixed(2);
      const w = (n * CELL_ASPECT).toFixed(2);
      out.push(`M${px} ${y}h${w}v1h-${w}z`);
      x += n;
    }
  }
  return out.join("");
}

export type { GlyphKey };
