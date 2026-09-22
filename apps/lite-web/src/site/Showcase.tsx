import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import "./showcase.css";
import "./showcaseFonts.css";
import { SHOWCASE_ILLUSTRATIONS } from "./showcasePresets";
import { SHOWCASE_FONT_STACKS, SHOWCASE_THEMES } from "./showcaseThemes";
import type { ShowcaseConfig, ShowcaseKind, ShowcaseWork } from "./showcaseTypes";

export interface ShowcaseProps {
  config: ShowcaseConfig;
  works: ShowcaseWork[];
  narrow?: boolean;
  editing?: boolean;
  heroImageUrl?: string;
  avatarUrl?: string;
}

const SECTION_LABELS: Record<ShowcaseKind, string> = {
  writing: "写作",
  reading: "阅读",
  project: "项目",
};

export function safeShowcaseWorkPath(path?: string): string | undefined {
  if (!path) return undefined;
  return /^\/s\/[A-Za-z0-9_-]+$/.test(path) ? path : undefined;
}

export function selectedShowcaseWorks(works: ShowcaseWork[], selectedWorkIds: string[]): ShowcaseWork[] {
  const byId = new Map(works.map((work) => [work.id, work]));
  const seen = new Set<string>();
  return selectedWorkIds.flatMap((id) => {
    if (seen.has(id)) return [];
    seen.add(id);
    const work = byId.get(id);
    return work ? [work] : [];
  });
}

function WorkTitle({ work, children }: { work: ShowcaseWork; children: ReactNode }) {
  const path = work.kind === "project" ? undefined : safeShowcaseWorkPath(work.publicPath);
  return path ? <a href={path}>{children}</a> : <>{children}</>;
}

export function safeShowcaseDate(value?: string): string | undefined {
  if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return undefined;
  const parsed = new Date(`${value}T00:00:00Z`);
  return Number.isNaN(parsed.valueOf()) || parsed.toISOString().slice(0, 10) !== value ? undefined : value;
}

function dateLabel(value?: string): string {
  const date = safeShowcaseDate(value);
  return date ? date.replace(/-/g, ".") : "未记录日期";
}

export function timelineShowcaseWorks(works: ShowcaseWork[]): ShowcaseWork[] {
  return works.map((work, index) => ({ work, index, date: safeShowcaseDate(work.date) }))
    .sort((a, b) => {
      if (a.date && b.date) return b.date.localeCompare(a.date) || a.index - b.index;
      if (a.date) return -1;
      if (b.date) return 1;
      return a.index - b.index;
    })
    .map(({ work }) => work);
}

function CompactWork({ work, index }: { work: ShowcaseWork; index: number }) {
  return (
    <article className="showcase-compact-work">
      <span aria-hidden>{String(index + 1).padStart(2, "0")}</span>
      <div><h3><WorkTitle work={work}>{work.title}</WorkTitle></h3>{work.summary && <p>{work.summary}</p>}</div>
      <time dateTime={safeShowcaseDate(work.date)}>{dateLabel(work.date)}</time>
    </article>
  );
}

function Portfolio({ works, mode }: { works: ShowcaseWork[]; mode: Exclude<NonNullable<ShowcaseConfig["portfolioLayout"]>, "sections"> }) {
  const dated = works.filter((work) => safeShowcaseDate(work.date));
  const undated = works.filter((work) => !safeShowcaseDate(work.date));
  if (mode === "list") return <div className="showcase-portfolio-list">{works.map((work, index) => <CompactWork key={work.id} work={work} index={index} />)}</div>;
  if (mode === "timeline") return <div className="showcase-timeline">{timelineShowcaseWorks(works).map((work, index) => <CompactWork key={work.id} work={work} index={index} />)}</div>;
  if (mode === "planets") return (
    <div className="showcase-planets" role="list" aria-label="作品星球">
      {works.map((work) => <article role="listitem" className="showcase-planet" key={work.id}><span aria-hidden>{SECTION_LABELS[work.kind]}</span><h3><WorkTitle work={work}>{work.title}</WorkTitle></h3>{work.summary && <p>{work.summary}</p>}</article>)}
    </div>
  );
  if (mode === "cloud") return (
    <div className="showcase-title-cloud" role="list" aria-label="作品云">
      {works.map((work) => <article role="listitem" key={work.id}><span>{SECTION_LABELS[work.kind]}</span><h3><WorkTitle work={work}>{work.title}</WorkTitle></h3></article>)}
    </div>
  );
  const groups = new Map<string, ShowcaseWork[]>();
  for (const work of dated) {
    const date = safeShowcaseDate(work.date)!;
    groups.set(date, [...(groups.get(date) ?? []), work]);
  }
  return (
    <div className="showcase-calendar">
      <div className="showcase-calendar-grid" role="list" aria-label="有作品记录的日期">
        {[...groups.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([date, items]) => (
          <div className="showcase-calendar-day" role="listitem" key={date} data-level={Math.min(items.length, 4)}>
            <time dateTime={date}>{dateLabel(date)}</time><strong>{items.length}</strong>
            <ul>{items.map((work) => <li key={work.id}><WorkTitle work={work}>{work.title}</WorkTitle></li>)}</ul>
          </div>
        ))}
      </div>
      {undated.length > 0 && <div className="showcase-undated"><h3>未记录日期</h3>{undated.map((work, index) => <CompactWork key={work.id} work={work} index={index} />)}</div>}
    </div>
  );
}

function EmptySection({ kind }: { kind: ShowcaseKind }) {
  return (
    <div className="showcase-empty">
      <span>{SECTION_LABELS[kind]}</span>
      <p>公开作品将在这里显示。</p>
    </div>
  );
}

