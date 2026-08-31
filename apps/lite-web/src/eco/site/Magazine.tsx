import { Banner, Ground, MONO, Plate, hair, mix, type LayoutProps } from "./parts";
import { cx } from "../ui";

/**
 * 方案 C · 一个作品打头 — 图先行 · 强对比.
 *
 * This is the shape a Chinese personal blog actually has, and the references
 * (keyork.github.io, terrifyzhao.github.io) agree on every piece of it:
 * a full-bleed banner with the site's name and a spaced 分类 line, a nav strip
 * under it, then two columns — post cards on the left, and a sidebar carrying
 * the avatar, the one-line self-description, 文章分类 with counts, 标签, and
 * 站点信息.
 *
 * That furniture is the reason it reads as a blog in the first half-second.
 *
 * 🚨 The banner is the SITE'S OWN artwork (`Banner` in `parts.tsx`), never a
 * crop of one of her projects. It does a different job from a thumbnail: it
 * says whose page this is before a word is read. A project image up there
 * makes the top of the page look like the first row of a list.
 */
export function Magazine({ site, theme, stage, narrow }: LayoutProps) {
  const lead = site.projects[0];

  return (
    <Ground theme={theme}>
      {/* 头图：这一页自己的画，不是作品缩略图 */}
      <div className="relative">
        <Banner height={narrow ? 250 : 400} ink={theme.ink} />
        <div
          className="absolute inset-0 flex flex-col items-center justify-center px-6 text-center"
          style={{ background: "linear-gradient(180deg,rgba(12,10,20,.18),rgba(12,10,20,.42))" }}
        >
          <h1
            className={cx("font-bold text-white", narrow ? "text-[30px]" : "text-[48px]")}
            style={{ letterSpacing: "-0.01em", textShadow: "0 2px 24px rgba(0,0,0,.35)" }}
          >
            {site.name}
          </h1>
          <p
            className={cx("mt-4", narrow ? "text-[11px]" : "text-[12.5px]")}
            style={{ ...MONO, color: "rgba(255,255,255,.82)", letterSpacing: "0.42em" }}
          >
            {site.motto.join(" · ")}
          </p>
        </div>
      </div>

      {/* 导航条 */}
      <nav
        className="flex flex-wrap items-center justify-center gap-x-8 gap-y-2 py-4"
        style={{ borderBottom: `1px solid ${hair(0.12)}` }}
      >
        {["文章", "项目", "标签", "关于"].map((n, i) => (
          <span
            key={n}
            className="text-[14px]"
            style={{ color: i === 0 ? "var(--st-accent)" : mix(0.6) }}
          >
            {n}
          </span>
        ))}
      </nav>

      <div
        className={cx(
          "mx-auto w-full max-w-[1040px] gap-8 px-5 py-10",
          narrow ? "" : "flex px-8 py-12",
        )}
      >
        {/* 主栏 */}
        <main className={cx("min-w-0", narrow ? "" : "flex-1")}>
          <h2 className={cx("font-bold", narrow ? "text-[22px]" : "text-[26px]")}>
            {site.headline}
          </h2>
          <p
            className={cx("mt-3", narrow ? "text-[15px]" : "text-[16px]")}
            style={{ lineHeight: 1.95, color: mix(0.7) }}
          >
            {site.lead}
          </p>

          <div className="mt-9 space-y-5">
            {site.posts.map((p) => (
              <article
                key={p.id}
                className="rounded-[6px] p-5"
                style={{ border: `1px solid ${hair(0.13)}`, background: mix(0.02) }}
              >
                <h3 className={cx("font-bold", narrow ? "text-[18px]" : "text-[20px]")}>
                  {p.title}
                </h3>
                <p
                  className="mt-2 text-[15px]"
                  style={{ lineHeight: 1.9, color: mix(0.68) }}
                >
                  {p.blurb}
                </p>
                <div
                  className="mt-4 flex flex-wrap items-center justify-between gap-2 pt-3"
                  style={{ borderTop: `1px solid ${hair(0.1)}` }}
                >
                  <span className="text-[11.5px]" style={{ ...MONO, color: mix(0.42) }}>
                    {p.date} · {p.kind} · {p.words} 字
                  </span>
                  <span className="text-[13px]" style={{ color: "var(--st-accent)" }}>
                    阅读全文 →
                  </span>
                </div>
              </article>
            ))}
          </div>

          {/* 作品：小图排在文章后面 */}
          {site.projects.length ? (
            <>
              <h3
                className="mt-12 text-[12px] tracking-[0.22em]"
                style={{ ...MONO, color: "var(--st-accent)" }}
              >
                项目
              </h3>
              <div className={cx("mt-5 grid gap-5", narrow ? "grid-cols-1" : "grid-cols-2")}>
                {site.projects.map((p) => (
                  <article key={p.id}>
                    <Plate
                      plate={p.plate}
                      height={narrow ? 140 : 160}
                      filled={stage >= 3}
                      radius={4}
                    />
                    <p
                      className="mt-3 text-[11.5px] tracking-[0.08em]"
                      style={{ ...MONO, color: mix(0.42) }}
                    >
                      {p.year} · {p.kind}
                    </p>
                    <h4 className="mt-1.5 text-[17px] font-bold">{p.title}</h4>
                    <p className="mt-2 text-[14px]" style={{ lineHeight: 1.85, color: mix(0.66) }}>
                      {p.blurb}
                    </p>
                  </article>
                ))}
              </div>
            </>
          ) : null}
        </main>

        {/* 侧栏 */}
        <aside className={cx("shrink-0", narrow ? "mt-12" : "w-[264px]")}>
          <Box>
            <div className="flex flex-col items-center text-center">
              <span
                className="flex h-16 w-16 items-center justify-center rounded-full text-[22px] font-bold"
                style={{
                  background: lead
                    ? `radial-gradient(120% 100% at 30% 20%, ${lead.plate[0]}, ${lead.plate[1]})`
                    : "var(--st-accent)",
                  color: "#fff",
                }}
              >
                {site.name.slice(-2, -1)}
              </span>
              <p className="mt-3 text-[16px] font-semibold">{site.name}</p>
              <p className="mt-1 text-[12.5px]" style={{ lineHeight: 1.7, color: mix(0.55) }}>
                {site.role}
              </p>
              {stage >= 3 ? (
                <a
                  href={`mailto:${site.email}`}
                  className="mt-3 text-[12.5px]"
                  style={{ ...MONO, color: "var(--st-accent)" }}
                >
                  {site.email}
                </a>
              ) : null}
            </div>
          </Box>

          <Box title="文章分类">
            {countBy(site.posts.map((p) => p.kind)).map(([kind, n]) => (
              <div key={kind} className="flex items-baseline justify-between py-1.5 text-[13px]">
                <span style={{ color: mix(0.7) }}>{kind}</span>
                <span style={{ ...MONO, color: mix(0.4) }}>({n})</span>
              </div>
            ))}
          </Box>

          <Box title="标签">
            <div className="flex flex-wrap gap-x-3 gap-y-2">
              {site.tags.map((t, i) => (
                <span
                  key={t}
                  style={{
                    color: mix(0.68),
                    fontSize: [15, 13, 14, 12, 13, 12][i % 6],
                  }}
                >
                  {t}
                </span>
              ))}
            </div>
          </Box>

          <Box title="站点信息">
            {[...site.stats, { label: "最后更新", value: site.updated }].map((row) => (
              <div key={row.label} className="flex items-baseline justify-between py-1.5 text-[13px]">
                <span style={{ color: mix(0.55) }}>{row.label}</span>
                <span style={{ color: mix(0.75) }}>{row.value}</span>
              </div>
            ))}
          </Box>

          <Box title="现在">
            {site.nowList.map((n) => (
              <p key={n} className="py-1 text-[13px]" style={{ lineHeight: 1.8, color: mix(0.66) }}>
                {n}
              </p>
            ))}
          </Box>
        </aside>
      </div>

      <footer
        className="px-6 py-8 text-center text-[11.5px]"
        style={{ ...MONO, color: mix(0.4), borderTop: `1px solid ${hair(0.12)}` }}
      >
        © 2026 {site.name} · {site.domain} · 用 思维印记 搭建
      </footer>
    </Ground>
  );
}

function Box({ title, children }: { title?: string; children: React.ReactNode }) {
  return (
    <section
      className="mb-5 rounded-[6px] p-4"
      style={{ border: `1px solid ${hair(0.13)}`, background: mix(0.02) }}
    >
      {title ? (
        <p
          className="mb-2 pb-2 text-[12px] tracking-[0.16em]"
          style={{ ...MONO, color: mix(0.5), borderBottom: `1px solid ${hair(0.1)}` }}
        >
          {title}
        </p>
      ) : null}
      {children}
    </section>
  );
}

function countBy(xs: string[]): [string, number][] {
  const m = new Map<string, number>();
  for (const x of xs) m.set(x, (m.get(x) ?? 0) + 1);
  return [...m.entries()];
}
