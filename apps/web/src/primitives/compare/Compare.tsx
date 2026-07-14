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

      {/* No pair-connector rendering here: CompareState.pairs is always []
          in practice — see ViewFrame.buildCompareState's doc comment for
          why SIFT's own field layout can never populate it, and why that
          computation (and this rendering) was removed as dead code rather
          than kept "just in case" (FIX-B review finding [1]). A future
          compare card whose two panes genuinely share dimensions can add
          pair rendering back deliberately, against its own field layout. */}
    </div>
  );
}
