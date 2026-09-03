import { Banner, Blank, Ground, MONO, Plate, cx, hair, mix, type LayoutProps } from "./parts";

/**
 * 方案 C · 一个作品打头 — 图先行 · 强对比.
 *
 * 这是一个中文个人博客实际的样子，参考站（keyork、terrifyzhao 那一类）在每一
 * 块上都一致：一张通栏头图压着站名和一行拉开字距的分类，底下一条导航，再往下
 * 两栏——左边文章卡片，右边侧栏放着头像、一行自我介绍、文章分类和计数、标签、
 * 站点信息。
 *
 * 让它在半秒内就被读成一个博客的，正是这些家具。
 *
 * 🚨 头图是**这一页自己的画**（parts.tsx 的 Banner），由她的名字派生，绝不是她
 * 某个作品的裁切。两者做的是不同的事：头图在读到第一个字之前说清这是谁的页面，
 * 作品缩略图是列表里的一行。把作品图放上去，页顶就变成了列表的第一行。
 */
export function Magazine({ site, theme, narrow, editing }: LayoutProps) {
  const lead = site.projects[0];
  const avatar = site.name.trim().slice(-1) || "·";

  return (
    <Ground theme={theme}>
      {/* 头图：这一页自己的画 */}
      <div className="relative">
        <Banner height={narrow ? 250 : 400} seed={site.seed} />
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
          {site.motto.length ? (
            <p
              className={cx("mt-4", narrow ? "text-[11px]" : "text-[12.5px]")}
              style={{ ...MONO, color: "rgba(255,255,255,.82)", letterSpacing: "0.42em" }}
            >
              {site.motto.join(" · ")}
            </p>
          ) : null}
        </div>
      </div>

      {/* 导航条 */}
      <nav
        className="flex flex-wrap items-center justify-center gap-x-8 gap-y-2 py-4"
        style={{ borderBottom: `1px solid ${hair(0.12)}` }}
      >
        {["文章", "项目", "在读", "关于"].map((n, i) => (
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
          {site.headline ? (
            <h2 className={cx("font-bold", narrow ? "text-[22px]" : "text-[26px]")}>
              {site.headline}
            </h2>
          ) : (
            <Blank what="首屏那句话还没写" editing={editing} />
          )}
          {site.lead ? (
            <p
              className={cx("mt-3", narrow ? "text-[15px]" : "text-[16px]")}
              style={{ lineHeight: 1.95, color: mix(0.7) }}
            >
              {site.lead}
            </p>
          ) : null}

          {site.posts.length ? (
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
                  {p.blurb ? (
                    <p className="mt-2 text-[15px]" style={{ lineHeight: 1.9, color: mix(0.68) }}>
                      {p.blurb}
                    </p>
                  ) : (
                    <p className="mt-2">
                      <Blank what="给这篇写一句" editing={editing} />
                    </p>
                  )}
                  <div
                    className="mt-4 flex flex-wrap items-center justify-between gap-2 pt-3"
                    style={{ borderTop: `1px solid ${hair(0.1)}` }}
                  >
                    <span className="text-[11.5px]" style={{ ...MONO, color: mix(0.42) }}>
                      {[p.date, p.kind, p.words ? `${p.words} 字` : ""].filter(Boolean).join(" · ")}
                    </span>
                  </div>
                </article>
              ))}
            </div>
          ) : null}

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
                    <Plate plate={p.plate} height={narrow ? 140 : 160} radius={4} />
                    <p
                      className="mt-3 text-[11.5px] tracking-[0.08em]"
                      style={{ ...MONO, color: mix(0.42) }}
                    >
                      {[p.year, p.kind].filter(Boolean).join(" · ")}
                    </p>
                    <h4 className="mt-1.5 text-[17px] font-bold">{p.title}</h4>
                    {p.blurb ? (
                      <p className="mt-2 text-[14px]" style={{ lineHeight: 1.85, color: mix(0.66) }}>
                        {p.blurb}
                      </p>
                    ) : (
                      <p className="mt-2">
                        <Blank what="这件事你想怎么说" editing={editing} />
                      </p>
                    )}
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
                {avatar}
              </span>
              <p className="mt-3 text-[16px] font-semibold">{site.name}</p>
              {site.role ? (
                <p className="mt-1 text-[12.5px]" style={{ lineHeight: 1.7, color: mix(0.55) }}>
                  {site.role}
                </p>
              ) : (
                <p className="mt-1">
                  <Blank what="你是谁，一行" editing={editing} />
                </p>
              )}
              {site.email ? (
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

          {site.tags.length ? (
            <Box title="标签">
              <div className="flex flex-wrap gap-x-3 gap-y-2">
                {site.tags.map((t, i) => (
                  <span key={t} style={{ color: mix(0.68), fontSize: [15, 13, 14, 12, 13, 12][i % 6] }}>
                    {t}
                  </span>
                ))}
              </div>
            </Box>
          ) : null}

          {site.stats.length || site.updated ? (
            <Box title="站点信息">
              {[
                ...site.stats,
                ...(site.updated ? [{ label: "最后更新", value: site.updated }] : []),
              ].map((row) => (
                <div
                  key={row.label}
                  className="flex items-baseline justify-between py-1.5 text-[13px]"
                >
                  <span style={{ color: mix(0.55) }}>{row.label}</span>
                  <span style={{ color: mix(0.75) }}>{row.value}</span>
                </div>
              ))}
            </Box>
          ) : null}

          {site.nowList.length ? (
            <Box title="现在">
              {site.nowList.map((n) => (
                <p key={n} className="py-1 text-[13px]" style={{ lineHeight: 1.8, color: mix(0.66) }}>
                  {n}
                </p>
              ))}
            </Box>
          ) : null}

          {site.reads.length ? (
            <Box title="在读">
              {site.reads.map((r) => (
                <div key={r.id} className="py-1.5">
                  <p className="text-[13px]" style={{ lineHeight: 1.7, color: mix(0.75) }}>
                    {r.title}
                  </p>
                  {r.source ? (
                    <p className="text-[11.5px]" style={{ ...MONO, color: mix(0.4) }}>
                      {r.source}
                    </p>
                  ) : null}
                </div>
              ))}
            </Box>
          ) : null}
        </aside>
      </div>

      {site.about.length ? (
        <section
          className={cx("mx-auto w-full max-w-[1040px] px-5 pb-12", narrow ? "" : "px-8")}
          style={{ borderTop: `1px solid ${hair(0.12)}` }}
        >
          <h3
            className="mt-10 text-[12px] tracking-[0.22em]"
            style={{ ...MONO, color: "var(--st-accent)" }}
          >
            关于
          </h3>
          <div className="mt-4 max-w-[46em]">
            {site.about.map((para) => (
              <p
                key={para.slice(0, 12)}
                className="mb-4 text-[15px]"
                style={{ lineHeight: 2, color: mix(0.8) }}
              >
                {para}
              </p>
            ))}
          </div>
        </section>
      ) : null}

      <footer
        className="px-6 py-8 text-center text-[11.5px]"
        style={{ ...MONO, color: mix(0.4), borderTop: `1px solid ${hair(0.12)}` }}
      >
        © {new Date().getFullYear()} {site.name} · 用 思维印记 搭建
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
