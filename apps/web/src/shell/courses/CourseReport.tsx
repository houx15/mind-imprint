import { useEffect, useState } from "react";
import type { CourseReport as CourseReportT, CardCatalogEntry } from "@mind-imprint/contracts";
import { api } from "../../api";

function Stat({ value, label, color }: { value: string; label: string; color?: string }) {
  return (
    <div style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 14, padding: "16px 14px", textAlign: "center" }}>
      <div style={{ fontSize: 22, fontWeight: 800, color: color ?? "var(--mk-ink)" }}>{value}</div>
      <div style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 600, marginTop: 3 }}>{label}</div>
    </div>
  );
}

// mm:ss-ish humanization: under a minute reads in seconds, otherwise minutes
// (+ leftover seconds when non-zero) — matches the brief's "X 分 Y 秒" / "约 X
// 分钟" examples without inventing a third format.
export function formatSpent(secondsSpent: number): string {
  if (secondsSpent <= 0) return "0 分钟";
  if (secondsSpent < 60) return `${secondsSpent} 秒`;
  const m = Math.floor(secondsSpent / 60);
  const s = secondsSpent % 60;
  return s === 0 ? `约 ${m} 分钟` : `${m} 分 ${s} 秒`;
}

// CardCover renders the card's OSS cover art (signed coverUrl from the
// catalog), falling back to a text face — a colored block with the card's
// English name — when there's no art or the image fails to load.
function CardCover({ info }: { info?: CardCatalogEntry }) {
  const [failed, setFailed] = useState(false);
  const showImg = Boolean(info?.coverUrl) && !failed;
  return (
    <div style={{ flex: "none", width: 74, height: 98, borderRadius: 11, overflow: "hidden", border: "1px solid var(--mk-border)", background: "linear-gradient(150deg,var(--mk-accent-500) 0%,var(--mk-accent-600) 100%)", display: "flex", alignItems: "center", justifyContent: "center" }}>
      {showImg ? (
        <img src={info!.coverUrl} alt={info?.name ?? "工具卡"} onError={() => setFailed(true)} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
      ) : (
        <span style={{ padding: "0 6px", textAlign: "center", fontSize: 10.5, fontWeight: 700, color: "rgba(255,255,255,.75)", lineHeight: 1.35, letterSpacing: ".02em" }}>{info?.nameEn || info?.name || "工具卡"}</span>
      )}
    </div>
  );
}

