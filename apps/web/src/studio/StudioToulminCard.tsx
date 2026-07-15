import { useRef, useState } from "react";
import type { Anchor, CardInstance, CardSpec, GraphState, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";
import { Graph, anchorsToGraphState, graphStateToAnchors } from "../primitives/graph";
import type { LockedSource, Slot } from "../primitives/graph";

export type StudioToulminCardProps = {
  spec: CardSpec;
  // The bound card_instance's id — used to re-seed working state when a fresh
  // Toulmin card replaces this one in place (so a new instance never inherits
  // the previous one's half-filled nodes/edges).
  cardInstanceId: string;
  // Persisted anchors this card already carries — non-empty only on a reload
  // that rehydrates an in-progress card (FIX-E). Defaults to none.
  anchors?: Anchor[];
  // The CRAAP-locked materials the student may cite from a needSrc slot,
  // projected as {id, name} from the project's material list.
  lockedSources: LockedSource[];
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

// Center-pane host that binds the active `toulmin` card_instance to the schema-
// driven Graph primitive (design 结构 view). It holds the working GraphState,
// seeded on mount from persisted anchors so a reload restores in-progress
// slots, and serializes on lock — mirroring StudioAnnotateCard.handleLock's
// envelope shape (project-scoped card → material_id ""). The five roles, their
// questions and needSrc flags all arrive via `spec.params.slots`; this host
// never hardcodes Toulmin.
export function StudioToulminCard({ spec, cardInstanceId, anchors = [], lockedSources, onSubmit, onSkip }: StudioToulminCardProps) {
  const slots = ((spec.params ?? {}) as { slots?: Slot[] }).slots ?? [];

  // Working GraphState. Seeded lazily from the persisted anchors (rehydration),
  // and re-seeded whenever the bound card_instance changes — the derived-state
  // reset pattern, guarded by a ref so it fires once per instance rather than
  // every render (parity with StudioContainer's activeCardInstanceId reset for
  // the compare card's lateral pick).
  const [state, setState] = useState<GraphState>(() => anchorsToGraphState(anchors, slots));
  const seededFor = useRef(cardInstanceId);
  if (seededFor.current !== cardInstanceId) {
    seededFor.current = cardInstanceId;
    setState(anchorsToGraphState(anchors, slots));
  }

  function handleLock() {
    onSubmit({ ...newEnvelope(spec.id, ""), anchors: graphStateToAnchors(state) });
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
      <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, marginBottom: 16 }}>
        逐张卡片来：点开一张，先选相关素材（你在信源评估里锁定的），再基于素材把这一步写成句子。印记只提问、不代笔。
      </div>
      <Graph
        slots={slots}
        state={state}
        lockedSources={lockedSources}
        onChange={setState}
        onLock={handleLock}
        onSkip={handleSkip}
      />
    </div>
  );
}
