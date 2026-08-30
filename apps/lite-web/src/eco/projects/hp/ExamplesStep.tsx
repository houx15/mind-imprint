import { Check, ExternalLink } from "lucide-react";
import { useEco } from "../../store";
import { PAGE_EXAMPLES, PAGE_PRINCIPLES, WEIRD_DETAIL_NOTE } from "../../data/examples";
import type { PageExample } from "../../data/types";
import { Bold, Panel, Sys, cx } from "../../ui";

/**
 * Step 1 · 看看别人的家.
 *
 * A student asked to "make a personal page" with no examples makes a résumé.
 * So the first thing she sees is six pages that are visibly not résumés, each
 * annotated with what it actually does and ONE move to steal.
 *
 * The thumbnails are ABSTRACT (`shape` + two colours), never screenshots or
 * imitations — these are real people's sites and we are pointing at them, not
 * reproducing them.
 *
 * Choosing one is the step's output: it becomes 「直接的例子」, the first of the
 * three things a clear instruction needs.
 */
export function ExamplesStep() {
  const { state, hpPickExample } = useEco();
  const chosen = state.homepage.exampleId;

  return (
    <div>
      <p className="max-w-[62ch] text-mk-body-lg leading-[1.9] text-mk-secondary">
        下面六个是真实存在的个人主页，作者是设计师、程序员、小说家。先看，再挑一个你希望自己的页面像它的。
        <strong className="font-semibold text-mk-ink">挑哪个不重要，说得出为什么才重要。</strong>
      </p>

      <ul className="mt-6 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {PAGE_EXAMPLES.map((ex, i) => (
          <li key={ex.id} className="eco-in" style={{ ["--i" as string]: i }}>
            <ExampleCard ex={ex} chosen={chosen === ex.id} onChoose={() => hpPickExample(ex.id)} />
          </li>
        ))}
      </ul>

      <Panel className="mt-8 p-6">
        <Sys>看完之后 · 五条原则</Sys>
        <h3 className="mt-1.5 text-mk-h1 text-mk-ink">一个好的个人主页，好在哪</h3>
        <ol className="mt-5 space-y-4">
          {PAGE_PRINCIPLES.map((p) => (
            <li key={p.n} className="flex gap-4">
              <span className="font-mono text-mk-report-numeral leading-none text-mk-accent-300">{p.n}</span>
              <span className="min-w-0">
                <span className="block text-mk-h3 text-mk-ink">{p.title}</span>
                <span className="mt-1 block text-mk-body leading-[1.85] text-mk-secondary">{p.body}</span>
              </span>
            </li>
          ))}
        </ol>
        <div
          className="mt-6 rounded-mk-md p-4"
          style={{ background: "var(--mk-butter-bg)", border: "1px solid var(--mk-butter)" }}
        >
          <p className="text-mk-body leading-[1.9] text-[#7A5A1D]">
            <Bold text={WEIRD_DETAIL_NOTE} className="font-bold" />
          </p>
        </div>
      </Panel>
    </div>
  );
}

function ExampleCard({
  ex,
  chosen,
  onChoose,
}: {
  ex: PageExample;
  chosen: boolean;
  onChoose: () => void;
}) {
  return (
    <div
      className={cx(
        "flex h-full flex-col overflow-hidden rounded-mk-lg border bg-mk-surface transition-all duration-[160ms] ease-mk",
        chosen ? "border-mk-accent shadow-mk-md" : "border-mk-border hover:shadow-mk-sm",
      )}
    >
      <Thumb ex={ex} />
      <div className="flex min-w-0 flex-1 flex-col p-5">
        <div className="flex items-baseline justify-between gap-3">
          <h3 className="text-mk-h2 text-mk-ink">{ex.name}</h3>
          <span className="inline-flex shrink-0 items-center gap-1 font-mono text-[11px] text-mk-faint">
            <ExternalLink size={11} strokeWidth={2} />
            {ex.url}
          </span>
        </div>
        <p className="mt-1 text-mk-small text-mk-muted">{ex.who}</p>

        <ul className="mt-4 space-y-2">
          {ex.good.map((g, i) => (
            <li key={i} className="flex gap-2.5 text-mk-body leading-[1.8] text-mk-secondary">
              <span className="mt-2 h-1 w-1 shrink-0 rounded-mk-full" style={{ background: ex.swatch[1] }} />
              <span>
                <Bold text={g} />
              </span>
            </li>
          ))}
        </ul>

        <div
          className="mt-4 rounded-mk-md p-3.5"
          style={{ background: "var(--mk-accent-50)", border: "1px solid var(--mk-accent-100)" }}
        >
          <Sys className="!text-mk-accent-700">你能偷走的那一招</Sys>
          <p className="mt-1 text-mk-body font-medium leading-[1.8] text-mk-accent-800">{ex.steal}</p>
        </div>

        <button
          type="button"
          onClick={onChoose}
          className={cx(
            "mt-4 flex w-full items-center justify-center gap-2 rounded-mk-sm px-4 py-2.5 text-mk-body font-medium",
            "transition-colors duration-[140ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
            chosen
              ? "bg-mk-accent text-white"
              : "border border-mk-border text-mk-secondary hover:border-mk-accent hover:text-mk-accent-700",
          )}
        >
          {chosen ? <Check size={16} strokeWidth={2.4} /> : null}
          {chosen ? "我要它像这个" : "选它当我的例子"}
        </button>
      </div>
    </div>
  );
}

