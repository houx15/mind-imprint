// Mirrors dc.html's levelColor/tint helpers verbatim (docs/design/teacher end/project/思维印记 教师端.dc.html:1513),
// including its FIXED test order: L1/0级/1级 -> red, then L2/2级 -> orange, then L4/4级/5级 -> green,
// then L3/3级 -> blue, else grey. A decimal A-axis value ("4.2") is first rounded and prefixed to "L4"
// (mirrors dc.html's `levelColor('L'+Math.round(parseFloat(a)))`) before running through the same chain.
//
// Shared by ClassDetailView (roster D/A pill chips) and StudentDetailView (header D/A badge cards) —
// both need the same fg/bg pair for a badge's raw text, so the logic lives here once.
export function badgeColor(rawText: string): { fg: string; bg: string } {
  let text = rawText;
  if (!/L\d/.test(text)) {
    const dMatch = text.match(/^(\d+(?:\.\d+)?)/);
    if (dMatch) text = `L${Math.round(Number(dMatch[1]))}`;
  }
  if (/L1|(?:^|[^\d])0级|(?:^|[^\d])1级/.test(text)) return { fg: "#C4574D", bg: "#F7E6E4" };
  if (/L2|2级/.test(text)) return { fg: "#C68A3A", bg: "#F6EED9" };
  if (/L4|4级|5级/.test(text)) return { fg: "#3E8A6E", bg: "#E4F0EA" };
  if (/L3|3级/.test(text)) return { fg: "#3E7CA8", bg: "#E1EDF5" };
  return { fg: "#8A92A3", bg: "#EEF0F4" };
}
