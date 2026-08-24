import { useEffect, useState } from "react";
import type { CourseReport as CourseReportT, CardCatalogEntry, CourseSummary, CourseAnswerReport } from "@mind-imprint/contracts";
import { api } from "../../api";
import { coverGradientStyle, PebbleInlineSpinner } from "@/ui";
import { CourseLoading } from "./CourseLoading";

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

// A tool card in the report shows the full picture, not just a name: cover art ·
// 中文名 (+ English + category) · purpose (它做什么) · 何时使用 (阶段 chips + a
// concrete example). All fields come from the shared card catalog
// (CardCatalogEntry); everything after the name degrades gracefully when the
// catalog fetch failed (info undefined → name falls back to the id).
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

function SectionCard({ title, hint, children, dataTour }: { title: string; hint?: string; children: React.ReactNode; dataTour?: string }) {
  return (
    <div style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 16, padding: "22px 24px" }} data-tour={dataTour}>
      <div style={{ fontSize: 15, fontWeight: 800, color: "var(--mk-ink)", marginBottom: hint ? 4 : 14 }}>{title}</div>
      {hint && <div style={{ fontSize: 12.5, color: "var(--mk-muted)", marginBottom: 16 }}>{hint}</div>}
      {children}
    </div>
  );
}

// AnswerBadge renders per-item correctness: 正确 / 未答对 (graded) · 已作答 (ungraded
// survey/reflection, correct === null) · 未作答 (answered === false).
function AnswerBadge({ answered, correct }: { answered: boolean; correct: boolean | null }) {
  let text = "未作答", fg = "var(--mk-muted)", bg = "var(--mk-paper)";
  if (answered) {
    if (correct === true) { text = "正确"; fg = "var(--mk-success)"; bg = "var(--mk-success-bg)"; }
    else if (correct === false) { text = "未答对"; fg = "var(--mk-danger)"; bg = "var(--mk-danger-bg)"; }
    else { text = "已作答"; fg = "var(--mk-accent-600)"; bg = "var(--mk-accent-50)"; }
  }
  return <span style={{ flex: "none", fontSize: 11.5, fontWeight: 700, color: fg, background: bg, padding: "3px 10px", borderRadius: 999 }}>{text}</span>;
}

