import { COVER_ARTS } from "../data/projects";
import type { SiteContent, SiteTheme } from "../data/site";
import type { Cover } from "../data/types";
import { cx } from "../ui";

/**
 * 她的网站 — the actual thing the 个人主页 project builds.
 *
 * ## One component, three places
 * The build step's preview, the published URL, and the 我的主页 tab all render
 * THIS. A preview that is a different component is a lie waiting to happen:
 * she approves one thing and ships another.
 *
 * ## Why `narrow` is a prop and not a media query
 * The preview renders this inside a 390px frame that is scaled down on a
 * 1400px screen. A `md:` breakpoint would read the real viewport and lay the
 * phone preview out as a desktop — the preview would be wrong in exactly the
 * situation the student is checking it. So the layout branches on a prop.
 *
 * ## Why `stage`
 * 印记 builds this over three rounds and admits, each round, what is still
 * wrong with its own work. Those admissions have to be VISIBLE or they are
 * decoration: at stage 1 the headline really is too big and really does wrap
 * to three lines on a phone; the project photos really are missing until she
 * supplies them. Her feedback visibly changes the page.
 */
export function BuiltSite({
  site,
  theme,
  stage = 3,
  narrow = false,
  chrome = true,
}: {
  site: SiteContent;
  theme: SiteTheme;
  stage?: 1 | 2 | 3;
  narrow?: boolean;
  /** The published page draws its own top bar; embedded previews do not. */
  chrome?: boolean;
}) {
  const pad = narrow ? "px-5" : "px-10";
  return (
    <div
      className="eco-site min-h-full"
      style={
        {
          "--st-paper": theme.paper,
          "--st-ink": theme.ink,
          "--st-accent": theme.accent,
          background: theme.paper,
          color: theme.ink,
          fontFamily: theme.font,
        } as React.CSSProperties
      }
    >
      {chrome ? <TopBar site={site} narrow={narrow} pad={pad} /> : null}

      <div className={cx("mx-auto w-full max-w-[760px]", pad)}>
        <Hero site={site} stage={stage} narrow={narrow} />
        <Works site={site} stage={stage} narrow={narrow} />
        <Posts site={site} narrow={narrow} />
        <About site={site} narrow={narrow} />
        <Reads site={site} />
        <Foot site={site} stage={stage} narrow={narrow} />
      </div>
    </div>
  );
}

/** The site sets its own mono stack; `.eco-mono` belongs to the app chrome
 *  and hard-codes 10px + uppercase, which would leak into her page. */
const MONO = { fontFamily: 'ui-monospace,"SF Mono","PingFang SC",monospace' } as const;

/* ── pieces ───────────────────────────────────────────────────────────── */

const NAV = ["作品", "写的", "关于", "在读"];

function TopBar({ site, narrow, pad }: { site: SiteContent; narrow: boolean; pad: string }) {
  return (
    <div
      className="sticky top-0 z-10 w-full backdrop-blur"
      style={{
        background: `color-mix(in srgb, var(--st-paper) 88%, transparent)`,
        borderBottom: `1px solid ${hair(0.1)}`,
      }}
    >
      <div className={cx("mx-auto flex max-w-[760px] items-center justify-between py-3.5", pad)}>
        <span className="flex items-center gap-2">
          <span
            className="flex h-6 w-6 items-center justify-center rounded-full text-[12px] font-bold"
            style={{ background: "var(--st-accent)", color: "var(--st-paper)" }}
          >
            {site.name.slice(-2, -1)}
          </span>
          <span className="text-[15px] font-semibold tracking-tight">{site.name}</span>
        </span>
        <nav className="flex items-center gap-4">
          {(narrow ? NAV.slice(0, 3) : NAV).map((n) => (
            <span key={n} className="text-[13px]" style={{ color: soft(0.6) }}>
              {n}
            </span>
          ))}
        </nav>
      </div>
    </div>
  );
}

