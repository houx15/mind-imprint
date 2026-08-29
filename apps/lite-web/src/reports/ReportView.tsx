import type { LiteReport } from "@lite/api/reports";

/**
 * ReportView — the report itself, as a warm poster rather than a dashboard.
 *
 * Pure presentation: props in, markup out. No fetching, no share controls,
 * no export button — Task 10 wraps this with the room chrome and share
 * controls, Task 12 mounts this exact component on a public page that runs
 * with NO session at all. Neither caller should need to know anything about
 * this component's internals beyond `<ReportView report={LiteReport} />`.
 *
 * Every section below is independently optional and renders `null` when its
 * data is absent — no empty headings, no "暂无" placeholders, no skeleton
 * rows. A student who wrote two sentences gets one stat and nothing else,
 * and the page still reads as composed rather than broken. This is the case
 * to look at first; it is the one nobody notices when it ships broken.
 *
 * 铁律②: this is a RECORD of what she did, never a verdict on it, and never
 * a comparison to anyone else. No score, no grade, no rank, no streak, no
 * badge — the copy here is deliberately free of that vocabulary, and a test
 * asserts it stays that way.
 *
 * Colour comes from the fixed "macaron" `mk-` tokens (`--mk-peach` …
 * `--mk-berry`, each with a paired `-bg`/`-fg`). Every one of those is read
 * as `var(--mk-…)` in an inline `style`, never as a Tailwind class with `/NN`
 * alpha — `mk-*` are bare CSS custom properties, so `bg-mk-peach-bg/30`
 * would emit no CSS at all and silently ship an invisible card. The palette
 * cycles per item via `macaron()` below so a multi-stat, multi-quote report
 * reads as colourful without any one section drowning in a single hue.
 */

const MACARON = [
  { bg: "var(--mk-peach-bg)", fg: "var(--mk-peach-fg)" },
  { bg: "var(--mk-lake-bg)", fg: "var(--mk-lake-fg)" },
  { bg: "var(--mk-berry-bg)", fg: "var(--mk-berry-fg)" },
  { bg: "var(--mk-matcha-bg)", fg: "var(--mk-matcha-fg)" },
  { bg: "var(--mk-taro-bg)", fg: "var(--mk-taro-fg)" },
  { bg: "var(--mk-butter-bg)", fg: "var(--mk-butter-fg)" },
  { bg: "var(--mk-mist-bg)", fg: "var(--mk-mist-fg)" },
] as const;

function macaron(i: number) {
  // `i % MACARON.length` is always a valid index into a non-empty literal
  // array — the `?? MACARON[0]` only satisfies noUncheckedIndexedAccess.
  return MACARON[i % MACARON.length] ?? MACARON[0];
}

/** Absolute date, not "今天"/"昨天": this can be read back weeks later, or by
 *  someone she shared it with — a relative day would be meaningless to them. */
