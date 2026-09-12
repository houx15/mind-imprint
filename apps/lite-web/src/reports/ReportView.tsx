import type { LiteReport, ReportStat } from "@lite/api/reports";
import { displayStat } from "./statLabels";

/**
 * ReportView — a wide, colourful record of one session, not a column of prose.
 *
 * ## Why it looks like this
 *
 * The first version of this file was a 640px column of stacked text sections.
 * The product owner's verdict on it was blunt: "is that a real report? do
 * people will have any intent to share it??? make it a wide screen colorful,
 * data-plenty, keypoints shining thing." So the layout law here is:
 *
 *   - **Wide.** Up to 1180px, laid out in a real grid. A report is read on a
 *     laptop and screenshotted; a phone-width column wastes the whole screen.
 *   - **Data-plenty.** The stat strip is auto-fit, so seven facts land as a
 *     wide band on a laptop and a 2-up grid on a phone with no breakpoint
 *     list to maintain.
 *   - **Keypoints shining.** 我的收获 and 金句 get the largest type and the
 *     strongest colour on the page. Everything else is support.
 *
 * Pure presentation: props in, markup out. No fetching, no share controls, no
 * export button — `ReportPanel` wraps this with the room chrome, and
 * `PublicReportPage` mounts this exact component with NO session at all.
 *
 * ## The rules that outlive any redesign
 *
 * Every section is independently optional and renders `null` when its data is
 * absent — no empty headings, no 「暂无」 placeholders, no skeleton rows. A
 * student who wrote two sentences gets one stat and nothing else, and the page
 * still reads as composed rather than broken. That is the case to look at
 * first; it is the one nobody notices when it ships broken.
 *
 * 铁律②: this is a RECORD of what she did, never a verdict on it, and never a
 * comparison to anyone else. No score, no grade, no rank, no streak, no badge
 * — the copy here is deliberately free of that vocabulary, and a test asserts
 * it stays that way.
 *
 * R4 / whose words are whose: three sections quote text, and each one labels
 * its source out loud, because two of them are NOT hers. 金句 are her own
 * sentences (the server validates every one as a literal substring of a
 * hers-only corpus). 我的笔记 pairs the ARTICLE's sentence (`note.quote`,
 * labelled 原文) with HER note. 我用透镜查到的 pairs the ARTICLE's sentence she
 * picked (labelled 我选的句子) with the room's 发现. Merging any of those pairs
 * into one undifferentiated quote would put the article's words under her name
 * — never do it, on this page or on the exported picture.
 *
 * ## Colour
 *
 * The fixed "macaron" `mk-` tokens (`--mk-peach` … `--mk-berry`, each with a
 * paired `-bg`/`-fg`), cycled per item by `macaron()` so a multi-stat,
 * multi-quote report reads as colourful without any one section drowning in a
 * single hue. Every one is read as `var(--mk-…)` in an inline `style`, never
 * as a Tailwind class with `/NN` alpha — `mk-*` are bare CSS custom
 * properties, so `bg-mk-peach-bg/30` emits no CSS at all and would silently
 * ship an invisible card.
 *
 * Motion and the heavier gradients live in `.mk-rp-*` rules in
 * `apps/lite-web/src/index.css`, each with a `prefers-reduced-motion` escape.
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

/** Entrance stagger. Each animated block reads `--i` off its own style and
 *  multiplies it into its delay (see `.mk-rp-rise` in index.css), so a long
 *  report cascades in instead of popping as one slab. */
function rise(i: number): React.CSSProperties {
  return { ["--i" as string]: i } as React.CSSProperties;
}

