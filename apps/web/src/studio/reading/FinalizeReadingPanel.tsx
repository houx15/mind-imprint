import type { TakeawayDraft } from "@mind-imprint/contracts";

// FinalizeReadingPanel — S2 · 完成这篇. A calm modal over the reading room
// (same backdrop/dialog idiom as LensLibrary), split in two exactly like the
// server split: the RECORD half (findings/credibility/keyQuotes) is her
// already-confirmed work re-assembled server-side — read-only, 克制, never
// re-decided here — while the SYNTHESIS half (new leads / proposal impact) is
// seeded by one isolated compose call and left fully editable: she authors it,
// 确认归纳 persists it. This component holds no fetch/save logic itself — the
// room owns the draft/loading/saving state and passes it down, mirroring
// LensLibrary's "caller decides when to close" contract.
export type FinalizeReadingPanelProps = {
  loading: boolean;
  draft: TakeawayDraft | null;
  leadsText: string;
  onLeadsChange: (v: string) => void;
  impactText: string;
  onImpactChange: (v: string) => void;
  saving: boolean;
  done: boolean;
  onConfirm: () => void;
  onClose: () => void;
};

export function FinalizeReadingPanel({
  loading,
  draft,
  leadsText,
  onLeadsChange,
  impactText,
  onImpactChange,
  saving,
  done,
  onConfirm,
  onClose,
}: FinalizeReadingPanelProps) {
  const record = draft?.record;
  return (
    <div className="mk-finalize-panel__backdrop" role="presentation" onClick={onClose}>
      <div
        className="mk-finalize-panel"
        role="dialog"
        aria-modal="true"
        aria-label="完成这篇"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="mk-finalize-panel__head">
          <div>
            <span className="mk-finalize-panel__kicker">完成这篇</span>
            <h2>把这篇的阅读成果归纳一下</h2>
          </div>
          <button type="button" className="mk-finalize-panel__close" aria-label="关闭" onClick={onClose}>
            ×
          </button>
        </header>

        <div className="mk-finalize-panel__body">
          {loading ? (
            <p className="mk-finalize-panel__loading">正在整理你的阅读发现…</p>
          ) : (
            <>
              <section className="mk-finalize-panel__record">
                <h3>你的阅读记录 · 只读</h3>
                <div className="mk-finalize-panel__field">
                  <span>发现</span>
                  {record && record.findings.length > 0 ? (
                    <ul>
                      {record.findings.map((f, i) => (
                        <li key={i}>{f}</li>
                      ))}
                    </ul>
                  ) : (
                    <p className="mk-finalize-panel__empty">还没有确认过的阅读发现——先完成一次透镜练习。</p>
                  )}
                </div>
                <div className="mk-finalize-panel__field">
                  <span>可信度</span>
                  <p>
                    {record?.credibility.verdict ? (
                      <>
                        <strong>{record.credibility.verdict}</strong> — {record.credibility.why}
                      </>
                    ) : (
                      <span className="mk-finalize-panel__empty">尚未评估</span>
                    )}
                  </p>
                </div>
                <div className="mk-finalize-panel__field">
                  <span>关键引句</span>
                  {record && record.keyQuotes.length > 0 ? (
                    <ul className="mk-finalize-panel__quotes">
                      {record.keyQuotes.map((q, i) => (
                        <li key={i}>
                          <blockquote>{q.quote}</blockquote>
                          {q.why && <p>{q.why}</p>}
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="mk-finalize-panel__empty">还没有引句。</p>
                  )}
                </div>
              </section>

              <section className="mk-finalize-panel__synthesis">
                <h3>你的归纳 · 可编辑</h3>
                <label className="mk-finalize-panel__field">
                  <span>新的线索</span>
                  <textarea
                    value={leadsText}
                    onChange={(e) => onLeadsChange(e.target.value)}
                    rows={3}
                    placeholder="这篇给你带来了什么新的线索？（一行一条）"
                    disabled={done}
                  />
                </label>
                <label className="mk-finalize-panel__field">
                  <span>对论点的影响</span>
                  <textarea
                    value={impactText}
                    onChange={(e) => onImpactChange(e.target.value)}
                    rows={3}
                    placeholder="这篇对你的论点有什么影响？"
                    disabled={done}
                  />
                </label>
              </section>
            </>
          )}
        </div>

        <footer className="mk-finalize-panel__footer">
          {done ? (
            <>
              <span className="mk-finalize-panel__done">已归纳 ✓</span>
              <button type="button" className="mk-finalize-panel__confirm" onClick={onClose}>
                关闭
              </button>
            </>
          ) : (
            <>
              <button type="button" className="mk-finalize-panel__cancel" onClick={onClose}>
                取消
              </button>
              <button
                type="button"
                className="mk-finalize-panel__confirm"
                onClick={onConfirm}
                disabled={loading || saving}
              >
                {saving ? "保存中…" : "确认归纳"}
              </button>
            </>
          )}
        </footer>
      </div>
    </div>
  );
}
