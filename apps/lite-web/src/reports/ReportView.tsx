import { ReportVisualSummary } from "./ReportVisualSummary";
import { studentArtwork } from "../learning/StudentArtwork";
import type { LiteReport } from "@lite/api/reports";
import { displayStat } from "./statLabels";
import { WordCards } from "../readings/WordCards";

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

export function macaron(i: number) {
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
export function rise(i: number): React.CSSProperties {
  return { ["--i" as string]: i } as React.CSSProperties;
}

export function ReportView({
  report,
  actions,
  sharePanel,
  onBackToArticle,
  proseStuck = false,
  onRetryProse,
  viewer = "owner",
}: {
  report: LiteReport;
  /** 谁在看这一页。`owner` 是她自己（开场那句说「我」，转折里她那一半标「我」），
   *  `guest` 是拿着分享链接打开它的人（两处都改用她的名字）。
   *  `PublicReportPage` 传 `guest`；房间里不传，默认就是她自己。 */
  viewer?: "owner" | "guest";
  /** 那一次自动补请求已经回来了，而金句还是没有。见 ProsePending。 */
  proseStuck?: boolean;
  /** 她按「重试生成」时再问一次。不给就不显示那颗按钮（公开分享页）。 */
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

  if (report.kind === "reading") return (
    <article className="mk-rp mk-rp-measure student-reading-journal">
      <header className="journal-masthead"><span>READING JOURNAL · 阅读手记</span><div>{actions}</div></header>
      {sharePanel}
      <div className="journal-cover">
        <div>
          <p className="journal-byline">{report.studentName} <span>／ {date}</span></p>
          <Opening ordinal={report.ordinal} name={report.studentName} viewer={viewer} kind={report.kind} />
      <RevisionNote revision={report.revision} revisedAt={report.revisedAt} viewer={viewer} />
          <h1>{report.title}</h1>
        </div>
        <img src={studentArtwork.keepsake} alt="" />
      </div>
      {/* 数据带紧跟着标题，整幅宽。放进封面那一栏里试过：那条栅格是 auto-fit
          的，挤进半幅宽之后 7 个数字会折成两行并留下一块空洞。她给的参照是
          「大标题，然后一条总数据」—— 这样就是那个顺序，而且不会折。 */}
      <ReportVisualSummary stats={stats} />
      <Keep keep={report.keep} name={report.studentName} kind={report.kind} />
      <Moments moments={report.moments} />
      <TurningPoints points={report.turningPoints} name={report.studentName} viewer={viewer} />
      <ArticleEntry article={report.article} title={report.title} />
      <MyExcerpts excerpts={report.excerpts} />
      <NotesAndLenses notes={report.notes} lensNotes={report.lensNotes} />
      <Summary summary={report.summary} />
      <Boards boards={report.boards} />
      <Toolkit toolkit={report.toolkit} />
      <Gains gains={report.gains} />
      <ProsePending pending={report.prosePending} stuck={proseStuck} onRetry={onRetryProse} />
      <footer className="journal-colophon">{report.studentName} · 阅读手记 <span>{date}</span></footer>
    </article>
  );

  return (
    <article className="mk-rp mk-rp-measure flex flex-col gap-8 py-8 sm:py-12">
      <Hero
        kind={report.kind}
        kindLabel={kindLabel}
        title={report.title}
        name={report.studentName}
        date={date}
        actions={actions}
      />
      {sharePanel}
      <Opening ordinal={report.ordinal} name={report.studentName} viewer={viewer} kind={report.kind} />
      <RevisionNote revision={report.revision} revisedAt={report.revisedAt} viewer={viewer} />
      <BackToArticle onBack={onBackToArticle} />
      <ReportVisualSummary stats={stats} />
      <Keep keep={report.keep} name={report.studentName} kind={report.kind} />
      <Moments moments={report.moments} />
      <TurningPoints points={report.turningPoints} name={report.studentName} viewer={viewer} />
      <MyExcerpts excerpts={report.excerpts} />
      <NotesAndLenses notes={report.notes} lensNotes={report.lensNotes} />
      <Summary summary={report.summary} />
      <Boards boards={report.boards} />
      <Toolkit toolkit={report.toolkit} />
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
          <p className="text-mk-small text-mk-muted">金句与收获暂未生成。</p>
          <button
            type="button"
            onClick={onRetry}
            className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700"
          >
            重试生成
          </button>
        </div>
      ) : (
        <p className="text-mk-small text-mk-muted">正在生成，完成后将自动显示。</p>
      )}
    </section>
  );
}

