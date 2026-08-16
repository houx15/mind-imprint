import { useCallback, useEffect, useRef, useState } from "react";
import type { VideoPositionPayload } from "@mind-imprint/course-runtime";
import type { BlockRenderer, VideoBlock } from "../types";
import { useVideoEngine } from "../../media/videoEngine";
import { useMediaHandleRegistry } from "../../media/mediaRegistry";
import { VideoInteractionController } from "./VideoInteractionController";

/**
 * §9.4 / §17.10 — the accessible video player. Renders a `<video>` (poster +
 * WebVTT caption track, resolved through the asset resolver) and drives playback
 * through the injectable {@link VideoEngine} seam so tests stay deterministic
 * under jsdom.
 *
 * The Slice Workflow treats the video as ONE component: the renderer registers a
 * media handle (`play`/`pause`/`reset`) so `playBlock`/`pauseBlock`/`resetBlock`
 * effects reach the element, emits `video.started`/`video.paused`/`video.ended`,
 * and emits `block.completed` only when its configured completion rule is met.
 * Under `video-ended-and-interactions-completed`, completion is gated on all
 * required timeline cues completing — reported up by {@link VideoInteractionController}.
 *
 * P2-03: `video.started`/`video.paused` are sourced from the engine's
 * `onPlay`/`onPause` — the native DOM `play`/`pause` events fire regardless of
 * WHO triggered playback (the custom buttons below, a workflow `playBlock`
 * effect via the media handle, or the learner using the native
 * `<video controls>` UI directly), so this is the single unified source
 * instead of only the button handlers observing themselves. `enabled` is
 * honored (hides native controls, disables the custom buttons); `reset` is
 * real (clears the video AND the cue controller's fired/completed/gate
 * state); a `play()` rejection (autoplay policy or otherwise) surfaces a
 * learner-recoverable retry affordance instead of leaving the workflow
 * silently stuck.
 */
export const VideoRenderer: BlockRenderer<VideoBlock> = ({ block, assetResolver, visible, enabled, emit }) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const getEl = useCallback(() => videoRef.current, []);
  const engine = useVideoEngine(getEl);
  const registry = useMediaHandleRegistry();

  // Latest emit for the mount-once ended subscription.
  const emitRef = useRef(emit);
  emitRef.current = emit;

  const gated = block.completion?.rule === "video-ended-and-interactions-completed";
  const videoEndedRef = useRef(false);
  const requiredCuesCompleteRef = useRef(!gated);
  const completedRef = useRef(false);
  const [cueResetSignal, setCueResetSignal] = useState(0);
  const [playError, setPlayError] = useState(false);

  const maybeComplete = useCallback(() => {
    if (completedRef.current) return;
    if (!videoEndedRef.current) return;
    if (gated && !requiredCuesCompleteRef.current) return;
    if (!block.completion) return;
    completedRef.current = true;
    emitRef.current(block.id, "block.completed");
  }, [block.completion, block.id, gated]);

  const play = useCallback(() => {
    if (!enabled) return;
    setPlayError(false);
    engine.play();
  }, [engine, enabled]);

  const pause = useCallback(() => {
    if (!enabled) return;
    engine.pause();
  }, [engine, enabled]);

  const reset = useCallback(() => {
    engine.reset();
    videoEndedRef.current = false;
    completedRef.current = false;
    requiredCuesCompleteRef.current = !gated;
    setPlayError(false);
    setCueResetSignal((n) => n + 1);
  }, [engine, gated]);

  // Register the media handle so play/pause/reset effects reach this element.
  useEffect(() => {
    if (!registry) return;
    return registry.register(block.id, { play, pause, reset });
  }, [registry, block.id, play, pause, reset]);

  // Native play/pause → the single source of `video.started`/`video.paused`.
  useEffect(() => {
    return engine.onPlay(() => {
      emitRef.current(block.id, "video.started");
    });
  }, [engine, block.id]);

  useEffect(() => {
    return engine.onPause(() => {
      const payload: VideoPositionPayload = { positionSeconds: engine.currentTime() };
      emitRef.current(block.id, "video.paused", payload);
    });
  }, [engine, block.id]);

  // Surface a play() rejection (e.g. autoplay policy) as a recoverable state
  // instead of leaving a gated workflow silently waiting for `video.started`.
  useEffect(() => {
    return engine.onPlayError(() => {
      setPlayError(true);
    });
  }, [engine]);

  // Subscribe to the engine's ended event.
  useEffect(() => {
    return engine.onEnded(() => {
      videoEndedRef.current = true;
      const payload: VideoPositionPayload = { positionSeconds: engine.currentTime() };
      emitRef.current(block.id, "video.ended", payload);
      maybeComplete();
    });
  }, [engine, block.id, maybeComplete]);

  // The cue timeline reports when all required cues have completed (gated rule).
  const onRequiredCuesComplete = useCallback(() => {
    requiredCuesCompleteRef.current = true;
    maybeComplete();
  }, [maybeComplete]);

  return (
    <div
      data-block-id={block.id}
      data-block-type="video"
      hidden={!visible}
      aria-hidden={!visible}
      aria-disabled={!enabled}
      className="course-block course-block--video"
    >
      <video
        ref={videoRef}
        className="course-video__player"
        controls={enabled}
        tabIndex={enabled ? undefined : -1}
        src={assetResolver.resolve(block.source)}
        poster={block.poster ? assetResolver.resolve(block.poster) : undefined}
      >
        {block.captions ? (
          <track kind="captions" src={assetResolver.resolve(block.captions)} default />
        ) : null}
      </video>
      <div className="course-video__controls">
        <button type="button" className="course-video__play" disabled={!enabled} onClick={play}>
          播放
        </button>
        <button type="button" className="course-video__pause" disabled={!enabled} onClick={pause}>
          暂停
        </button>
      </div>
      {playError ? (
        <p className="course-video__play-error" role="alert">
          播放未能开始，可能是浏览器阻止了自动播放。
          <button type="button" className="course-video__play-retry" onClick={play}>
            重试播放
          </button>
        </p>
      ) : null}
      {block.interaction ? (
        <VideoInteractionController
          block={block}
          engine={engine}
          assetResolver={assetResolver}
          emit={emit}
          onRequiredCuesComplete={onRequiredCuesComplete}
          visible={visible}
          resetSignal={cueResetSignal}
        />
      ) : null}
    </div>
  );
};
