import { useEffect, useRef, useState, type CSSProperties } from "react";
import { getPublicShowcaseWorks } from "../api/site";
import { safeShowcaseExternalURL, safeShowcaseWorkPath } from "./Showcase";
import { SHOWCASE_FONT_STACKS, SHOWCASE_THEMES } from "./showcaseThemes";
import type { ShowcaseConfig, ShowcaseKind, ShowcaseWork } from "./showcaseTypes";

type WorkFilter = "all" | ShowcaseKind;

const FILTERS: ReadonlyArray<{ value: WorkFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "writing", label: "写作" },
  { value: "reading", label: "阅读" },
  { value: "project", label: "项目" },
];

const KIND_LABELS: Record<ShowcaseKind, string> = { writing: "写作", reading: "阅读", project: "项目" };

function workHref(work: ShowcaseWork): string | undefined {
  return work.kind === "project" ? safeShowcaseExternalURL(work.externalUrl) : safeShowcaseWorkPath(work.publicPath);
}

export function PublicShowcaseWorks({ token, config }: { token: string; config: ShowcaseConfig }) {
  const [kind, setKind] = useState<WorkFilter>("all");
  const [items, setItems] = useState<ShowcaseWork[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const requestID = useRef(0);

  useEffect(() => {
    let active = true;
    const currentRequest = ++requestID.current;
    setStatus("loading");
    getPublicShowcaseWorks(token, { kind, limit: 12 })
      .then((page) => {
        if (!active || currentRequest !== requestID.current) return;
        setItems(page.items);
        setNextCursor(page.nextCursor);
        setTotal(page.total);
        setStatus("ready");
      })
      .catch(() => active && currentRequest === requestID.current && setStatus("error"));
    return () => { active = false; };
  }, [kind, token]);

  const loadMore = () => {
    if (!nextCursor || status === "loading") return;
    const currentRequest = ++requestID.current;
    setStatus("loading");
    getPublicShowcaseWorks(token, { kind, limit: 12, cursor: nextCursor })
      .then((page) => {
        if (currentRequest !== requestID.current) return;
        setItems((current) => [...current, ...page.items.filter((item) => !current.some((old) => old.id === item.id))]);
        setNextCursor(page.nextCursor);
        setTotal(page.total);
        setStatus("ready");
      })
      .catch(() => currentRequest === requestID.current && setStatus("error"));
  };

  const theme = SHOWCASE_THEMES[config.palette];
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

  return (
    <main className="showcase-collection" style={style} data-layout={config.layout} data-style={config.style ?? "classic"}>
      <header>
        <a href={`/p/${encodeURIComponent(token)}`}>← 返回主页</a>
        <p>作品集</p>
        <h1>{config.name ? `${config.name}的作品` : "全部作品"}</h1>
        <span>{total} 项公开作品</span>
      </header>
      <nav aria-label="作品类型">
        {FILTERS.map((filter) => <button type="button" key={filter.value} aria-pressed={kind === filter.value} onClick={() => setKind(filter.value)}>{filter.label}</button>)}
      </nav>
      {status === "error" && items.length === 0 ? <p className="showcase-collection-status">加载作品失败，请刷新后重试。</p> : null}
      {status === "ready" && items.length === 0 ? <p className="showcase-collection-status">这一类还没有公开作品。</p> : null}
      <section className="showcase-collection-grid" aria-live="polite" aria-busy={status === "loading"}>
        {items.map((work, index) => {
          const href = workHref(work);
          const external = work.kind === "project" && Boolean(href);
          return <article key={work.id}>
            <div><span>{KIND_LABELS[work.kind]}</span><span>{String(index + 1).padStart(2, "0")}</span></div>
            <h2>{href ? <a href={href} target={external ? "_blank" : undefined} rel={external ? "noreferrer" : undefined}>{work.title}</a> : work.title}</h2>
            {work.summary && <p>{work.summary}</p>}
            {work.date && <time dateTime={work.date}>{work.date.replace(/-/g, ".")}</time>}
          </article>;
        })}
      </section>
      {nextCursor && <button className="showcase-collection-more" type="button" onClick={loadMore} disabled={status === "loading"}>{status === "loading" ? "加载中" : "加载更多"}</button>}
    </main>
  );
}
