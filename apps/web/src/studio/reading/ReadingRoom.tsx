import type { AnnotateState, MaterialSource } from "@mind-imprint/contracts";
import { Annotate } from "../../primitives/annotate";
import { anchorToSpan } from "../material/SourceDossier";
import "./ReadingRoom.css";

type AnnotateSpan = AnnotateState["spans"][number];

export type ReadingRoomProps = {
  source: MaterialSource;
  onBack: () => void;
  // loop props added in Task 10; this task renders article + coach shell +
  // back only.
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
export function ReadingRoom({ source, onBack }: ReadingRoomProps) {
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
          />
        </div>
      </div>
    </div>
  );
}
