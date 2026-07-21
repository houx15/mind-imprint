import type { MatrixState, MatrixRow } from "@mind-imprint/contracts";
import type { Col } from "./serialize";

export type MatrixProps = {
  cols: Col[];
  state: MatrixState;
  minItems: number;
  rowPrompt: string;
  onChange: (s: MatrixState) => void;
  onLock: () => void;
  onSkip: () => void;
};

let seq = 0;
const nid = () => `mxr_${++seq}`;

function isLabeled(row: MatrixRow): boolean {
  return row.label.trim() !== "";
}
function isComplete(row: MatrixRow, cols: Col[]): boolean {
  if (!isLabeled(row)) return false;
  return cols.every((c) => (row.cells[c.id] ?? "").trim() !== "");
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

// Controlled add-row list in the sort/scale mold: props in, onChange out, no
// internal persistence, no model calls (RL-4 — every row this component
// creates is author: "student"; there is no AI-authored path here at all).
// It renders a fixed vocabulary of COLUMNS arriving via `cols` from the card
// spec's params — it does not know the words 立场主张/依据/盲区; a new
// matrix card is a new JSON, not a renderer edit. Rendered as a STACKED LIST
// OF ROW-CARDS rather than an HTML table: this lives in the CoachRail, a
// 388px-wide column, where a 3-column grid would be unreadable. Unlike sort
// and scale (fixed rows classified into a fixed vocabulary), here the ROWS
// themselves are student-authored — identifying whose perspective is
// missing IS the thinking, which is why the card cannot pre-fill rows.
export function Matrix({ cols, state, minItems, rowPrompt, onChange, onLock, onSkip }: MatrixProps) {
  // 铁律 2: plain-text progress only, never a score/streak/bar/celebration.
  // completeCount mirrors the server exactly — completeMatrixRows /
  // firstIncompleteMatrixRow (apps/api/internal/agent/card_effects.go,
  // card_completion.go) count a row as complete once its label is non-blank
  // AND every declared column has a non-blank answer, then ask whether that
  // COUNT reaches minItems — never whether every row qualifies. A student
  // who has two complete perspectives and starts a third must still be able
  // to lock; an `every(isComplete)` gate would wall her out of a lock the
  // backend would happily accept.
  const completeCount = state.rows.filter((r) => isComplete(r, cols)).length;
  const remaining = Math.max(0, minItems - completeCount);
  const canLock = completeCount >= minItems;

  function handleLabelChange(id: string, value: string) {
    onChange({ rows: state.rows.map((r) => (r.id === id ? { ...r, label: value } : r)) });
  }

  function handleCellChange(id: string, colId: string, value: string) {
    onChange({
      rows: state.rows.map((r) => (r.id === id ? { ...r, cells: { ...r.cells, [colId]: value } } : r)),
    });
  }

  function handleRemove(id: string) {
    onChange({ rows: state.rows.filter((r) => r.id !== id) });
  }

  function handleAdd() {
    // Initialize cells with every column key up front (blank), in `cols`
    // order — this is what keeps matrixStateToAnchors' "cell order follows
    // cols order" true, since Object.entries preserves insertion order.
    const cells = Object.fromEntries(cols.map((c) => [c.id, ""] as const));
    onChange({ rows: [...state.rows, { id: nid(), label: "", cells, author: "student" }] });
  }

  function handleLock() {
    if (!canLock) return;
    onLock();
  }

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {state.rows.map((row, index) => (
        <div
          key={row.id}
          style={{
            background: "#fff",
            border: "1px solid #EAECF2",
            borderRadius: 14,
            padding: "13px 14px",
            marginBottom: 11,
          }}
        >
          <div style={{ display: "flex", alignItems: "flex-start", gap: 8, marginBottom: 9 }}>
            <input
              type="text"
              value={row.label}
              onChange={(e) => handleLabelChange(row.id, e.target.value)}
              placeholder={rowPrompt}
              style={{
                flex: 1,
                border: "1px solid #E1E4ED",
                borderRadius: 10,
                padding: "8px 10px",
                fontSize: 13.5,
                fontWeight: 700,
                color: "#1C2333",
                background: "#F9FAFC",
                outline: "none",
                fontFamily: "inherit",
              }}
            />
            <button
              type="button"
              aria-label={`删除第 ${index + 1} 个视角`}
              onClick={() => handleRemove(row.id)}
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

          {cols.map((col) => (
            <div key={col.id} style={{ marginBottom: 8 }}>
              <div style={{ fontSize: 11, fontWeight: 700, color: "#6B7384", marginBottom: 4 }}>{col.label}</div>
              <textarea
                value={row.cells[col.id] ?? ""}
                onChange={(e) => handleCellChange(row.id, col.id, e.target.value)}
                rows={2}
                placeholder={col.q}
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
        <PlusIcon />＋ 添加一个视角
      </button>

      <div style={{ fontSize: 12, color: "#8A93A6", marginBottom: 12 }}>
        已完成 {completeCount} 个视角 · 还差 {remaining} 个
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
