import { useRef, useState } from "react";
import type { Anchor, CardInstance, CardSpec, SortState, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";
import { Sort, anchorsToSortState, sortStateToAnchors } from "../primitives/sort";
import type { Bucket } from "../primitives/sort";

export type StudioSortCardProps = {
  spec: CardSpec;
  // The bound card_instance's id — used to re-seed working state when a
  // fresh sort card replaces this one in place (so a new instance never
  // inherits the previous one's half-classified rows).
  cardInstanceId: string;
  // Persisted anchors this card already carries — non-empty only on a
  // reload that rehydrates an in-progress card (FIX-E). Defaults to none.
  anchors?: Anchor[];
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

// Center-pane / rail host that binds the active `sort` card_instance to the
// schema-driven Sort primitive. It holds the working SortState, seeded on
// mount from persisted anchors so a reload restores in-progress rows, and
// serializes on lock — mirroring StudioToulminCard's shape exactly. The
// buckets, minimum row count, and both prompts all arrive via `spec.params`;
// this host never hardcodes a bucket vocabulary (a new sort card is a new
// JSON, not a renderer edit).
export function StudioSortCard({ spec, cardInstanceId, anchors = [], onSubmit, onSkip }: StudioSortCardProps) {
  const p = (spec.params ?? {}) as {
    buckets?: Bucket[];
    min_items?: number;
    item_prompt?: string;
    reason_prompt?: string;
  };
  const buckets = p.buckets ?? [];
  const minItems = p.min_items ?? 1;
  const itemPrompt = p.item_prompt ?? "";
  const reasonPrompt = p.reason_prompt ?? "";

  // Working SortState. Seeded lazily from the persisted anchors
  // (rehydration), and re-seeded whenever the bound card_instance changes —
  // the derived-state reset pattern, guarded by a ref so it fires once per
  // instance rather than every render (parity with StudioToulminCard).
  const [state, setState] = useState<SortState>(() => anchorsToSortState(anchors, buckets));
  const seededFor = useRef(cardInstanceId);
  if (seededFor.current !== cardInstanceId) {
    seededFor.current = cardInstanceId;
    setState(anchorsToSortState(anchors, buckets));
  }

  function handleLock() {
    onSubmit({ ...newEnvelope(spec.id, ""), anchors: sortStateToAnchors(state) });
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
      <Sort
        buckets={buckets}
        state={state}
        minItems={minItems}
        itemPrompt={itemPrompt}
        reasonPrompt={reasonPrompt}
        onChange={setState}
        onLock={handleLock}
        onSkip={handleSkip}
      />
    </div>
  );
}
