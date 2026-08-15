import { useCallback, useEffect, useRef } from "react";
import type { BlockRenderer, VideoBlock } from "../types";
import { useVideoEngine } from "../../media/videoEngine";
import { useMediaHandleRegistry } from "../../media/mediaRegistry";

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
 */
export const VideoRenderer: BlockRenderer<VideoBlock> = ({ block, assetResolver, visible, emit }) => {
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

  const maybeComplete = useCallback(() => {
    if (completedRef.current) return;
    if (!videoEndedRef.current) return;
    if (gated && !requiredCuesCompleteRef.current) return;
    if (!block.completion) return;
    completedRef.current = true;
    emitRef.current(block.id, "block.completed");
  }, [block.completion, block.id, gated]);

  const play = useCallback(() => {
    engine.play();
    emitRef.current(block.id, "video.started");
  }, [engine, block.id]);

  const pause = useCallback(() => {
    engine.pause();
    emitRef.current(block.id, "video.paused");
  }, [engine, block.id]);

  const reset = useCallback(() => {
    engine.reset();
    videoEndedRef.current = false;
    completedRef.current = false;
  }, [engine]);

  // Register the media handle so play/pause/reset effects reach this element.
  useEffect(() => {
    if (!registry) return;
    return registry.register(block.id, { play, pause, reset });
  }, [registry, block.id, play, pause, reset]);

  // Subscribe to the engine's ended event.
  useEffect(() => {
    return engine.onEnded(() => {
      videoEndedRef.current = true;
      emitRef.current(block.id, "video.ended");
      maybeComplete();
    });
  }, [engine, block.id, maybeComplete]);

  return (
    <div
      data-block-id={block.id}
      data-block-type="video"
      hidden={!visible}
      aria-hidden={!visible}
      className="course-block course-block--video"
    >
      <video
        ref={videoRef}
        className="course-video__player"
        controls
        src={assetResolver.resolve(block.source)}
        poster={block.poster ? assetResolver.resolve(block.poster) : undefined}
      >
        {block.captions ? (
          <track kind="captions" src={assetResolver.resolve(block.captions)} default />
        ) : null}
      </video>
      <div className="course-video__controls">
        <button type="button" className="course-video__play" onClick={play}>
          播放
        </button>
        <button type="button" className="course-video__pause" onClick={pause}>
          暂停
        </button>
      </div>
    </div>
  );
};
