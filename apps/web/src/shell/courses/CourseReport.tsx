import { useEffect, useState } from "react";
import type { CourseReport as CourseReportT, CardCatalogEntry } from "@mind-imprint/contracts";
import { api } from "../../api";

function Stat({ value, label, color }: { value: string; label: string; color?: string }) {
  return (
    <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "16px 14px", textAlign: "center" }}>
      <div style={{ fontSize: 22, fontWeight: 800, color: color ?? "#1C2333" }}>{value}</div>
      <div style={{ fontSize: 12, color: "#8A92A3", fontWeight: 600, marginTop: 3 }}>{label}</div>
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

// A card chip's label comes from the catalog (student-facing 中文名), never
// the raw card id — falls back to the id only if the catalog fetch failed or
// somehow omitted this card, so the chip is never blank.
function CardChip({ cardId, name }: { cardId: string; name?: string }) {
  return (
    <span
      title={name ?? cardId}
      style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "#EDEFF9", color: "#2A3B7A", fontSize: 13, fontWeight: 700, padding: "9px 14px", borderRadius: 11 }}
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="6" width="18" height="13" rx="2.5" /></svg>
      {name ?? cardId}
    </span>
  );
}

// courseId is really the course slug (CourseSummary/CoursePlayerPayload no
// longer have a numeric/string `id` — the slug is the sole identifier). The
// prop is named courseId to stay compatible with CoursesContainer's existing
// call site (Task 11 keeps that wiring, only the internal data model
// changes).
export function CourseReport({ courseId, onBackToCourses, onGoPortal }: { courseId: string; onBackToCourses: () => void; onGoPortal: () => void }) {
  const [report, setReport] = useState<CourseReportT | null>(null);
  const [error, setError] = useState(false);
  const [cardNames, setCardNames] = useState<Record<string, string>>({});

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
      // Card chip labels are a best-effort lookup — the report itself must
      // still render (id-fallback chips) if the catalog fetch fails.
      try {
        const cat = await api.getCardsCatalog();
        if (cancelled) return;
        const m: Record<string, string> = {};
        for (const c of cat.cards as CardCatalogEntry[]) m[c.cardId] = c.name;
        setCardNames(m);
      } catch { /* chip labels fall back to the raw card id */ }
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  if (error) return <div style={{ padding: 40, color: "#B0432E" }}>学习报告暂时没能生成，稍后再看看。</div>;
  if (!report) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的学习报告…</div>;

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        {/* hero */}
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>学习报告 · 课程完成</div>
            <div style={{ fontSize: 24, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>{report.title}</div>
            <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>恭喜你走完这一程，下面是你留下的印记。</div>
          </div>
        </div>

        {/* stats */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 12, marginTop: 16 }}>
          <Stat value={formatSpent(report.secondsSpent)} label="用时" />
          <Stat value={`${report.completedStepTitles.length}`} label="阶段完成" />
          <Stat value={`${report.quiz.correct} / ${report.quiz.total}`} label="小测表现" color="#D98263" />
        </div>

        {/* 学到了什么 */}
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>你学到了什么</div>
          <div style={{ fontSize: 14.5, color: "#2B3346", lineHeight: 1.7 }}>{report.goal}</div>
          {report.teaching_thread && (
            <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.7, marginTop: 10 }}>{report.teaching_thread}</div>
          )}
          {report.completedStepTitles.length > 0 && (
            <div style={{ marginTop: 16 }}>
              {report.completedStepTitles.map((title, i) => (
                <div key={i} style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 10 }}>
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 2 }}><path d="M20 6L9 17l-5-5" /></svg>
                  <span style={{ fontSize: 14, color: "#2B3346", lineHeight: 1.6 }}>{title}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 学到的工具卡 — no empty state: the design has none, and an empty
            block would wrongly imply nothing was learned. Chips are
            non-navigating: there is no existing hook to deep-link the card
            gallery to a specific card (only course→card via CardCatalogEntry
            .courseId, the opposite direction), so this renders the card's
            catalog name as a plain label rather than inventing new routing. */}
        {report.cardIds.length > 0 && (
          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>学到的工具卡</div>
            <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
              {report.cardIds.map((cardId, i) => (
                <CardChip key={`${cardId}-${i}`} cardId={cardId} name={cardNames[cardId]} />
              ))}
            </div>
          </div>
        )}

        {/* actions — a plain 重新开始 is gone (no more sessions to restart in
            the linear self-paced player, Task 10); just back + onward. */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 22 }}>
          <button type="button" onClick={onBackToCourses} style={{ flex: "none", background: "#fff", border: "1px solid #E1E4ED", color: "#6B7384", fontSize: 14, fontWeight: 700, padding: "13px 20px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>返回课程</button>
          <button type="button" onClick={onGoPortal} style={{ flex: 1, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: "#4C9A82", color: "#fff", border: "none", fontSize: 14.5, fontWeight: 700, padding: 13, borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>
            去写作工作室，用起来
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
