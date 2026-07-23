import { useRef, useState } from "react";
import type { Anchor, CardInstance, CardSpec, MatrixState, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";
import { Matrix, anchorsToMatrixState, matrixStateToAnchors } from "../primitives/matrix";
import type { Col } from "../primitives/matrix";

export type StudioMatrixCardProps = {
  spec: CardSpec;
  // The bound card_instance's id — used to re-seed working state when a
  // fresh matrix card replaces this one in place (so a new instance never
  // inherits the previous one's half-filled rows).
  cardInstanceId: string;
  // Persisted anchors this card already carries — non-empty only on a
  // reload that rehydrates an in-progress card (FIX-E). Defaults to none.
  anchors?: Anchor[];
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

// Center-pane / rail host that binds the active `matrix` card_instance to the
// schema-driven Matrix primitive. It holds the working MatrixState, seeded on
// mount from persisted anchors so a reload restores in-progress rows, and
// serializes on lock — mirroring StudioToulminCard's shape exactly. The
// columns, minimum row count, and row prompt all arrive via `spec.params`;
// this host never hardcodes a column vocabulary (a new matrix card is a new
// JSON, not a renderer edit).
export function StudioMatrixCard({ spec, cardInstanceId, anchors = [], onSubmit, onSkip }: StudioMatrixCardProps) {
  const p = (spec.params ?? {}) as {
    cols?: Col[];
    min_items?: number;
    row_prompt?: string;
    row_noun?: string;
  };
  const cols = p.cols ?? [];
  const minItems = p.min_items ?? 1;
  const rowPrompt = p.row_prompt ?? "";
  const rowNoun = p.row_noun ?? "视角";

  // Working MatrixState. Seeded lazily from the persisted anchors
  // (rehydration), and re-seeded whenever the bound card_instance changes —
  // the derived-state reset pattern, guarded by a ref so it fires once per
  // instance rather than every render (parity with StudioToulminCard).
  const [state, setState] = useState<MatrixState>(() => anchorsToMatrixState(anchors, cols));
  const seededFor = useRef(cardInstanceId);
  if (seededFor.current !== cardInstanceId) {
    seededFor.current = cardInstanceId;
    setState(anchorsToMatrixState(anchors, cols));
  }

  function handleLock() {
    onSubmit({ ...newEnvelope(spec.id, ""), anchors: matrixStateToAnchors(state) });
  }

  function handleSkip() {
    const scaffold = newEnvelope(spec.id, "");
    onSkip(scaffold.event_trace);
  }

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
        <span style={{ fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "3px 10px", borderRadius: 999 }}>
          工具卡 · {spec.category}
        </span>
        <span style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>{spec.name}</span>
      </div>
      <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, marginBottom: 16 }}>{spec.purpose}</div>
      <Matrix
        cols={cols}
        state={state}
        minItems={minItems}
        rowPrompt={rowPrompt}
        rowNoun={rowNoun}
        onChange={setState}
        onLock={handleLock}
        onSkip={handleSkip}
      />
    </div>
  );
}
