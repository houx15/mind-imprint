const rgb = (hex: string) => /^#[0-9a-f]{6}$/i.test(hex) ? [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16)) : null;
const luminance = (channels: number[]) => channels.reduce((sum, value, i) => {
  const n = value / 255;
  return sum + (n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4) * [.2126, .7152, .0722][i]!;
}, 0);
export function contrastRatio(a: number[], b: number[]): number {
  const x = luminance(a), y = luminance(b);
  return (Math.max(x, y) + .05) / (Math.min(x, y) + .05);
}
// Text uses its own color floor. Decorative backgrounds and hairlines keep their original mix.
export function readableText(ink: string, paper: string): { ink: string; minimum: number } {
  const background = rgb(paper), foreground = rgb(ink);
  if (!background || !foreground) return { ink, minimum: 100 };
  let color = foreground;
  if (contrastRatio(color, background) < 4.5) {
    color = contrastRatio([0, 0, 0], background) >= contrastRatio([255, 255, 255], background) ? [0, 0, 0] : [255, 255, 255];
    ink = color[0] === 0 ? "#000000" : "#FFFFFF";
  }
  for (let minimum = 0; minimum <= 100; minimum++) {
    const blended = color.map((c, i) => c * minimum / 100 + background[i]! * (1 - minimum / 100));
    if (contrastRatio(blended, background) >= 4.5) return { ink, minimum };
  }
  return { ink, minimum: 100 };
}
