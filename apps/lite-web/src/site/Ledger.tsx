import { Blank, Ground, MONO, cx, hair, mix, type LayoutProps } from "./parts";

/**
 * 方案 B · 索引式 — 密 · 像一份档案柜.
 *
 * 她读到的是：*首页是一张列表：日期 + 标题 + 一句话，一屏看到十几条。顶部一行
 * 小字说明这里是什么。密度优先。* 所以这里是一张表，不是一摞分了区的段落——她
 * 做的、写的、读的，全在一张按日期排的索引里，带一列分类。小字、紧行距、细线，
 * 没有卡片，没有图。
 *
 * 它和 `Essay` 是故意相反的两页。如果两个选项都做成了一列带标题的区块，那她写
 * 了理由的那个选择，就退化成了换一次配色。
 */
export function Ledger({ site, theme, narrow, editing }: LayoutProps) {
  const pad = narrow ? "px-5" : "px-9";

  type Row = { id: string; when: string; kind: string; title: string; blurb: string };
  const rows: Row[] = [
    ...site.projects.map((p) => ({
      id: p.id,
      when: p.year,
      kind: p.kind,
      title: p.title,
      blurb: p.blurb,
    })),
    ...site.posts.map((p) => ({
      id: p.id,
      when: p.date,
      kind: "写的",
      title: p.title,
      blurb: p.blurb,
    })),
    ...site.reads.map((r) => ({
      id: r.id,
      when: "",
      kind: "在读",
      title: r.title,
      blurb: [r.source, r.takeaway].filter(Boolean).join("　"),
    })),
  ];
  // 一份索引不按日期排就不是索引。有日期的新的在前，没日期的沉到底。
  rows.sort((a, b) => {
    if (!a.when && !b.when) return 0;
    if (!a.when) return 1;
    if (!b.when) return -1;
    return a.when < b.when ? 1 : -1;
  });

  return (
    <Ground theme={theme}>
      <div className={cx("mx-auto w-full max-w-[860px]", pad, narrow ? "py-10" : "py-16")}>
        {/* 顶部一行小字说明这里是什么 */}
        <header style={{ ...MONO }}>
          <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1">
            <p className={cx("font-semibold", narrow ? "text-[13px]" : "text-[14px]")}>
              {site.name}
            </p>
            {site.role ? (
              <p className="text-[11px] tracking-[0.14em]" style={{ color: mix(0.42) }}>
                {site.role}
              </p>
            ) : (
              <Blank what="你是谁，一行" editing={editing} />
            )}
          </div>
          {site.headline ? (
            <p
              className={cx(
                "mt-4 font-semibold",
                narrow ? "text-[18px] leading-[1.45]" : "text-[22px] leading-[1.45]",
              )}
            >
              {site.headline}
            </p>
          ) : (
            <p className="mt-4">
              <Blank what="首屏那句话还没写" editing={editing} />
            </p>
          )}
          {site.lead ? (
            <p className="mt-3 text-[13px]" style={{ lineHeight: 1.85, color: mix(0.62) }}>
              {site.lead}
            </p>
          ) : null}
          {site.now || site.email ? (
            <p className="mt-3 text-[13px]" style={{ color: mix(0.5) }}>
              {site.now ? `现在：${site.now}` : ""}
              {site.email ? (
                <>
                  {site.now ? "　" : ""}
                  <a href={`mailto:${site.email}`} style={{ color: "var(--st-accent)" }}>
                    {site.email}
                  </a>
                </>
              ) : null}
            </p>
          ) : null}
        </header>

        {/* 一张表，全在这儿 */}
        {rows.length ? (
          <div className="mt-10" style={{ ...MONO, borderTop: `1px solid ${hair(0.2)}` }}>
            {rows.map((r) => (
              <div
                key={r.id}
                className={cx("py-2.5", narrow ? "" : "flex gap-5")}
                style={{ borderBottom: `1px solid ${hair(0.12)}` }}
              >
                <span
                  className="shrink-0 text-[12px]"
                  style={{ color: mix(0.45), width: narrow ? undefined : 82 }}
                >
                  {r.when || "—"}
                </span>
                <span
                  className={cx("shrink-0 text-[12px]", narrow && "ml-3")}
                  style={{ color: "var(--st-accent)", width: narrow ? undefined : 68 }}
                >
                  {r.kind}
                </span>
                <span className="min-w-0">
                  <span className="block text-[14px] font-semibold">{r.title}</span>
                  {r.blurb ? (
                    <span
                      className="mt-1 block text-[12.5px]"
                      style={{ lineHeight: 1.75, color: mix(0.58) }}
                    >
                      {r.blurb}
                    </span>
                  ) : (
                    <span className="mt-1 block">
                      <Blank what="写一句" editing={editing} />
                    </span>
                  )}
                </span>
              </div>
            ))}
          </div>
        ) : null}

        {/* 关于放在最后，短 */}
        {site.about.length || site.nowList.length ? (
          <section className="mt-14 max-w-[62ch]" style={{ ...MONO }}>
            <p className="text-[12px] tracking-[0.18em]" style={{ color: "var(--st-accent)" }}>
              关于
            </p>
            {site.about.map((para) => (
              <p
                key={para.slice(0, 12)}
                className="mt-4 text-[13px]"
                style={{ lineHeight: 1.95, color: mix(0.78) }}
              >
                {para}
              </p>
            ))}
            {site.nowList.length ? (
              <p className="mt-4 text-[13px]" style={{ lineHeight: 1.95, color: mix(0.55) }}>
                {site.nowList.join("　")}
              </p>
            ) : null}
          </section>
        ) : null}

        <footer
          className="mt-14 flex flex-wrap gap-x-6 gap-y-1 pt-4 text-[11px]"
          style={{
            ...MONO,
            color: mix(0.38),
            lineHeight: 1.9,
            borderTop: `1px solid ${hair(0.14)}`,
          }}
        >
          {site.stats.map((r) => (
            <span key={r.label}>
              {r.label} {r.value}
            </span>
          ))}
          <span className="ml-auto">
            © {new Date().getFullYear()} {site.name} · 用 思维印记 搭建
          </span>
        </footer>
      </div>
    </Ground>
  );
}
