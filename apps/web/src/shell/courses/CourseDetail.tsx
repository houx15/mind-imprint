import { useEffect, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Button } from "@/ui";

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

export function CourseDetail({ slug, onStart, onBack }: { slug: string; onStart: () => void; onBack: () => void }) {
  const [course, setCourse] = useState<CourseSummary | null | undefined>(undefined); // undefined=loading, null=not found
  const [pct, setPct] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    void api.listCourses().then((cs) => {
      if (cancelled) return;
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

  if (course === undefined) {
    return <div style={{ height: "100%", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--mk-faint)", fontSize: 14 }}>正在加载课程…</div>;
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
            {intro.takeaways.length > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 10 }}>带走什么</div>
                <ul style={{ margin: 0, paddingLeft: 20, display: "flex", flexDirection: "column", gap: 8 }}>
                  {intro.takeaways.map((t, i) => <li key={i} style={{ fontSize: 14.5, color: "var(--mk-secondary)", lineHeight: 1.7 }}>{t}</li>)}
                </ul>
              </section>
            )}
            {(intro.alignment.ib.length + intro.alignment.otherIntl.length + intro.alignment.domestic.length) > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 12 }}>学科对标</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 24 }}>
                  <AlignmentColumn title="IB" items={intro.alignment.ib} />
                  <AlignmentColumn title="其他国际" items={intro.alignment.otherIntl} />
                  <AlignmentColumn title="国内" items={intro.alignment.domestic} />
                </div>
              </section>
            )}
            {intro.keywords.length > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 12 }}>关键词</div>
                <Chips items={intro.keywords} />
              </section>
            )}
          </div>
        ) : (
          <p style={{ fontSize: 15, color: "var(--mk-secondary)", lineHeight: 1.75, marginTop: 24 }}>{course.blurb}</p>
        )}

        <div style={{ marginTop: 40 }}>
          <button type="button" onClick={onStart} style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "var(--mk-accent-500)", color: "var(--mk-surface)", border: "none", padding: "12px 22px", borderRadius: 12, fontSize: 15, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            {cta}
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
