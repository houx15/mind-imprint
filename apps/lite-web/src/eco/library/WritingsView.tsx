import { MessageCircle, PenLine } from "lucide-react";
import { useEco } from "../store";
import { WRITINGS } from "../data/library";
import { fieldById } from "../data/tree";
import { go } from "../route";
import { Btn, SectionHead, Sys, cx } from "../ui";

/**
 * 写作 · what she has written.
 *
 * The card leads with the SPINE she used (立场式 / 起承转合 / 钩子式 / 记叙),
 * not the word count — lite teaches structure by name, so the shelf should
 * show her that she has a repertoire, and where it repeats.
 */
export function WritingsView() {
  const { openCoach } = useEco();
  const words = WRITINGS.reduce((s, w) => s + w.words, 0);

  return (
    <div className="mx-auto max-w-[1080px] px-8 py-8">
      <SectionHead
        index="我写的 · WRITING SHELF"
        title="我写过的"
        sub="每一篇下面写着你用的结构。看看哪一种你已经用熟了，哪一种还没试过。"
        right={
          <div className="flex items-center gap-5">
            <span className="text-right">
              <Sys>篇数</Sys>
              <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">{WRITINGS.length}</span>
            </span>
            <span className="h-8 w-px" style={{ background: "var(--mk-border)" }} />
            <span className="text-right">
              <Sys>总字数</Sys>
              <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">{words}</span>
            </span>
          </div>
        }
      />

      <ul className="grid gap-3 md:grid-cols-2">
        {WRITINGS.map((w, i) => {
          const f = fieldById(w.field);
          return (
            <li key={w.id} className="eco-in" style={{ ["--i" as string]: i }}>
              <button
                type="button"
                onClick={() => go({ name: "writings", id: w.id })}
                className={cx(
                  "flex h-full w-full flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left",
                  "transition-all duration-[160ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200",
                  "hover:shadow-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                )}
              >
                <div className="flex items-center gap-2">
                  <span className="h-2 w-2 rounded-mk-full" style={{ background: f.hue }} />
                  <Sys>{w.date}</Sys>
                  <span className="ml-auto font-mono text-[11px] text-mk-faint">{w.words} 字</span>
                </div>
                <h3 className="mt-2 font-mk-piece text-mk-h2 text-mk-ink">{w.title}</h3>
                <p className="mt-3 line-clamp-3 text-mk-body leading-[1.8] text-mk-secondary">
                  {w.body[0]}
                </p>
                <div className="mt-auto flex items-center gap-2 pt-4">
                  <span
                    className="rounded-mk-full px-2.5 py-1 text-mk-small font-medium"
                    style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-700)" }}
                  >
                    {w.spine}
                  </span>
                  {w.keywords.slice(0, 2).map((k) => (
                    <span key={k} className="text-mk-small text-mk-muted">
                      {k}
                    </span>
                  ))}
                </div>
              </button>
            </li>
          );
        })}
      </ul>

      <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
        <Btn iconStart={<PenLine size={16} strokeWidth={1.8} />} onClick={() => openCoach("writing")}>
          开始写一篇
        </Btn>
        <Btn variant="quiet" iconStart={<MessageCircle size={16} strokeWidth={1.8} />} onClick={() => openCoach("writing")}>
          先和印记聊聊写什么
        </Btn>
      </div>
    </div>
  );
}
