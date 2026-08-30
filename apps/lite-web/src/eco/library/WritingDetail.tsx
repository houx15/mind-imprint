import { ArrowLeft, MessageCircle, Sprout } from "lucide-react";
import { useEco } from "../store";
import { writingById } from "../data/library";
import { KEYWORDS, fieldById } from "../data/tree";
import { go } from "../route";
import { Btn, Empty, Sys } from "../ui";

/** One piece of hers, read as an article — same page face as a reading, so
 *  her writing sits at the same level as what she reads. That equivalence is
 *  the point; it is not a stylistic accident. */
export function WritingDetail({ id }: { id: string }) {
  const { openCoach } = useEco();
  const w = writingById(id);

  if (!w) {
    return (
      <div className="mx-auto max-w-[720px] px-8 py-16">
        <Empty
          title="找不到这一篇"
          body="它可能还没写完，或者链接过期了。"
          action={<Btn onClick={() => go({ name: "writings" })}>回到我写过的</Btn>}
        />
      </div>
    );
  }

  const f = fieldById(w.field);
  const grew = KEYWORDS.filter((k) => k.sources.some((s) => s.kind === "writing" && s.id === w.id));

  return (
    <div className="mx-auto max-w-[820px] px-8 py-8">
      <Btn variant="quiet" size="sm" iconStart={<ArrowLeft size={15} />} onClick={() => go({ name: "writings" })}>
        我写过的
      </Btn>

      <header className="mt-5">
        <Sys>
          {w.date} · {w.words} 字 · 结构：{w.spine}
        </Sys>
        <h1 className="mt-2 font-mk-piece text-mk-report-title text-mk-ink">{w.title}</h1>
      </header>

      <hr className="eco-hair my-6" />

      <article className="font-mk-piece text-mk-report-piece text-mk-ink">
        {w.body.map((p, i) => (
          <p key={i} className={i > 0 ? "mt-5" : undefined}>
            {p}
          </p>
        ))}
      </article>

      <hr className="eco-hair my-8" />

      <section>
        <div className="flex items-center gap-2">
          <Sprout size={16} strokeWidth={1.9} color="var(--mk-success)" />
          <h2 className="text-mk-h3 text-mk-ink">这一篇长出的词</h2>
        </div>
        <ul className="mt-3 flex flex-wrap gap-2">
          {grew.map((k) => {
            const kf = fieldById(k.field);
            return (
              <li key={k.id}>
                <button
                  type="button"
                  onClick={() => go({ name: "home", view: "tree" })}
                  className="rounded-mk-full border bg-mk-surface px-3 py-1.5 text-mk-body transition-colors
                             duration-[120ms] hover:bg-mk-accent-50 focus-visible:outline-none
                             focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  style={{ borderColor: `color-mix(in srgb, ${kf.hue} 70%, transparent)` }}
                >
                  {k.text}
                </button>
              </li>
            );
          })}
        </ul>
      </section>

      <div className="mt-8 flex flex-wrap gap-3">
        <Btn variant="outline" iconStart={<MessageCircle size={16} strokeWidth={1.8} />} onClick={() => openCoach("writing")}>
          让印记看看这一篇
        </Btn>
        <Btn variant="quiet" onClick={() => go({ name: "homepage" })}>
          把它放上我的主页
        </Btn>
      </div>

      <p className="mt-6 text-mk-small text-mk-faint" style={{ borderLeft: `3px solid ${f.hue}`, paddingLeft: 12 }}>
        这一篇是你自己写的。印记只在结构和论证上提过问题，一个字都没有代写。
      </p>
    </div>
  );
}
