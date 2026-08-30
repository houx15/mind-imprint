import { useEffect, useState } from "react";
import QRCode from "qrcode";
import { shareReport, unshareReport, type AtomKind } from "../api/reports";

/**
 * SharePanel — the opt-in that lets her show someone else the report she
 * just got: 生成分享链接 mints a public link and a QR code for it; 停止分享
 * takes both back immediately. Mounted inside `ReportPanel`, below the
 * report itself.
 *
 * ## The URL is built here, never trusted from the server
 *
 * `shareReport` returns both `token` and `url`. This component uses ONLY the
 * token — the URL is always `${window.location.origin}/s/${token}`, built
 * client-side. The server's own `url` is deliberately ignored.
 *
 * Why: the server infers that field from the request's `Origin` header,
 * falling back to a configured CORS origin, then to the request host.
 * Behind a proxy that strips `Origin`, that inference can produce a link
 * that doesn't resolve — and for a link she is about to hand to someone
 * else, a broken one is worse than none, because she finds out only after
 * she's already sent it. The browser always knows its own origin correctly,
 * so building the URL here removes that failure class outright rather than
 * testing around it.
 *
 * ## F2: starts from whatever is already true, not always "off"
 *
 * `ReportPanel` now reads `shareToken` off the same GET .../report call
 * that fetches the report (see api/reports.ts's `getReportEnvelope`) and
 * passes it down as `initialShareToken`. Before this fix this panel always
 * started at {phase:"off"} regardless of server state, because there was no
 * way to ask "is this already shared?" — so a student who shared, closed
 * the tab, and came back saw the OFF state's copy describe as hypothetical
 * ("分享之后，任何拿到这个链接的人都能打开这份报告") something that was
 * currently true, with no visible 停止分享 at all. Starting from
 * `initialShareToken` when it is present fixes that; starting at
 * {phase:"off"} when it is absent (or omitted, e.g. in isolation/tests) is
 * still the safe default for a minor's own schoolwork: nothing is ever
 * shown as shared unless the server itself says a token already exists.
 *
 * ## Copy
 *
 * The off-state line names the real consequence plainly — anyone with the
 * link can open it without signing in, and it comes down the moment she
 * asks — the way a teacher explains a real consequence, not a consent
 * dialog's boilerplate and not a cheerful "great, let's share!".
 */
type ShareState =
  | { phase: "off" }
  | { phase: "sharing" }
  | { phase: "on"; url: string; qr: string | null }
  | { phase: "unsharing"; url: string; qr: string | null };

/** The one place a share token turns into the student-facing link — used by
 *  both `handleShare` (a freshly minted token) and the mount-time restore of
 *  an `initialShareToken`, so the two paths can never disagree on the URL
 *  shape. See the file comment: always built from the browser's own origin,
 *  never trusted from the server. */
function buildShareUrl(token: string): string {
  return `${window.location.origin}/s/${token}`;
}