/**
 * An abstract sketch of the page's SHAPE — essay column, grid, terminal,
 * playful cards, minimal, notebook. It carries the layout idea without
 * pretending to be the site.
 */
function Thumb({ ex }: { ex: PageExample }) {
  const [bg, fg] = ex.swatch;
  const bar = (w: string, h = 6, o = 1) => (
    <span className="block rounded-[2px]" style={{ width: w, height: h, background: fg, opacity: o }} />
  );
  return (
    <div className="relative h-[132px] w-full overflow-hidden p-4" style={{ background: bg }}>
      <span className="eco-mono absolute right-3 top-3" style={{ color: fg, opacity: 0.45 }}>
        {ex.shape}
      </span>
      {ex.shape === "essay" ? (
        <div className="flex flex-col gap-2">
          {bar("42%", 9)}
          {bar("78%", 5, 0.5)}
          <span className="my-1 block h-px w-full" style={{ background: fg, opacity: 0.2 }} />
          {[0.75, 0.6, 0.85, 0.55].map((w, i) => (
            <span key={i} className="flex items-center gap-2">
              <span className="block h-1 w-6 rounded" style={{ background: fg, opacity: 0.3 }} />
              {bar(`${w * 70}%`, 4, 0.45)}
            </span>
          ))}
        </div>
      ) : null}
      {ex.shape === "notebook" ? (
        <div className="flex flex-col gap-2">
          {bar("58%", 9)}
          <div className="mt-1 flex gap-1.5">
            {["种子", "生长中", "常青"].map((t) => (
              <span
                key={t}
                className="rounded-full px-2 py-0.5 text-[8px]"
                style={{ border: `1px solid ${fg}`, color: fg, opacity: 0.75 }}
              >
                {t}
              </span>
            ))}
          </div>
          <div className="mt-1 grid grid-cols-3 gap-1.5">
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <span key={i} className="h-6 rounded" style={{ background: fg, opacity: 0.14 + (i % 3) * 0.07 }} />
            ))}
          </div>
        </div>
      ) : null}
      {ex.shape === "minimal" ? (
        <div className="flex flex-col gap-1.5">
          {bar("34%", 8)}
          {[...Array(7)].map((_, i) => (
            <span key={i} className="flex items-center gap-2">
              <span className="block h-1 w-4 rounded" style={{ background: fg, opacity: 0.25 }} />
              {bar(`${45 + ((i * 13) % 40)}%`, 3, 0.4)}
            </span>
          ))}
        </div>
      ) : null}
      {ex.shape === "playful" ? (
        <div className="flex flex-col gap-2">
          {bar("48%", 9)}
          <div className="mt-1 grid grid-cols-3 gap-2">
            {[0, 1, 2].map((i) => (
              <span
                key={i}
                className="h-[54px] rounded-md"
                style={{ background: fg, opacity: 0.18 + i * 0.1, border: `1px solid ${fg}` }}
              />
            ))}
          </div>
        </div>
      ) : null}
      {ex.shape === "grid" ? (
        <div className="flex flex-col gap-2">
          <div className="flex items-baseline gap-2">
            {bar("30%", 9)}
            <span className="flex gap-1.5">
              {[0, 1, 2].map((i) => (
                <span key={i} className="block h-1.5 w-5 rounded" style={{ background: fg, opacity: 0.4 }} />
              ))}
            </span>
          </div>
          <div className="mt-1 grid grid-cols-2 gap-1.5">
            {[...Array(4)].map((_, i) => (
              <span key={i} className="h-9 rounded" style={{ background: fg, opacity: 0.12 + (i % 2) * 0.08 }} />
            ))}
          </div>
        </div>
      ) : null}
      {ex.shape === "terminal" ? (
        <div className="flex flex-col gap-2 font-mono">
          <span className="text-[13px]" style={{ color: fg }}>
            AaBbCc123
          </span>
          <span className="flex gap-2">
            {["about", "projects", "work", "photo", "shop"].map((t) => (
              <span key={t} className="text-[8px]" style={{ color: fg, opacity: 0.6 }}>
                {t}
              </span>
            ))}
          </span>
          <span className="mt-1 block h-px w-full" style={{ background: fg, opacity: 0.25 }} />
          {[...Array(4)].map((_, i) => (
            <span key={i} className="block h-1 rounded" style={{ width: `${70 - i * 9}%`, background: fg, opacity: 0.3 }} />
          ))}
        </div>
      ) : null}
    </div>
  );
}
