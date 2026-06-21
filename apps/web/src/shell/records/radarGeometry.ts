export interface RadarDot { cx: number; cy: number; }
export interface RadarAxis { x1: number; y1: number; x2: number; y2: number; }
export interface RadarLabel { x: number; y: number; anchor: string; text: string; }
export interface RadarGeometry {
  rings: { points: string }[];
  axes: RadarAxis[];
  polygonPoints: string;
  dots: RadarDot[];
  labels: RadarLabel[];
}

export function radarGeometry(
  levels: number[],
  names: string[],
  cx = 140, cy = 128, r = 96,
): RadarGeometry {
  const n = levels.length;
  const angle = (i: number) => -Math.PI / 2 + (i * 2 * Math.PI) / n; // start at top
  const pt = (i: number, radius: number) => ({
    x: cx + radius * Math.cos(angle(i)),
    y: cy + radius * Math.sin(angle(i)),
  });

  const rings = [0.25, 0.5, 0.75, 1].map((f) => ({
    points: Array.from({ length: n }, (_, i) => {
      const p = pt(i, r * f); return `${p.x.toFixed(1)},${p.y.toFixed(1)}`;
    }).join(" "),
  }));

  const axes: RadarAxis[] = Array.from({ length: n }, (_, i) => {
    const p = pt(i, r); return { x1: cx, y1: cy, x2: +p.x.toFixed(1), y2: +p.y.toFixed(1) };
  });

  const dots: RadarDot[] = levels.map((lv, i) => {
    const p = pt(i, (r * lv) / 4); return { cx: +p.x.toFixed(1), cy: +p.y.toFixed(1) };
  });

  const polygonPoints = dots.map((d) => `${d.cx},${d.cy}`).join(" ");

  const labels: RadarLabel[] = names.map((text, i) => {
    const p = pt(i, r + 16);
    const anchor = Math.abs(p.x - cx) < 4 ? "middle" : p.x > cx ? "start" : "end";
    return { x: +p.x.toFixed(1), y: +p.y.toFixed(1), anchor, text };
  });

  return { rings, axes, polygonPoints, dots, labels };
}