// A tool card in the report now shows the full picture, not just a name:
// cover art · 中文名 (+ English + category) · purpose (它做什么) · 何时使用
// (阶段 chips + a concrete example). All fields come from the shared card
// catalog (CardCatalogEntry); everything after the name degrades gracefully
// when the catalog fetch failed (info undefined → name falls back to the id).
function ToolCardDetail({ cardId, info }: { cardId: string; info?: CardCatalogEntry }) {
  const name = info?.name ?? cardId;
  return (
    <div style={{ display: "flex", gap: 14, border: "1px solid var(--mk-border)", borderRadius: 14, padding: 14, background: "var(--mk-paper)" }}>
      <CardCover info={info} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 8, flexWrap: "wrap" }}>
          <span style={{ fontSize: 15.5, fontWeight: 800, color: "var(--mk-ink)" }}>{name}</span>
          {info?.nameEn && <span style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 600 }}>{info.nameEn}</span>}
          {info?.category && (
            <span style={{ fontSize: 10.5, fontWeight: 700, color: "var(--mk-accent-500)", background: "var(--mk-accent-50)", padding: "2px 8px", borderRadius: 999 }}>{info.category}</span>
          )}
        </div>
        {info?.purpose && (
          <div style={{ fontSize: 13.5, color: "var(--mk-ink)", lineHeight: 1.65, marginTop: 8 }}>{info.purpose}</div>
        )}
        {(info?.whenToUse || (info?.stages && info.stages.length > 0) || info?.example) && (
          <div style={{ marginTop: 10, borderTop: "1px dashed var(--mk-border)", paddingTop: 10 }}>
            <div style={{ fontSize: 11, fontWeight: 700, color: "var(--mk-muted)", letterSpacing: ".04em", marginBottom: 6 }}>何时使用</div>
            {info?.whenToUse && (
              <div style={{ fontSize: 13, color: "var(--mk-ink)", lineHeight: 1.6, marginBottom: (info?.stages && info.stages.length > 0) || info?.example ? 7 : 0 }}>{info.whenToUse}</div>
            )}
            {info?.stages && info.stages.length > 0 && (
              <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginBottom: info?.example ? 7 : 0 }}>
                {info.stages.map((stage, i) => (
                  <span key={`${stage}-${i}`} style={{ fontSize: 12, fontWeight: 600, color: "var(--mk-matcha-fg)", background: "var(--mk-matcha-bg)", padding: "3px 10px", borderRadius: 8 }}>{stage}</span>
                ))}
              </div>
            )}
            {info?.example && (
              <div style={{ fontSize: 12.5, color: "var(--mk-secondary)", lineHeight: 1.6 }}>例：{info.example}</div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// courseId is really the course slug (CourseSummary/CoursePlayerPayload no
// longer have a numeric/string `id` — the slug is the sole identifier). The
// prop is named courseId to stay compatible with CoursesContainer's existing
// call site (Task 11 keeps that wiring, only the internal data model
// changes).
export function CourseReport({ courseId, onBackToCourses, onGoPortal, onRestart }: { courseId: string; onBackToCourses: () => void; onGoPortal: () => void; onRestart?: () => void }) {
  const [report, setReport] = useState<CourseReportT | null>(null);
  const [error, setError] = useState(false);
  const [cardInfo, setCardInfo] = useState<Record<string, CardCatalogEntry>>({});

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const r = await api.getCourseReport(courseId);
        if (cancelled) return;
        setReport(r);
      } catch {
        if (!cancelled) setError(true);
        return;
      }
      // Card details are a best-effort lookup — the report itself must still
      // render (id-fallback names) if the catalog fetch fails.
      try {
        const cat = await api.getCardsCatalog();
        if (cancelled) return;
        const m: Record<string, CardCatalogEntry> = {};
        for (const c of cat.cards as CardCatalogEntry[]) m[c.cardId] = c;
        setCardInfo(m);
      } catch { /* cards fall back to id-only name, no cover/purpose */ }
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  if (error) return <div style={{ padding: 40, color: "var(--mk-danger)" }}>学习报告暂时没能生成，稍后再看看。</div>;
  if (!report) return <div style={{ padding: 40, color: "var(--mk-muted)" }}>正在整理你的学习报告…</div>;

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "var(--mk-paper)" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        {/* hero */}
        <div style={{ background: "linear-gradient(135deg,var(--mk-accent-500) 0%,var(--mk-accent-600) 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(234,81,64,.20)" }}>
          <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="var(--mk-surface)" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "rgba(255,255,255,.62)" }}>学习报告 · 课程完成</div>
            <div style={{ fontSize: 24, fontWeight: 800, color: "var(--mk-surface)", marginTop: 6, lineHeight: 1.3 }}>{report.title}</div>
            <div style={{ fontSize: 13, color: "rgba(255,255,255,.8)", marginTop: 6 }}>恭喜你走完这一程，下面是你留下的印记。</div>
          </div>
        </div>

        {/* stats */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 12, marginTop: 16 }}>
          <Stat value={formatSpent(report.secondsSpent)} label="用时" />
          <Stat value={`${report.completedStepTitles.length}`} label="阶段完成" />
          <Stat value={`${report.quiz.correct} / ${report.quiz.total}`} label="小测表现" color="var(--mk-peach)" />
        </div>

        {/* 学到了什么 */}
        <div style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 14 }}>你学到了什么</div>
          <div style={{ fontSize: 14.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{report.goal}</div>
          {report.teaching_thread && (
            <div style={{ fontSize: 13.5, color: "var(--mk-secondary)", lineHeight: 1.7, marginTop: 10 }}>{report.teaching_thread}</div>
          )}
          {report.completedStepTitles.length > 0 && (
            <div style={{ marginTop: 16 }}>
              {report.completedStepTitles.map((title, i) => (
                <div key={i} style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 10 }}>
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="var(--mk-success)" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 2 }}><path d="M20 6L9 17l-5-5" /></svg>
                  <span style={{ fontSize: 14, color: "var(--mk-ink)", lineHeight: 1.6 }}>{title}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 学到的工具卡 — no empty state: the design has none, and an empty
            block would wrongly imply nothing was learned. Each card shows its
            cover, name, purpose and 何时使用 (from the shared card catalog) so
            the report is a real takeaway, not just a list of names. */}
        {report.cardIds.length > 0 && (
          <div style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
            <div style={{ fontSize: 15, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 4 }}>学到的工具卡</div>
            <div style={{ fontSize: 12.5, color: "var(--mk-muted)", marginBottom: 16 }}>这门课带你上手的思维工具——记住它们能在什么时候帮到你。</div>
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              {report.cardIds.map((cardId, i) => (
                <ToolCardDetail key={`${cardId}-${i}`} cardId={cardId} info={cardInfo[cardId]} />
              ))}
            </div>
          </div>
        )}

        {/* actions — 返回课程 · 重新学一遍 (wipes progress and restarts from the
            top) · 去写作工作室 (the onward CTA). */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 22 }}>
          <button type="button" onClick={onBackToCourses} style={{ flex: "none", background: "var(--mk-surface)", border: "1px solid var(--mk-input-border)", color: "var(--mk-secondary)", fontSize: 14, fontWeight: 700, padding: "13px 20px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>返回课程</button>
          {onRestart && (
            <button type="button" onClick={onRestart} style={{ flex: "none", display: "inline-flex", alignItems: "center", gap: 6, background: "var(--mk-surface)", border: "1px solid var(--mk-input-border)", color: "var(--mk-secondary)", fontSize: 14, fontWeight: 700, padding: "13px 18px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" /></svg>
              重新学一遍
            </button>
          )}
          <button type="button" onClick={onGoPortal} style={{ flex: 1, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: "var(--mk-success)", color: "var(--mk-surface)", border: "none", fontSize: 14.5, fontWeight: 700, padding: 13, borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>
            去写作工作室，用起来
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--mk-surface)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
