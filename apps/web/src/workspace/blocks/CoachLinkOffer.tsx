import { Icon } from "../Icon";

// CoachLinkOffer — the link-in-coach → resource bridge chip (2026-07-30). When a
// student drops a URL into the coach, the reply carries a linkOffer; this renders
// a gentle, dismissable chip offering to add it to the library or read it
// together. NOTHING fetches or opens on render — the tap is the student's
// confirmation (铁律: triggering is automatic, opening is confirmed). 跳过 hides
// it locally; a per-turn, spend-free detection has nothing to "keep offering".
export type LinkOfferStatus = "idle" | "adding" | "added";

export function CoachLinkOffer({
  url,
  status,
  onAdd,
  onReadTogether,
  onDismiss,
}: {
  url: string;
  status: LinkOfferStatus;
  onAdd: () => void;
  onReadTogether: () => void;
  onDismiss: () => void;
}) {
  const busy = status === "adding";
  const added = status === "added";
  // A readable label — the host, so the chip isn't a wall of query string.
  let host = url;
  try {
    host = new URL(url).host.replace(/^www\./, "");
  } catch {
    /* keep the raw url */
  }
  return (
    <div className="rounded-mk-lg border border-mk-accent/30 bg-mk-accent-50 px-3.5 py-3 text-[13px] text-mk-ink">
      <div className="flex items-start gap-2">
        <span className="mt-0.5 flex-none text-mk-accent">
          <Icon name="reading" size={15} />
        </span>
        <div className="min-w-0">
          <p className="font-semibold leading-snug">看起来你贴了一篇资料</p>
          <p className="mt-0.5 truncate text-[12px] text-mk-muted" title={url}>
            {host}
          </p>
        </div>
      </div>
      <div className="mt-2.5 flex flex-wrap items-center gap-2">
        {added ? (
          <span className="inline-flex items-center gap-1 rounded-mk-full bg-mk-accent/10 px-3 py-1.5 text-[12px] font-bold text-mk-accent">
            已加入文献库 ✓
          </span>
        ) : (
          <button
            type="button"
            onClick={onAdd}
            disabled={busy}
            className="inline-flex items-center gap-1 rounded-mk-full border border-mk-accent bg-mk-surface px-3.5 py-1.5 text-[12px] font-bold text-mk-accent transition hover:bg-mk-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
          >
            <Icon name="plan" size={13} /> {busy ? "加入中…" : "加入文献库"}
          </button>
        )}
        <button
          type="button"
          onClick={onReadTogether}
          disabled={busy}
          className="inline-flex items-center gap-1 rounded-mk-full bg-mk-accent px-3.5 py-1.5 text-[12px] font-bold text-white transition hover:bg-mk-accent-600 disabled:cursor-not-allowed disabled:opacity-60"
        >
          一起读这篇 <Icon name="arrow" size={13} />
        </button>
        <button
          type="button"
          onClick={onDismiss}
          className="rounded-mk-full px-2.5 py-1.5 text-[12px] font-semibold text-mk-faint transition hover:text-mk-muted"
        >
          跳过
        </button>
      </div>
    </div>
  );
}
