import { useEffect, useMemo, useRef, useState } from "react";
import type { Anchor, AnnotateState, MaterialSource, SelectionEval } from "@mind-imprint/contracts";
import { Annotate } from "../../primitives/annotate";
import { anchorToSpan } from "../material/SourceDossier";
import { HangingCard, type HangingCardStatus, anchorBlockId } from "./HangingCard";
import { useReadingLoop, type ReadingLoopApi } from "./readingLoop";
import "./ReadingRoom.css";

type AnnotateSpan = AnnotateState["spans"][number];

// Built internally (Task 10) from the live `useReadingLoop` state — no
// longer an external prop; `exampleBlockId`/`studentBlockId` feed
// `anchorBlockId` (the signature anchor-move) to decide which paragraph the
// card hangs under.
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
  projectId: string;
  source: MaterialSource;
  onBack: () => void;
  // The loop controller's own api slice (readTurn/activateProjectCard/
  // evaluateCardSelection/submitProjectCard/skipProjectCard) — ReadingRoom
  // owns the loop (`useReadingLoop`) internally now that Task 10 wires it.
  api: ReadingLoopApi;
  // Reinstates the reading-time logging that used to fire from
  // SourceDossier's open/close lifecycle (Task 8 binding) — StudioContainer
  // passes its existing `onOpenLogged` callback through here. Optional so a
  // bare render (e.g. a component test with no logging concern) still works.
  onOpenLogged?: (materialId: string, timeSpentS: number) => void;
};

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="#5C4A8A" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// The focused reading surface (spec: "read together"): coach column on the
// left, article on the right. Task 10 wires the full loop in: the coach
// composer drives `sendTurn`, the article enters select-mode once a card is
// `active`, and the hanging card renders from the loop's live status/eval.
export function ReadingRoom({ projectId, source, onBack, api, onOpenLogged }: ReadingRoomProps) {
  const loop = useReadingLoop(projectId, source, api);
  const [draft, setDraft] = useState("");

  // Reinstates the reading-time logging that used to fire from
  // SourceDossier's open/close lifecycle (a Task 8 binding — the move to
  // this focused surface left it with no reachable trigger). ReadingRoom is
  // only ever mounted for exactly one source at a time (the container fully
  // swaps it out on 返回工作区/close), so a single mount-timestamp + unmount
  // report — no dependency on source.id changing mid-mount — is sufficient,
  // and the null-out guard keeps this idempotent even if the cleanup effect
  // were ever invoked more than once (React StrictMode double-invokes
  // effects in dev).
  const openedAtRef = useRef<number | null>(null);
  useEffect(() => {
    openedAtRef.current = Date.now();
    return () => {
      const openedAt = openedAtRef.current;
      openedAtRef.current = null;
      if (openedAt == null || !onOpenLogged) return;
      const timeSpentS = Math.round((Date.now() - openedAt) / 1000);
      onOpenLogged(source.id, timeSpentS);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const card: ReadingRoomCard | null =
    loop.status === "idle"
      ? null
      : {
          status: loop.status,
          cardName: loop.cardName,
          exampleBlockId: loop.exampleBlockId,
          studentBlockId: loop.studentSpan?.blockId ?? null,
          exampleWhy: loop.exampleWhy,
          eval: loop.eval,
          onStartPick: loop.startPick,
          onConfirm: loop.confirm,
          onRepick: loop.repick,
        };

  // The article's own spans = the source's persisted anchors, plus (once the
  // loop has them) the AI's live example anchor and the student's own live
  // pick — both need to render highlighted even though neither is persisted
  // yet (the example never is; the student's pick only becomes one on
  // confirm()).
  const spans = useMemo(() => {
    const extra: Anchor[] = [];
    if (loop.exampleAnchor) extra.push(loop.exampleAnchor);
    if (loop.studentAnchor) extra.push(loop.studentAnchor);
    return [...source.anchors, ...extra].map(anchorToSpan).filter((s): s is AnnotateSpan => s !== null);
  }, [source.anchors, loop.exampleAnchor, loop.studentAnchor]);

  return (
    <div className="mk-reading-room">
      <div className="mk-reading-room__coach">
        <button type="button" className="mk-reading-room__back" onClick={onBack}>
          <BackIcon />
          返回工作区
        </button>
        <div className="mk-reading-room__coach-lines">
          {loop.coachLines.map((line, i) => (
            <div key={i} className="mk-reading-room__coach-line">
              {line}
            </div>
          ))}
        </div>
        <form
          className="mk-reading-room__composer"
          onSubmit={(e) => {
            e.preventDefault();
            const text = draft.trim();
            if (!text) return;
            setDraft("");
            void loop.sendTurn(text);
          }}
        >
          <textarea
            className="mk-reading-room__composer-input"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="说说你读到这里的想法…"
            disabled={loop.status !== "idle"}
          />
          <button type="submit" className="mk-reading-room__composer-send" disabled={loop.status !== "idle"}>
            发送
          </button>
        </form>
      </div>
      <div className="mk-reading-room__article">
        <div className="mk-reading-room__article-inner">
          <div className="mk-reading-room__title">{source.title}</div>
          <Annotate
            blocks={source.blocks}
            state={{
              material_id: source.id,
              spans,
            }}
            activeSpanId={null}
            onSelectSpan={() => {}}
            selectMode={loop.status === "active" ? { dimension: loop.cardName, onCancel: loop.repick } : null}
            onCreateSpan={loop.pickSentence}
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
