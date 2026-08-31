import { ArrowRight, Globe, MessageCircle, Plus, Sparkles } from "lucide-react";
import { useEco } from "../store";
import { TRACKS, trackById } from "../data/projects";
import { cardById, cardProgress, motiveOf } from "../data/cards";
import { PBL_COVENANT } from "../data/method";
import { go } from "../route";
import { Btn, Panel, SectionHead, Sys, cx } from "../ui";

/**
 * 项目 · the PBL hub.
 *
 * ## No gate (2026-08-31)
 * v1 locked every track until she published a homepage. The reason given was
 * honest — published projects render on that page — but the effect was a
 * padlock on the first screen of the most ambitious part of the product. The
 * homepage is now simply one of the things you can make (a `website` project,
 * proposed first), and a project published before the page exists waits for
 * it. Nothing here is a reward mechanic (铁律②).
 *
 * ## What the hub has to establish in ten seconds
 *   ① a project here means making a real thing for a real person,
 *   ② 印记 works WITH her — it can build, it cannot decide,
 *   ③ two doors in: pick a kind, or let it read her tree.
 */
export function ProjectsHub() {
  const { state, setNewProjectMode } = useEco();
  const running = state.projects.filter((p) => p.status !== "published");
  const published = state.projects.filter((p) => p.status === "published");

  return (
    <div className="mx-auto max-w-[1080px] px-8 py-8">
      <SectionHead
        index="项目 · PROJECT BASED LEARNING"
        title="做一个真的东西"
        sub="从你自己在乎的问题出发，做出一件能交给别人看的东西。印记会一路给你工具卡。"
        right={
          <div className="flex flex-wrap gap-2">
            <Btn
              variant="outline"
              iconStart={<MessageCircle size={16} strokeWidth={1.8} />}
              onClick={() => {
                setNewProjectMode("talk");
                go({ name: "project-new" });
              }}
            >
              让印记提三个
            </Btn>
            <Btn
              iconStart={<Plus size={16} strokeWidth={2} />}
              onClick={() => {
                setNewProjectMode("pick");
                go({ name: "project-new" });
              }}
            >
              开新项目
            </Btn>
          </div>
        }
      />

      {/* ── the covenant, up front ─────────────────────────────────────── */}
      {/* It belongs on the FIRST screen, not buried in a project: a student
          deciding whether to start needs to know what the AI will and will not
          do for her before she commits three weeks. */}
      <Panel className="mb-8 p-6">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <Sys>{PBL_COVENANT.title}</Sys>
          <span className="text-mk-body-lg font-semibold text-mk-ink">{PBL_COVENANT.line}</span>
        </div>
        <div className="mt-4 grid gap-5 sm:grid-cols-2">
          <div>
            <p className="text-mk-small font-semibold text-mk-accent-700">印记可以动手做的</p>
            <ul className="mt-1.5 space-y-1.5">
              {PBL_COVENANT.can.map((c) => (
                <li key={c} className="text-mk-body leading-[1.8] text-mk-secondary">
                  · {c}
                </li>
              ))}
            </ul>
          </div>
          <div>
            <p className="text-mk-small font-semibold text-mk-ink">这些必须是你的</p>
            <ul className="mt-1.5 space-y-1.5">
              {PBL_COVENANT.must.map((c) => (
                <li key={c} className="text-mk-body leading-[1.8] text-mk-secondary">
                  · {c}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </Panel>

      {/* ── in flight ──────────────────────────────────────────────────── */}
      {running.length > 0 ? (
        <section className="mb-9">
          <Sys className="mb-2.5 block">在做的</Sys>
          <ul className="grid gap-3 md:grid-cols-2">
            {running.map((p, i) => {
              const t = trackById(p.track);
              const { done, total } = cardProgress(p.cards, p.track);
              const motive = motiveOf(p);
              const next = p.cards.find((c) => c.status !== "done");
              const nextSpec = next ? cardById(next.cardId) : undefined;
              return (
                <li key={p.id} className="eco-in" style={{ ["--i" as string]: i }}>
                  <button
                    type="button"
                    onClick={() => go({ name: "project", id: p.id })}
                    className="h-full w-full rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left
                               transition-colors duration-[140ms] ease-mk hover:border-mk-accent-200
                               hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2
                               focus-visible:ring-mk-accent-200"
                  >
                    <span className="flex items-center justify-between gap-3">
                      <span className="flex items-center gap-2">
                        <span className="h-2 w-2 rounded-mk-full" style={{ background: t.hue }} />
                        <Sys>{t.label}</Sys>
                      </span>
                      <span className="font-mono text-mk-small tabular-nums text-mk-muted">
                        {done}/{total} 张卡
                      </span>
                    </span>
                    <span className="mt-1.5 block text-mk-h2 text-mk-ink">{p.title}</span>
                    {motive ? (
                      <span className="mt-1.5 block text-mk-small leading-[1.8] text-mk-muted">
                        为{motive.who}
                      </span>
                    ) : null}
                    <span
                      className="mt-3 block h-1.5 overflow-hidden rounded-mk-full"
                      style={{ background: "var(--mk-border)" }}
                    >
                      <span
                        className="block h-full rounded-mk-full"
                        style={{ width: `${(done / total) * 100}%`, background: t.hue }}
                      />
                    </span>
                    {nextSpec ? (
                      <span className="mt-3 flex items-center gap-1.5 text-mk-small text-mk-accent-700">
                        <ArrowRight size={13} strokeWidth={2} />
                        下一张：{nextSpec.title}
                      </span>
                    ) : null}
                  </button>
                </li>
              );
            })}
          </ul>
        </section>
      ) : null}

      {/* ── the five kinds ─────────────────────────────────────────────── */}
      <section className="mb-9">
        <Sys className="mb-2.5 block">可以做哪几类</Sys>
        <ul className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
          {TRACKS.map((t, i) => (
            <li key={t.id} className="eco-in" style={{ ["--i" as string]: i }}>
              <button
                type="button"
                onClick={() => {
                  setNewProjectMode("pick");
                  go({ name: "project-new" });
                }}
                className={cx(
                  "h-full w-full rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left",
                  "transition-colors duration-[140ms] ease-mk hover:border-mk-accent-200",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                )}
              >
                <span className="flex items-center gap-2">
                  <span className="text-[17px]" style={{ color: t.hue }}>
                    {t.glyph}
                  </span>
                  <span className="text-mk-h3 text-mk-ink">{t.label}</span>
                </span>
                <span className="mt-1.5 block text-mk-body leading-[1.8] text-mk-secondary">
                  {t.blurb}
                </span>
                <span className="mt-2.5 block text-mk-small leading-[1.75] text-mk-muted">
                  <span className="text-mk-faint">做完你会有：</span>
                  {t.ends}
                </span>
                <span className="mt-2.5 block text-mk-small leading-[1.75] text-mk-faint">
                  {t.examples.slice(0, 2).join(" · ")}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </section>

      {/* ── the homepage, as a suggestion rather than a gate ───────────── */}
      {!state.homepage.published ? (
        <Panel className="mb-9 p-6">
          <div className="flex flex-col gap-5 md:flex-row md:items-center">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5">
                <Sparkles size={14} strokeWidth={2} className="text-mk-accent-700" />
                <Sys className="!text-mk-accent-700">如果不知道从哪开始</Sys>
              </div>
              <h3 className="mt-1.5 text-mk-h1 text-mk-ink">先做你自己的主页</h3>
              <p className="mt-2 max-w-[58ch] text-mk-body leading-[1.9] text-mk-secondary">
                它是「一个网站」这一类里最小的一个，做完你会有一个能发出去的网址。
                顺带你会学到这套流程里最值钱的一课：怎么把「我想要好看一点」变成一条 AI
                真的能执行的指令。你之后发布的每个项目也会出现在这一页上。
              </p>
            </div>
            <div className="flex shrink-0 flex-col gap-2">
              <Btn
                iconStart={<ArrowRight size={16} />}
                onClick={() => {
                  setNewProjectMode("pick");
                  go({ name: "project-new" });
                }}
              >
                做我的主页
              </Btn>
              <Btn variant="outline" onClick={() => go({ name: "homepage" })}>
                只想挑内容和风格
              </Btn>
            </div>
          </div>
        </Panel>
      ) : null}

      {/* ── published ──────────────────────────────────────────────────── */}
      {published.length > 0 ? (
        <section>
          <Sys className="mb-2.5 block">已发布</Sys>
          <ul className="space-y-3">
            {published.map((p) => {
              const t = trackById(p.track);
              return (
                <li key={p.id}>
                  <button
                    type="button"
                    onClick={() => go({ name: "project", id: p.id })}
                    className="w-full rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left
                               transition-colors duration-[140ms] hover:border-mk-accent-200
                               focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  >
                    <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                      <Globe size={14} strokeWidth={1.9} color="var(--mk-success)" />
                      <span className="h-2 w-2 rounded-mk-full" style={{ background: t.hue }} />
                      <Sys>{t.label}</Sys>
                      <span className="text-mk-h3 text-mk-ink">{p.title}</span>
                    </span>
                    <span className="mt-2 block max-w-[76ch] text-mk-body leading-[1.9] text-mk-secondary">
                      {p.summary}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </section>
      ) : null}
    </div>
  );
}
