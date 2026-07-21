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

// Controlled add-row list in the annotate/compare/graph mold: props in,
// onChange out, no internal persistence, no model calls (RL-4 — every item
// this component creates is author: "student"; there is no AI-authored path
// here at all). It renders a fixed vocabulary of buckets arriving via
// `buckets` from the card spec's params — it does not know the words
// 事实/观点/价值判断; a new sort card is a new JSON, not a renderer edit.
export function Sort({ buckets, state, minItems, itemPrompt, reasonPrompt, onChange, onLock, onSkip }: SortProps) {
  // 铁律 2: this counts classification progress in plain language, never a
  // score/streak/bar. classifiedCount ("已归类") is rows sorted into a
  // bucket; the lock gate additionally requires every filled row to also
  // carry a non-empty reason (a row can be classified but not yet complete).
  const classifiedCount = state.items.filter(isClassified).length;
  const remaining = Math.max(0, minItems - classifiedCount);
  const canLock = classifiedCount >= minItems && state.items.filter(isFilled).every(isComplete);

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
                    color: on ? "#2A3B7A" : "#6B7384",
                    background: on ? "#EDEFF9" : "#F4F5F8",
                    border: "1px solid " + (on ? "#C4CCE8" : "#E7E9F0"),
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
          marginBottom: 10,
          fontFamily: "inherit",
        }}
      >
        <PlusIcon />＋ 添加一句
      </button>

      <div style={{ fontSize: 12, color: "#8A93A6", marginBottom: 12 }}>
        已归类 {classifiedCount} 句 · 还差 {remaining} 句
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
