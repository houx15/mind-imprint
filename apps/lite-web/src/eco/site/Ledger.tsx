import { Ground, MONO, hair, mix, type LayoutProps } from "./parts";
import { cx } from "../ui";

/**
 * 方案 B · 索引式 — 密 · 像一份档案柜.
 *
 * Her option: *首页是一张列表：日期 + 标题 + 一句话，一屏看到十几条。顶部一行小字
 * 说明这里是什么。密度优先。* So this is one table, not a stack of sections —
 * everything she has made, written and read, in one date-ordered index with a
 * kind column. Small type, tight leading, hairlines, no cards, no pictures.
 *
 * It is the opposite page from `Essay` on purpose. If both her options had
 * produced a column of headed sections, the choice she wrote a reason for
 * would have been a colour swap.
 */
export function Ledger({ site, theme, stage, narrow }: LayoutProps) {
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
      when: "—",
      kind: "在读",
      title: r.title,
      blurb: `${r.source}　${r.takeaway}`,
    })),
  ];
  // An index is in date order or it is not an index. Dated rows newest-first;
  // the undated 在读 rows settle at the bottom.
  rows.sort((a, b) => (a.when === b.when ? 0 : a.when < b.when ? 1 : -1));

  return (
    <Ground theme={theme}>
      <div className={cx("mx-auto w-full max-w-[860px]", pad, narrow ? "py-10" : "py-16")}>
        {/* 顶部一行小字说明这里是什么 */}
        <header style={{ ...MONO }}>
          <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1">
            <p className={cx("font-semibold", narrow ? "text-[13px]" : "text-[14px]")}>
              {site.name} — {site.domain}
            </p>
            <p className="text-[11px] tracking-[0.14em]" style={{ color: mix(0.42) }}>
              {site.role}
            </p>
          </div>
          <p
            className={cx(
              "mt-4 font-semibold",
              narrow
                ? stage === 1
                  ? "text-[22px] leading-[1.35]"
                  : "text-[16px] leading-[1.5]"
                : "text-[22px] leading-[1.45]",
            )}
          >
            {site.headline}
          </p>
          <p className="mt-3 text-[13px]" style={{ lineHeight: 1.85, color: mix(0.62) }}>
            {site.lead}
          </p>
          <p className="mt-3 text-[13px]" style={{ color: mix(0.5) }}>
            现在：{site.now}
            {stage >= 3 ? (
              <>
                {"　"}
                <a href={`mailto:${site.email}`} style={{ color: "var(--st-accent)" }}>
                  {site.email}
                </a>
              </>
            ) : null}
          </p>
        </header>

        {/* 一张表，全在这儿 */}
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
                {r.when}
              </span>
              <span
                className={cx("shrink-0 text-[12px]", narrow && "ml-3")}
                style={{ color: "var(--st-accent)", width: narrow ? undefined : 68 }}
              >
                {r.kind}
              </span>
              <span className="min-w-0">
                <span className={cx("block font-semibold", narrow ? "text-[14px]" : "text-[14px]")}>
                  {r.title}
                </span>
                <span
                  className="mt-1 block text-[12.5px]"
                  style={{ lineHeight: 1.75, color: mix(0.58) }}
                >
                  {r.blurb}
                </span>
              </span>
            </div>
          ))}
        </div>

        {/* 关于放在最后，短 */}
        <section className="mt-14 max-w-[62ch]" style={{ ...MONO }}>
          <p className="text-[12px] tracking-[0.18em]" style={{ color: "var(--st-accent)" }}>
            关于
          </p>
          {site.about.map((para) => (
            <p
              key={para.slice(0, 10)}
              className="mt-4 text-[13px]"
              style={{ lineHeight: 1.95, color: mix(0.78) }}
            >
              {para}
            </p>
          ))}
          <p className="mt-4 text-[13px]" style={{ lineHeight: 1.95, color: mix(0.55) }}>
            {site.nowList.join(" ")}
          </p>
        </section>

        <footer
          className="mt-14 flex flex-wrap gap-x-6 gap-y-1 pt-4 text-[11px]"
          style={{ ...MONO, color: mix(0.38), lineHeight: 1.9, borderTop: `1px solid ${hair(0.14)}` }}
        >
          {site.stats.map((r) => (
            <span key={r.label}>
              {r.label} {r.value}
            </span>
          ))}
          <span className="ml-auto">© 2026 {site.name} · 用 思维印记 搭建</span>
        </footer>
      </div>
    </Ground>
  );
}