export function ReportView({
  report,
  actions,
  sharePanel,
  onBackToArticle,
  proseStuck = false,
  onRetryProse,
}: {
  report: LiteReport;
  /** 那一次自动补请求已经回来了，而金句还是没有。见 ProsePending。 */
  proseStuck?: boolean;
  /** 她按「再看一次」时再问一次。不给就不显示那颗按钮（公开分享页）。 */
  onRetryProse?: () => void;
  /** 导出/分享 icon buttons, pinned in the hero's upper-right corner. Omitted
   *  entirely on the public share page: a visitor is not the owner and must
   *  never be shown controls over someone else's report. */
  actions?: React.ReactNode;
  /** Rendered directly under the hero, so the panel the share icon opens
   *  appears next to the icon that opened it rather than at the far end of a
   *  long page. */
  sharePanel?: React.ReactNode;
  /** Back to her piece. Present only when there IS an article to go back to —
   *  a writing with a stored `piece`. A reading has no article and passes
   *  nothing, and so does a writing report generated before `piece` existed
   *  (no backfill), where a link back would land on an empty page. */
  onBackToArticle?: () => void;
}) {
  const kindLabel = report.kind === "reading" ? "一次阅读的记录" : "一次写作的记录";
  const date = formatDate(report.finishedAt);
  // A stat of 0 is absence, not a fact worth stating — a wall of coloured
  // zeros reads as a broken page, not as a record.
  // Labels resolved client-side from `stat.key` — a stored report keeps the
  // wording it was generated with, and this is what lets a rename reach every
  // report already in the database. See statLabels.ts.
  const stats = report.stats.filter((stat) => stat.value !== 0).map((stat) => displayStat(stat, report.kind));

  return (
    <article className="mk-rp mk-rp-measure flex flex-col gap-8 py-8 sm:py-12">
      <Hero
        kindLabel={kindLabel}
        title={report.title}
        name={report.studentName}
        date={date}
        actions={actions}
      />
      {sharePanel}
      <BackToArticle onBack={onBackToArticle} />
      <StatStrip stats={stats} />
      <Keep keep={report.keep} name={report.studentName} kind={report.kind} />
      <Moments moments={report.moments} />
      <NotesAndLenses notes={report.notes} lensNotes={report.lensNotes} />
      <Gains gains={report.gains} />
      <ProsePending pending={report.prosePending} stuck={proseStuck} onRetry={onRetryProse} />
    </article>
  );
}

/**
 * 金句 和 这次的收获 还在处理中。
 *
 * 这两节是报告上唯一需要一次模型调用的部分（`assess` 档，`reasoning: "max"`），
 * 服务端因此把它们放到**第二个**请求里，先把她真的做过的那些东西（数字、
 * 她的笔记、她的透镜、她自己写的收获）立刻存下并返回。见
 * `ensureAtomReport` 的两段式说明。
 *
 * 所以这里必须说出来。不说的话，一份还差这两节的报告在她眼里就是一份
 * **少了两节的报告**——她会以为它就是这样，而不是还没好。
 *
 * `Moments` / `Gains` 自己在空数组时返回 null，所以这一条挂在它们后面，
 * 不去改那两个组件的「有内容才渲染」规则。
 *
 * ⚠️ 分享出去的报告永远看不到这个：服务端把 `prosePending` 从公开 payload
 * 里摘掉了（访客没法去轮询一个需要登录的接口），所以 `PublicReportPage`
 * 渲染同一个组件时这里恒为 false。
 */
