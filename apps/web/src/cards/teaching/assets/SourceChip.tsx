/**
 * SourceChip — inline source credibility pill.
 * Ported from sift-interactions.html .src chip design.
 * Pure presentational — no local state, no envelope writes.
 */

type Verdict = "ok" | "q" | "bad";

interface SourceChipProps {
  name: string;
  verdict: Verdict;
}

const VERDICT_STYLES: Record<Verdict, { color: string; background: string; icon: string }> = {
  ok: { color: "#4C9A82", background: "#E7F3EE", icon: "✓" },
  q: { color: "#C9743C", background: "#FBEBDD", icon: "?" },
  bad: { color: "#C2557A", background: "#FBE7EF", icon: "✕" },
};

export function SourceChip({ name, verdict }: SourceChipProps) {
  const style = VERDICT_STYLES[verdict];
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "5px",
        fontSize: "10px",
        fontWeight: 600,
        padding: "4px 9px",
        borderRadius: "999px",
        color: style.color,
        background: style.background,
        border: "1px solid transparent",
        lineHeight: 1.4,
      }}
    >
      {name}
      <span aria-hidden="true">{style.icon}</span>
    </span>
  );
}