/** The band across the top: the title at display size, her name and the date
 *  under it, over a slow-moving gradient. This is the part that gets
 *  screenshotted, so it carries the identity and nothing operational.
 *  Exported for `parentReport/ParentReportView`, which uses the same band. */
export function Hero({
  kind,
  kindLabel,
  title,
  name,
  date,
  actions,
}: {
  kind: keyof typeof studentArtwork;
  kindLabel: string;
  title: string;
  name: string;
  date: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="mk-rp-hero mk-rp-rise relative overflow-hidden rounded-mk-lg px-6 py-9 sm:px-10 sm:py-12" style={rise(0)}>
      <img className="student-report-art" src={studentArtwork[kind]} alt="" />
      {actions && <div className="absolute right-4 top-4 z-10 sm:right-5 sm:top-5">{actions}</div>}
      <div className="student-report-heading relative flex flex-col gap-4">
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

/**
 * 开场那一句。
 *
 * 产品负责人 2026-09-16 给的参照就是这一句：「这是我和 xx 一起阅读的第 x 篇
 * 文章！」。在这之前这一页的标题是**文章的标题**，于是它读起来像那篇文章的
 * 页面，而不像她的记录 —— 一页没有身份的报告，没人会想发出去。
 *
 * `ordinal` 为 0 = 这份报告早于那个字段（服务端 omitempty），整句不渲染：
 * 「第 0 篇」比不渲染糟得多。
 *
 * 人称按谁在看：她自己看是「我」，访客看到的是她的名字。服务端存的是数字
 * 不是句子，就是为了让这一句能在两种语境下各说各的（见 api/reports.ts 的
 * `ordinal`）。
 *
 * 感叹号留着 —— 完成一篇是规则 9 说的那种「真正的节点」，而且这是她的原话。
 */
/**
 * 这份报告更新过。
 *
 * 🚨 产品负责人 2026-09-17：「it is saved. then regenerated would change the
 * content. but we need to let students know.」报告是存下来的；她「继续阅读」
 * 再完成之后，它按新的记录重新生成，金句、收获、数字都可能变。这一行让她（和拿着
 * 分享链接的人）知道眼前这份不是原来那一份。第 1 版不显示。
 */
function RevisionNote({
  revision,
  revisedAt,
  viewer,
}: {
  revision?: number;
  revisedAt?: string;
  viewer: "owner" | "guest";
}) {
  if (!revision || revision < 2) return null;
  const when = revisedAt ? formatDate(revisedAt) : "";
  return (
    <p className="text-mk-small text-mk-muted">
      第 {revision} 版{when ? ` · ${when} 更新` : ""}
      {viewer === "owner"
        ? "：继续阅读后再次完成，报告已按新的阅读记录重新生成，内容可能与之前不同。"
        : "：作者继续阅读后，报告已按新的阅读记录重新生成。"}
    </p>
  );
}

function Opening({
  ordinal,
  name,
  viewer,
  kind,
}: {
  ordinal: number;
  name: string;
  viewer: "owner" | "guest";
  kind: LiteReport["kind"];
}) {
  if (ordinal <= 0) return null;
  const who = viewer === "owner" ? "我" : name;
  const verb = kind === "reading" ? "一起读的第" : "一起写的第";
  return (
    <p className="text-mk-h3" style={{ color: "var(--mk-accent-700)" }}>
      这是{who}和印记{verb} {ordinal} 篇文章！
    </p>
  );
}

/**
 * 对话里的转折 — 这一版报告上唯一真正新的**内容**。
 *
 * 在这之前，一次 12 轮的对话在这一页上就是一个数字「12」。她做过的最值得看的
 * 那部分（她问出关键问题、她改主意、印记指出她读错了而她接住了）一个字都没有。
 *
 * 🚨 两句话必须各自标明是谁说的。这是这份报告上唯一同时印着她的话和印记的话
 * 的一节 —— 混在一起就是把印记的话记在她名下。服务端那一侧同样守着这条：
 * 正文是按编号从 atom_message 里逐字取的，模型只写了 `why`。
 */
function TurningPoints({
  points,
  name,
  viewer,
}: {
  points: LiteReport["turningPoints"];
  name: string;
  viewer: "owner" | "guest";
}) {
  if (points.length === 0) return null;
  const me = viewer === "owner" ? "我" : name;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>对话里的转折</SectionTitle>
      <div className="flex flex-col gap-3">
        {points.map((p, i) => {
          const { bg, fg } = macaron(i + 1);
          return (
            <div
              key={p.turn}
              className="mk-rp-rise overflow-hidden rounded-mk-lg px-6 py-6"
              style={{ background: bg, ...rise(i + 4) }}
            >
              <p className="text-mk-label" style={{ color: fg }}>
                {p.why}
              </p>
              <p className="mt-4 text-mk-label text-mk-faint">{me}</p>
              <p className="mt-1 whitespace-pre-wrap text-mk-body-lg text-mk-ink">{p.student}</p>
              {p.coach && (
                <>
                  <p className="mt-4 text-mk-label text-mk-faint">印记</p>
                  <p className="mt-1 whitespace-pre-wrap text-mk-body text-mk-secondary">{p.coach}</p>
                </>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

/**
 * 我读的这篇 — 拿到分享链接的人凭这一块知道她读的是什么。
 *
 * 🚨 `excerpt` 是**一段摘录**，服务端封了 200 字。这一页不放全文：分级阅读库
 * 是第三方素材，报告是她的记录不是一次转载。她自己那一面要看全文，走完成页的
 * 「原文」那一格（要登录、要归属）。
 */
function ArticleEntry({ article, title }: { article: LiteReport["article"]; title: string }) {
  if (!article) return null;
  return (
    <section className="mk-rp-card rounded-mk-lg p-5">
      <h2 className="text-mk-label text-mk-faint">我读的这篇</h2>
      <p className="mt-2 text-mk-h3 text-mk-ink">{title}</p>
      {article.excerpt && <p className="mt-2 text-mk-body text-mk-muted">{article.excerpt}</p>}
      {article.sourceUrl && (
        <a
          className="mt-3 inline-block text-mk-small underline"
          style={{ color: "var(--mk-accent-700)" }}
          href={article.sourceUrl}
          target="_blank"
          rel="noreferrer noopener"
        >
          原文{article.host ? ` · ${article.host}` : ""}
        </a>
      )}
    </section>
  );
}

/** 我的摘抄 — the sentences she underlined and kept.
 *
 *  产品负责人 2026-09-22：「she can click 摘抄, and these sentences will have
 *  some kind of underline and be recorded in 阅读成果 and revealed in report」.
 *
 *  🚨 它和下面「我的笔记」是两节，不能并成一节。笔记是她**写了字**的批注，
 *  摘抄是她一个字没写 —— 摘抄的时候刻意不追问为什么。把它们摆在一起，
 *  摘抄那几条就成了「笔记，但是空的」，读起来像她少做了一件事；而她做的
 *  那件事是完整的：从一整篇里挑出了这几句。
 *
 *  所以这一节的标签说清是谁的话（R4）：句子是文章的原话，挑出它的判断是她的。 */
function MyExcerpts({ excerpts }: { excerpts: LiteReport["excerpts"] }) {
  if (excerpts.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>我的摘抄</SectionTitle>
      <p className="text-mk-small text-mk-muted">阅读时保存的原文摘抄。</p>
      <div className="flex flex-col gap-3">
        {excerpts.map((quote, i) => {
          const { fg } = macaron(i);
          return (
            <div
              key={`${quote}-${i}`}
              className="mk-rp-card mk-rp-rise rounded-mk-lg p-5"
              style={rise(i + 4)}
            >
              <p className="mk-rp-source pl-3 text-mk-body-lg text-mk-ink" style={{ borderColor: fg }}>
                {quote}
              </p>
            </div>
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

/**
 * 段落工具 —— 她在这一篇上拆过什么、学了哪些词、自己写了什么。
 *
 * 产品负责人 2026-09-17：「these things, students' actions, their learns, the
 * shadow writing, the words. can be revealed in their reading report.」
 *
 * 四小块，哪块没有就不显示：用过的工具（几段）、学过的词（词卡原样）、拆过的
 * 句子和语法点、她在想一想 / 仿写 底下写的那几段。
 *
 * 🚨 R4：最后那一块里，**她写的**是正文大小的黑字；印记 的那一行题目
 * （「仿写 · 第3段：……」）是它上面一行小灰字。反过来摆，读的人会以为那一行是
 * 她写的。
 */
/**
 * 「我摆的板」—— 她在阅读里做过的判断，原样留下。
 *
 * 产品负责人 2026-09-18：报告上的成果不该只有透镜。板是她这一篇里做过的**判断**
 * （哪一句是事实、哪一句是某一方的说法、几件事按什么先后发生），而在这之前它
 * 只活在对话记录里。
 *
 * 🚨 板上那几句是**文章的原话**，不是她写的句子 —— 这一节的说明因此写的是
 * 「你放的位置」，而不是「你写的」。
 */
/**
 * 全文总结：导读里的核心问题 / 关键结论 / 结构。同事 2026-09-18：「can also
 * appear in the reading report?」和阅读室里读完之后摆出来的是同一份。
 */
function Summary({ summary }: { summary: LiteReport["summary"] }) {
  if (!summary) return null;
  const rows = [
    ["核心问题", summary.question],
    ["关键结论", summary.conclusion],
    ["结构", summary.structure],
  ].filter((r): r is [string, string] => Boolean(r[1]));
  if (rows.length === 0) return null;
  return (
    <section className="mk-rp-sec">
      <h2 className="mk-rp-h2">全文总结</h2>
      <dl className="mk-rp-summary">
        {rows.map(([k, v]) => (
          <div key={k}>
            <dt>{k}</dt>
            <dd>{v}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function Boards({ boards }: { boards: LiteReport["boards"] }) {
  if (!boards || boards.length === 0) return null;
  return (
    <section className="mk-rp-sec">
      <h2 className="mk-rp-h2">阅读成果</h2>
      <p className="mk-rp-note">句子来自原文，位置是你的判断。</p>
      {boards.map((b, i) => (
        <div key={i} className="mk-rp-board">
          <h3 className="mk-rp-board__title">{b.title}</h3>
          {b.kind === "label" ? (
            <ul className="mk-rp-board__bins">
              {(b.groups ?? []).map((g) => (
                <li key={g.bin}>
                  <span className="mk-rp-board__bin">{g.bin}</span>
                  <ul>
                    {g.quotes.map((q, j) => (
                      <li key={j}>{q}</li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>
          ) : (
            <ol className="mk-rp-board__order">
              {(b.order ?? []).map((q, j) => (
                <li key={j}>{q}</li>
              ))}
            </ol>
          )}
        </div>
      ))}
    </section>
  );
}

function Toolkit({ toolkit }: { toolkit: LiteReport["toolkit"] }) {
  if (!toolkit) return null;
  const { tools, words, grammar, writings } = toolkit;
  if (tools.length + words.length + grammar.length + writings.length === 0) return null;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>段落工具</SectionTitle>
      {tools.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {tools.map((t, i) => {
            const { bg, fg } = macaron(i);
            return (
              <span
                key={t.label}
                className="rounded-mk-full px-3 py-1 text-mk-small"
                style={{ background: bg, color: fg }}
              >
                {t.label} · {t.blocks} 段
              </span>
            );
          })}
        </div>
      )}
      {words.length > 0 && (
        <div className="mk-rp-card mk-rp-rise rounded-mk-lg p-5">
          <p className="mb-3 text-mk-label text-mk-muted">学过的词</p>
          <WordCards words={words} />
        </div>
      )}
      {grammar.length > 0 && (
        <div className="mk-rp-card mk-rp-rise flex flex-col gap-3 rounded-mk-lg p-5">
          <p className="text-mk-label text-mk-muted">拆过的句子</p>
          {grammar.map((g) => (
            <div key={g.sentence} className="flex flex-col gap-1.5">
              <p className="mk-rp-source pl-3 text-mk-small text-mk-muted">
                <span className="mr-1.5 text-mk-label">原文</span>
                {g.sentence}
              </p>
              {g.points.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pl-3">
                  {g.points.map((p) => (
                    <span key={p} className="rounded-mk-full bg-mk-accent-50 px-2 py-0.5 text-mk-caption text-mk-accent-700">
                      {p}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
      {writings.length > 0 && (
        <div className="flex flex-col gap-3">
          {writings.map((w, i) => (
            <div key={`${w.prompt}-${i}`} className="mk-rp-card mk-rp-rise rounded-mk-lg p-5" style={rise(i + 6)}>
              <p className="text-mk-small text-mk-muted">
                <span className="mr-1.5 text-mk-label">{w.tool ? `我的${w.tool}` : "我写的"}</span>
                {w.prompt}
              </p>
              <p className="mt-2 whitespace-pre-wrap text-mk-body-lg text-mk-ink">{w.text}</p>
            </div>
          ))}
        </div>
      )}
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
export function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="mk-rp-title flex items-center gap-3 text-mk-label text-mk-faint">
      <span>{children}</span>
      <span aria-hidden="true" className="mk-rp-title__rule" />
    </h2>
  );
}
