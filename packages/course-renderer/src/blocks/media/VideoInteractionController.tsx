import { createContext, useContext, useEffect, useMemo, useRef, useState } from "react";
import {
  validateVideoInteraction,
  type VideoInteractionCue,
  type VideoInteractionDocument,
} from "@mind-imprint/course-contract";
import type { BlockSessionState } from "@mind-imprint/course-contract";
import type { AssetResolver, SliceEmitter } from "@mind-imprint/course-runtime";
import type { FillBlankBlock, SingleChoiceBlock, VideoBlock } from "../types";
import type { VideoEngine } from "../../media/videoEngine";
import { SingleChoiceRenderer } from "../assessment/SingleChoiceRenderer";
import { FillBlankRenderer } from "../assessment/FillBlankRenderer";

/**
 * The host resolves a Video Block's `interaction.source` to its parsed
 * {@link VideoInteractionDocument}. Injected so previews, drafts, and tests can
 * supply the JSON directly; returns `null` when no document is available.
 */
export type InteractionLoader = (source: string) => VideoInteractionDocument | null;

const InteractionLoaderContext = createContext<InteractionLoader | null>(null);
export const InteractionLoaderProvider = InteractionLoaderContext.Provider;
export function useInteractionLoader(): InteractionLoader | null {
  return useContext(InteractionLoaderContext);
}

export interface VideoInteractionControllerProps {
  block: VideoBlock;
  engine: VideoEngine;
  assetResolver: AssetResolver;
  emit: SliceEmitter;
  /** Called once every required cue has completed (gates the gated completion rule). */
  onRequiredCuesComplete: () => void;
  /** Loads the interaction document; defaults to the injected {@link InteractionLoader}. */
  loadDocument?: () => VideoInteractionDocument | null;
}

const CUE_STATE: BlockSessionState = { visible: true, enabled: true, completed: false, attempts: 0 };

/**
 * §14 / §17.10 — a declarative media timeline (NOT a second workflow engine). It
 * watches the engine's reported `currentTime`, pauses at each cue, renders the
 * cue activity by reusing the Slice 4 assessment renderers, and resumes on
 * completion. Cue firing is idempotent (each cue shows once) and tolerates seeks
 * (a completed cue never re-fires). It reports up when all required cues are
 * done so {@link VideoRenderer} can gate `block.completed`.
 */
export function VideoInteractionController({
  block,
  engine,
  assetResolver,
  emit,
  onRequiredCuesComplete,
  loadDocument,
}: VideoInteractionControllerProps) {
  const injectedLoader = useInteractionLoader();

  const document = useMemo<VideoInteractionDocument | null>(() => {
    const load = loadDocument ?? (block.interaction && injectedLoader ? () => injectedLoader(block.interaction!.source) : null);
    const doc = load ? load() : null;
    if (!doc) return null;
    // Only drive a document that validates against the owning block (§14).
    return validateVideoInteraction(doc, block).length === 0 ? doc : null;
    // block.interaction.source is stable for a mounted controller.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [block]);

  const cues = document?.video.cues ?? [];
  const requiredIds = useMemo(() => cues.filter((c) => c.required).map((c) => c.id), [cues]);

  const firedRef = useRef<Set<string>>(new Set());
  const completedRef = useRef<Set<string>>(new Set());
  const requiredDoneRef = useRef(false);
  const [activeCue, setActiveCue] = useState<VideoInteractionCue | null>(null);
  // Mirror activeCue into a ref so the time listener always sees the latest value.
  const activeCueRef = useRef<VideoInteractionCue | null>(null);
  activeCueRef.current = activeCue;

  const checkRequiredDone = () => {
    if (requiredDoneRef.current) return;
    if (requiredIds.every((id) => completedRef.current.has(id))) {
      requiredDoneRef.current = true;
      onRequiredCuesComplete();
    }
  };

  // No required cues at all ⇒ the gate is satisfied immediately.
  useEffect(() => {
    if (document) checkRequiredDone();
    // Run once after the document resolves.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [document]);

  // Watch reported time; fire the first not-yet-shown cue whose time has passed.
  useEffect(() => {
    if (!document) return;
    return engine.onTimeUpdate((t) => {
      if (activeCueRef.current) return; // one cue at a time
      for (const cue of cues) {
        if (firedRef.current.has(cue.id)) continue;
        if (t >= cue.atSeconds) {
          firedRef.current.add(cue.id);
          if (cue.pauseVideo) engine.pause();
          emit(block.id, "video.interaction.shown", { interactionId: cue.id });
          setActiveCue(cue);
          break;
        }
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [document, engine]);

  const completeCue = (cue: VideoInteractionCue) => {
    if (completedRef.current.has(cue.id)) return;
    completedRef.current.add(cue.id);
    emit(block.id, "video.interaction.completed", { interactionId: cue.id });
    setActiveCue(null);
    if (cue.pauseVideo) engine.play(); // resume
    checkRequiredDone();
  };

  if (!activeCue) return null;

  // A local emitter isolates the cue's answer events from the Slice Workflow
  // (§14): only the inner `block.completed` becomes a cue completion.
  const cueEmit: SliceEmitter = (_sourceId, type) => {
    if (type === "block.completed") completeCue(activeCue);
  };

  return (
    <div className="course-video__cue" data-cue-id={activeCue.id} role="group" aria-label={activeCue.prompt}>
      <p className="course-video__cue-prompt">{activeCue.prompt}</p>
      {renderCueActivity(activeCue, assetResolver, cueEmit)}
    </div>
  );
}

/** Renders the cue's activity by reusing the Slice 4 assessment renderers. */
function renderCueActivity(cue: VideoInteractionCue, assetResolver: AssetResolver, emit: SliceEmitter) {
  if (cue.activity.type === "singleChoice") {
    const block: SingleChoiceBlock = {
      id: cue.id,
      type: "singleChoice",
      prompt: cue.prompt,
      options: cue.activity.options,
      assessment: cue.activity.assessment,
      completion: cue.activity.completion,
    };
    return (
      <SingleChoiceRenderer block={block} assetResolver={assetResolver} state={CUE_STATE} visible enabled emit={emit} />
    );
  }
  const block: FillBlankBlock = {
    id: cue.id,
    type: "fillBlank",
    prompt: cue.prompt,
    assessment: cue.activity.assessment,
    completion: cue.activity.completion,
  };
  return <FillBlankRenderer block={block} assetResolver={assetResolver} state={CUE_STATE} visible enabled emit={emit} />;
}
