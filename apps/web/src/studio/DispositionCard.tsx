import { useState } from "react";

export type DispositionCardProps = {
  tag: string;
  anchor: string;
  body: string;
  onDisposition: (choice: "accept" | "revise" | "reject", reason: string) => void;
};

// Three-key disposition on an AI suggestion.
// Design: docs/design/思维印记_工作区.dc.html ~L1322-1341 (dispKeys / dispReason /
// dispReasonHint logic ~L2276-2284). The reason gate is ">=15" in the mock via
// `.trim().length` (UTF-16 units); we gate on RUNE count instead
// ([...reason].length) to match the backend's rune-based validation.
const DISPOSITION_KEYS: Array<{ choice: "accept" | "revise" | "reject"; label: string }> = [
  { choice: "accept", label: "接受" },
  { choice: "revise", label: "我自己改" },
  { choice: "reject", label: "不采纳" },
];

function runeCount(text: string): number {
  return [...text.trim()].length;
}

export function DispositionCard({ tag, anchor, body, onDisposition }: DispositionCardProps) {
  const [choice, setChoice] = useState<"accept" | "revise" | "reject" | null>(null);
  const [reason, setReason] = useState("");

  const rlen = runeCount(reason);
  const reasonOk = rlen >= 15;
  const reasonColor = rlen === 0 ? "#AEB4C2" : reasonOk ? "#4C9A82" : "#D9A23D";
  const reasonHint =
    rlen === 0
      ? "留一句理由才算数（≥15 字）"
      : reasonOk
        ? `✓ 已记录 · ${rlen} 字，进入你的成长记录`
        : `再写一点 · ${rlen}/15 字`;

  const canSubmit = choice !== null && reasonOk;

  function submit() {
    if (!canSubmit || choice === null) return;
    onDisposition(choice, reason);
  }

  return (
    <div
      style={{
        border: "1px solid #E7EAF1",
        borderRadius: 14,
        overflow: "hidden",
        boxShadow: "0 3px 14px rgba(20,30,60,.05)",
        fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif",
      }}
    >
      <div style={{ padding: "12px 15px 10px", borderBottom: "1px solid #F2F3F7" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 6 }}>
          <span
            style={{
              fontSize: 10.5,
              fontWeight: 700,
              color: "#fff",
              background: "#2A3B7A",
              padding: "2px 9px",
              borderRadius: 999,
            }}
          >
            {tag}
          </span>
          <span style={{ fontSize: 10.5, color: "#AEB4C2", fontWeight: 600 }}>锚定 {anchor}</span>
        </div>
        <div style={{ fontSize: 13.5, lineHeight: 1.7, color: "#2B3346" }}>{body}</div>
      </div>
      <div style={{ padding: "11px 15px 14px", background: "#FAFBFD" }}>
        <div style={{ fontSize: 11, fontWeight: 700, color: "#8A92A3", marginBottom: 8 }}>
          三键处置 · 怎么选都要留一句理由
        </div>
        <div style={{ display: "flex", gap: 7, marginBottom: 10 }}>
          {DISPOSITION_KEYS.map((k) => {
            const picked = choice === k.choice;
            return (
              <div
                key={k.choice}
                onClick={() => setChoice(k.choice)}
                role="button"
                style={{
                  flex: 1,
                  padding: "8px 6px",
                  borderRadius: 9,
                  fontSize: 12.5,
                  fontWeight: 700,
                  cursor: "pointer",
                  textAlign: "center",
                  background: picked ? "#2A3B7A" : "#fff",
                  color: picked ? "#fff" : "#6B7384",
                  border: `1px solid ${picked ? "#2A3B7A" : "#E1E4ED"}`,
                }}
              >
                {k.label}
              </div>
            );
          })}
        </div>
        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={2}
          placeholder="写下你的理由（至少 15 字）——这句话本身会进你的成长记录"
          style={{
            width: "100%",
            border: "1px solid #E1E4ED",
            borderRadius: 10,
            padding: "9px 11px",
            fontSize: 12.5,
            lineHeight: 1.6,
            color: "#1C2333",
            background: "#fff",
            outline: "none",
            resize: "vertical",
            fontFamily: "inherit",
          }}
        />
        <div style={{ fontSize: 10.5, fontWeight: 600, marginTop: 6, color: reasonColor }}>{reasonHint}</div>
        <button
          type="button"
          onClick={submit}
          disabled={!canSubmit}
          style={{
            marginTop: 10,
            width: "100%",
            padding: "9px 12px",
            borderRadius: 10,
            border: "none",
            fontSize: 12.5,
            fontWeight: 700,
            fontFamily: "inherit",
            cursor: canSubmit ? "pointer" : "not-allowed",
            background: canSubmit ? "#2A3B7A" : "#E1E4ED",
            color: canSubmit ? "#fff" : "#AEB4C2",
          }}
        >
          提交
        </button>
      </div>
    </div>
  );
}
