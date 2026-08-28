import { useState } from "react";
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
 * ## Off by default, no fetch on mount
 *
 * Unlike `ReportPanel`, this never asks the server "is this already
 * shared?" — there is no such endpoint (Task 6 only gave us `shareReport`/
 * `unshareReport`), and starting closed is the safe default for a minor
 * publishing her own schoolwork: nothing is ever shown as shared unless
 * SHE, in this sitting, clicked the button that shares it.
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

export function SharePanel({ kind, atomId }: { kind: AtomKind; atomId: string }) {
  const [state, setState] = useState<ShareState>({ phase: "off" });
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function handleShare() {
    setError(null);
    setState({ phase: "sharing" });
    try {
      const { token } = await shareReport(kind, atomId);
      const url = `${window.location.origin}/s/${token}`;
      let qr: string | null = null;
      try {
        qr = await QRCode.toDataURL(url);
      } catch {
        qr = null; // link still works without the image
      }
      setState({ phase: "on", url, qr });
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
          这份报告现在任何拿到链接的人都能打开，不用登录。想收回的时候，点下面的「停止分享」就会立刻失效。
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
            alt="分享二维码，扫码可以直接打开这份报告"
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
        分享之后，任何拿到这个链接的人都能打开这份报告，不用登录也能看——如果你想收回，随时点「停止分享」就会立刻失效。
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
