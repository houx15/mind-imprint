import type { ScaleState, ScaleItem } from "@mind-imprint/contracts";
import type { Bucket } from "./serialize";

export type ScaleProps = {
  stops: Bucket[];
  state: ScaleState;
  minItems: number;
  itemPrompt: string;
  reasonPrompt: string;
  rewritePrompt: string;
  onChange: (s: ScaleState) => void;
  onLock: () => void;
  onSkip: () => void;
};

let seq = 0;
const nid = () => `scale_${++seq}`;

function isFilled(item: ScaleItem): boolean {
  return item.text.trim() !== "";
}
function isPlaced(item: ScaleItem): boolean {
  return isFilled(item) && item.stop !== "";
}
function isComplete(item: ScaleItem): boolean {
  return isPlaced(item) && item.reason.trim() !== "";
}

function RemoveIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth={2.4} strokeLinecap="round" aria-hidden="true">
      <path d="M6 6l12 12M18 6L6 18" />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 5v14M5 12h14" stroke="#2A3B7A" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// Controlled add-row list in the sort/annotate/compare/graph mold: props in,
// onChange out, no internal persistence, no model calls (RL-4 — every item
// this component creates is author: "student"; there is no AI-authored path
// here at all). It renders a fixed ORDERED vocabulary of stops arriving via
// `stops` from the card spec's params — it does not know the words
// 个人猜测/有据推断/强证据/科学共识/逻辑必然; a new scale card is a new JSON,
// not a renderer edit.
export function Scale({ stops, state, minItems, itemPrompt, reasonPrompt, rewritePrompt, onChange, onLock, onSkip }: ScaleProps) {
  // 铁律 2: this counts placement progress in plain language, never a
  // score/streak/bar. placedCount is rows placed onto a stop (may still lack
  // a reason).
  //
  // The lock gate counts COMPLETE rows the way the server does —
  // bucketedCount in apps/api/internal/agent/card_completion.go counts
  // anchors whose dimension is a declared stop AND whose answer is
  // non-empty, then asks whether that count reaches the minimum; it never
  // demands that every anchor qualify. An `every(isComplete)` gate would
  // wall a student who has finished her minimum and then started one more
  // row she has not reasoned about yet. The second predicate,
  // field_written_by{field:"rewrite"}, only applies when the card actually
  // asks for a rewrite (rewritePrompt non-empty) — a card with no rewrite
  // field must not gate on one.
  const placedCount = state.items.filter(isPlaced).length;
  const completeCount = state.items.filter(isComplete).length;
  const remaining = Math.max(0, minItems - placedCount);
  const rewriteOk = rewritePrompt.trim() === "" || state.rewrite.trim() !== "";
  const canLock = completeCount >= minItems && rewriteOk;

  function handleTextChange(id: string, value: string) {
    onChange({ ...state, items: state.items.map((it) => (it.id === id ? { ...it, text: value } : it)) });
  }

  function handleStopSelect(id: string, stopId: string) {
    onChange({ ...state, items: state.items.map((it) => (it.id === id ? { ...it, stop: stopId } : it)) });
  }

  function handleReasonChange(id: string, value: string) {
    onChange({ ...state, items: state.items.map((it) => (it.id === id ? { ...it, reason: value } : it)) });
  }

  function handleRemove(id: string) {
    onChange({ ...state, items: state.items.filter((it) => it.id !== id) });
  }

  function handleAdd() {
    onChange({ ...state, items: [...state.items, { id: nid(), text: "", stop: "", reason: "", author: "student" }] });
  }

  function handleRewriteChange(value: string) {
    onChange({ ...state, rewrite: value });
  }

  function handleLock() {
    if (!canLock) return;
    onLock();
  }

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {/* Ordered axis header — decorative, conveys the stop order (leftmost
          = first stop) with a hairline connector. Not itself interactive:
          placement happens per-item below, via the same stop vocabulary
          rendered as a compact chip row (a wrapped/compact row of labels,
          not a drag interaction). */}
      <div style={{ position: "relative", padding: "4px 2px 14px", marginBottom: 4 }}>
        <div style={{ position: "absolute", left: 6, right: 6, top: 13, height: 1, background: "#DEE1EA" }} aria-hidden="true" />
        <div style={{ display: "flex", justifyContent: "space-between", gap: 2 }}>
          {stops.map((s) => (
            <div key={s.id} title={s.hint} style={{ position: "relative", zIndex: 1, textAlign: "center", flex: 1, minWidth: 0 }}>
              <div style={{ width: 6, height: 6, borderRadius: "50%", background: "#C4CCE8", margin: "0 auto 4px" }} aria-hidden="true" />
              <div style={{ fontSize: 10.5, fontWeight: 700, color: "#6B7384", lineHeight: 1.25 }}>{s.label}</div>
            </div>
          ))}
        </div>
      </div>

      {state.items.map((item, index) => (
        <div
          key={item.id}
          style={{
            background: "#fff",
            border: "1px solid #EAECF2",
            borderRadius: 14,
            padding: "13px 14px",
            marginBottom: 11,
          }}
        >
          <div style={{ display: "flex", alignItems: "flex-start", gap: 8 }}>
            <textarea
              value={item.text}
              onChange={(e) => handleTextChange(item.id, e.target.value)}
              rows={2}
              placeholder={itemPrompt}
              style={{
                flex: 1,
                border: "1px solid #E1E4ED",
                borderRadius: 10,
                padding: "8px 10px",
                fontSize: 13.5,
                lineHeight: 1.55,
                color: "#1C2333",
                background: "#fff",
                outline: "none",
                resize: "vertical",
                fontFamily: "inherit",
              }}
            />
            <button
              type="button"
              aria-label={`删除第 ${index + 1} 句`}
              onClick={() => handleRemove(item.id)}
              style={{
                background: "none",
                border: "none",
                cursor: "pointer",
                padding: 4,
                marginTop: 2,
                lineHeight: 0,
              }}
            >
              <RemoveIcon />
            </button>
          </div>

          <div style={{ display: "flex", gap: 6, flexWrap: "wrap", margin: "9px 0" }}>
            {stops.map((s) => {
              const on = item.stop === s.id;
              return (
                <button
                  key={s.id}
                  type="button"
                  title={s.hint}
                  onClick={() => handleStopSelect(item.id, s.id)}
                  style={{
                    fontSize: 11.5,
                    fontWeight: 700,
                    cursor: "pointer",
                    padding: "5px 9px",
                    borderRadius: 9,
                    color: on ? "#2A3B7A" : "#6B7384",
                    background: on ? "#EDEFF9" : "#F4F5F8",
                    border: "1px solid " + (on ? "#C4CCE8" : "#E7E9F0"),
                    fontFamily: "inherit",
                  }}
                >
                  {s.label}
                </button>
              );
            })}
          </div>

          <textarea
            value={item.reason}
            onChange={(e) => handleReasonChange(item.id, e.target.value)}
            rows={2}
            placeholder={reasonPrompt}
            style={{
              width: "100%",
              border: "1px solid #E1E4ED",
              borderRadius: 10,
              padding: "8px 10px",
              fontSize: 13,
              lineHeight: 1.55,
              color: "#1C2333",
              background: "#fff",
              outline: "none",
              resize: "vertical",
              fontFamily: "inherit",
            }}
          />
        </div>
      ))}

      <button
        type="button"
        onClick={handleAdd}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          background: "none",
          border: "1px dashed #C4CCE8",
          color: "#2A3B7A",
          borderRadius: 10,
          fontSize: 12.5,
          fontWeight: 700,
          cursor: "pointer",
          padding: "8px 13px",
          marginBottom: 14,
          fontFamily: "inherit",
        }}
      >
        <PlusIcon />＋ 添加一句
      </button>

      {rewritePrompt.trim() !== "" && (
        <div style={{ marginBottom: 14 }}>
          <textarea
            value={state.rewrite}
            onChange={(e) => handleRewriteChange(e.target.value)}
            rows={2}
            placeholder={rewritePrompt}
            style={{
              width: "100%",
              border: "1px solid #E1E4ED",
              borderRadius: 10,
              padding: "8px 10px",
              fontSize: 13,
              lineHeight: 1.55,
              color: "#1C2333",
              background: "#fff",
              outline: "none",
              resize: "vertical",
              fontFamily: "inherit",
            }}
          />
        </div>
      )}

      <div style={{ fontSize: 12, color: "#8A93A6", marginBottom: 12 }}>
        已放置 {placedCount} 句 · 还差 {remaining} 句
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <button
          type="button"
          onClick={onSkip}
          style={{ background: "none", border: "none", color: "#C2557A", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: "6px 0", fontFamily: "inherit" }}
        >
          跳过这张卡
        </button>
        <button
          type="button"
          onClick={handleLock}
          disabled={!canLock}
          style={{
            fontSize: 13,
            fontWeight: 700,
            cursor: canLock ? "pointer" : "not-allowed",
            opacity: canLock ? 1 : 0.5,
            padding: "9px 15px",
            borderRadius: 10,
            color: "#fff",
            background: "#2A3B7A",
            border: "1px solid #2A3B7A",
            fontFamily: "inherit",
          }}
        >
          完成并钉到过程树
        </button>
      </div>
    </div>
  );
}
