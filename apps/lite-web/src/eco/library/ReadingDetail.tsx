import { ArrowLeft, MessageCircle, Sprout } from "lucide-react";
import { useEco } from "../store";
import { readingById } from "../data/library";
import { KEYWORDS, fieldById } from "../data/tree";
import { go } from "../route";
import { Btn, Empty, Panel, Sys } from "../ui";

/**
 * One reading, read.
 *
 * The article itself is set in the serif page face (`font-mk-piece`, lite's
 * own token) so it reads as a text and not as chrome. Below it, the two links
 * back into the ecosystem: which keywords this reading grew, and the takeaway
 * she wrote — which is what the tree quotes as evidence.
 */
export function ReadingDetail({ id }: { id: string }) {
  const { openCoach, hpPickAndCompose } = useEco();
  const r = readingById(id);

  if (!r) {
    return (
      <div className="mx-auto max-w-[720px] px-8 py-16">
        <Empty
          title="找不到这一篇"
          body="链接可能过期了，或者这篇还没有进你的书架。"
          action={<Btn onClick={() => go({ name: "readings" })}>回到书架</Btn>}
        />
      </div>
    );
  }

  const f = fieldById(r.field);
  const grew = KEYWORDS.filter((k) => k.sources.some((s) => s.kind === "reading" && s.id === r.id));

  return (
    <div className="mx-auto max-w-[860px] px-8 py-8">
      <Btn variant="quiet" size="sm" iconStart={<ArrowLeft size={15} />} onClick={() => go({ name: "readings" })}>
        书架
      </Btn>

      <header className="mt-5">
        <div className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-mk-full" style={{ background: f.hue }} />
          <Sys>
            {f.label} · {r.source} · {r.date} · {r.minutes} 分钟
          </Sys>
        </div>
        <h1 className="mt-2 font-mk-piece text-mk-report-title text-mk-ink">{r.title}</h1>
      </header>

      <hr className="eco-hair my-6" />

      <article className="font-mk-piece text-mk-report-piece text-mk-ink">
        {r.excerpt.map((p, i) => (
          <p key={i} className={i > 0 ? "mt-5" : undefined}>
            {p}
          </p>
        ))}
      </article>

      {r.quote ? (
        <blockquote
          className="mt-8 rounded-mk-lg border-l-4 p-5"
          style={{ borderColor: f.hue, background: "var(--mk-surface)" }}
        >
          <Sys>你读到这里时写的</Sys>
          <p className="mt-2 text-mk-report-quote text-mk-ink">「{r.quote}」</p>
        </blockquote>
      ) : null}

      {r.takeaway ? (
        <Panel className="mt-4 p-5">
          <Sys>你的收获</Sys>
          <p className="mt-2 text-mk-body-lg leading-[1.9] text-mk-ink">{r.takeaway}</p>
        </Panel>
      ) : null}

      <hr className="eco-hair my-8" />

      <section>
        <div className="flex items-center gap-2">
          <Sprout size={16} strokeWidth={1.9} color="var(--mk-success)" />
          <h2 className="text-mk-h3 text-mk-ink">这一篇在你树上长出了什么</h2>
        </div>
        {grew.length === 0 ? (
          <p className="mt-2 text-mk-body text-mk-muted">
            还没有。一篇阅读要留下痕迹，通常需要你写下一句自己的话。
          </p>
        ) : (
          <ul className="mt-3 flex flex-wrap gap-2">
            {grew.map((k) => {
              const kf = fieldById(k.field);
              return (
                <li key={k.id}>
                  <button
                    type="button"
                    onClick={() => go({ name: "home", view: "tree" })}
                    className="flex items-center gap-2 rounded-mk-full border bg-mk-surface py-1.5 pl-3 pr-2.5
                               text-mk-body transition-colors duration-[120ms] hover:bg-mk-accent-50
                               focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                    style={{ borderColor: `color-mix(in srgb, ${kf.hue} 70%, transparent)` }}
                  >
                    {k.text}
                    <span
                      className="eco-mono rounded-mk-full px-1.5 py-0.5"
                      style={{ background: `color-mix(in srgb, ${kf.hue} 24%, var(--mk-surface))`, letterSpacing: 0 }}
                    >
                      {k.sources.length}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <div className="mt-8 flex flex-wrap gap-3">
        <Btn variant="outline" iconStart={<MessageCircle size={16} strokeWidth={1.8} />} onClick={() => openCoach("reading")}>
          和印记聊这一篇
        </Btn>
        <Btn
          variant="quiet"
          onClick={() => {
            // Pick it, THEN go — dropping her on step 1 of the studio with no
            // record of what she asked for reads as the button doing nothing.
            hpPickAndCompose("readings", r.id);
            go({ name: "homepage" });
          }}
        >
          把它放上我的主页
        </Btn>
      </div>
    </div>
  );
}