export function SharePanel({
  kind,
  atomId,
  initialShareToken = null,
  onSharedChange,
}: {
  kind: AtomKind;
  atomId: string;
  /** F2: a token already minted server-side, e.g. from a previous sitting —
   *  when present, the panel starts at {phase:"on"} instead of "off". */
  initialShareToken?: string | null;
  /** Fired with the token whenever a link starts being live, and with `null`
   *  when it stops. The parent keeps it as the authoritative `initialShareToken`
   *  so a panel that is unmounted and remounted (the action bar toggles it) never
   *  reopens holding a token she has already revoked. Only ever called for a
   *  CONFIRMED server outcome, never for the optimistic in-flight phases. */
  onSharedChange?: (token: string | null) => void;
}) {
  /**
   * What the link actually opens, in her words.
   *
   * On a writing the shared page LEADS WITH HER PIECE and carries the record
   * underneath (see `ReportView`'s `Piece`), so calling the link 「这份报告」
   * described the wrong thing — she is sending someone her article. The
   * copy has to match what the reader will see, or 分享 reads as a chore
   * rather than as something she'd want to do.
   */
  const shared = kind === "writing" ? "这篇文章" : "这份报告";

  const [state, setState] = useState<ShareState>(() =>
    initialShareToken ? { phase: "on", url: buildShareUrl(initialShareToken), qr: null } : { phase: "off" },
  );
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Mount-only: fill in the QR image for a share that was ALREADY live when
  // this panel mounted (initialShareToken). handleShare below generates its
  // own QR inline as part of minting a NEW share — this effect only covers
  // the "reopened on an already-shared report" path, and runs once, so it
  // never fights with handleShare's own qr write.
  useEffect(() => {
    if (!initialShareToken) return;
    let cancelled = false;
    QRCode.toDataURL(buildShareUrl(initialShareToken))
      .then((qr) => {
        if (!cancelled) setState((s) => (s.phase === "on" ? { ...s, qr } : s));
      })
      .catch(() => {
        /* link still works without the image */
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleShare() {
    setError(null);
    setState({ phase: "sharing" });
    try {
      const { token } = await shareReport(kind, atomId);
      const url = buildShareUrl(token);
      let qr: string | null = null;
      try {
        qr = await QRCode.toDataURL(url);
      } catch {
        qr = null; // link still works without the image
      }
      setState({ phase: "on", url, qr });
      onSharedChange?.(token);
    } catch {
      setState({ phase: "off" });
      setError("刚才没能生成链接，你可以再试一次。");
    }
  }

  async function handleUnshare() {
    if (state.phase !== "on") return;
    setError(null);
    setState({ phase: "unsharing", url: state.url, qr: state.qr });
    try {
      await unshareReport(kind, atomId);
      setCopied(false);
      setState({ phase: "off" });
      onSharedChange?.(null);
    } catch {
      setState({ phase: "on", url: state.url, qr: state.qr });
      setError("刚才没能停止分享，你可以再试一次。");
    }
  }

  async function handleCopy() {
    if (state.phase !== "on") return;
    try {
      await navigator.clipboard.writeText(state.url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable — the URL is still visible to copy by hand */
    }
  }

  if (state.phase === "on" || state.phase === "unsharing") {
    return (
      <section className="flex flex-col gap-4 rounded-mk-lg border border-mk-border bg-mk-surface p-6">
        <p className="text-mk-small text-mk-muted">
          {shared}现在任何拿到链接的人都能打开，不用登录。想收回的时候，点下面的「停止分享」就会立刻失效。
        </p>

        <div className="flex items-center gap-2">
          <input
            readOnly
            value={state.url}
            aria-label="分享链接"
            className="min-w-0 flex-1 rounded-mk-sm border border-mk-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink"
          />
          <button
            type="button"
            onClick={handleCopy}
            className="shrink-0 rounded-mk-full border border-mk-accent-200 px-3 py-2 text-mk-small text-mk-accent-700 hover:bg-mk-accent-50"
          >
            {copied ? "已复制" : "复制"}
          </button>
        </div>

        {state.qr && (
          <img
            src={state.qr}
            alt={`分享二维码，扫码可以直接打开${shared}`}
            className="h-32 w-32 self-start rounded-mk-sm border border-mk-border"
          />
        )}

        <button
          type="button"
          onClick={handleUnshare}
          disabled={state.phase === "unsharing"}
          className="w-fit rounded-mk-full px-4 py-2 text-mk-small text-mk-danger disabled:cursor-not-allowed disabled:opacity-60"
          style={{ background: "var(--mk-danger-bg)" }}
        >
          {state.phase === "unsharing" ? "正在停止…" : "停止分享"}
        </button>

        {error && (
          <p role="alert" className="text-mk-small text-mk-danger">
            {error}
          </p>
        )}
      </section>
    );
  }

  return (
    <section className="flex flex-col gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-6">
      <p className="text-mk-small text-mk-muted">
        分享之后，任何拿到这个链接的人都能打开{shared}，不用登录也能看——如果你想收回，随时点「停止分享」就会立刻失效。
      </p>
      <button
        type="button"
        onClick={handleShare}
        disabled={state.phase === "sharing"}
        className="w-fit rounded-mk-full px-4 py-2 text-mk-small text-white disabled:cursor-not-allowed disabled:opacity-60"
        style={{ background: "var(--mk-accent-500)" }}
      >
        {state.phase === "sharing" ? "生成中…" : "生成分享链接"}
      </button>
      {error && (
        <p role="alert" className="text-mk-small text-mk-danger">
          {error}
        </p>
      )}
    </section>
  );
}
