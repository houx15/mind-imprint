import { useEffect, useMemo, useRef, useState } from "react";
import type { BlockRendererProps, InteractiveHtmlBlock } from "../types";
import type { SliceEmitter } from "@mind-imprint/course-runtime";
import { PROTOCOL_NAME, PROTOCOL_VERSION, parseFrameMessage } from "./protocol";

/**
 * §9.5 / §17.11 / §20 — the sandboxed-iframe boundary for `interactiveHtml`
 * blocks. This is the ONLY place course-authored code runs, so it is the
 * tightest trust boundary in the runtime.
 *
 * The renderer loads one self-contained HTML file into a `sandbox="allow-scripts"`
 * iframe — scripts only, NO `allow-same-origin` (so the frame has an opaque
 * origin and cannot reach app cookies/storage), no forms/popups/top-navigation.
 * It mints a per-mount `sessionToken`, posts a handshake on load so the frame
 * echoes the token, and accepts a message ONLY after validating the source
 * Window, protocol name, version, session token, and known `type`. Everything
 * else is dropped (optionally surfaced via `onRejected`) — never an Event.
 */

interface HtmlMessageHandlerDeps {
  /** Returns the frame's `contentWindow`; a message's `source` must equal this. */
  getExpectedSource: () => Window | null;
  /** The per-mount minted token; the frame must echo it back. */
  sessionToken: string;
  block: InteractiveHtmlBlock;
  emit: SliceEmitter;
  /** Diagnostics-only sink for rejected messages. Default no-op at the callsite. */
  onRejected: (reason: string) => void;
  /** Fired once, after a valid completion under `interaction-complete`. */
  onCompleted?: () => void;
}

/**
 * Pure factory for the `(data, source)` message handler. Kept independent of the
 * `window` 'message' listener (which is a thin adapter passing `event.data,
 * event.source`) so every acceptance/rejection predicate is unit-testable —
 * jsdom cannot freely set `MessageEvent.source`, so we test this directly.
 *
 * Every predicate must pass or the message is DROPPED: `source` Window, then
 * protocol/version/token/type via {@link parseFrameMessage}. A valid message
 * becomes `interaction.<type>`; a valid `completed` (when the block's rule is
 * `interaction-complete`) additionally emits `block.completed`, exactly once.
 */
export function createHtmlMessageHandler(deps: HtmlMessageHandlerDeps): (data: unknown, source: unknown) => void {
  let completed = false;
  return (data, source) => {
    if (source !== deps.getExpectedSource()) {
      deps.onRejected("source");
      return;
    }
    const result = parseFrameMessage(data, {
      sessionToken: deps.sessionToken,
      expectedVersion: deps.block.protocolVersion,
    });
    if (!result.ok) {
      deps.onRejected(result.reason);
      return;
    }
    deps.emit(deps.block.id, `interaction.${result.type}`, result.payload);
    if (result.type === "completed" && deps.block.completion?.rule === "interaction-complete") {
      if (completed) return;
      completed = true;
      deps.emit(deps.block.id, "block.completed");
      deps.onCompleted?.();
    }
  };
}

const ASPECT_CSS: Record<InteractiveHtmlBlock["aspectRatio"], string> = {
  "1:1": "1 / 1",
  "4:3": "4 / 3",
};

const defaultTokenFactory = (): string => globalThis.crypto.randomUUID();

export interface HtmlInteractionRendererProps extends BlockRendererProps<InteractiveHtmlBlock> {
  /** Injected for determinism; default mints a random per-mount token. */
  tokenFactory?: () => string;
  /** Diagnostics sink for dropped messages; default no-op. */
  onRejected?: (reason: string) => void;
}

