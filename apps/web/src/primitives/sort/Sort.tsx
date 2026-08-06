import type { SortState, SortItem } from "@mind-imprint/contracts";
import type { Bucket } from "./serialize";

export type SortProps = {
  buckets: Bucket[];
  state: SortState;
  minItems: number;
  itemPrompt: string;
  reasonPrompt: string;
  onChange: (s: SortState) => void;
  onLock: () => void;
  onSkip: () => void;
};

let seq = 0;
const nid = () => `sort_${++seq}`;

function isFilled(item: SortItem): boolean {
  return item.text.trim() !== "";
}
function isClassified(item: SortItem): boolean {
  return isFilled(item) && item.bucket !== "";
}
function isComplete(item: SortItem): boolean {
  return isClassified(item) && item.reason.trim() !== "";
}

function RemoveIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--mk-faint)" strokeWidth={2.4} strokeLinecap="round" aria-hidden="true">
      <path d="M6 6l12 12M18 6L6 18" />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 5v14M5 12h14" stroke="var(--mk-accent)" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// Controlled add-row list in the annotate/compare/graph mold: props in,
// onChange out, no internal persistence, no model calls (RL-4 — every item
// this component creates is author: "student"; there is no AI-authored path
// here at all). It renders a fixed vocabulary of buckets arriving via
// `buckets` from the card spec's params — it does not know the words
// 事实/观点/价值判断; a new sort card is a new JSON, not a renderer edit.
export function Sort({ buckets, state, minItems, itemPrompt, reasonPrompt, onChange, onLock, onSkip }: SortProps) {
  // 铁律 2: this counts progress in plain language, never a score/streak/bar.
  //
  // The lock gate counts COMPLETE rows only, and it counts them the same way
  // the server does — `bucketedCount` in apps/api/internal/agent/card_completion.go
  // counts anchors whose dimension is a declared bucket AND whose answer is
  // non-empty, then asks whether that count reaches the minimum. It never
  // demands that every anchor qualify. Mirroring that exactly matters: an
  // `every(isComplete)` gate would wall a student who has finished her
  // minimum and then started one more row she has not reasoned about yet —
  // blocking a lock the backend would happily accept.
  //
  // Whole-branch review IMPORTANT 2: the progress line must be counted from
  // the SAME number the lock gate uses. It used to read from classifiedCount
  // (bucketed, reason or not) while the lock read completeCount, so a student
  // who bucketed 3 sentences but reasoned about only 2 saw "还差 0 句" next to
  // a greyed-out, unexplained lock button — a dead end. The line now names
  // exactly what the lock needs: bucketed AND reasoned.
  const completeCount = state.items.filter(isComplete).length;
  const remaining = Math.max(0, minItems - completeCount);
  const canLock = completeCount >= minItems;

  function handleTextChange(id: string, value: string) {
    onChange({ items: state.items.map((it) => (it.id === id ? { ...it, text: value } : it)) });
  }

  function handleBucketSelect(id: string, bucketId: string) {
    onChange({ items: state.items.map((it) => (it.id === id ? { ...it, bucket: bucketId } : it)) });
  }

  function handleReasonChange(id: string, value: string) {
    onChange({ items: state.items.map((it) => (it.id === id ? { ...it, reason: value } : it)) });
  }

  function handleRemove(id: string) {
    onChange({ items: state.items.filter((it) => it.id !== id) });
  }

  function handleAdd() {
    onChange({ items: [...state.items, { id: nid(), text: "", bucket: "", reason: "", author: "student" }] });
  }

  function handleLock() {
    if (!canLock) return;
    onLock();
  }

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {state.items.map((item, index) => (
        <div
          key={item.id}
          style={{
            background: "var(--mk-surface)",
            border: "1px solid var(--mk-border)",
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
                border: "1px solid var(--mk-input-border)",
                borderRadius: 10,
                padding: "8px 10px",
                fontSize: 13.5,
                lineHeight: 1.55,
                color: "var(--mk-ink)",
                background: "var(--mk-surface)",
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

          <div style={{ display: "flex", gap: 7, flexWrap: "wrap", margin: "9px 0" }}>
            {buckets.map((b) => {
              const on = item.bucket === b.id;
              return (
                <button
                  key={b.id}
                  type="button"
                  title={b.hint}
                  onClick={() => handleBucketSelect(item.id, b.id)}
                  style={{
                    fontSize: 12,
                    fontWeight: 700,
                    cursor: "pointer",
                    padding: "6px 11px",
                    borderRadius: 9,
                    color: on ? "var(--mk-accent)" : "var(--mk-secondary)",
                    background: on ? "var(--mk-accent-50)" : "var(--mk-paper)",
                    border: "1px solid " + (on ? "var(--mk-accent-200)" : "var(--mk-border)"),
                    fontFamily: "inherit",
                  }}
                >
                  {b.label}
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
              border: "1px solid var(--mk-input-border)",
              borderRadius: 10,
              padding: "8px 10px",
              fontSize: 13,
              lineHeight: 1.55,
              color: "var(--mk-ink)",
              background: "var(--mk-surface)",
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
          border: "1px dashed var(--mk-accent-200)",
          color: "var(--mk-accent)",
          borderRadius: 10,
          fontSize: 12.5,
          fontWeight: 700,
          cursor: "pointer",
          padding: "8px 13px",
          marginBottom: 10,
          fontFamily: "inherit",
        }}
      >
        <PlusIcon />
        添加一句
      </button>

      <div style={{ fontSize: 12, color: "var(--mk-muted)", marginBottom: 12 }}>
        已归类并写下理由 {completeCount} 句 · 还差 {remaining} 句
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <button
          type="button"
          onClick={onSkip}
          style={{ background: "none", border: "none", color: "var(--mk-faint)", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: "6px 0", fontFamily: "inherit" }}
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
            color: "var(--mk-surface)",
            background: "var(--mk-accent)",
            border: "1px solid var(--mk-accent)",
            fontFamily: "inherit",
          }}
        >
          完成并钉到过程树
        </button>
      </div>
    </div>
  );
}
