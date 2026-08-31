import { Ground, MONO, Plate, hair, mix, type LayoutProps } from "./parts";
import { cx } from "../ui";

/**
 * 方案 A · 一句话开场 — 长页 · 无导航 · 很空.
 *
 * Her option says it in three bullets: the first screen is one sentence, then
 * the things she made with a line each, and **no navigation bar, because there
 * is only one page**. So there is no navigation bar.
 *
 * The reference for the feel is lixiaolai.com: a very large serif statement, a
 * name-and-role line under it, and a mono small-caps metadata table off to the
 * side. What makes that read as a person's site rather than a product is the
 * ratio — one enormous sentence against a lot of quiet, precise small type.
 */
export function Essay({ site, theme, stage, narrow }: LayoutProps) {
  const pad = narrow ? "px-6" : "px-12";
  // 🚨 印记 admits at round one that this line breaks into three on a phone. At
  // this size it genuinely does — which is what makes the admission something
  // she can check instead of something she has to believe.
  const h1 = narrow
    ? stage === 1
      ? "text-[40px] leading-[1.18]"
      : "text-[28px] leading-[1.4]"
    : stage === 1
      ? "text-[68px] leading-[1.1]"
      : "text-[56px] leading-[1.16]";

  return (
    <Ground theme={theme}>
      <div className={cx("mx-auto w-full max-w-[880px]", pad)}>
        {/* 报头：一行小字，没有导航 */}
        <div
          className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1 pt-8 pb-16 text-[11px] tracking-[0.16em]"
          style={{ ...MONO, color: mix(0.45) }}
        >
          <span>{site.domain.toUpperCase()}</span>
          <span>最后更新 {site.updated}</span>
        </div>

        {/* 第一屏：一句话 */}
        <header className={cx(narrow ? "pb-10" : "pb-14")}>
          <h1 className={cx("font-bold", h1)}>{site.headline}</h1>

          <div className={cx("mt-12 gap-12", narrow ? "" : "flex")}>
            <div className={cx(narrow ? "" : "flex-1")}>
              <p className={cx("font-semibold", narrow ? "text-[17px]" : "text-[19px]")}>
                {site.name}
              </p>
              <p className="mt-1.5 text-[14px]" style={{ color: mix(0.55) }}>
                {site.role}
              </p>
              <p
                className={cx("mt-6 max-w-[40em]", narrow ? "text-[15px]" : "text-[17px]")}
                style={{ lineHeight: 2.05, color: mix(0.8) }}
              >
                {site.lead}
              </p>
              {stage >= 3 ? (
                <a
                  href={`mailto:${site.email}`}
                  className="mt-6 inline-block border-b pb-0.5 text-[14px]"
                  style={{ ...MONO, color: "var(--st-accent)", borderColor: "var(--st-accent)" }}
                >
                  {site.email}
                </a>
              ) : (
                <p className="mt-6 text-[13px]" style={{ color: mix(0.4) }}>
                  （联系方式还没加上）
                </p>
              )}
            </div>

            {/* 小字资料表 */}
            <dl
              className={cx("shrink-0", narrow ? "mt-10" : "w-[280px]")}
              style={{ ...MONO, borderTop: `1px solid ${hair(0.18)}` }}
            >
              {[...site.stats, { label: "现在", value: site.now }].map((row) => (
                <div
                  key={row.label}
                  className="flex gap-4 py-2.5"
                  style={{ borderBottom: `1px solid ${hair(0.1)}` }}
                >
                  <dt
                    className="w-14 shrink-0 text-[11px] tracking-[0.14em]"
                    style={{ color: mix(0.45) }}
                  >
                    {row.label}
                  </dt>
                  <dd className="text-[12.5px]" style={{ lineHeight: 1.7, color: mix(0.75) }}>
                    {row.value}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        </header>

        <Head n={site.posts.length}>文章</Head>
        <ul className="mt-8">
          {site.posts.map((p, i) => (
            <li
              key={p.id}
              className="py-7"
              style={i === 0 ? undefined : { borderTop: `1px solid ${hair(0.12)}` }}
            >
              <h3 className={cx("font-bold", narrow ? "text-[19px]" : "text-[23px]")}>{p.title}</h3>
              <p
                className={cx("mt-2.5", narrow ? "text-[15px]" : "text-[16px]")}
                style={{ lineHeight: 1.95, color: mix(0.72) }}
              >
                {p.blurb}
              </p>
              <p className="mt-3 text-[11.5px] tracking-[0.08em]" style={{ ...MONO, color: mix(0.42) }}>
                {p.date} · {p.kind} · {p.words} 字
              </p>
            </li>
          ))}
        </ul>

        <Head n={site.projects.length}>项目</Head>
        <div className="mt-8 space-y-16">
          {site.projects.map((p) => (
            <article key={p.id}>
              <Plate plate={p.plate} height={narrow ? 170 : 260} filled={stage >= 3} radius={2} />
              <p className="mt-5 text-[11.5px] tracking-[0.1em]" style={{ ...MONO, color: mix(0.45) }}>
                {p.year} · {p.kind}
              </p>
              <h3 className={cx("mt-2 font-bold", narrow ? "text-[21px]" : "text-[26px]")}>
                {p.title}
              </h3>
              <p
                className={cx("mt-3 max-w-[42em]", narrow ? "text-[15px]" : "text-[16px]")}
                style={{ lineHeight: 2, color: mix(0.76) }}
              >
                {p.blurb}
              </p>
            </article>
          ))}
        </div>

        <Head>关于</Head>
        <div className="mt-8 max-w-[42em]">
          {site.about.map((para) => (
            <p
              key={para.slice(0, 10)}
              className={cx("mb-6", narrow ? "text-[15px]" : "text-[16.5px]")}
              style={{ lineHeight: 2.15, color: mix(0.84) }}
            >
              {para}
            </p>
          ))}
          <ul className="mt-10 space-y-2.5" style={{ borderTop: `1px solid ${hair(0.12)}` }}>
            {site.nowList.map((n) => (
              <li key={n} className="flex gap-3 pt-3 text-[15px]" style={{ lineHeight: 1.9 }}>
                <span className="text-[11px]" style={{ ...MONO, color: "var(--st-accent)" }}>
                  现在
                </span>
                <span style={{ color: mix(0.72) }}>{n}</span>
              </li>
            ))}
          </ul>
        </div>

        <Head>在读</Head>
        <ul className="mt-8 space-y-7">
          {site.reads.map((r) => (
            <li key={r.id}>
              <p className={cx("font-semibold", narrow ? "text-[16px]" : "text-[17px]")}>
                {r.title}
                <span
                  className="ml-3 text-[11px] font-normal tracking-[0.08em]"
                  style={{ ...MONO, color: mix(0.42) }}
                >
                  {r.source}
                </span>
              </p>
              <p className="mt-2 text-[15px]" style={{ lineHeight: 1.95, color: mix(0.68) }}>
                {r.takeaway}
              </p>
            </li>
          ))}
        </ul>

        <footer
          className={cx("mt-28 pt-6", narrow ? "pb-16" : "pb-24")}
          style={{ borderTop: `1px solid ${hair(0.16)}` }}
        >
          <p className="text-[11px] tracking-[0.14em]" style={{ ...MONO, color: mix(0.4) }}>
            © 2026 {site.name} · 用 思维印记 搭建
          </p>
        </footer>
      </div>
    </Ground>
  );
}

/** One label per section, in the words people actually use, with a count —
 *  the way a blog's own section headers read. */
function Head({ children, n }: { children: React.ReactNode; n?: number }) {
  return (
    <div
      className="mt-24 flex items-baseline gap-3 pb-1"
      style={{ borderBottom: `1px solid ${hair(0.18)}` }}
    >
      <h2 className="text-[12px] tracking-[0.24em]" style={{ ...MONO, color: "var(--st-accent)" }}>
        {children}
      </h2>
      {n ? (
        <span className="text-[11px]" style={{ ...MONO, color: mix(0.35) }}>
          {n}
        </span>
      ) : null}
    </div>
  );
}
