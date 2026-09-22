import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import "./showcase.css";
import { SHOWCASE_FONT_STACKS, SHOWCASE_THEMES } from "./showcaseThemes";
import type { ShowcaseConfig, ShowcaseKind, ShowcaseWork } from "./showcaseTypes";

export interface ShowcaseProps {
  config: ShowcaseConfig;
  works: ShowcaseWork[];
  narrow?: boolean;
  editing?: boolean;
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

export function Showcase({ config, works, narrow = false, editing = false }: ShowcaseProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const [measuredNarrow, setMeasuredNarrow] = useState(false);
  const theme = SHOWCASE_THEMES[config.palette];
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
    "--show-font": SHOWCASE_FONT_STACKS[config.font],
  } as CSSProperties;
  const isNarrow = narrow || measuredNarrow;
  const visibleOrder = config.sectionOrder.filter((kind, index, order) => order.indexOf(kind) === index);

  return (
    <div ref={rootRef} className="showcase" style={style} data-layout={config.layout} data-narrow={isNarrow || undefined}>
      <header className="showcase-hero">
        <div className="showcase-orbit" aria-hidden><i /><i /><i /></div>
        <p className="showcase-kicker">个人主页 · {new Date().getFullYear()}</p>
        <h1>{config.name || (editing ? "姓名" : "")}</h1>
        {(config.tagline || editing) && <p className="showcase-tagline">{config.tagline || "主页介绍"}</p>}
        {(config.bio || editing) && <p className="showcase-bio">{config.bio || "个人简介将在这里显示。"}</p>}
        {config.interests.length > 0 && (
          <ul className="showcase-interests" aria-label="兴趣">
            {config.interests.map((interest, index) => <li key={`${interest}-${index}`}>{interest}</li>)}
          </ul>
        )}
      </header>
      <main className="showcase-main">
        {visibleOrder.map((kind) => (
          <WorkSection key={kind} kind={kind} works={selected.filter((work) => work.kind === kind)} config={config} editing={editing} />
        ))}
      </main>
      {(config.name || editing) && <footer className="showcase-footer"><span>{config.name || "个人主页"}</span><i aria-hidden /><span>作品与思考记录</span></footer>}
    </div>
  );
}
