import { Check } from "lucide-react";
import { useEco } from "../../store";
import { PAGE_STYLES } from "../../data/homepage";
import { STUDENT } from "../../data/library";
import { Sys, cx } from "../../ui";

/**
 * Step 3 · 选风格.
 *
 * Four styles, each previewed with HER OWN content (or a placeholder that
 * reads like real content). A style picker showing lorem ipsum teaches
 * nothing — the choice she is actually making is "does my sentence look right
 * in this", and she can only answer that with her sentence in it.
 *
 * Each card also states the constraint the style imposes (one typeface, one
 * accent), because 「少即是自信」is principle 05 and this is where it becomes a
 * decision rather than a slogan.
 */
export function StyleStep() {
  const { state, hpSetStyle } = useEco();
  const hp = state.homepage;
  const intro =
    hp.sections.find((s) => s.id === "intro")?.value.trim() ||
    "我是林知遥，初二。我最近一直在想：一个例外能代表多少？";
  const question =
    hp.sections.find((s) => s.id === "question")?.value.trim() ||
    "普通人的日常，怎么变成历史？";

  return (
    <div>
      <p className="max-w-[62ch] text-mk-body-lg leading-[1.9] text-mk-secondary">
        四种风格，每一种都是<strong className="font-semibold text-mk-ink">一种字体、一个主色</strong>
        ——不是四种模板，是四种态度。下面的预览用的是你自己的句子。
      </p>

      <ul className="mt-6 grid gap-4 md:grid-cols-2">
        {PAGE_STYLES.map((s, i) => {
          const on = hp.style === s.id;
          return (
            <li key={s.id} className="eco-in" style={{ ["--i" as string]: i }}>
              <button
                type="button"
                onClick={() => hpSetStyle(s.id)}
                className={cx(
                  "flex h-full w-full flex-col overflow-hidden rounded-mk-lg border text-left transition-all",
                  "duration-[160ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                  on ? "border-mk-accent shadow-mk-md" : "border-mk-border hover:shadow-mk-sm",
                )}
              >
                {/* live preview */}
                <div className="px-7 py-8" style={{ background: s.paper, color: s.ink, fontFamily: s.font }}>
                  <p
                    className="text-[10px] uppercase tracking-[0.18em]"
                    style={{ color: s.accent, fontFamily: 'ui-monospace,"SF Mono",monospace' }}
                  >
                    {STUDENT.handle} · {STUDENT.grade}
                  </p>
                  <p
                    className={cx(
                      "mt-2",
                      s.headline === "serif-xl" && "text-[26px] leading-[1.35] font-bold",
                      s.headline === "mono-caps" && "text-[19px] leading-[1.4] font-semibold tracking-[0.04em]",
                      s.headline === "sans-tight" && "text-[30px] leading-[1.12] font-extrabold tracking-[-0.02em]",
                      s.headline === "serif-italic" && "text-[24px] leading-[1.4] font-bold italic",
                    )}
                  >
                    {STUDENT.name}
                  </p>
                  <p className="mt-2.5 text-[13px] leading-[1.85]" style={{ opacity: 0.86 }}>
                    {intro}
                  </p>
                  <div className="mt-4 h-px w-full" style={{ background: s.ink, opacity: 0.16 }} />
                  <p className="mt-3 text-[12px] leading-[1.7]" style={{ opacity: 0.6 }}>
                    我在乎的问题 —— {question}
                  </p>
                  <span
                    className="mt-4 inline-block rounded-full px-2.5 py-1 text-[10px]"
                    style={{ border: `1px solid ${s.accent}`, color: s.accent }}
                  >
                    我做过的 · 3
                  </span>
                </div>

                <div className="flex items-start gap-3 border-t border-mk-border bg-mk-surface p-4">
                  <span
                    className={cx(
                      "mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full border",
                      on ? "border-mk-accent bg-mk-accent" : "border-mk-input-border",
                    )}
                  >
                    {on ? <Check size={12} strokeWidth={3} color="#fff" /> : null}
                  </span>
                  <span className="min-w-0">
                    <span className="flex items-baseline gap-2">
                      <span className="text-mk-h3 text-mk-ink">{s.label}</span>
                      <Sys>{s.en}</Sys>
                    </span>
                    <span className="mt-0.5 block text-mk-body leading-relaxed text-mk-secondary">{s.blurb}</span>
                  </span>
                </div>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