function Hero({ site, stage, narrow }: { site: SiteContent; stage: number; narrow: boolean }) {
  // 🚨 The stage-1 headline is deliberately oversized. That is the defect
  // 印记 admits in round one ("手机上第一屏那句话会断成三行") and she has to be
  // able to SEE it in the phone preview, otherwise her feedback is a guess.
  const size = narrow
    ? stage === 1
      ? "text-[40px] leading-[1.2]"
      : "text-[26px] leading-[1.35]"
    : stage === 1
      ? "text-[46px] leading-[1.18]"
      : "text-[40px] leading-[1.22]";

  return (
    <header className={cx(narrow ? "pt-10" : "pt-16")}>
      <span className="inline-flex items-center gap-2 text-[12px]" style={{ color: soft(0.62) }}>
        <span
          className="h-1.5 w-1.5 rounded-full"
          style={{ background: "var(--st-accent)" }}
        />
        现在 · {site.now}
      </span>

      <h1 className={cx("mt-5 font-bold tracking-tight", size)}>{site.headline}</h1>

      <p
        className={cx("mt-6 max-w-[620px]", narrow ? "text-[15px]" : "text-[17px]")}
        style={{ lineHeight: 1.95, color: soft(0.82) }}
      >
        {site.lead}
      </p>

      <div className="mt-7 flex flex-wrap items-center gap-x-5 gap-y-2">
        <Link label="看我做的" />
        <Link label="读我写的" />
        {stage >= 3 ? <Link label={site.email} accent /> : null}
      </div>
    </header>
  );
}

function Works({ site, stage, narrow }: { site: SiteContent; stage: number; narrow: boolean }) {
  return (
    <Section label="作品" title="我做过的">
      <div className={cx("grid gap-4", narrow ? "grid-cols-1" : "grid-cols-2")}>
        {site.projects.map((p) => (
          <article
            key={p.id}
            className="overflow-hidden rounded-[12px]"
            style={{ border: `1px solid ${hair(0.14)}`, background: soft(0.03) }}
          >
            {stage >= 3 ? (
              <CoverBand cover={p.cover} narrow={narrow} />
            ) : (
              <div
                className="flex items-center justify-center text-[12px]"
                style={{
                  height: narrow ? 76 : 104,
                  border: `1px dashed ${hair(0.28)}`,
                  margin: 10,
                  borderRadius: 8,
                  color: soft(0.45),
                }}
              >
                图位 · 等你拍的照片
              </div>
            )}
            <div className="px-4 pb-4 pt-3.5">
              <p className="text-[11px] tracking-wide" style={{ ...MONO, color: soft(0.5) }}>
                {p.year} · {p.kind}
              </p>
              <h3 className="mt-1.5 text-[17px] font-bold leading-snug">{p.title}</h3>
              <p className="mt-2 text-[14px]" style={{ lineHeight: 1.85, color: soft(0.78) }}>
                {p.line}
              </p>
              {p.why ? (
                <p
                  className="mt-3 border-l-2 pl-2.5 text-[13px]"
                  style={{ lineHeight: 1.75, color: soft(0.6), borderColor: "var(--st-accent)" }}
                >
                  为什么做它：{p.why}
                </p>
              ) : null}
            </div>
          </article>
        ))}
      </div>
    </Section>
  );
}

function Posts({ site, narrow }: { site: SiteContent; narrow: boolean }) {
  return (
    <Section label="写的" title="我写的">
      <ul>
        {site.posts.map((p, i) => (
          <li
            key={p.id}
            className={cx("flex gap-4 py-3.5", narrow && "flex-col gap-1")}
            style={i === 0 ? undefined : { borderTop: `1px solid ${hair(0.12)}` }}
          >
            <span
              className="shrink-0 pt-[3px] text-[12px]"
              style={{ ...MONO, color: soft(0.45), width: narrow ? undefined : 78 }}
            >
              {p.date.slice(2)}
            </span>
            <span className="min-w-0">
              <span className="block text-[16px] font-semibold leading-snug">{p.title}</span>
              <span className="mt-1 block text-[13px]" style={{ lineHeight: 1.8, color: soft(0.62) }}>
                {p.line}…
              </span>
            </span>
          </li>
        ))}
      </ul>
    </Section>
  );
}