function ProsePending({
  pending,
  stuck,
  onRetry,
}: {
  pending: boolean;
  stuck?: boolean;
  onRetry?: () => void;
}) {
  if (!pending) return null;
  return (
    <section className="mk-rp-section" role="status" aria-live="polite">
      <SectionTitle>金句 · 这次的收获</SectionTitle>
      {/* 🚨 这句话原来写的是「处理中，稍后刷新可见」，而**根本不需要她刷新**：
          ReportPanel 在 prosePending 的时候自己又发了一次请求，那一次回来
          就把这两节加到她眼前的报告上（那个请求本身就是在等模型，所以要等
          一会儿）。屏幕上没有刷新按钮，也不该有。

          2026-09-11 第十轮线上走查，她为这一句连着卡了四步：
            「它说处理中稍后刷新可见，但我不知道怎么刷新，也没有刷新按钮」
            「页面说处理中，不知道是该等还是该点『回到这篇文章』」
            「金句和收获那里还在转圈没出来，不知道该等还是该点回去看文章」

          一句她照做不了的指令，比不说更糟：她会去找一颗不存在的按钮，
          然后以为是自己哪里弄错了。说事实就行 —— 它会自己出现。 */}
      {/* 🚨 2026-09-12：上面那段说的「它会自己出现」只在**顺利的时候**是真的。
          ReportPanel 只补发**一次**请求（那一次要等一个 180s 的旗舰调用，
          而且每问一次就真的再算一次，所以不能轮询）。那一次要是失败了、
          或者回来时金句仍然没好，屏幕上这句话就变成了一个不会兑现的承诺 ——
          她只能一直等。走查里她连着四步盯着它：
            「金句那里写着『处理中，好了会自己出现』，一直在转」

          上一版把「稍后刷新可见」换成这一句，修的是「一句她照做不了的指令」，
          方向对；但只修了一半 —— 换来的是一句我们守不住的承诺。
          两半都要：**顺利的时候说事实，不顺利的时候给她一颗真的能按的按钮。** */}
      {stuck && onRetry ? (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-mk-small text-mk-muted">这两节还没整理出来。</p>
          <button
            type="button"
            onClick={onRetry}
            className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700"
          >
            再看一次
          </button>
        </div>
      ) : (
        <p className="text-mk-small text-mk-muted">处理中，好了会自己出现。</p>
      )}
    </section>
  );
}

/** The band across the top: the title at display size, her name and the date
 *  under it, over a slow-moving gradient. This is the part that gets
 *  screenshotted, so it carries the identity and nothing operational. */
function Hero({
  kindLabel,
  title,
  name,
  date,
  actions,
}: {
  kindLabel: string;
  title: string;
  name: string;
  date: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="mk-rp-hero mk-rp-rise relative overflow-hidden rounded-mk-lg px-6 py-9 sm:px-10 sm:py-12" style={rise(0)}>
      <div className="mk-rp-hero__glow" aria-hidden="true" />
      {actions && <div className="absolute right-4 top-4 z-10 sm:right-5 sm:top-5">{actions}</div>}
      <div className="relative flex flex-col gap-4">
        <span className="mk-rp-chip w-fit rounded-mk-full px-3 py-1 text-mk-label">{kindLabel}</span>
        {/* No right inset needed: the icons sit ABOVE this line, level with the
            chip, in the hero's own top padding. */}
        <h1 className="max-w-[22ch] text-mk-display text-mk-ink sm:text-mk-report-hero">{title}</h1>
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-mk-body text-mk-muted">
          <span className="mk-rp-name text-mk-h3 text-mk-ink">{name}</span>
          {date && <span>{date}</span>}
        </p>
      </div>
    </header>
  );
}

/** The wide band of numerals. `auto-fit` rather than a breakpoint ladder: the
 *  same rule lays seven tiles across a laptop and two across a phone, and it
 *  keeps working when a report carries three stats or nine. */
function StatStrip({ stats }: { stats: ReportStat[] }) {
  if (stats.length === 0) return null;
  return (
    <section className="mk-rp-stats" aria-label="这次的数据">
      {stats.map((stat, i) => {
        const { bg, fg } = macaron(i);
        return (
          <div
            key={stat.key}
            className="mk-rp-stat mk-rp-rise rounded-mk-lg px-4 py-4 sm:px-5 sm:py-5"
            style={{ background: bg, ...rise(i + 1) }}
          >
            {/* `.mk-rp-stat__value` is the baseline flex row that keeps a
                four-digit value and its unit on one line; the SIZE comes from
                the `mk-report-stat` token, not from that class. */}
            <span className="mk-rp-stat__value text-mk-report-stat" style={{ color: fg }}>
              {stat.value.toLocaleString("zh-CN")}
              {stat.unit && <span className="text-mk-h3">{stat.unit}</span>}
            </span>
            <span className="mt-2 block text-mk-label" style={{ color: fg }}>
              {stat.label}
            </span>
          </div>
        );
      })}
    </section>
  );
}

