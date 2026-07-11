type BeanProps = { color?: string; size?: number };

// Ported from the Claude Design Bean.dc.html: a rounded "bean" body with two
// blinking eyes; eye color contrasts with the fill (light eyes on dark beans).
export function Bean({ color = "#2A3B7A", size = 38 }: BeanProps) {
  const m = /^#?([0-9a-f]{6})$/i.exec(color.trim());
  let eye = "#17223B";
  if (m) {
    const n = parseInt(m[1]!, 16);
    const r = (n >> 16) & 255, g = (n >> 8) & 255, b = n & 255;
    const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
    eye = lum < 0.55 ? "#FFFFFF" : "#17223B";
  }
  return (
    <svg viewBox="0 0 120 120" width={size} height={size} style={{ display: "block", overflow: "visible" }} aria-hidden="true">
      <g style={{ transformOrigin: "60px 60px" }}>
        <path
          d="M59.5 24.8 C75.7 23.7 93.3 31.9 98.9 47.2 C105.1 64.1 96.2 82.8 80.5 91.2 C64.6 99.7 42.4 96.4 30.2 83.4 C18.5 71.0 18.8 51.2 30.9 38.7 C37.8 31.6 48.1 25.6 59.5 24.8Z"
          fill={color}
        />
        <ellipse cx="49.4" cy="58.2" rx="2.6" ry="3.9" fill={eye} />
        <ellipse cx="68.3" cy="58.2" rx="2.6" ry="3.9" fill={eye} />
      </g>
    </svg>
  );
}
