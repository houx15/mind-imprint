import type { LiteReport } from "@lite/api/reports";

/**
 * ArticleView — her finished piece, presented as an article.
 *
 * ## What this is not
 *
 * It is not the report. The report — stats, 金句, 我的收获, 这次的收获 — is a
 * separate page now (`ReportView`, reached from the link at the foot of this
 * one). They were one long scroll for about an hour, and that made the essay
 * read as the preamble to a dashboard. What she shares is her writing; the
 * record of how it got written is a thing you can go and look at afterwards.
 *
 * ## The rules this page is set by
 *
 * Four verdicts from the product owner, in the order they arrived, all about
 * the same thing — this is a page you READ:
 *
 *   1. *"don't use card for articles. this is not good for reading."* No
 *      panel, no border, no fill. Every bordered box on the report page is
 *      framing DATA; an article in one reads as another widget, with edges
 *      pressing in on the text. The measure does the framing here.
 *   2. *"think about New York Times, or some beautiful blogs, how they would
 *      present articles."* Serif, one column, 19px with real leading, and
 *      nothing competing in the band.
 *   3. *"don't make the title a block."* The title is type, not a coloured
 *      gradient masthead. That band belongs to the report page, which IS a
 *      poster and was asked for as one; an article's title is just the
 *      biggest words on the page.
 *   4. *"make this page has the author name, date, word count."* One byline
 *      line under the title, the way a newspaper sets it.
 *
 * ## Whose words
 *
 * `report.piece` is the draft verbatim — entirely hers, since 印记 never
 * authors her prose (铁律①) — so unlike 我的笔记 there is no second origin to
 * separate out. The byline is what keeps it from floating anonymously, and it
 * is the only attribution on the page; nothing here may render another
 * person's or the model's words.
 */
export function ArticleView({
  report,
  actions,
  sharePanel,
  onOpenRecord,
}: {
  report: LiteReport;
  /** 导出/分享 icon buttons. Omitted entirely on the public page — a visitor
   *  is not the owner and must never see controls over someone else's work. */
  actions?: React.ReactNode;
  sharePanel?: React.ReactNode;
  /** Go to 这一篇是怎么写出来的. */
  onOpenRecord: () => void;
}) {
  const paragraphs = splitParagraphs(report.piece);

  return (
    <article className="mk-rp mx-auto flex w-full max-w-[34rem] flex-col px-5 py-10 sm:py-16">
      <header className="mk-rp-rise flex flex-col gap-4" style={rise(0)}>
        {actions && <div className="flex justify-end">{actions}</div>}
        {/* Type, not a masthead. Serif so the title belongs to the article
            below it rather than to the app's chrome. */}
        <h1 className="font-mk-piece text-mk-report-title text-mk-ink">{report.title}</h1>
        <Byline report={report} />
      </header>

      {sharePanel && <div className="mt-6">{sharePanel}</div>}

      <div className="mt-9 flex flex-col gap-6">
        {paragraphs.map((p, i) => (
          <p key={i} className="whitespace-pre-wrap font-mk-piece text-mk-report-piece text-mk-ink">
            {p}
          </p>
        ))}
        {paragraphs.length === 0 && (
          // A finished writing with an empty draft is possible (she pressed
          // 完成这篇 on a blank page). Say so plainly rather than rendering a
          // headline over nothing.
          <p className="text-mk-body text-mk-muted">这一篇还没有正文。</p>
        )}
      </div>

      {/* The way to the record. A quiet line at the foot, the way a blog puts
          its footer matter — never a call to action, and never a score. */}
      <footer className="mt-12 border-t pt-6" style={{ borderColor: "var(--mk-border)" }}>
        <button
          type="button"
          onClick={onOpenRecord}
          className="text-mk-body text-mk-accent-700 underline decoration-dotted underline-offset-4 hover:text-mk-ink"
        >
          这一篇是怎么写出来的 →
        </button>
      </footer>
    </article>
  );
}

/**
 * Name · date · word count, on one line under the title.
 *
 * The word count is read off the report's own `words` stat rather than
 * counted here: the server already counts it per language
 * (`countWordsForLang`, atom_report.go), and a second count in the client
 * would quietly disagree with the number the report page shows for the same
 * piece. A missing or zero stat simply drops that segment — 0 字 is absence,
 * not a fact worth printing, the same rule the stat strip follows.
 */
function Byline({ report }: { report: LiteReport }) {
  const words = report.stats.find((s) => s.key === "words")?.value ?? 0;
  const parts = [report.studentName, formatDate(report.finishedAt), words > 0 ? `${words.toLocaleString("zh-CN")} 字` : ""];
  return (
    <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-mk-small text-mk-muted">
      {parts
        .filter((p) => p !== "")
        .map((p, i) => (
          <span key={i} className={i === 0 ? "text-mk-body text-mk-ink" : undefined}>
            {p}
          </span>
        ))}
    </p>
  );
}

/** Blank-line-separated paragraphs, as real `<p>`s. `\r` is stripped because
 *  a draft can arrive from a Windows paste. Shared shape with the report
 *  page's own prose handling — kept here because this is now the only place
 *  the piece renders. */
export function splitParagraphs(piece: string): string[] {
  return piece
    .split(/\n\s*\n/)
    .map((p) => p.replace(/\r/g, "").trim())
    .filter((p) => p !== "");
}

/** Absolute date, never 今天/昨天: this is read weeks later, and by people she
 *  shared it with, to whom a relative day means nothing. */
function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`;
}

function rise(i: number): React.CSSProperties {
  return { ["--i" as string]: i } as React.CSSProperties;
}