// AnswerDrawer — the scrollable side panel with the student's per-question
// answers for THIS attempt (the "click 小测 → detail" affordance). Lazy: the
// answer report is fetched only when it opens. Reads back exactly what the
// runtime recorded (final answer + attempts + graded correctness) per slice,
// with the time spent on each slice.
function AnswerDrawer({ courseId, attemptId, onClose }: { courseId: string; attemptId?: string; onClose: () => void }) {
  const [data, setData] = useState<CourseAnswerReport | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setError(false);
    void (async () => {
      try {
        const r = await api.getCourseAnswerReport(courseId, attemptId);
        if (!cancelled) setData(r);
      } catch {
        if (!cancelled) setError(true);
      }
    })();
    return () => { cancelled = true; };
  }, [courseId, attemptId]);

  const empty = data != null && data.slices.length === 0;

  return (
    <div
      onClick={onClose}
      style={{ position: "fixed", inset: 0, zIndex: 60, background: "rgba(15,17,26,.38)", display: "flex", justifyContent: "flex-end" }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="我的答案"
        style={{ width: "min(560px, 94vw)", height: "100%", background: "var(--mk-paper)", boxShadow: "-16px 0 40px rgba(15,17,26,.18)", display: "flex", flexDirection: "column" }}
      >
        <div style={{ flex: "none", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "18px 22px", borderBottom: "1px solid var(--mk-border)", background: "var(--mk-surface)" }}>
          <div>
            <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)" }}>我的答案</div>
            <div style={{ fontSize: 12.5, color: "var(--mk-muted)", marginTop: 2 }}>这一次学习里，你逐题的作答与用时。</div>
          </div>
          <button type="button" onClick={onClose} aria-label="关闭" style={{ flex: "none", width: 32, height: 32, borderRadius: 999, border: "1px solid var(--mk-input-border)", background: "var(--mk-surface)", color: "var(--mk-secondary)", cursor: "pointer", fontSize: 17, lineHeight: 1, fontFamily: "inherit" }}>×</button>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "20px 22px 40px" }}>
          {error ? (
            <div style={{ color: "var(--mk-danger)", fontSize: 14, padding: "20px 0" }}>作答记录暂时没能加载，稍后再看看。</div>
          ) : data == null ? (
            <div style={{ display: "flex", alignItems: "center", gap: 9, color: "var(--mk-muted)", fontSize: 14, padding: "20px 0" }}>
              <PebbleInlineSpinner size={18} />正在整理你的作答…
            </div>
          ) : empty ? (
            <div style={{ color: "var(--mk-muted)", fontSize: 14, padding: "20px 0" }}>这门课没有需要作答的小测题。</div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
              {data.slices.map((sl) => (
                <div key={sl.sliceId}>
                  <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", gap: 10, marginBottom: 10 }}>
                    <div style={{ fontSize: 14, fontWeight: 800, color: "var(--mk-ink)" }}>{sl.title}</div>
                    <div style={{ flex: "none", fontSize: 12, color: "var(--mk-muted)", fontWeight: 600 }}>用时 {formatSpent(sl.timeSpentSeconds)}</div>
                  </div>
                  <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                    {sl.items.map((it) => (
                      <div key={it.blockId} style={{ border: "1px solid var(--mk-border)", borderRadius: 12, padding: "12px 14px", background: "var(--mk-surface)" }}>
                        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
                          <div style={{ fontSize: 13.5, color: "var(--mk-ink)", lineHeight: 1.6, fontWeight: 600 }}>{it.prompt || "（题目）"}</div>
                          <AnswerBadge answered={it.answered} correct={it.correct} />
                        </div>
                        <div style={{ fontSize: 13.5, color: it.answered ? "var(--mk-secondary)" : "var(--mk-faint)", lineHeight: 1.6, marginTop: 7 }}>
                          <span style={{ color: "var(--mk-muted)", fontWeight: 600 }}>你的答案：</span>
                          {it.answered ? it.yourAnswer : "未作答"}
                        </div>
                        {it.attempts > 1 && (
                          <div style={{ fontSize: 12, color: "var(--mk-faint)", marginTop: 6 }}>作答 {it.attempts} 次</div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// courseId is really the course slug (CourseSummary/CoursePlayerPayload no
// longer have a numeric/string `id` — the slug is the sole identifier). The
// prop is named courseId to stay compatible with CoursesContainer's existing
// call site.
export function CourseReport({ courseId, attemptId, exampleReport, onBackToCourses, onGoPortal, onRestart }: { courseId: string; attemptId?: string; exampleReport?: CourseReportT; onBackToCourses: () => void; onGoPortal: () => void; onRestart?: () => void }) {
  const [report, setReport] = useState<CourseReportT | null>(null);
  const [error, setError] = useState(false);
  const [cardInfo, setCardInfo] = useState<Record<string, CardCatalogEntry>>({});
  // The course's own cover + introduction come from the catalog (the report
  // itself carries neither) — a best-effort enrich; the report still renders
  // fully if this fetch fails (no cover, no intro block).
  const [summary, setSummary] = useState<CourseSummary | null>(null);
  const [answersOpen, setAnswersOpen] = useState(false);

  useEffect(() => {
    // Example mode (tour): the report is a frozen fixture, never a live
    // attempt — skip every fetch (report, cards catalog, course summary).
    if (exampleReport) { setReport(exampleReport); setError(false); return; }
    let cancelled = false;
    void (async () => {
      try {
        const r = await api.getCourseReport(courseId, attemptId);
        if (cancelled) return;
        setReport(r);
      } catch {
        if (!cancelled) setError(true);
        return;
      }
      // Card details and the course summary (cover/introduction) are two
      // INDEPENDENT best-effort lookups — the report renders fully if either
      // fails, and one failing must not suppress the other (keep them in
      // separate try/catch, never a single Promise.all).
      try {
        const cat = await api.getCardsCatalog();
        if (cancelled) return;
        const m: Record<string, CardCatalogEntry> = {};
        for (const c of cat.cards as CardCatalogEntry[]) m[c.cardId] = c;
        setCardInfo(m);
      } catch { /* cards fall back to id-only names, no cover/purpose */ }
      try {
        const courses = await api.listCourses();
        if (cancelled) return;
        setSummary(courses.find((c) => c.slug === courseId) ?? null);
      } catch { /* no course summary → no cover / 关于这门课 block */ }
    })();
    return () => { cancelled = true; };
  }, [courseId, attemptId, exampleReport]);

  // NOTE: this component's parent (CoursesTab's content wrapper) is a plain
  // block, not a flex container — so `flex:1` here is inert and the content
  // would overflow and be clipped. Use `height:100%` (the wrapper has a
  // definite height) so `overflowY:auto` actually scrolls.
  if (error)
    return (
      <div style={{ height: "100%", minHeight: 0, display: "flex", alignItems: "center", justifyContent: "center", padding: 40, color: "var(--mk-danger)", background: "var(--mk-paper)" }}>
        学习报告暂时没能生成，稍后再看看。
      </div>
    );
  if (!report)
    return (
      <CourseLoading
        captions={["正在整理你逐题的作答…", "正在计算用时与正确率…", "正在生成你的学习报告…"]}
      />
    );

  const intro = summary?.introduction ?? null;
  const hasIntro = Boolean(intro && (intro.hook || intro.whatYouDo || (intro.keywords?.length ?? 0) > 0));

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto", background: "var(--mk-paper)" }} data-tour="course-report">
      <div style={{ maxWidth: 1360, margin: "0 auto", padding: "34px 40px 56px" }}>
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

        {/* two columns: left = cover + 关于这门课 + 你学到了什么; right = 本次学习
            overview + 我的答案 (逐题回看) + 学到的工具卡 */}
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1.25fr) minmax(0, 0.9fr)", gap: 20, marginTop: 20, alignItems: "start" }}>
          {/* LEFT */}
          <div style={{ display: "flex", flexDirection: "column", gap: 16, minWidth: 0 }}>
            {summary?.coverUrl ? (
              <img src={summary.coverUrl} alt={report.title} style={{ width: "100%", aspectRatio: "16 / 9", objectFit: "cover", borderRadius: 16, border: "1px solid var(--mk-border)", display: "block" }} />
            ) : (
              <div style={{ width: "100%", aspectRatio: "16 / 9", borderRadius: 16, border: "1px solid var(--mk-border)", ...coverGradientStyle(courseId) }} />
            )}

            {hasIntro && intro && (
              <SectionCard title="关于这门课">
                {intro.hook && <p style={{ fontSize: 14.5, color: "var(--mk-ink)", lineHeight: 1.75, margin: 0 }}>{intro.hook}</p>}
                {intro.whatYouDo && <p style={{ fontSize: 14, color: "var(--mk-secondary)", lineHeight: 1.75, margin: intro.hook ? "10px 0 0" : 0 }}>{intro.whatYouDo}</p>}
                {(intro.keywords?.length ?? 0) > 0 && (
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 7, marginTop: 14 }}>
                    {(intro.keywords ?? []).map((k, i) => (
                      <span key={`${k}-${i}`} style={{ fontSize: 12, fontWeight: 600, color: "var(--mk-secondary)", background: "var(--mk-paper)", border: "1px solid var(--mk-border)", padding: "3px 10px", borderRadius: 999 }}>{k}</span>
                    ))}
                  </div>
                )}
              </SectionCard>
            )}

            <SectionCard title="你学到了什么">
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
            </SectionCard>
          </div>

          {/* RIGHT */}
          <div style={{ display: "flex", flexDirection: "column", gap: 16, minWidth: 0 }}>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 12 }} data-tour="course-report-stats">
              <Stat value={formatSpent(report.secondsSpent)} label="用时" />
              <Stat value={`${report.completedStepTitles.length}`} label="阶段完成" />
            </div>

            {/* 小测 & 我的答案 — the clickable card that opens the per-question
                answer drawer (the recorded data, read back on demand). */}
            <div style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 16, padding: "22px 24px" }} data-tour="course-report-quiz">
              <div style={{ fontSize: 15, fontWeight: 800, color: "var(--mk-ink)" }}>小测表现</div>
              <div style={{ display: "flex", alignItems: "baseline", gap: 8, marginTop: 12 }}>
                <span style={{ fontSize: 34, fontWeight: 800, color: "var(--mk-peach)", lineHeight: 1 }}>{report.quiz.correct}</span>
                <span style={{ fontSize: 18, fontWeight: 700, color: "var(--mk-muted)" }}>/ {report.quiz.total}</span>
                <span style={{ fontSize: 13, color: "var(--mk-muted)", marginLeft: 2 }}>题答对</span>
              </div>
              {/* Example mode (tour) has no real attempt to read answers from —
                  hide the trigger rather than open a drawer that would fetch. */}
              {!exampleReport && (
                <button
                  type="button"
                  onClick={() => { if (!exampleReport) setAnswersOpen(true); }}
                  style={{ marginTop: 16, width: "100%", display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: "var(--mk-paper)", border: "1px solid var(--mk-input-border)", color: "var(--mk-secondary)", fontSize: 13.5, fontWeight: 700, padding: "11px 14px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}
                >
                  查看我的答案（逐题）
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6" /></svg>
                </button>
              )}
            </div>

            {/* 学到的工具卡 — no empty state: the design has none, and an empty
                block would wrongly imply nothing was learned. */}
            {report.cardIds.length > 0 && (
              <SectionCard title="学到的工具卡" hint="这门课带你上手的思维工具——记住它们能在什么时候帮到你。" dataTour="course-report-cards">
                <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                  {report.cardIds.map((cardId, i) => (
                    <ToolCardDetail key={`${cardId}-${i}`} cardId={cardId} info={cardInfo[cardId]} />
                  ))}
                </div>
              </SectionCard>
            )}
          </div>
        </div>

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

      {!exampleReport && answersOpen && <AnswerDrawer courseId={courseId} attemptId={attemptId} onClose={() => setAnswersOpen(false)} />}
    </div>
  );
}
