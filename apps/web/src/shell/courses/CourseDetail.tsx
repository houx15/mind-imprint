import { useEffect, useState } from "react";
import type { CourseSummary, CardCatalogEntry } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Button } from "@/ui";
import { CardDetailModal } from "@/shell/growth/CardDetailModal";
import { CourseLoading } from "./CourseLoading";

// One mapped tool card in the course summary: cover art (falls back to a text
// face) + 中文名 + purpose. Rendering one per course.card_ids means a course
// with several cards shows all of them, not just a count. Clicking it opens the
// same card detail modal the 图鉴 gallery uses.
function CourseCardChip({ id, info, onOpen }: { id: string; info?: CardCatalogEntry; onOpen: () => void }) {
  const [failed, setFailed] = useState(false);
  const name = info?.name ?? id;
  const showImg = Boolean(info?.coverUrl) && !failed;
  return (
    <button type="button" onClick={onOpen} title={name} style={{ display: "flex", gap: 12, border: "1px solid var(--mk-border)", borderRadius: 12, padding: 10, background: "var(--mk-surface)", textAlign: "left", cursor: "pointer", fontFamily: "inherit", width: "100%" }}>
      <div style={{ flex: "none", width: 52, height: 70, borderRadius: 8, overflow: "hidden", border: "1px solid var(--mk-border)", background: "linear-gradient(150deg,var(--mk-accent-500),var(--mk-accent-600))", display: "flex", alignItems: "center", justifyContent: "center" }}>
        {showImg ? (
          <img src={info!.coverUrl} alt={name} onError={() => setFailed(true)} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
        ) : (
          <span style={{ padding: "0 4px", textAlign: "center", fontSize: 9.5, fontWeight: 700, color: "rgba(255,255,255,.8)", lineHeight: 1.3 }}>{info?.nameEn || name}</span>
        )}
      </div>
      <div style={{ minWidth: 0, flex: 1 }}>
        <div style={{ fontSize: 13.5, fontWeight: 700, color: "var(--mk-ink)", lineHeight: 1.35 }}>{name}</div>
        {info?.purpose && (
          <div style={{ fontSize: 12, color: "var(--mk-muted)", lineHeight: 1.5, marginTop: 4, display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>{info.purpose}</div>
        )}
      </div>
    </button>
  );
}

function Chips({ items }: { items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
      {items.map((t, i) => (
        <span key={i} style={{ fontSize: 12.5, fontWeight: 600, color: "var(--mk-secondary)", background: "var(--mk-paper)", border: "1px solid var(--mk-border)", padding: "5px 11px", borderRadius: 999 }}>{t}</span>
      ))}
    </div>
  );
}

function AlignmentColumn({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div style={{ flex: 1, minWidth: 180 }}>
      <div style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-muted)", marginBottom: 8 }}>{title}</div>
      <ul style={{ margin: 0, paddingLeft: 18, display: "flex", flexDirection: "column", gap: 6 }}>
        {items.map((t, i) => <li key={i} style={{ fontSize: 13.5, color: "var(--mk-secondary)", lineHeight: 1.6 }}>{t}</li>)}
      </ul>
    </div>
  );
}

export function CourseDetail({ slug, onStart, onBack, onOpenCourse }: { slug: string; onStart: () => void; onBack: () => void; onOpenCourse?: (slug: string) => void }) {
  const [course, setCourse] = useState<CourseSummary | null | undefined>(undefined); // undefined=loading, null=not found
  const [pct, setPct] = useState<number | null>(null);
  const [cardInfo, setCardInfo] = useState<Record<string, CardCatalogEntry>>({});
  const [courses, setCourses] = useState<CourseSummary[]>([]); // full list, for the card modal's related courses
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void api.listCourses().then((cs) => {
      if (cancelled) return;
      setCourses(cs);
      const found = cs.find((c) => c.slug === slug) ?? null;
      setCourse(found);
      if (found && found.step_count > 0) {
        void Promise.resolve(api.getCourseProgress(slug)).then((p) => {
          if (cancelled || !p) return;
          const n = p.completed_ordinals.length;
          setPct(n > 0 ? Math.round((n / found.step_count) * 100) : null);
        }).catch(() => {});
      }
    }).catch(() => { if (!cancelled) setCourse(null); });
    return () => { cancelled = true; };
  }, [slug]);

  // Card details for the mapped tool cards — best-effort; the page still renders
  // (id-fallback names, gradient faces) if the catalog fetch fails.
  useEffect(() => {
    let cancelled = false;
    void api.getCardsCatalog().then((cat) => {
      if (cancelled) return;
      const m: Record<string, CardCatalogEntry> = {};
      for (const c of cat.cards as CardCatalogEntry[]) m[c.cardId] = c;
      setCardInfo(m);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

  if (course === undefined) {
    return <CourseLoading />;
  }
  if (course === null) {
    return (
      <div style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12, color: "var(--mk-secondary)", fontSize: 14 }}>
        <div>找不到这门课程。</div>
        <Button variant="ghost" onClick={onBack}>返回课程</Button>
      </div>
    );
  }

  const intro = course.introduction;
  const cta = pct == null ? "开始学习" : pct >= 100 ? "回顾" : "继续";

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 780, margin: "0 auto", padding: "40px 40px 64px" }}>
        <button type="button" onClick={onBack} style={{ background: "none", border: "none", color: "var(--mk-muted)", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: 0, fontFamily: "inherit" }}>← 返回课程</button>
        <div style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 600, marginTop: 20 }}>{course.branch} · {course.step_count} 个任务 · {course.card_ids.length} 个工具 · {course.time_label}</div>
        <h1 style={{ fontSize: 30, fontWeight: 800, color: "var(--mk-ink)", lineHeight: 1.35, marginTop: 8, letterSpacing: "-0.01em" }}>{course.title}</h1>

        {intro ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 30, marginTop: 24 }}>
            {intro.hook && <p style={{ fontSize: 15.5, color: "var(--mk-secondary)", lineHeight: 1.75 }}>{intro.hook}</p>}
            {intro.whatYouDo && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 10 }}>学生做什么</div>
                <p style={{ fontSize: 14.5, color: "var(--mk-secondary)", lineHeight: 1.75 }}>{intro.whatYouDo}</p>
              </section>
            )}
            {(intro.takeaways?.length ?? 0) > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 10 }}>带走什么</div>
                <ul style={{ margin: 0, paddingLeft: 20, display: "flex", flexDirection: "column", gap: 8 }}>
                  {(intro.takeaways ?? []).map((t, i) => <li key={i} style={{ fontSize: 14.5, color: "var(--mk-secondary)", lineHeight: 1.7 }}>{t}</li>)}
                </ul>
              </section>
            )}
            {((intro.alignment?.ib?.length ?? 0) + (intro.alignment?.otherIntl?.length ?? 0) + (intro.alignment?.domestic?.length ?? 0)) > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 12 }}>学科对标</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 24 }}>
                  <AlignmentColumn title="IB" items={intro.alignment?.ib ?? []} />
                  <AlignmentColumn title="其他国际" items={intro.alignment?.otherIntl ?? []} />
                  <AlignmentColumn title="国内" items={intro.alignment?.domestic ?? []} />
                </div>
              </section>
            )}
            {(intro.keywords?.length ?? 0) > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 12 }}>关键词</div>
                <Chips items={intro.keywords ?? []} />
              </section>
            )}
          </div>
        ) : (
          <p style={{ fontSize: 15, color: "var(--mk-secondary)", lineHeight: 1.75, marginTop: 24 }}>{course.blurb}</p>
        )}

        {course.card_ids.length > 0 && (
          <section style={{ marginTop: 34 }}>
            <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 14 }}>这门课会用到的思维工具卡</div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(240px,1fr))", gap: 12 }}>
              {course.card_ids.map((id) => <CourseCardChip key={id} id={id} info={cardInfo[id]} onOpen={() => setSelectedCardId(id)} />)}
            </div>
          </section>
        )}

        <div style={{ marginTop: 40 }}>
          {/* Guided tour anchor (§P5 Task 7): the real bridge from this browse
              page into the live player — `courses-enter-1` targets it. */}
          <button type="button" data-tour="course-detail-start" onClick={onStart} style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "var(--mk-accent-500)", color: "var(--mk-surface)", border: "none", padding: "12px 22px", borderRadius: 12, fontSize: 15, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            {cta}
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>

      {selectedCardId && cardInfo[selectedCardId] && (
        <CardDetailModal
          c={cardInfo[selectedCardId]!}
          courses={courses}
          onClose={() => setSelectedCardId(null)}
          onOpenCourse={onOpenCourse ? (s) => { setSelectedCardId(null); onOpenCourse(s); } : undefined}
        />
      )}
    </div>
  );
}
