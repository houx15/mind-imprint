/**
 * Radar — 5-axis pentagon chart for CRAAP dimensions.
 * Polygon vertices computed from values (each 0..5).
 * Ported from sift-interactions.html CRAAP pentagon design.
 * Pure presentational — no local state, no envelope writes.
 */

interface RadarProps {
  values: number[];
  labels: string[];
}

const AXIS_COUNT = 5;
const CENTER_X = 60;
const CENTER_Y = 62;
const MAX_RADIUS = 44;
const LABEL_RADIUS = 56;
const MAX_VALUE = 5;

/** Compute (x, y) for a given axis index, distance from center */
function toXY(index: number, radius: number, cx = CENTER_X, cy = CENTER_Y): [number, number] {
  // Start from top (−π/2) and go clockwise
  const angle = (Math.PI * 2 * index) / AXIS_COUNT - Math.PI / 2;
  return [cx + radius * Math.cos(angle), cy + radius * Math.sin(angle)];
}

function pointsStr(pts: [number, number][]) {
  return pts.map(([x, y]) => `${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
}

export function Radar({ values, labels }: RadarProps) {
  const axes = Math.min(AXIS_COUNT, values.length, labels.length);

  // Outer reference pentagon (at MAX_RADIUS)
  const outerPts = Array.from({ length: axes }, (_, i) => toXY(i, MAX_RADIUS));

  // Filled data polygon
  const dataPts = Array.from({ length: axes }, (_, i) => {
    const clamped = Math.max(0, Math.min(MAX_VALUE, values[i] ?? 0));
    return toXY(i, (clamped / MAX_VALUE) * MAX_RADIUS);
  });

  // Label positions
  const labelPts = Array.from({ length: axes }, (_, i) => toXY(i, LABEL_RADIUS));

  const svgW = 120;
  const svgH = 124;

  return (
    <svg width={svgW} height={svgH} viewBox={`0 0 ${svgW} ${svgH}`} fill="none" xmlns="http://www.w3.org/2000/svg">
      {/* Reference pentagon */}
      <polygon
        points={pointsStr(outerPts)}
        fill="none"
        stroke="#C7D0E6"
        strokeWidth="1.4"
      />
      {/* Axis lines */}
      {outerPts.map(([x, y], i) => (
        <line
          key={i}
          x1={CENTER_X}
          y1={CENTER_Y}
          x2={x}
          y2={y}
          stroke="#EAECF2"
          strokeWidth="1"
        />
      ))}
      {/* Filled data polygon */}
      <polygon
        points={pointsStr(dataPts)}
        fill="#C7D6FF"
        stroke="#2A3B7A"
        strokeWidth="2"
        opacity="0.85"
      />
      {/* Axis dots */}
      {outerPts.map(([x, y], i) => (
        <circle key={i} cx={x} cy={y} r="2.2" fill="#2A3B7A" />
      ))}
      {/* Labels */}
      {labelPts.map(([x, y], i) => (
        <text
          key={i}
          x={x}
          y={y}
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize="8"
          fill="#6B7384"
        >
          {labels[i]}
        </text>
      ))}
    </svg>
  );
}
