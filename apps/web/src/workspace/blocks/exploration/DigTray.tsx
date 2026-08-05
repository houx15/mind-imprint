import type { DigCandidate } from "@mind-imprint/contracts";

// A6 · the dig tray. When the student taps 深挖 on a node, 印记 proposes related
// papers here — they live in a client-side tray, NOT on the map (铁律①: 印记
// 只推荐，学生确认才落到地图上). 采纳 is the explicit student action that adopts
// one under the dug node; 丢弃 removes it from the tray only. Never auto-adopts.
//
// Docked as a bottom panel so the tree above stays scrollable while the tray
// stays put. Plain copy only — no rabbit metaphor in any rendered string.

export function trayKey(c: DigCandidate): string {
  return c.url || c.doi || c.title;
}

export type DigTrayProps = {
  // The text of the node the student dug from — shown so the tray reads as
  // "papers related to THIS question/paper", not a context-free list.
  targetText: string;
  candidates: DigCandidate[];
  digging: boolean;
  error: boolean;
  // Which candidates are mid-adopt (keyed by trayKey) — disables their 采纳.
  adopting: Set<string>;
  onAdopt: (c: DigCandidate) => void;
  onDiscard: (c: DigCandidate) => void;
  onClose: () => void;
};

export function DigTray({ targetText, candidates, digging, error, adopting, onAdopt, onDiscard, onClose }: DigTrayProps) {
  return (
    <div className="flex-none border-t border-mk-border bg-mk-surface px-6 py-4 shadow-[0_-8px_24px_rgba(28,35,51,0.08)]">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-[13.5px] font-bold text-mk-ink">刚挖到这些论文</h3>
          <p className="mt-0.5 truncate text-[12px] text-mk-muted-2">
            顺着「{targetText}」找到的相关论文——挑你觉得有用的采纳，其余丢弃就好
          </p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="flex-none rounded-full px-2 py-0.5 text-[13px] leading-none text-mk-muted-2 hover:text-mk-ink"
        >
          收起
        </button>
      </div>

      {digging ? (
        <p className="py-3 text-[12.5px] text-mk-muted-2">印记在找相关论文……</p>
      ) : error ? (
        <p className="py-3 text-[12.5px] font-semibold text-mk-accent">刚才没接上，再试一次？</p>
      ) : candidates.length === 0 ? (
        <p className="py-3 text-[12.5px] text-mk-muted-2">这次没找到相关论文——换一条问题或论文再挖一次。</p>
      ) : (
        <div className="flex max-h-64 flex-col gap-2 overflow-y-auto pr-1">
          {candidates.map((c) => {
            const key = trayKey(c);
            const meta = [c.authors, c.year, c.journal].map((s) => s?.trim()).filter(Boolean).join(" · ");
            const busy = adopting.has(key);
            return (
              <div key={key} className="rounded-mk border border-mk-border bg-mk-bg/50 p-3">
                <p className="text-[13px] font-semibold leading-snug text-mk-ink">{c.title}</p>
                {meta && <p className="mt-0.5 text-[11.5px] text-mk-muted-2">{meta}</p>}
                {c.abstract?.trim() && (
                  <p className="mt-1.5 max-h-16 overflow-y-auto text-[12px] leading-relaxed text-mk-muted">{c.abstract}</p>
                )}
                <div className="mt-2 flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => onAdopt(c)}
                    disabled={busy}
                    className="rounded-mk bg-mk-primary px-3 py-1 text-[12px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
                  >
                    {busy ? "采纳中…" : "采纳"}
                  </button>
                  <button
                    type="button"
                    onClick={() => onDiscard(c)}
                    disabled={busy}
                    className="rounded-mk border border-mk-border bg-mk-surface px-3 py-1 text-[12px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-50"
                  >
                    丢弃
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