/** 我的收获 — the one thing worth taking away, given the biggest type on the
 *  page.
 *
 *  Two sources, and the difference is stated on screen every time. `student`
 *  is her own takeaway, verbatim, signed with her name. `coach` is 印记's
 *  summary of the session, written because she left no takeaway — 完成这篇
 *  stopped asking for one, so `coach` is the normal case now and `student` is
 *  the legacy path.
 *
 *  The attribution line is not decoration and is never conditional on space:
 *  a generated paragraph printed under a student's name, on a page she can
 *  publish to anyone, is the one dishonest thing this card could do. */
function Keep({ keep, name, kind }: { keep: LiteReport["keep"]; name: string; kind: LiteReport["kind"] }) {
  if (!keep) return null;
  const byStudent = keep.source === "student";
  return (
    <section className="mk-rp-keep mk-rp-rise relative overflow-hidden rounded-mk-lg px-6 py-8 sm:px-10 sm:py-10" style={rise(2)}>
      <h2 className="text-mk-label" style={{ color: "var(--mk-accent-700)" }}>
        {keep.label}
      </h2>
      <p className="mt-4 whitespace-pre-wrap text-mk-report-lede text-mk-ink">{keep.text}</p>
      <p className="mt-4 text-mk-small text-mk-muted">
        {byStudent ? `—— ${name}` : `印记根据你这次${kind === "reading" ? "阅读" : "写作"}整理`}
      </p>
    </section>
  );
}

/** 金句 — her own sentences, pulled out and given room. Auto-fit cards so two
 *  or three sit side by side on a wide screen instead of stacking into a
 *  ribbon of whitespace. Each carries its own 来自 attribution. */
function Moments({ moments }: { moments: LiteReport["moments"] }) {
  if (moments.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>金句</SectionTitle>
      <div className="mk-rp-moments">
        {moments.map((m, i) => {
          const { bg, fg } = macaron(i + 2);
          return (
            <blockquote
              key={`${m.where}-${i}`}
              className="mk-rp-moment mk-rp-rise relative overflow-hidden rounded-mk-lg py-7 pl-10 pr-6"
              style={{ background: bg, ...rise(i + 3) }}
            >
              <span aria-hidden="true" className="mk-rp-moment__mark" style={{ color: fg }}>
                “
              </span>
              <p className="text-mk-report-quote text-mk-ink">{m.quote}</p>
              <footer className="mt-4 text-mk-small" style={{ color: fg }}>
                来自：{m.where}
              </footer>
            </blockquote>
          );
        })}
      </div>
    </section>
  );
}

/** The two evidence columns, side by side on a wide screen: what she wrote in
 *  the margins, and what her 透镜 work turned up. Both are lists of quote+prose
 *  pairs, so pairing them in one grid is what makes the page read as a spread
 *  rather than an endless scroll — and when only one of the two has content,
 *  the grid collapses to a single full-width column on its own. */
function NotesAndLenses({
  notes,
  lensNotes,
}: {
  notes: LiteReport["notes"];
  lensNotes: LiteReport["lensNotes"];
}) {
  if (notes.length === 0 && lensNotes.length === 0) return null;
  return (
    <div className="mk-rp-columns">
      <MyNotes notes={notes} />
      <LensNotes notes={lensNotes} />
    </div>
  );
}

/** 我的笔记 — every margin note she left, each hanging off the article
 *  sentence it was about.
 *
 *  Whose words are whose is the entire design of this card: the article's
 *  sentence sits in a muted, indented rail under the label 原文, and HER note
 *  is the black, full-size text above it. If you ever find yourself tempted to
 *  drop the 原文 label or to promote the quote to the same weight as her note,
 *  read this file's R4 paragraph again. */
