import type { ReactNode } from "react";
import type { SelectionEval } from "@mind-imprint/contracts";

export type HangingCardStatus = "proposed" | "active" | "evaluating" | "feedback";

export type HangingCardProps = {
  cardName: string;
  status: HangingCardStatus;
  exampleWhy: string; // AI's explanation of the example (shown at proposed)
  eval?: SelectionEval | null; // shown at feedback
  onStartPick: () => void; // proposed → active
  onConfirm: () => void; // feedback → completed
  onRepick: () => void; // feedback → active
  // onSkip — deadlock prevention: available at BOTH "proposed" and "active"
  // (not just the feedback repick), so an in-flight lens is ALWAYS
  // dismissable. Optional so a caller that truly can't offer a skip path
  // (none today) still compiles.
  onSkip?: () => void;
  // hasExample — false when the lens was summoned but the AI couldn't ground a
  // single illustrative sentence (graceful-degrade summon). The "proposed"
  // state then drops the "看懂示范" framing and invites her to pick her own
  // sentence directly. Defaults true (the router/normal summon always has one).
  hasExample?: boolean;
};

// The signature move (demo app.js:422-424): the card hangs under the AI's
// example block while `proposed` (she hasn't picked yet), and MOVES to hang
// under her own chosen sentence the moment she has one AND is past
// `proposed` — `active`/`evaluating`/`feedback` all read her pick, never the
// example, once she has actually picked something. If she is `active` but
// has not picked yet (studentBlockId null), it stays put on the example —
// there's nothing else to hang it on.
export function anchorBlockId(exampleBlockId: string, studentBlockId: string | null, status: HangingCardStatus): string {
  if (studentBlockId && status !== "proposed") return studentBlockId;
  return exampleBlockId;
}

const VERDICT_TONE: Record<SelectionEval["verdict"], { fg: string; bg: string }> = {
  strong: { fg: "#4C9A82", bg: "#EAF6F0" },
  partial: { fg: "#B5872F", bg: "#FBF3E3" },
  rethink: { fg: "#C2557A", bg: "#FBEAF0" },
};

const CHECK_TONE: Record<SelectionEval["checks"][number]["status"], { fg: string; bg: string; label: string }> = {
  pass: { fg: "#4C9A82", bg: "#EAF6F0", label: "✓" },
  partial: { fg: "#B5872F", bg: "#FBF3E3", label: "~" },
  miss: { fg: "#C2557A", bg: "#FBEAF0", label: "✕" },
};

function PrimaryButton({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 7,
        fontSize: 13,
        fontWeight: 700,
        cursor: "pointer",
        padding: "9px 15px",
        borderRadius: 10,
        color: "#fff",
        background: "#5C4A8A",
        border: "1px solid #5C4A8A",
        fontFamily: "inherit",
      }}
    >
      {children}
    </button>
  );
}

// The inline hanging card — the signature "read together" mechanic. It
// hangs under exactly one paragraph at a time (ReadingRoom, via
// anchorBlockId above, guarantees only one render). Exactly ONE primary
// action per status (focus mandate, 铁律 3: one thing at a time).
function SkipLink({ onSkip }: { onSkip: () => void }) {
  return (
    <button
      type="button"
      onClick={onSkip}
      style={{
        display: "block",
        marginTop: 10,
        background: "none",
        border: "none",
        color: "#8A92A3",
        fontSize: 12,
        fontWeight: 600,
        cursor: "pointer",
        padding: 0,
        textDecoration: "underline",
        fontFamily: "inherit",
      }}
    >
      跳过这副透镜
    </button>
  );
}

export function HangingCard({ cardName, status, exampleWhy, eval: selectionEval, onStartPick, onConfirm, onRepick, onSkip, hasExample = true }: HangingCardProps) {
  return (
    <div
      style={{
        position: "relative",
        margin: "4px 0 22px 18px",
        border: "1px solid #E3DCF2",
        borderRadius: 14,
        background: "#fff",
        boxShadow: "0 3px 14px rgba(92,74,138,.10)",
        overflow: "hidden",
      }}
    >
      <div className="lens-connector" aria-hidden="true" />
      <div style={{ height: 4, background: "#5C4A8A" }} />
      <div style={{ padding: "12px 15px 14px" }}>
        <div style={{ fontSize: 10.5, fontWeight: 700, color: "#5C4A8A", marginBottom: 8 }}>{cardName}</div>

        {status === "proposed" && hasExample && (
          <>
            <details style={{ marginBottom: 10 }}>
              <summary style={{ fontSize: 12, fontWeight: 600, color: "#8A92A3", cursor: "pointer" }}>为什么是这句（方法说明）</summary>
              <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#3A4256", marginTop: 6 }}>{exampleWhy}</div>
            </details>
            <PrimaryButton onClick={onStartPick}>看懂示范，开始选句</PrimaryButton>
            {onSkip && <SkipLink onSkip={onSkip} />}
          </>
        )}

        {status === "proposed" && !hasExample && (
          <>
            <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#3A4256", marginBottom: 10 }}>{exampleWhy}</div>
            <PrimaryButton onClick={onStartPick}>开始选句</PrimaryButton>
            {onSkip && <SkipLink onSkip={onSkip} />}
          </>
        )}

        {status === "active" && (
          <>
            <div style={{ fontSize: 13, lineHeight: 1.6, color: "#2B3346" }}>在文章里点出你自己的证据句</div>
            {onSkip && <SkipLink onSkip={onSkip} />}
          </>
        )}

        {status === "evaluating" && (
          <div style={{ fontSize: 13, lineHeight: 1.6, color: "#8A92A3" }}>印记正在看你的选择…</div>
        )}

        {status === "feedback" && selectionEval && (
          <>
            <div
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                fontSize: 12,
                fontWeight: 700,
                padding: "3px 10px",
                borderRadius: 999,
                color: VERDICT_TONE[selectionEval.verdict].fg,
                background: VERDICT_TONE[selectionEval.verdict].bg,
                marginBottom: 8,
              }}
            >
              {selectionEval.verdictLabel}
            </div>
            <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#3A4256", marginBottom: 10 }}>{selectionEval.verdictReason}</div>

            {selectionEval.checks.map((check) => (
              <div key={check.key} style={{ display: "flex", gap: 8, alignItems: "flex-start", marginBottom: 6 }}>
                <span
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    width: 16,
                    height: 16,
                    borderRadius: 999,
                    fontSize: 10,
                    fontWeight: 700,
                    color: CHECK_TONE[check.status].fg,
                    background: CHECK_TONE[check.status].bg,
                    flexShrink: 0,
                    marginTop: 1,
                  }}
                >
                  {CHECK_TONE[check.status].label}
                </span>
                <div>
                  <div style={{ fontSize: 12, fontWeight: 700, color: "#1C2333" }}>{check.label}</div>
                  <div style={{ fontSize: 12, lineHeight: 1.55, color: "#5A6072" }}>{check.explanation}</div>
                </div>
              </div>
            ))}

            <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#2B3346", marginTop: 10, marginBottom: 12 }}>{selectionEval.nextStep}</div>

            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <button
                type="button"
                onClick={onRepick}
                style={{ background: "none", border: "none", color: "#5C4A8A", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: 0, textDecoration: "underline", fontFamily: "inherit" }}
              >
                重新选一句
              </button>
              <PrimaryButton onClick={onConfirm}>记下这条发现</PrimaryButton>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
