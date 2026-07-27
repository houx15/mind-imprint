import type { AnnotateState, MaterialSource, SelectionEval } from "@mind-imprint/contracts";
import { Annotate } from "../../primitives/annotate";
import { anchorToSpan } from "../material/SourceDossier";
import { HangingCard, type HangingCardStatus, anchorBlockId } from "./HangingCard";
import "./ReadingRoom.css";

type AnnotateSpan = AnnotateState["spans"][number];

// The read-together loop (Task 10) drives this via live status/eval; this
// task only wires the shape through so the card renders correctly given
// props. `exampleBlockId`/`studentBlockId` feed `anchorBlockId` (the
// signature anchor-move) to decide which paragraph the card hangs under.
export type ReadingRoomCard = {
  status: HangingCardStatus;
  cardName: string;
  exampleBlockId: string;
  studentBlockId: string | null;
  exampleWhy: string;
  eval?: SelectionEval | null;
  onStartPick: () => void;
  onConfirm: () => void;
  onRepick: () => void;
};

export type ReadingRoomProps = {
  source: MaterialSource;
  onBack: () => void;
  // Optional — the read-together loop (Task 10) supplies this to drive the
  // inline hanging card. Absent/null renders exactly as today (no card).
  card?: ReadingRoomCard | null;
};

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="#5C4A8A" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// The focused reading surface (spec: "read together"): coach column on the
// left, article on the right. No reading loop/card yet — that lands in a
// later task; this task is the shell + article render + back navigation
// only.
export function ReadingRoom({ source, onBack, card }: ReadingRoomProps) {
  return (
    <div className="mk-reading-room">
      <div className="mk-reading-room__coach">
        <button type="button" className="mk-reading-room__back" onClick={onBack}>
          <BackIcon />
          返回工作区
        </button>
        {/* Coach column placeholder — the reading loop's live coach thread
            lands in a later task. */}
        <div />
      </div>
      <div className="mk-reading-room__article">
        <div className="mk-reading-room__article-inner">
          <div className="mk-reading-room__title">{source.title}</div>
          <Annotate
            blocks={source.blocks}
            state={{
              material_id: source.id,
              spans: source.anchors.map(anchorToSpan).filter((s): s is AnnotateSpan => s !== null),
            }}
            activeSpanId={null}
            onSelectSpan={() => {}}
            renderAfterBlock={(blockId) => {
              if (!card) return null;
              // Only ever render ONE card (focus mandate) — this equality
              // guard is what guarantees that: anchorBlockId resolves to
              // exactly one block id, so only that block's slot renders it.
              if (anchorBlockId(card.exampleBlockId, card.studentBlockId, card.status) !== blockId) return null;
              return (
                <HangingCard
                  cardName={card.cardName}
                  status={card.status}
                  exampleWhy={card.exampleWhy}
                  eval={card.eval}
                  onStartPick={card.onStartPick}
                  onConfirm={card.onConfirm}
                  onRepick={card.onRepick}
                />
              );
            }}
          />
        </div>
      </div>
    </div>
  );
}
