import { useState } from "react";

// N3f: the shared work-order row rendered by both the 整稿体检 (S5 writing
// review, which has a band) and the station spot-checks (S3 信源体检 / S4
// 论证体检, which have no band by design — see SpotCheckItem's own comment:
// a credibility verdict on the source dossier "has no honest producer and
// would have to be fabricated"). `band` is OPTIONAL and, when absent, no
// chip is rendered at all — never a chip with an empty string.
export type WorkOrderRow = {
  interventionId: string;
  label: string; // criterion (review) | targetName (spot-check)
  band?: string; // review only — OPTIONAL, never blank-stringed
  evidence: string;
  missing: string;
  fix: string;
  disposition: { action: "accept" | "rewrite" | "reject"; reason: string } | null;
};

// Design (docs/design/思维印记_工作区.dc.html:2259): WBT maps each review
// item's model-produced band string to a tone. These three band strings are
// the ONLY ones the skill's posture prompt is ever asked to choose among
// (apps/api/internal/agent/review.go's tests fix them at "3–4 段"/"5–6
// 段"/"7–8 段") — an unrecognized value (should never happen) falls back to a
// neutral chip rather than guessing a tone.
const BAND_TONE: Record<string, [color: string, background: string]> = {
  "3–4 段": ["#C96F4F", "#FBEEE7"],
  "5–6 段": ["#B8892F", "#FBF4E2"],
  "7–8 段": ["#4C9A82", "#E7F3EE"],
};

function bandChipStyle(band: string): React.CSSProperties {
  const [color, background] = BAND_TONE[band] ?? ["#6B7384", "#EEF0F5"];
  return { fontSize: 11, fontWeight: 700, color, background, padding: "2px 9px", borderRadius: 999 };
}

// Design (docs/design/思维印记_工作区.dc.html:2260): KEY_LABELS =
// [['keep','保持原样'],['revise','我来改'],['explain','说明为什么不改']] — the
// design's own local key ids ('keep'/'revise'/'explain') aren't the wire
// contract's disposition enum, so this maps the verbatim Chinese labels
// straight to the stored action (per the task brief: 保持原样→accept,
// 我来改→rewrite, 说明为什么不改→reject).
const REVIEW_KEYS: Array<{ label: string; action: "accept" | "rewrite" | "reject" }> = [
  { label: "保持原样", action: "accept" },
  { label: "我来改", action: "rewrite" },
  { label: "说明为什么不改", action: "reject" },
];

// Mirrors DispositionCard's own rune-based gate (backend validates on runes,
// not UTF-16 code units) — kept local since this is the only other place a
// work-order row's disposition reason is composed.
function runeCount(text: string): number {
  return [...text.trim()].length;
}

export function WorkOrderItem({
  row,
  onDisposition,
}: {
  row: WorkOrderRow;
  onDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
}) {
  const [choice, setChoice] = useState<"accept" | "rewrite" | "reject" | null>(row.disposition?.action ?? null);
  const [reason, setReason] = useState(row.disposition?.reason ?? "");
  const hasFix = row.fix.trim().length > 0;
  const rlen = runeCount(reason);
  const reasonOk = rlen >= 15;
  const canSubmit = choice !== null && reasonOk;

  function submit() {
    if (!canSubmit || choice === null) return;
    onDisposition?.(row.interventionId, choice, reason);
  }

  return (
    <div style={{ border: "1px solid #ECEEF3", borderRadius: 13, padding: "14px 16px", marginBottom: 11 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 7 }}>
        <span style={{ fontSize: 13, fontWeight: 800, color: "#1C2333" }}>{row.label}</span>
        {row.band !== undefined && <span style={bandChipStyle(row.band)}>{row.band}</span>}
      </div>
      <div style={{ fontSize: 13, lineHeight: 1.65, color: "#5B6373" }}>{row.evidence}</div>
      {row.missing && (
        <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#8A92A3", marginTop: 6 }}>还缺：{row.missing}</div>
      )}
      {hasFix && (
        <div style={{ marginTop: 11, paddingTop: 11, borderTop: "1px dashed #ECEEF3" }}>
          <div style={{ fontSize: 12, color: "#8A92A3", marginBottom: 8 }}>建议：{row.fix}</div>
          <div style={{ display: "flex", gap: 7 }}>
            {REVIEW_KEYS.map((k) => {
              const picked = choice === k.action;
              return (
                <div
                  key={k.action}
                  role="button"
                  onClick={() => setChoice(k.action)}
                  style={{
                    flex: 1,
                    padding: "7px 6px",
                    borderRadius: 8,
                    fontSize: 11.5,
                    fontWeight: 700,
                    cursor: "pointer",
                    textAlign: "center",
                    fontFamily: "inherit",
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
          {choice !== null && (
            <div style={{ marginTop: 9 }}>
              <textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={2}
                placeholder="写下你的理由（至少 15 字）"
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
              <div style={{ fontSize: 10.5, fontWeight: 600, marginTop: 6, color: rlen === 0 ? "#AEB4C2" : reasonOk ? "#4C9A82" : "#D9A23D" }}>
                {rlen === 0 ? "留一句理由才算数（≥15 字）" : reasonOk ? `✓ 已记录 · ${rlen} 字` : `再写一点 · ${rlen}/15 字`}
              </div>
              <button
                type="button"
                onClick={submit}
                disabled={!canSubmit}
                style={{
                  marginTop: 8,
                  padding: "7px 12px",
                  borderRadius: 9,
                  border: "none",
                  fontSize: 12,
                  fontWeight: 700,
                  fontFamily: "inherit",
                  cursor: canSubmit ? "pointer" : "not-allowed",
                  background: canSubmit ? "#2A3B7A" : "#E1E4ED",
                  color: canSubmit ? "#fff" : "#AEB4C2",
                }}
              >
                记录处置
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