function About({ site, narrow }: { site: SiteContent; narrow: boolean }) {
  return (
    <Section label="关于" title="关于我">
      {site.about.map((para) => (
        <p
          key={para.slice(0, 12)}
          className={cx("mb-4 max-w-[660px]", narrow ? "text-[15px]" : "text-[16px]")}
          style={{ lineHeight: 2, color: soft(0.85) }}
        >
          {para}
        </p>
      ))}

      <p
        className="mt-5 rounded-[10px] px-4 py-3.5 text-[15px]"
        style={{
          lineHeight: 1.9,
          background: soft(0.05),
          border: `1px solid ${hair(0.12)}`,
        }}
      >
        {site.detail}
      </p>

      <p className="mt-8 text-[11px] tracking-wide" style={{ ...MONO, color: soft(0.5) }}>
        现在
      </p>
      <ul className="mt-2.5 space-y-2">
        {site.nowList.map((n) => (
          <li key={n} className="flex gap-2.5 text-[15px]" style={{ lineHeight: 1.85 }}>
            <span style={{ color: "var(--st-accent)" }}>—</span>
            <span style={{ color: soft(0.8) }}>{n}</span>
          </li>
        ))}
      </ul>
    </Section>
  );
}

function Reads({ site }: { site: SiteContent }) {
  return (
    <Section label="在读" title="最近在读">
      <ul className="space-y-5">
        {site.reads.map((r) => (
          <li key={r.id}>
            <span className="flex flex-wrap items-baseline gap-x-2.5">
              <span className="text-[16px] font-semibold">{r.title}</span>
              <span className="text-[11px]" style={{ ...MONO, color: soft(0.45) }}>
                {r.source}
              </span>
            </span>
            <p
              className="mt-1.5 border-l-2 pl-3 text-[14px]"
              style={{ lineHeight: 1.85, color: soft(0.7), borderColor: "var(--st-accent)" }}
            >
              {r.takeaway}
            </p>
          </li>
        ))}
      </ul>
    </Section>
  );
}

function Foot({ site, stage, narrow }: { site: SiteContent; stage: number; narrow: boolean }) {
  return (
    <footer
      className={cx("mt-16 pb-16 pt-8", narrow && "pb-12")}
      style={{ borderTop: `1px solid ${hair(0.14)}` }}
    >
      {stage >= 3 ? (
        <>
          <p className="text-[15px] font-semibold">找我</p>
          <p className="mt-1.5 text-[15px]" style={{ color: "var(--st-accent)" }}>
            {site.email}
          </p>
        </>
      ) : (
        <p className="text-[13px]" style={{ color: soft(0.45) }}>
          （联系方式这一块还没加）
        </p>
      )}
      <p className="mt-6 text-[12px]" style={{ lineHeight: 1.8, color: soft(0.45) }}>
        这一页上的每一句话都是我自己写的。最后更新 {site.updated}。
        <br />
        {site.domain} · 用 思维印记 做的
      </p>
    </footer>
  );
}

/* ── small parts ──────────────────────────────────────────────────────── */

function Section({
  label,
  title,
  children,
}: {
  label: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="mt-16">
      <p className="text-[11px] tracking-[0.18em]" style={{ ...MONO, color: "var(--st-accent)" }}>
        {label}
      </p>
      <h2 className="mt-2 text-[22px] font-bold tracking-tight">{title}</h2>
      <div className="mt-5">{children}</div>
    </section>
  );
}

function Link({ label, accent = false }: { label: string; accent?: boolean }) {
  return (
    <span
      className="border-b pb-0.5 text-[14px]"
      style={{
        color: accent ? "var(--st-accent)" : soft(0.75),
        borderColor: accent ? "var(--st-accent)" : hair(0.3),
      }}
    >
      {label}
    </span>
  );
}

function CoverBand({ cover, narrow }: { cover: Cover; narrow: boolean }) {
  const art = COVER_ARTS[cover.art] ?? COVER_ARTS[0]!;
  return (
    <div
      className="flex items-center justify-center"
      style={{
        height: narrow ? 84 : 112,
        background: `linear-gradient(140deg, ${art.from}, ${art.to})`,
        color: art.ink,
        fontSize: narrow ? 30 : 38,
      }}
      aria-hidden
    >
      {cover.glyph}
    </div>
  );
}

/* Ink mixed toward paper — text at 45–85%, backgrounds at 3–5%.
 * 🚨 Written as `color-mix` and never as a Tailwind alpha modifier: these are
 * bare CSS variables, and `text-[var(--x)]/60` silently emits no CSS at all. */
function soft(amount: number): string {
  return `color-mix(in srgb, var(--st-ink) ${Math.round(amount * 100)}%, var(--st-paper))`;
}

function hair(amount: number): string {
  return `color-mix(in srgb, var(--st-ink) ${Math.round(amount * 100)}%, transparent)`;
}