export const HtmlInteractionRenderer = ({
  block,
  assetResolver,
  visible,
  enabled,
  emit,
  tokenFactory = defaultTokenFactory,
  onRejected = () => {},
}: HtmlInteractionRendererProps) => {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);

  // Mint the token once per mount; a re-mount (new instance) mints a new token,
  // so tokens from a previous mount are dead.
  const sessionTokenRef = useRef<string | undefined>(undefined);
  if (sessionTokenRef.current === undefined) sessionTokenRef.current = tokenFactory();
  const sessionToken = sessionTokenRef.current;

  // P1-11 — the iframe `src` is captured ONCE at mount, not recomputed inline
  // from `assetResolver.resolve()` on every render. A signed-URL refresh
  // re-renders the whole tree; reloading an ACTIVE interactive-HTML frame on
  // an unrelated background refresh would destroy the interaction's internal
  // JS state (its own DOM/variables live inside the sandboxed document, which
  // a `src` change tears down and reloads from scratch). The src only
  // changes on an explicit load error (below) — and even then only if the
  // resolver actually returns something different.
  const [frameSrc, setFrameSrc] = useState(() => assetResolver.resolve(block.source));
  const frameSrcRef = useRef(frameSrc);
  frameSrcRef.current = frameSrc;

  // Latest emit/onRejected for the mount-stable handler.
  const emitRef = useRef(emit);
  emitRef.current = emit;
  const onRejectedRef = useRef(onRejected);
  onRejectedRef.current = onRejected;

  const handleMessage = useMemo(
    () =>
      createHtmlMessageHandler({
        getExpectedSource: () => iframeRef.current?.contentWindow ?? null,
        sessionToken,
        block,
        emit: (sourceId, type, payload) => emitRef.current(sourceId, type, payload),
        onRejected: (reason) => onRejectedRef.current(reason),
      }),
    [sessionToken, block],
  );

  useEffect(() => {
    const listener = (event: MessageEvent) => handleMessage(event.data, event.source);
    window.addEventListener("message", listener);
    return () => window.removeEventListener("message", listener);
  }, [handleMessage]);

  // P1-11 — an explicit load error is the ONE case that re-resolves the src
  // (recovering from an expired URL is worth the reload; an unrelated
  // background refresh is not). React does not wire a delegated `onError`
  // for `<iframe>` — only `onLoad` (confirmed in PdfRenderer, Slice 6 Task 1)
  // — so the native `error` event is bound directly on the element.
  useEffect(() => {
    const el = iframeRef.current;
    if (!el) return;
    const handleError = () => {
      const fresh = assetResolver.resolve(block.source);
      if (fresh !== frameSrcRef.current) setFrameSrc(fresh);
    };
    el.addEventListener("error", handleError);
    return () => el.removeEventListener("error", handleError);
  }, [assetResolver, block.source]);

  const handleLoad = () => {
    const win = iframeRef.current?.contentWindow;
    if (!win) return;
    // Handshake: the frame is opaque-origin, so targetOrigin must be "*". The
    // message carries only the token (no secrets) so the frame can echo it back.
    win.postMessage({ protocol: PROTOCOL_NAME, version: PROTOCOL_VERSION, sessionToken }, "*");
  };

  return (
    <div
      data-block-id={block.id}
      data-block-type="interactiveHtml"
      data-aspect-ratio={block.aspectRatio}
      hidden={!visible}
      aria-hidden={!visible}
      aria-disabled={!enabled}
      className="course-block course-block--interactive-html"
      style={{ aspectRatio: ASPECT_CSS[block.aspectRatio] }}
    >
      <iframe
        ref={iframeRef}
        className="course-interactive-html__frame"
        title={`interactive-${block.id}`}
        // §20: scripts only — opaque origin, no same-origin/forms/popups/top-nav
        // and no network affordances. Interactivity gating under enabled=false is
        // advisory only: the sandbox already isolates the frame.
        sandbox="allow-scripts"
        src={frameSrc}
        onLoad={handleLoad}
        style={{ width: "100%", height: "100%", border: "0" }}
      />
    </div>
  );
};
