import { useRef } from "react";

/**
 * ProseSurface — the writing page.
 *
 * A textarea, not a contentEditable. A textarea cannot space paragraphs
 * differently from lines; contentEditable can, and is also the first step
 * toward the document editor 铁律 rules out. At 1.9 leading a blank line
 * already yields a ~32px gap, which is how iA Writer and Bear look, so the
 * limitation is not one she will perceive.
 *
 * Because it IS a textarea, a comment cannot highlight inside it. The mirrored
 * layer behind it renders the same text with the same metrics and marks the
 * quoted span. 🔑 Both read PROSE_TYPOGRAPHY — define the metrics twice and
 * they drift, and the highlight lands on the wrong line.
 */
export const PROSE_TYPOGRAPHY = Object.freeze({
  fontSize: "17px",
  lineHeight: "1.9",
  letterSpacing: "0",
  padding: "48px 32px 40vh",
});

/**
 * Splits `text` on the first occurrence of `highlight` (a literal substring,
 * never a regex/markup match) into [before, match, after]. Returns null when
 * there is no highlight, or it isn't found — callers render the plain text.
 */
function splitOnHighlight(
  text: string,
  highlight: string | null | undefined,
): [string, string, string] | null {
  if (!highlight) return null;
  const idx = text.indexOf(highlight);
  if (idx === -1) return null;
  return [text.slice(0, idx), highlight, text.slice(idx + highlight.length)];
}

export function ProseSurface({
  value,
  onChange,
  highlight,
  placeholder,
}: {
  value: string;
  onChange: (next: string) => void;
  /** A literal substring of `value` to mark in the layer behind the textarea. */
  highlight?: string | null;
  placeholder?: string;
}) {
  const layerRef = useRef<HTMLDivElement>(null);

  // Keep the mirrored layer scrolling with the textarea, or the highlight
  // detaches the moment the draft grows past one screen.
  function syncScroll(e: React.UIEvent<HTMLTextAreaElement>) {
    const layer = layerRef.current;
    if (!layer) return;
    layer.scrollTop = e.currentTarget.scrollTop;
    layer.scrollLeft = e.currentTarget.scrollLeft;
  }

  const parts = splitOnHighlight(value, highlight);

  return (
    <div className="relative max-w-[68ch] mx-auto bg-mk-paper">
      <div
        ref={layerRef}
        data-prose-layer
        aria-hidden="true"
        className="absolute inset-0 whitespace-pre-wrap break-words pointer-events-none overflow-hidden text-mk-ink"
        style={PROSE_TYPOGRAPHY}
      >
        {parts ? (
          <>
            {parts[0]}
            <mark
              style={{
                background: "color-mix(in srgb, var(--mk-accent-500) 25%, transparent)",
                color: "inherit",
              }}
            >
              {parts[1]}
            </mark>
            {parts[2]}
          </>
        ) : (
          value
        )}
      </div>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onScroll={syncScroll}
        placeholder={placeholder}
        spellCheck={false}
        className="relative w-full min-h-[60vh] bg-transparent resize-none border-none outline-none focus:outline-none focus:ring-0 caret-mk-accent-500 selection:bg-mk-accent-500/30 text-transparent"
        style={{ ...PROSE_TYPOGRAPHY, caretColor: "var(--mk-accent-500)" }}
      />
    </div>
  );
}