function MyNotes({ notes }: { notes: LiteReport["notes"] }) {
  if (notes.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>我的笔记</SectionTitle>
      <div className="flex flex-col gap-3">
        {notes.map((n, i) => {
          const { fg } = macaron(i);
          return (
            <div
              key={`${n.quote}-${i}`}
              className="mk-rp-card mk-rp-rise rounded-mk-lg p-5"
              style={rise(i + 4)}
            >
              <p className="text-mk-body-lg text-mk-ink">{n.note}</p>
              {n.quote && (
                <p className="mk-rp-source mt-3 pl-3 text-mk-small text-mk-muted" style={{ borderColor: fg }}>
                  <span className="mr-1.5 text-mk-label" style={{ color: fg }}>
                    原文
                  </span>
                  {n.quote}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

/** 我用透镜查到的 — what her 透镜 work produced: for each lens card she
 *  submitted, the sentence SHE picked out of the article, plus the 发现 the
 *  room drew from it.
 *
 *  A DIFFERENT kind of thing from 金句 above — a 金句 is a sentence of hers the
 *  MODEL picked out as noteworthy prose; a lens note is her own act of picking
 *  a sentence out of the ARTICLE, which is real work and real thinking even
 *  though the words themselves are the article's. Hence the explicit
 *  「我选的句子」 label, and hence the deliberately plain card: no giant
 *  quotation mark, no full-bleed macaron, so the two sections never read as
 *  the same content in different clothes. */
function LensNotes({ notes }: { notes: LiteReport["lensNotes"] }) {
  if (notes.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>我用透镜查到的</SectionTitle>
      <div className="flex flex-col gap-3">
        {notes.map((n, i) => {
          const { bg, fg } = macaron(i + 4);
          return (
            <div
              key={`${n.lens}-${i}`}
              className="mk-rp-card mk-rp-rise rounded-mk-lg p-5"
              style={rise(i + 5)}
            >
              <span
                className="w-fit rounded-mk-full px-2.5 py-0.5 text-mk-caption"
                style={{ background: bg, color: fg }}
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
          );
        })}
      </div>
    </section>
  );
}

/** 这次的收获 — 2–4 short lines, as numbered coloured cards across the foot of
 *  the page. Numbered for reading order only; deliberately NOT a checklist, a
 *  progress bar, or anything that could be read as a tally out of some total. */
function Gains({ gains }: { gains: LiteReport["gains"] }) {
  if (gains.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>这次的收获</SectionTitle>
      <ul className="mk-rp-gains">
        {gains.map((g, i) => {
          const { bg, fg } = macaron(i + 1);
          return (
            <li
              key={i}
              className="mk-rp-gain mk-rp-rise flex items-start gap-3 rounded-mk-lg p-5"
              style={{ background: bg, ...rise(i + 6) }}
            >
              <span aria-hidden="true" className="mk-rp-gain__no text-mk-report-numeral" style={{ color: fg }}>
                {i + 1}
              </span>
              <span className="text-mk-body-lg text-mk-ink">{g}</span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/** The way back to her piece, at the top of the record.
 *
 *  The record is a SEPARATE PAGE now ("make the report a new page"), so the
 *  article is no longer a scroll away — it needs a door. Rendered only when
 *  the caller supplies one: a reading has no article, and neither does a
 *  writing report generated before `piece` existed (no backfill), and a link
 *  to an empty page is worse than no link.
 */
function BackToArticle({ onBack }: { onBack?: () => void }) {
  if (!onBack) return null;
  return (
    <button
      type="button"
      onClick={onBack}
      className="w-fit text-mk-body text-mk-accent-700 underline decoration-dotted underline-offset-4 hover:text-mk-ink"
    >
      ← 回到这篇文章
    </button>
  );
}


/** One section heading, rendered as a real `<h2>` with a hairline running off
 *  to the right — the page's only repeated chrome. */
function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="mk-rp-title flex items-center gap-3 text-mk-label text-mk-faint">
      <span>{children}</span>
      <span aria-hidden="true" className="mk-rp-title__rule" />
    </h2>
  );
}
