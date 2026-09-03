import { Blank, Ground, MONO, Plate, cx, hair, mix, type LayoutProps } from "./parts";

/**
 * 方案 A · 一句话开场 — 长页 · 无导航 · 很空.
 *
 * 她读到的三句话是：第一屏只有一句话、往下滚是她做过的事、没有导航栏因为只有
 * 一页。所以这里没有导航栏。
 *
 * 手感的参考是 lixiaolai.com：一句很大的衬线字，底下一行名字和身份，右边一张
 * 等宽小字的资料表。让它读起来像一个人的站而不是一个产品的，是那个比例——一句
 * 巨大的话，压着一大片安静而精确的小字。
 *
 * 🚨 空的地方是空的。原型在这里有一整套兜底文案，于是一个什么都没填的页面看上
 * 去是满的——那正是「这一页写的是别人」这个 bug 藏身的地方。
 */
export function Essay({ site, theme, narrow, editing }: LayoutProps) {
  const pad = narrow ? "px-6" : "px-12";
  const h1 = narrow ? "text-[36px] leading-[1.24]" : "text-[62px] leading-[1.1]";

  return (
    <Ground theme={theme}>
      <div className={cx("mx-auto w-full max-w-[880px]", pad)}>
        {/* 报头：一行小字，没有导航。这里放她的名字，不是一个我们编的域名。 */}
        <div
          className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1 pt-8 pb-16 text-[11px] tracking-[0.16em]"
          style={{ ...MONO, color: mix(0.45) }}
        >
          <span>{site.name}</span>
          {site.updated ? <span>最后更新 {site.updated}</span> : null}
        </div>

        {/* 第一屏：一句话 */}
        <header className={cx(narrow ? "pb-10" : "pb-14")}>
          {site.headline ? (
            <h1 className={cx("font-bold", h1)}>{site.headline}</h1>
          ) : (
            <Blank what="首屏那句话还没写" editing={editing} />
          )}

          <div className={cx("mt-12 gap-12", narrow ? "" : "flex")}>
            <div className={cx(narrow ? "" : "flex-1")}>
              <p className={cx("font-semibold", narrow ? "text-[17px]" : "text-[19px]")}>
                {site.name}
              </p>
              {site.role ? (
                <p className="mt-1.5 text-[14px]" style={{ color: mix(0.55) }}>
                  {site.role}
                </p>
              ) : (
                <p className="mt-1.5">
                  <Blank what="你是谁，一行" editing={editing} />
                </p>
              )}
              {site.lead ? (
                <p
                  className={cx("mt-6 max-w-[40em]", narrow ? "text-[15px]" : "text-[17px]")}
                  style={{ lineHeight: 2.05, color: mix(0.8) }}
                >
                  {site.lead}
                </p>
              ) : null}
              {site.email ? (
                <a
                  href={`mailto:${site.email}`}
                  className="mt-6 inline-block border-b pb-0.5 text-[14px]"
                  style={{ ...MONO, color: "var(--st-accent)", borderColor: "var(--st-accent)" }}
                >
                  {site.email}
                </a>
              ) : null}
            </div>

            {/* 小字资料表 */}
            {site.stats.length || site.now ? (
              <dl
                className={cx("shrink-0", narrow ? "mt-10" : "w-[280px]")}
                style={{ ...MONO, borderTop: `1px solid ${hair(0.18)}` }}
              >
                {[...site.stats, ...(site.now ? [{ label: "现在", value: site.now }] : [])].map(
                  (row) => (
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
                  ),
                )}
              </dl>
            ) : null}
          </div>
        </header>

        {site.posts.length ? (
          <>
            <Head n={site.posts.length}>文章</Head>
            <ul className="mt-8">
              {site.posts.map((p, i) => (
                <li
                  key={p.id}
                  className="py-7"
                  style={i === 0 ? undefined : { borderTop: `1px solid ${hair(0.12)}` }}
                >
                  <h3 className={cx("font-bold", narrow ? "text-[19px]" : "text-[23px]")}>
                    {p.title}
                  </h3>
                  {p.blurb ? (
                    <p
                      className={cx("mt-2.5", narrow ? "text-[15px]" : "text-[16px]")}
                      style={{ lineHeight: 1.95, color: mix(0.72) }}
                    >
                      {p.blurb}
                    </p>
                  ) : (
                    <p className="mt-2.5">
                      <Blank what="给这篇写一句" editing={editing} />
                    </p>
                  )}
                  <p
                    className="mt-3 text-[11.5px] tracking-[0.08em]"
                    style={{ ...MONO, color: mix(0.42) }}
                  >
                    {[p.date, p.kind, p.words ? `${p.words} 字` : ""].filter(Boolean).join(" · ")}
                  </p>
                </li>
              ))}
            </ul>
          </>
        ) : null}

        {site.projects.length ? (
          <>
            <Head n={site.projects.length}>项目</Head>
            <div className="mt-8 space-y-16">
              {site.projects.map((p) => (
                <article key={p.id}>
                  <Plate plate={p.plate} height={narrow ? 170 : 260} radius={2} />
                  <p
                    className="mt-5 text-[11.5px] tracking-[0.1em]"
                    style={{ ...MONO, color: mix(0.45) }}
                  >
                    {[p.year, p.kind].filter(Boolean).join(" · ")}
                  </p>
                  <h3 className={cx("mt-2 font-bold", narrow ? "text-[21px]" : "text-[26px]")}>
                    {p.title}
                  </h3>
                  {p.blurb ? (
                    <p
                      className={cx("mt-3 max-w-[42em]", narrow ? "text-[15px]" : "text-[16px]")}
                      style={{ lineHeight: 2, color: mix(0.76) }}
                    >
                      {p.blurb}
                    </p>
                  ) : (
                    <p className="mt-3">
                      <Blank what="这件事你想怎么说" editing={editing} />
                    </p>
                  )}
                </article>
              ))}
            </div>
          </>
        ) : null}

        {site.about.length || site.nowList.length ? (
          <>
            <Head>关于</Head>
            <div className="mt-8 max-w-[42em]">
              {site.about.map((para) => (
                <p
                  key={para.slice(0, 12)}
                  className={cx("mb-6", narrow ? "text-[15px]" : "text-[16.5px]")}
                  style={{ lineHeight: 2.15, color: mix(0.84) }}
                >
                  {para}
                </p>
              ))}
              {site.nowList.length ? (
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
              ) : null}
            </div>
          </>
        ) : null}

        {site.reads.length ? (
          <>
            <Head>在读</Head>
            <ul className="mt-8 space-y-7">
              {site.reads.map((r) => (
                <li key={r.id}>
                  <p className={cx("font-semibold", narrow ? "text-[16px]" : "text-[17px]")}>
                    {r.title}
                    {r.source ? (
                      <span
                        className="ml-3 text-[11px] font-normal tracking-[0.08em]"
                        style={{ ...MONO, color: mix(0.42) }}
                      >
                        {r.source}
                      </span>
                    ) : null}
                  </p>
                  {r.takeaway ? (
                    <p className="mt-2 text-[15px]" style={{ lineHeight: 1.95, color: mix(0.68) }}>
                      {r.takeaway}
                    </p>
                  ) : null}
                </li>
              ))}
            </ul>
          </>
        ) : null}

        <footer
          className={cx("mt-28 pt-6", narrow ? "pb-16" : "pb-24")}
          style={{ borderTop: `1px solid ${hair(0.16)}` }}
        >
          <p className="text-[11px] tracking-[0.14em]" style={{ ...MONO, color: mix(0.4) }}>
            © {new Date().getFullYear()} {site.name} · 用 思维印记 搭建
          </p>
        </footer>
      </div>
    </Ground>
  );
}

/** 每个区块一个标题，用人们真的会用的词，带一个计数。 */
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