function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`;
}

export function ReportView({ report }: { report: LiteReport }) {
  const kindLabel = report.kind === "reading" ? "一次阅读的记录" : "一次写作的记录";
  const date = formatDate(report.finishedAt);

  return (
    <article className="mx-auto flex w-full max-w-[640px] flex-col gap-9 px-6 py-14">
      <header className="flex flex-col gap-3">
        <span
          className="w-fit rounded-mk-full px-2.5 py-1 text-mk-label"
          style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-700)" }}
        >
          {kindLabel}
        </span>
        <h1 className="text-mk-display text-mk-ink">{report.title}</h1>
        <p className="text-mk-small text-mk-muted">
          {report.studentName}
          {date && ` · ${date}`}
        </p>
      </header>

      <StatsRow stats={report.stats} />
      <Moments moments={report.moments} />
      <LensNotes notes={report.lensNotes} />
      <Keep keep={report.keep} />
      <Gains gains={report.gains} />
    </article>
  );
}

function StatsRow({ stats }: { stats: LiteReport["stats"] }) {
  // A stat of 0 is absence, not a fact worth stating — four coloured zeros
  // read as a broken page, not as a record. Same "absent rather than empty"
  // rule every other section here already follows: no row, no wrapper, no
  // gap when nothing survives.
  const nonZero = stats.filter((stat) => stat.value !== 0);
  if (nonZero.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-4">
      {nonZero.map((stat, i) => {
        const { bg, fg } = macaron(i);
        return (
          <div
            key={stat.key}
            className="flex min-w-[120px] flex-1 flex-col gap-1 rounded-mk-lg px-5 py-4"
            style={{ background: bg }}
          >
            <span className="text-mk-display leading-none" style={{ color: fg }}>
              {stat.value}
              {stat.unit && <span className="ml-0.5 text-mk-h3">{stat.unit}</span>}
            </span>
            <span className="text-mk-label" style={{ color: fg }}>
              {stat.label}
            </span>
          </div>
        );
      })}
    </div>
  );
}

/** 金句 — pull-quotes with room to breathe, not shrunk into a bullet list.
 *  Each carries its own small "来自：{where}" attribution underneath. */
function Moments({ moments }: { moments: LiteReport["moments"] }) {
  if (moments.length === 0) return null;
  return (
    <section className="flex flex-col gap-5">
      <span className="text-mk-label text-mk-faint">金句</span>
      <div className="flex flex-col gap-4">
        {moments.map((m, i) => {
          const { bg, fg } = macaron(i + 2);
          return (
            <blockquote
              key={`${m.where}-${i}`}
              className="relative rounded-mk-lg py-6 pl-9 pr-6"
              style={{ background: bg }}
            >
              <span
                aria-hidden="true"
                className="absolute left-3 top-2 text-[38px] leading-none"
                style={{ color: fg }}
              >
                “
              </span>
              <p className="text-mk-h2 text-mk-ink" style={{ lineHeight: 1.7 }}>
                {m.quote}
              </p>
              <footer className="mt-3 text-mk-small" style={{ color: fg }}>
                来自：{m.where}
              </footer>
            </blockquote>
          );
        })}
      </div>
    </section>
  );
}

/** 我用透镜查到的 — what her 透镜 work produced: for each lens card she
 *  submitted, the sentence SHE picked out of the article, plus the 发现 the
 *  room drew from it. This is a DIFFERENT kind of thing from 金句 above —
 *  a 金句 is a sentence of hers the MODEL picked out as noteworthy prose;
 *  a lens note is her own act of picking a sentence out of the ARTICLE,
 *  which is real work and real thinking even though the words themselves
 *  are the article's, not hers. So the labels here are deliberately
 *  explicit about whose words are whose ("我选的句子" — a sentence from the
 *  article, chosen by her), and the visual treatment is deliberately NOT
 *  the 金句 pull-quote (no giant quotation mark, no full-bleed macaron
 *  card) — a plain bordered card per lens, so the two sections never read
 *  as the same kind of content in different clothes. */
function LensNotes({ notes }: { notes: LiteReport["lensNotes"] }) {
  if (notes.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <span className="text-mk-label text-mk-faint">我用透镜查到的</span>
      <div className="flex flex-col gap-3">
        {notes.map((n, i) => (
          <div key={`${n.lens}-${i}`} className="rounded-mk-lg border border-mk-border bg-mk-surface p-5">
            <span
              className="w-fit rounded-mk-full px-2.5 py-0.5 text-mk-caption"
              style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-700)" }}
            >
              {n.lens}
            </span>
            {n.quote && (
              <p className="mt-3 text-mk-body-lg text-mk-ink">
                <span className="text-mk-small text-mk-muted">我选的句子：</span>「{n.quote}」
              </p>
            )}
            {n.finding && <p className="mt-2 text-mk-body text-mk-muted">{n.finding}</p>}
          </div>
        ))}
      </div>
    </section>
  );
}

/** Her own 收获, verbatim — quoted and attributed as hers via the
 *  server-supplied `label` (e.g. "我的收获"), never presented as the AI's
 *  summary of her. Same card idiom as the finished-reading panel's own
 *  「我的收获」 block, so the two surfaces read as one visual language. */
function Keep({ keep }: { keep: LiteReport["keep"] }) {
  if (!keep) return null;
  return (
    <section className="rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-sm">
      <h2 className="text-mk-label text-mk-faint">{keep.label}</h2>
      <p className="mt-3 whitespace-pre-wrap text-mk-body-lg text-mk-ink">“{keep.text}”</p>
    </section>
  );
}

/** 2–4 short gain lines. Plain, unstyled short lines — not a table, not a
 *  checklist, not a progress bar. */
function Gains({ gains }: { gains: LiteReport["gains"] }) {
  if (gains.length === 0) return null;
  return (
    <section className="flex flex-col gap-3">
      <span className="text-mk-label text-mk-faint">这次的收获</span>
      <ul className="flex flex-col gap-2.5">
        {gains.map((g, i) => (
          <li key={i} className="text-mk-body-lg text-mk-ink">
            {g}
          </li>
        ))}
      </ul>
    </section>
  );
}
