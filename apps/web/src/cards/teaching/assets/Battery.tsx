/**
 * Battery — energy meter cells.
 * Ported from inner-parts-cast.html .cells/.cell pattern.
 * Pure presentational — no local state, no envelope writes.
 *
 * Each cell carries data-cell="on"|"off" for test querying.
 * Cells at level ≤ 2 use rose #C2557A, else warm #C9743C.
 */

const ROSE = "#C2557A";
const WARM = "#C9743C";
const OFF_BG = "#F3F4F8";
const OFF_BORDER = "#EAECF2";

interface BatteryProps {
  level: number;
  max?: number;
  size?: number;
}

export function Battery({ level, max = 5, size }: BatteryProps) {
  // cell dimensions: width proportional to size or default
  const cellW = size ? Math.round(size * 0.3) : 18;
  const cellH = size ? Math.round(size * 0.5) : 30;
  const gap = 4;
  const isLow = level <= 2;
  const fillColor = isLow ? ROSE : WARM;

  return (
    <div
      style={{
        display: "inline-flex",
        gap: `${gap}px`,
        alignItems: "center",
      }}
    >
      {Array.from({ length: max }, (_, i) => {
        const on = i < level;
        return (
          <span
            key={i}
            data-cell={on ? "on" : "off"}
            style={{
              display: "inline-block",
              width: `${cellW}px`,
              height: `${cellH}px`,
              borderRadius: "4px",
              background: on ? fillColor : OFF_BG,
              border: `1px solid ${on ? fillColor : OFF_BORDER}`,
            }}
          />
        );
      })}
    </div>
  );
}
