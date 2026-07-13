import { useState } from "react";
import type { CompareState } from "@mind-imprint/contracts";
import { Annotate } from "../annotate";

export type CompareProps = {
  state: CompareState;
  // Opens 6b's 添加信源 flow — Compare never ingests a source itself (RL-2).
  onAddLateralSource: () => void;
  // Annotate renders text from material blocks, but CompareState (like
  // AnnotateState) carries only material_id + spans, not the material body.
  // The container owns fetching/holding material blocks; Compare just needs
  // somewhere to put them per pane. Optional + defaulted to [] so the exact
  // two-prop call the spec fixes (state, onAddLateralSource) still works —
  // callers that have blocks pass them, callers that don't get empty panes
  // instead of a crash.
  leftBlocks?: { id: string; text: string }[];
  rightBlocks?: { id: string; text: string }[];
};

const RELATION_LABEL: Record<"corroborates" | "contradicts" | "qualifies", string> = {
  corroborates: "印证",
  contradicts: "矛盾",
  qualifies: "需要限定",
};

function PlusIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 5v14M5 12h14" stroke="#2A3B7A" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function LateralSearchIcon() {
  return (
    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="10" cy="10" r="6.5" stroke="#B7ABDB" strokeWidth="2" />
      <path d="M14.8 14.8L20 20" stroke="#B7ABDB" strokeWidth="2" strokeLinecap="round" />
      <path d="M17 5l1.6 4.4L23 11l-4.4 1.6L17 17l-1.6-4.4L11 11l4.4-1.6L17 5z" fill="#D8CFF0" />
    </svg>
  );
}

function LinkIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M9 15l6-6M8 16l-2 2a3.5 3.5 0 0 1-5-5l3-3a3.5 3.5 0 0 1 5 0M16 8l2-2a3.5 3.5 0 0 1 5 5l-3 3a3.5 3.5 0 0 1-5 0" stroke="#8A7BB8" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

// The right pane before a lateral source exists — the entire point of SIFT's
// "leave the page" move. This is an invitation, not a loading/empty state:
// no spinner, no skeleton, no "no data" placeholder.
function LateralSourceAssignment({ onAddLateralSource }: { onAddLateralSource: () => void }) {
  return (
    <div
      data-testid="compare-pane"
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        textAlign: "center",
        gap: 10,
        background: "#FAF8FE",
        border: "1px dashed #D8CFF0",
        borderRadius: 14,
        padding: "36px 22px",
        minHeight: 220,
      }}
    >
      <LateralSearchIcon />
      <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>去找一个独立的来源</div>
      <div style={{ fontSize: 12.5, color: "#7A8296", lineHeight: 1.6, maxWidth: 240 }}>
        别在这一页上死磕——打开新的标签页，看看其他独立信源怎么说这件事。
      </div>
      <button
        type="button"
        onClick={onAddLateralSource}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          marginTop: 4,
          background: "#2A3B7A",
          color: "#fff",
          border: "none",
          borderRadius: 10,
          fontSize: 13,
          fontWeight: 700,
          padding: "8px 16px",
          cursor: "pointer",
          fontFamily: "inherit",
        }}
      >
        <PlusIcon />
        添加信源
      </button>
    </div>
  );
}

export function Compare({ state, onAddLateralSource, leftBlocks = [], rightBlocks = [] }: CompareProps) {
  const [leftActiveSpanId, setLeftActiveSpanId] = useState<string | null>(null);
  const [rightActiveSpanId, setRightActiveSpanId] = useState<string | null>(null);

  const findSpanTag = (side: "left" | "right", spanId: string) => {
    const pane = side === "left" ? state.left : state.right;
    return pane?.spans.find((s) => s.id === spanId)?.tag ?? spanId;
  };

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
        <div>
          <div style={{ fontSize: 12, fontWeight: 700, color: "#8A93A6", marginBottom: 8 }}>待查的来源</div>
          <div data-testid="compare-pane" style={{ background: "#fff", border: "1px solid #E4E6EE", borderRadius: 14, padding: "14px 16px" }}>
            <Annotate blocks={leftBlocks} state={state.left} activeSpanId={leftActiveSpanId} onSelectSpan={setLeftActiveSpanId} />
          </div>
        </div>

        <div>
          <div style={{ fontSize: 12, fontWeight: 700, color: "#8A93A6", marginBottom: 8 }}>独立信源</div>
          {state.right ? (
            <div data-testid="compare-pane" style={{ background: "#fff", border: "1px solid #E4E6EE", borderRadius: 14, padding: "14px 16px" }}>
              <Annotate blocks={rightBlocks} state={state.right} activeSpanId={rightActiveSpanId} onSelectSpan={setRightActiveSpanId} />
            </div>
          ) : (
            <LateralSourceAssignment onAddLateralSource={onAddLateralSource} />
          )}
        </div>
      </div>

      {/* Pair links visually connect l_span <-> r_span. Two independent
          Annotate instances live in unrelated DOM subtrees (they may even
          scroll independently), so a drawn connector line between arbitrary
          highlight positions would be fragile geometry for no real payoff.
          A connector list does the same job honestly: it names the two
          spans, and the relation + note are shown verbatim as the student
          wrote them (author is always "student" — the AI never proposes a
          pair, per the contract). */}
      {state.pairs.length > 0 && (
        <div style={{ marginTop: 16, display: "flex", flexDirection: "column", gap: 8 }}>
          <div style={{ fontSize: 12, fontWeight: 700, color: "#8A93A6" }}>对照笔记</div>
          {state.pairs.map((pair) => (
            <div
              key={pair.id}
              style={{
                display: "flex",
                alignItems: "flex-start",
                gap: 10,
                background: "#F7F5FB",
                border: "1px solid #E3DCF2",
                borderRadius: 12,
                padding: "10px 13px",
              }}
            >
              <LinkIcon />
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 12, color: "#5C4A8A", display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                  <span style={{ fontWeight: 700 }}>{findSpanTag("left", pair.l_span)}</span>
                  <span>↔</span>
                  <span style={{ fontWeight: 700 }}>{findSpanTag("right", pair.r_span)}</span>
                  <span
                    style={{
                      fontSize: 10.5,
                      fontWeight: 700,
                      padding: "1px 8px",
                      borderRadius: 999,
                      background: "#EAE4F7",
                      color: "#5C4A8A",
                    }}
                  >
                    {RELATION_LABEL[pair.relation]}
                  </span>
                </div>
                {pair.note.trim().length > 0 && (
                  <div style={{ fontSize: 13, color: "#3A4256", marginTop: 4, lineHeight: 1.5 }}>{pair.note}</div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
