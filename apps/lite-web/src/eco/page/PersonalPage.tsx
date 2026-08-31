import { useEco } from "../store";
import { styleById } from "../data/homepage";
import { READINGS, STUDENT, WRITINGS } from "../data/library";
import { trackById } from "../data/projects";
import { motiveOf } from "../data/cards";
import { go } from "../route";
import { Btn, Empty, cx } from "../ui";

/**
 * 她的个人主页 — the published page.
 *
 * Rendered in two situations and they are genuinely the same page:
 *  - `preview` — inside the studio's 发布 step, so what she approves is what
 *    ships. A "preview" that is a different component is a lie waiting to
 *    happen.
 *  - standalone at `/eco/p/:handle`, with NO app shell, because the person
 *    opening it is a friend or a parent with no account. Same reasoning as
 *    lite's `/s/:token`.
 *
 * Every visual decision comes from her chosen style token set, applied as
 * `--pg-*` custom properties consumed by `.eco-page` in `eco.css`. One
 * typeface, one accent — the constraint the style step taught.
 *
 * The page shows only what she PICKED. That is principle 03 made structural:
 * an automatic dump of everything she ever read would defeat the lesson.
 */
export function PersonalPage({ handle, preview = false }: { handle: string; preview?: boolean }) {
  const { state } = useEco();
  const hp = state.homepage;
  const style = styleById(hp.style ?? "morning");

  if (!preview && !hp.published) {
    return (
      <div className="mx-auto max-w-[640px] px-8 py-24">
        <Empty
          title="这个主页还没建好"
          body={`/p/${handle} 还没有发布。如果这是你的页面，回到项目 00 把它建完——六步，大约一小时。`}
          action={<Btn onClick={() => go({ name: "homepage" })}>去建我的主页</Btn>}
        />
      </div>
    );
  }

  const sections = hp.sections.filter((s) => s.enabled);
  const val = (id: string) => sections.find((s) => s.id === id)?.value.trim() ?? "";
  const picks = (id: string) => sections.find((s) => s.id === id)?.picked ?? [];

  const pickedProjects = state.projects.filter((p) => picks("projects").includes(p.id));
  const pickedWritings = WRITINGS.filter((w) => picks("writings").includes(w.id));
  const pickedReadings = READINGS.filter((r) => picks("readings").includes(r.id));

  return (
    <div
      className={cx("eco-page min-h-full", preview && "rounded-mk-lg")}
      style={
        {
          "--pg-paper": style.paper,
          "--pg-ink": style.ink,
          "--pg-accent": style.accent,
          "--pg-font": style.font,
        } as React.CSSProperties
      }
    >
      <div className={cx("mx-auto px-8", preview ? "max-w-[720px] py-12" : "max-w-[720px] py-20")}>
        {/* masthead */}
        <p
          className="text-[11px] uppercase tracking-[0.2em]"
          style={{ color: "var(--pg-accent)", fontFamily: 'ui-monospace,"SF Mono",monospace' }}
        >
          {STUDENT.handle} · {STUDENT.grade} · since {STUDENT.since}
        </p>
        <h1
          className={cx(
            "mt-3",
            style.headline === "serif-xl" && "text-[42px] font-bold leading-[1.25]",
            style.headline === "mono-caps" && "text-[30px] font-semibold leading-[1.3] tracking-[0.04em]",
            style.headline === "sans-tight" && "text-[52px] font-extrabold leading-[1.05] tracking-[-0.03em]",
            style.headline === "serif-italic" && "text-[38px] font-bold italic leading-[1.3]",
          )}
        >
          {STUDENT.name}
        </h1>

        {val("intro") ? (
          <p className="mt-5 text-[19px] leading-[1.85]">{val("intro")}</p>
        ) : (
          <p className="mt-5 text-[19px] leading-[1.85]" style={{ opacity: 0.4 }}>
            （一句话介绍还没写）
          </p>
        )}

        <div className="pg-rule mt-10" />

        {/* sections, in HER order */}
        {sections.map((s) => {
          if (s.id === "intro") return null;

          if (s.id === "question") {
            const v = val("question");
            if (!v) return null;
            return (
              <Block key={s.id} label="我在乎的问题">
                <p className="text-[22px] font-semibold leading-[1.7]">{v}</p>
              </Block>
            );
          }

          if (s.id === "detail") {
            const v = val("detail");
            if (!v) return null;
            return (
              <Block key={s.id} label="一个关于我的怪细节">
                <p className="text-[17px] leading-[1.9]">{v}</p>
              </Block>
            );
          }

          if (s.id === "contact") {
            const v = val("contact");
            if (!v) return null;
            return (
              <Block key={s.id} label="怎么找到我">
                <p className="text-[17px] leading-[1.9]">{v}</p>
              </Block>
            );
          }

          if (s.id === "projects") {
            if (pickedProjects.length === 0) return null;
            return (
              <Block key={s.id} label="我做过的">
                <ul className="space-y-4">
                  {pickedProjects.map((p) => {
                    const t = trackById(p.track);
                    return (
                      <li key={p.id} className="pg-card rounded-[10px] p-5">
                        <div className="flex items-baseline justify-between gap-3">
                          <h3 className="text-[19px] font-bold leading-snug">{p.title}</h3>
                          <span className="pg-chip shrink-0 rounded-full px-2.5 py-0.5 text-[11px]">
                            {p.status === "published" ? "已完成" : "进行中"}
                          </span>
                        </div>
                        <p className="mt-1 text-[12px] tracking-wide" style={{ opacity: 0.55 }}>
                          {t.label} · {p.startedAt}
                        </p>
                        {p.summary ? (
                          <p className="mt-2.5 text-[15px] leading-[1.9]" style={{ opacity: 0.86 }}>
                            {p.summary}
                          </p>
                        ) : null}
                        {motiveOf(p)?.who ? (
                          <p className="mt-2.5 text-[13px] leading-[1.8]" style={{ opacity: 0.62 }}>
                            为什么做它：为了{motiveOf(p)?.who}
                          </p>
                        ) : null}
                      </li>
                    );
                  })}
                </ul>
              </Block>
            );
          }

          if (s.id === "writings") {
            if (pickedWritings.length === 0) return null;
            return (
              <Block key={s.id} label="我写的">
                <ul className="space-y-5">
                  {pickedWritings.map((w) => (
                    <li key={w.id}>
                      <h3 className="text-[19px] font-bold leading-snug">{w.title}</h3>
                      <p className="mt-1 text-[12px]" style={{ opacity: 0.55 }}>
                        {w.date} · {w.words} 字 · {w.spine}
                      </p>
                      <p className="mt-2 text-[15px] leading-[1.95]" style={{ opacity: 0.86 }}>
                        {w.body[0]}
                      </p>
                    </li>
                  ))}
                </ul>
              </Block>
            );
          }

          if (s.id === "readings") {
            if (pickedReadings.length === 0) return null;
            return (
              <Block key={s.id} label="我读过的">
                <ul className="space-y-3">
                  {pickedReadings.map((r) => (
                    <li key={r.id} className="flex flex-col gap-1">
                      <span className="flex items-baseline gap-2">
                        <span className="text-[16px] font-semibold">{r.title}</span>
                        <span className="text-[12px]" style={{ opacity: 0.5 }}>
                          {r.source}
                        </span>
                      </span>
                      {r.takeaway ? (
                        <span
                          className="border-l-2 pl-3 text-[14px] leading-[1.85]"
                          style={{ opacity: 0.8, borderColor: "var(--pg-accent)" }}
                        >
                          {r.takeaway}
                        </span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </Block>
            );
          }

          return null;
        })}

        <div className="pg-rule mt-12" />
        <p className="mt-4 text-[12px]" style={{ opacity: 0.45 }}>
          这一页上的每一句话都是 {STUDENT.name} 自己写的。用 思维印记 做的。
        </p>
      </div>
    </div>
  );
}

function Block({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="mt-10">
      <h2
        className="text-[11px] uppercase tracking-[0.2em]"
        style={{ color: "var(--pg-accent)", fontFamily: 'ui-monospace,"SF Mono",monospace' }}
      >
        {label}
      </h2>
      <div className="mt-3">{children}</div>
    </section>
  );
}