function WorkSection({ kind, works, config, editing }: { kind: ShowcaseKind; works: ShowcaseWork[]; config: ShowcaseConfig; editing: boolean }) {
  if (works.length === 0 && !editing) return null;
  const mode = kind === "writing" ? config.writingStyle : kind === "reading" ? config.readingStyle : "cards";
  return (
    <section className={`showcase-section showcase-${kind}`} aria-labelledby={`showcase-${kind}`}>
      <div className="showcase-section-heading">
        <p>0{config.sectionOrder.indexOf(kind) + 1}</p>
        <h2 id={`showcase-${kind}`}>{SECTION_LABELS[kind]}</h2>
        <span>{works.length ? `${works.length} 项` : "待添加"}</span>
      </div>
      {works.length === 0 ? <EmptySection kind={kind} /> : (
        <div className={`showcase-work-grid showcase-mode-${mode}`}>
          {works.map((work, index) => (
            <article className="showcase-work" key={work.id}>
              <div className="showcase-work-mark" aria-hidden>{String(index + 1).padStart(2, "0")}</div>
              <div className="showcase-work-copy">
                <h3><WorkTitle work={work}>{work.title}</WorkTitle></h3>
                {work.summary && <p>{work.summary}</p>}
              </div>
              {work.kind !== "project" && safeShowcaseWorkPath(work.publicPath) && (
                <a className="showcase-work-link" href={safeShowcaseWorkPath(work.publicPath)} aria-label={`打开${work.title}`}>↗</a>
              )}
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

export function Showcase({ config, works, narrow = false, editing = false, heroImageUrl, avatarUrl }: ShowcaseProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const [measuredNarrow, setMeasuredNarrow] = useState(false);
  const theme = SHOWCASE_THEMES[config.palette];
  const visualStyle = config.style ?? "classic";
  const illustration = config.illustration ?? "none";
  const art = SHOWCASE_ILLUSTRATIONS.find((item) => item.id === illustration)?.src;
  const heroArt = heroImageUrl || art;
  const aboutLayout = config.aboutLayout ?? "classic";
  const portfolioLayout = config.portfolioLayout ?? "sections";
  const selected = useMemo(() => {
    return selectedShowcaseWorks(works, config.selectedWorkIds);
  }, [config.selectedWorkIds, works]);

  useEffect(() => {
    const node = rootRef.current;
    if (!node || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => {
      if (entry) setMeasuredNarrow(entry.contentRect.width < 620);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const style = {
    "--show-paper": theme.paper,
    "--show-ink": theme.ink,
    "--show-muted": theme.muted,
    "--show-accent": theme.accent,
    "--show-accent-ink": theme.accentInk,
    "--show-wash": theme.wash,
    "--show-line": theme.line,
    "--show-font": SHOWCASE_FONT_STACKS[["rounded", "handwritten", "display"].includes(config.font) ? "sans" : config.font],
  } as CSSProperties;
  const isNarrow = narrow || measuredNarrow;
  const visibleOrder = config.sectionOrder.filter((kind, index, order) => order.indexOf(kind) === index);

  return (
    <div ref={rootRef} className="showcase" style={style} data-layout={config.layout} data-style={visualStyle} data-illustration={illustration} data-editing={editing || undefined} data-narrow={isNarrow || undefined}>
      <header className={`showcase-hero ${heroArt ? "has-cover-art" : ""}`}>
        {heroArt && <img className="showcase-hero-art is-cover" src={heroArt} alt="" aria-hidden="true" />}
        <h1>{config.heroTitle?.trim() || (config.name ? `欢迎来到${config.name}的空间` : editing ? "欢迎来到我的空间" : "")}</h1>
        {(config.tagline || editing) && <p className="showcase-tagline">{config.tagline || "主页介绍"}</p>}
        <a className="showcase-enter" href="#showcase-about">进入我的空间 <span aria-hidden>↓</span></a>
      </header>
      <section className="showcase-about" id="showcase-about" data-about-layout={aboutLayout} aria-labelledby="showcase-about-title">
        <div className="showcase-avatar">
          {avatarUrl ? <img src={avatarUrl} alt="" /> : <span aria-hidden>{config.name.trim().slice(0, 1) || "·"}</span>}
        </div>
        <div className="showcase-about-copy">
          <p className="showcase-kicker">关于我</p>
          <h2 id="showcase-about-title">{config.name || (editing ? "姓名" : "")}</h2>
          {(config.bio || editing) && <p className="showcase-bio">{config.bio || "个人简介将在这里显示。"}</p>}
        </div>
        {config.interests.length > 0 && <ul className="showcase-interests" aria-label="兴趣">{config.interests.map((interest, index) => <li key={`${interest}-${index}`}>{interest}</li>)}</ul>}
      </section>
      <main className="showcase-main">
        {portfolioLayout === "sections" ? visibleOrder.map((kind) => <WorkSection key={kind} kind={kind} works={selected.filter((work) => work.kind === kind)} config={config} editing={editing} />) : (
          <section className="showcase-section showcase-portfolio" aria-labelledby="showcase-portfolio-title">
            <div className="showcase-section-heading"><p>01</p><h2 id="showcase-portfolio-title">作品</h2><span>{selected.length ? `${selected.length} 项` : "待添加"}</span></div>
            {selected.length ? <Portfolio works={selected} mode={portfolioLayout} /> : editing ? <EmptySection kind="project" /> : null}
          </section>
        )}
      </main>
      {(config.name || editing) && <footer className="showcase-footer"><span>{config.name || "个人主页"}</span><i aria-hidden /><span>作品与思考记录</span></footer>}
    </div>
  );
}
