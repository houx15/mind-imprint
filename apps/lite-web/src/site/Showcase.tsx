import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import "./showcase.css";
import "./showcaseFonts.css";
import { SHOWCASE_ILLUSTRATIONS } from "./showcasePresets";
import { SHOWCASE_FONT_STACKS, SHOWCASE_THEMES } from "./showcaseThemes";
import type { ShowcaseConfig, ShowcaseKind, ShowcaseWork } from "./showcaseTypes";
import { ShowcaseInterestTree, type ShowcaseInterestSnapshot } from "./ShowcaseInterestTree";
import { ShowcaseComponents } from "./ShowcaseComponents";

export interface ShowcaseProps {
  config: ShowcaseConfig;
  works: ShowcaseWork[];
  narrow?: boolean;
  editing?: boolean;
  heroImageUrl?: string;
  avatarUrl?: string;
  interestTree?: ShowcaseInterestSnapshot;
  allWorksHref?: string;
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

export function safeShowcaseExternalURL(value?: string): string | undefined {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === "https:" ? url.href : undefined;
  } catch {
    return undefined;
  }
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

export function featuredFirstShowcaseWorks(works: ShowcaseWork[], featuredWorkIds: string[] = []): ShowcaseWork[] {
  const byId = new Map(works.map(work => [work.id, work]));
  const featured = [...new Set(featuredWorkIds)].flatMap(id => byId.get(id) ? [byId.get(id)!] : []);
  const used = new Set(featured.map(work => work.id));
  return [...featured, ...works.filter(work => !used.has(work.id))];
}

function WorkTitle({ work, children }: { work: ShowcaseWork; children: ReactNode }) {
  const internalPath = work.kind === "project" ? undefined : safeShowcaseWorkPath(work.publicPath);
  const externalURL = work.kind === "project" ? safeShowcaseExternalURL(work.externalUrl) : undefined;
  if (internalPath) return <a href={internalPath}>{children}</a>;
  if (externalURL) return <a href={externalURL} target="_blank" rel="noreferrer">{children}</a>;
  return <>{children}</>;
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

export interface ShowcaseCalendarDay {
  date: string;
  works: ShowcaseWork[];
}

export interface ShowcaseCalendarYear {
  year: string;
  days: ShowcaseCalendarDay[];
}

function isoDay(date: Date): string {
  return date.toISOString().slice(0, 10);
}

export function showcaseCalendarYears(works: ShowcaseWork[]): ShowcaseCalendarYear[] {
  const byDate = new Map<string, ShowcaseWork[]>();
  for (const work of works) {
    const date = safeShowcaseDate(work.date);
    if (date) byDate.set(date, [...(byDate.get(date) ?? []), work]);
  }
  const byYear = new Map<string, string[]>();
  for (const date of [...byDate.keys()].sort()) {
    const year = date.slice(0, 4);
    byYear.set(year, [...(byYear.get(year) ?? []), date]);
  }
  return [...byYear.entries()].sort(([a], [b]) => b.localeCompare(a)).map(([year, dates]) => {
    const first = new Date(`${dates[0]}T00:00:00Z`);
    const last = new Date(`${dates[dates.length - 1]}T00:00:00Z`);
    first.setUTCDate(first.getUTCDate() - first.getUTCDay());
    last.setUTCDate(last.getUTCDate() + (6 - last.getUTCDay()));
    const days: ShowcaseCalendarDay[] = [];
    for (const cursor = new Date(first); cursor <= last; cursor.setUTCDate(cursor.getUTCDate() + 1)) {
      const date = isoDay(cursor);
      days.push({ date, works: byDate.get(date) ?? [] });
    }
    return { year, days };
  });
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

function PortfolioTile({ work, index }: { work: ShowcaseWork; index: number }) {
  return <article className="showcase-portfolio-tile" data-kind={work.kind}>
    <div className="showcase-portfolio-tile-meta"><span>{SECTION_LABELS[work.kind]}</span><span>{String(index + 1).padStart(2, "0")}</span></div>
    <div className="showcase-portfolio-tile-body"><h3><WorkTitle work={work}>{work.title}</WorkTitle></h3>{work.summary && <p>{work.summary}</p>}</div>
    <time dateTime={safeShowcaseDate(work.date)}>{safeShowcaseDate(work.date) ? dateLabel(work.date) : ""}</time>
  </article>;
}

function FilmPortfolio({ works }: { works: ShowcaseWork[] }) {
  const track = useRef<HTMLDivElement>(null);
  const move = (direction: -1 | 1) => track.current?.scrollBy({ left: direction * (track.current.clientWidth * .82), behavior: "smooth" });
  return <div className="showcase-film">
    <div className="showcase-film-heading"><span>精选放映 · {works.length} 件作品</span><div><button type="button" onClick={() => move(-1)} aria-label="上一张作品">←</button><button type="button" onClick={() => move(1)} aria-label="下一张作品">→</button></div></div>
    <div ref={track} className="showcase-film-track" role="region" aria-label="胶片作品，可横向滚动" tabIndex={0}>
      {works.map((work, index) => <div className="showcase-film-frame" key={work.id}><PortfolioTile work={work} index={index} /></div>)}
    </div>
  </div>;
}

function CalendarYear({ year, days }: ShowcaseCalendarYear) {
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const selected = days.find((day) => day.date === selectedDate);
  return <section className="showcase-calendar-year" aria-labelledby={`showcase-calendar-${year}`}>
    <h3 id={`showcase-calendar-${year}`}>{year}</h3>
    <div className="showcase-calendar-scroll"><div className="showcase-calendar-grid" aria-label={`${year}年作品日历`}>
      {days.map(({ date, works }) => works.length ? <button type="button" className="showcase-calendar-day" key={date} data-level={Math.min(works.length, 4)} aria-label={`${dateLabel(date)}，${works.length}项作品`} aria-pressed={selectedDate === date} onClick={() => setSelectedDate(current => current === date ? null : date)} /> : <span className="showcase-calendar-day" aria-hidden="true" key={date} data-level="0" />)}
    </div></div>
    {selected?.works.length ? <div className="showcase-calendar-selection"><time dateTime={selected.date}>{dateLabel(selected.date)}</time>{selected.works.map((work,index)=><CompactWork key={work.id} work={work} index={index}/>)}</div> : <p className="showcase-calendar-help">选择有颜色的日期，查看当天的作品。</p>}
  </section>;
}

function Portfolio({ works, mode }: { works: ShowcaseWork[]; mode: Exclude<NonNullable<ShowcaseConfig["portfolioLayout"]>, "sections"> }) {
  const dated = works.filter((work) => safeShowcaseDate(work.date));
  const undated = works.filter((work) => !safeShowcaseDate(work.date));
  if (mode === "list") return <div className="showcase-portfolio-list">{works.map((work, index) => <CompactWork key={work.id} work={work} index={index} />)}</div>;
  if (mode === "flow") return <div className="showcase-flow">{works.map((work, index) => <PortfolioTile key={work.id} work={work} index={index} />)}</div>;
  if (mode === "film") return <FilmPortfolio works={works} />;
  if (mode === "timeline") return <div className="showcase-timeline">{timelineShowcaseWorks(works).map((work, index) => <article className="showcase-timeline-item" key={work.id}><time dateTime={safeShowcaseDate(work.date)}>{dateLabel(work.date)}</time><PortfolioTile work={work} index={index}/></article>)}</div>;
  if (mode === "planets") return (
    <div className="showcase-planets" role="list" aria-label="作品星球">
      {works.map((work,index) => <div role="listitem" className="showcase-planet" key={work.id}><PortfolioTile work={work} index={index}/></div>)}
    </div>
  );
  if (mode === "cloud") return (
    <div className="showcase-title-cloud" role="list" aria-label="作品云">
      {works.map((work) => <article role="listitem" key={work.id} data-kind={work.kind}><span>{SECTION_LABELS[work.kind]}</span><h3><WorkTitle work={work}>{work.title}</WorkTitle></h3></article>)}
    </div>
  );
  const years = showcaseCalendarYears(dated);
  return (
    <div className="showcase-calendar">
      <div className="showcase-calendar-legend"><span>作品记录</span><i data-level="0" /><i data-level="1" /><i data-level="2" /><i data-level="3" /><i data-level="4" /></div>
      {years.map(({ year, days }) => <CalendarYear key={year} year={year} days={days}/>)}
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

function FeaturedWorks({ works }: { works: ShowcaseWork[] }) {
  if (!works.length) return null;
  return <section className="showcase-featured" aria-labelledby="showcase-featured-title">
    <div className="showcase-featured-heading"><p>重点作品</p><h2 id="showcase-featured-title">先看这些作品</h2></div>
    <div className="showcase-featured-grid">{works.map((work,index)=><article className="showcase-featured-work" key={work.id}>
      <div className="showcase-featured-meta"><span>{SECTION_LABELS[work.kind]} · {String(index+1).padStart(2,"0")}</span>{safeShowcaseDate(work.date)&&<time dateTime={work.date}>{dateLabel(work.date)}</time>}</div>
      <h3><WorkTitle work={work}>{work.title}</WorkTitle></h3>
      {work.summary&&<p>{work.summary}</p>}
    </article>)}</div>
  </section>;
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
              {(work.kind !== "project" ? safeShowcaseWorkPath(work.publicPath) : safeShowcaseExternalURL(work.externalUrl)) && (
                <a className="showcase-work-link" href={work.kind !== "project" ? safeShowcaseWorkPath(work.publicPath) : safeShowcaseExternalURL(work.externalUrl)} target={work.kind === "project" ? "_blank" : undefined} rel={work.kind === "project" ? "noreferrer" : undefined} aria-label={`打开${work.title}`}>↗</a>
              )}
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

export function Showcase({ config, works, narrow = false, editing = false, heroImageUrl, avatarUrl, interestTree, allWorksHref }: ShowcaseProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const [measuredNarrow, setMeasuredNarrow] = useState(false);
  const [avatarFailed, setAvatarFailed] = useState(false);
  const [heroFailed, setHeroFailed] = useState(false);
  const theme = SHOWCASE_THEMES[config.palette];
  const visualStyle = config.style ?? "classic";
  const illustration = config.illustration ?? "none";
  const art = SHOWCASE_ILLUSTRATIONS.find((item) => item.id === illustration)?.src;
  const heroArt = heroImageUrl || art;
  const displayedHeroArt = heroFailed ? undefined : heroArt;
  const aboutLayout = config.aboutLayout ?? "classic";
  const portfolioLayout = config.portfolioLayout ?? "sections";
  const selected = useMemo(() => {
    const ordered = featuredFirstShowcaseWorks(selectedShowcaseWorks(works, config.selectedWorkIds), config.featuredWorkIds);
    const limit = [3, 6, 9, 12].includes(config.homeWorkLimit ?? 6) ? (config.homeWorkLimit ?? 6) : 6;
    return ordered.slice(0, limit);
  }, [config.homeWorkLimit, config.selectedWorkIds, config.featuredWorkIds, works]);
  const featuredIDs = new Set(config.featuredWorkIds ?? []);
  const featuredWorks = selected.filter(work => featuredIDs.has(work.id));
  const regularWorks = selected.filter(work => !featuredIDs.has(work.id));

  useEffect(() => {
    const node = rootRef.current;
    if (!node || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => {
      if (entry) setMeasuredNarrow(entry.contentRect.width < 620);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  useEffect(() => setAvatarFailed(false), [avatarUrl]);
  useEffect(() => setHeroFailed(false), [heroArt]);

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
      <header className={`showcase-hero ${displayedHeroArt ? "has-cover-art" : ""}`}>
        {displayedHeroArt && <img className="showcase-hero-art is-cover" src={displayedHeroArt} alt="" aria-hidden="true" onError={() => setHeroFailed(true)} />}
        <h1>{config.heroTitle?.trim() || (config.name ? `欢迎来到${config.name}的空间` : editing ? "欢迎来到我的空间" : "")}</h1>
        {(config.tagline || editing) && <p className="showcase-tagline">{config.tagline || "主页介绍"}</p>}
        <a className="showcase-enter" href="#showcase-about">进入我的空间 <span aria-hidden>↓</span></a>
      </header>
      <section className="showcase-about" id="showcase-about" data-about-layout={aboutLayout} aria-labelledby="showcase-about-title">
        <div className="showcase-avatar">
          {avatarUrl && !avatarFailed ? <img src={avatarUrl} alt="" onError={() => setAvatarFailed(true)} /> : <span aria-hidden>{config.name.trim().slice(0, 1) || "·"}</span>}
        </div>
        <div className="showcase-about-copy">
          <p className="showcase-kicker">关于我</p>
          <h2 id="showcase-about-title">{config.name || (editing ? "姓名" : "")}</h2>
          {(config.bio || editing) && <p className="showcase-bio">{config.bio || "个人简介将在这里显示。"}</p>}
        </div>
        {config.interests.length > 0 && <ul className="showcase-interests" aria-label="兴趣">{config.interests.map((interest, index) => <li key={`${interest}-${index}`}>{interest}</li>)}</ul>}
      </section>
      <ShowcaseComponents components={config.components} placement="after-about" editing={editing} />
      <ShowcaseInterestTree interest={interestTree} />
      <main className="showcase-main">
        <FeaturedWorks works={featuredWorks}/>
        {portfolioLayout === "sections" ? visibleOrder.map((kind) => <WorkSection key={kind} kind={kind} works={regularWorks.filter((work) => work.kind === kind)} config={config} editing={editing && selected.length===0} />) : regularWorks.length || (editing && selected.length===0) ? (
          <section className="showcase-section showcase-portfolio" aria-labelledby="showcase-portfolio-title">
            <div className="showcase-section-heading"><p>01</p><h2 id="showcase-portfolio-title">作品</h2><span>{regularWorks.length ? `${regularWorks.length} 项` : "待添加"}</span></div>
            {regularWorks.length ? <Portfolio works={regularWorks} mode={portfolioLayout} /> : editing ? <EmptySection kind="project" /> : null}
          </section>
        ) : null}
      </main>
      <ShowcaseComponents components={config.components} placement="after-works" editing={editing} />
      {allWorksHref && <nav className="showcase-all-works-nav" aria-label="作品集"><a href={allWorksHref}>查看全部作品 <span aria-hidden>→</span></a></nav>}
      {(config.name || editing) && <footer className="showcase-footer"><span>{config.name || "个人主页"}</span><i aria-hidden /><span>作品与思考记录</span></footer>}
    </div>
  );
}
