import { ArrowRight, Home, Lock, MessageCircle, Sparkles } from "lucide-react";
import { useEco } from "../store";
import { TRACKS, trackById } from "../data/projects";
import { STUDENT } from "../data/library";
import { go } from "../route";
import { Btn, Panel, SectionHead, Sys, cx } from "../ui";

/**
 * 项目 · the PBL hub.
 *
 * ## The gate, and why it is not a gate
 * Until her homepage is published, the other tracks are closed. That could
 * read as an artificial lock, so the screen never says "unlock" — it says the
 * real reason: **每一个项目做完都会发布在你的主页上，所以先把家建好。** The
 * dependency is genuine (`PersonalPage` renders published projects), and the
 * copy points at it instead of at a padlock as a reward mechanic.
 *
 * Two doors into a project, always both offered: pick a track, or talk to 印记
 * first. A student who already knows what she wants should not have to sit
 * through a conversation; a student who doesn't should not have to guess from
 * five cards.
 */
export function ProjectsHub() {
  const { state, setNewProjectMode } = useEco();
  const unlocked = state.homepage.published;
  const running = state.projects.filter((p) => p.status !== "published");
  const published = state.projects.filter((p) => p.status === "published");

  return (
    <div className="mx-auto max-w-[1080px] px-8 py-8">
      <SectionHead
        index="项目 · PROJECT BASED LEARNING"
        title="做一个真的东西"
        sub="从你自己在乎的问题出发，做出一件能交给别人看的东西。"
        right={
          <Btn
            variant="outline"
            iconStart={<MessageCircle size={16} strokeWidth={1.8} />}
            onClick={() => {
              // Not the coach drawer: the REAL chat entrance lives on the
              // new-project screen, where 印记 reads her tree and proposes
              // three projects sourced from actual keywords.
              setNewProjectMode("talk");
              go({ name: "project-new" });
            }}
          >
            先和印记聊聊
          </Btn>
        }
      />

      {/* ── PBL#0 ───────────────────────────────────────────────────────── */}
      {!unlocked ? (
        <Panel className="mb-8 overflow-hidden">
          <div className="flex flex-col gap-6 p-7 md:flex-row md:items-center">
            <div className="min-w-0 flex-1">
              <Sys>第一个项目 · PROJECT 00</Sys>
              <h2 className="mt-2 text-mk-display text-mk-ink">建一个属于你的主页</h2>
              <p className="mt-3 max-w-[54ch] text-mk-body-lg leading-[1.9] text-mk-secondary">
                这不是一道门槛，是一个顺序问题：
                <strong className="font-semibold text-mk-ink">
                  之后每一个项目做完，都会发布在你的主页上
                </strong>
                。先有家，作品才有地方放。
              </p>
              <ul className="mt-4 space-y-1.5">
                {[
                  "看六个真实的个人主页，弄清楚什么叫「好」",
                  "学会给 AI 下清楚的指令 —— 这一招你以后到处都用得上",
                  "选风格、写内容、发布、拿到你自己的网址",
                ].map((t) => (
                  <li key={t} className="flex gap-2.5 text-mk-body text-mk-secondary">
                    <span className="mt-2 h-1 w-1 shrink-0 rounded-mk-full" style={{ background: "var(--mk-accent)" }} />
                    {t}
                  </li>
                ))}
              </ul>
              <div className="mt-5 flex flex-wrap gap-3">
                <Btn iconStart={<Home size={16} strokeWidth={1.8} />} onClick={() => go({ name: "homepage" })}>
                  {state.homepage.step > 0 ? "继续建我的主页" : "开始"}
                </Btn>
                <span className="self-center text-mk-small text-mk-muted">
                  六步，大约 60 分钟。中途可以停。
                </span>
              </div>
            </div>

            {/* A small live sketch of what she is about to make. */}
            <div className="w-full shrink-0 md:w-[268px]">
              <div
                className="rounded-mk-lg p-5 shadow-mk-sm"
                style={{ background: "var(--mk-paper)", border: "1px solid var(--mk-border)" }}
              >
                <div className="h-2 w-14 rounded-mk-full" style={{ background: "var(--mk-accent-300)" }} />
                <p className="mt-3 font-mk-piece text-mk-h2 text-mk-ink">{STUDENT.name}</p>
                <p className="mt-1.5 text-mk-small leading-relaxed text-mk-muted">
                  一句话介绍还没写
                </p>
                <div className="mt-4 space-y-1.5">
                  {["我在乎的问题", "我做过的", "我写的", "我读过的"].map((s) => (
                    <div key={s} className="flex items-center gap-2">
                      <span className="h-1.5 w-1.5 rounded-mk-full" style={{ background: "var(--mk-border)" }} />
                      <span className="h-2 flex-1 rounded-mk-full" style={{ background: "var(--mk-border)" }} />
                    </div>
                  ))}
                </div>
                <p className="eco-mono mt-4 text-mk-faint">mind.im/p/{STUDENT.handle}</p>
              </div>
            </div>
          </div>
        </Panel>
      ) : null}

      {/* ── tracks ──────────────────────────────────────────────────────── */}
      <div className="mb-3 flex items-end justify-between gap-4">
        <div>
          <Sys>五条赛道 · TRACKS</Sys>
          <h2 className="mt-1 text-mk-h1 text-mk-ink">你想做哪一种？</h2>
        </div>
        {!unlocked ? (
          <span className="inline-flex items-center gap-1.5 rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 text-mk-small text-mk-muted">
            <Lock size={13} strokeWidth={1.9} />
            主页发布后开放
          </span>
        ) : null}
      </div>

      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {TRACKS.map((t, i) => (
          <li key={t.id} className="eco-in" style={{ ["--i" as string]: i }}>
            <button
              type="button"
              disabled={!unlocked}
              onClick={() => {
                setNewProjectMode("pick");
                go({ name: "project-new" });
              }}
              className={cx(
                "flex h-full w-full flex-col rounded-mk-lg border bg-mk-surface p-5 text-left transition-all",
                "duration-[160ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                unlocked
                  ? "border-mk-border hover:-translate-y-0.5 hover:border-mk-accent-200 hover:shadow-mk-md"
                  : "cursor-not-allowed border-dashed border-mk-border opacity-60",
              )}
            >
              <span
                className="flex h-10 w-10 items-center justify-center rounded-mk-md font-mono text-[18px]"
                style={{ background: `color-mix(in srgb, ${t.hue} 26%, var(--mk-surface))`, color: "var(--mk-ink)" }}
              >
                {t.glyph}
              </span>
              <h3 className="mt-3 text-mk-h2 text-mk-ink">{t.label}</h3>
              <p className="mt-1.5 text-mk-body leading-[1.8] text-mk-secondary">{t.blurb}</p>
              <p className="mt-3 text-mk-small text-mk-muted">
                <span className="eco-mono">做完你会有</span>
                <br />
                {t.ends}
              </p>
              <ul className="mt-3 space-y-1">
                {t.examples.map((e) => (
                  <li key={e} className="text-mk-small text-mk-faint">
                    · {e}
                  </li>
                ))}
              </ul>
            </button>
          </li>
        ))}
      </ul>

      {/* ── her projects ────────────────────────────────────────────────── */}
      {(running.length > 0 || published.length > 0) && (
        <>
          <hr className="eco-hair my-9" />
          <SectionHead index="我的项目 · MINE" title="我做过和正在做的" />
          <ul className="grid gap-3 md:grid-cols-2">
            {[...running, ...published].map((p) => {
              const t = trackById(p.track);
              const done = p.steps.filter((s) => s.done).length;
              return (
                <li key={p.id}>
                  <button
                    type="button"
                    onClick={() => go({ name: "project", id: p.id })}
                    className="flex w-full flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left
                               transition-all duration-[160ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200
                               hover:shadow-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  >
                    <div className="flex items-center gap-2">
                      <span className="h-2 w-2 rounded-mk-full" style={{ background: t.hue }} />
                      <Sys>{t.label}</Sys>
                      <span
                        className="ml-auto rounded-mk-full px-2 py-0.5 text-[11px] font-medium"
                        style={
                          p.status === "published"
                            ? { background: "var(--mk-success-bg)", color: "var(--mk-success)" }
                            : { background: "var(--mk-butter-bg)", color: "var(--mk-butter-fg)" }
                        }
                      >
                        {p.status === "published" ? "已发布" : "进行中"}
                      </span>
                    </div>
                    <h3 className="mt-2 text-mk-h2 text-mk-ink">{p.title}</h3>
                    {p.motivation?.who ? (
                      <p className="mt-2 flex gap-2 text-mk-small leading-relaxed text-mk-secondary">
                        <Sparkles size={13} strokeWidth={2} className="mt-0.5 shrink-0" color="#C9962B" />
                        为了 {p.motivation.who}
                      </p>
                    ) : null}
                    <div className="mt-4 flex items-center gap-3">
                      <div className="h-1.5 flex-1 overflow-hidden rounded-mk-full" style={{ background: "var(--mk-border)" }}>
                        <div
                          className="h-full rounded-mk-full transition-all duration-500 ease-mk"
                          style={{ width: `${(done / p.steps.length) * 100}%`, background: t.hue }}
                        />
                      </div>
                      <span className="font-mono text-[11px] tabular-nums text-mk-muted">
                        {done}/{p.steps.length}
                      </span>
                      <ArrowRight size={15} strokeWidth={1.9} className="text-mk-muted" />
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        </>
      )}
    </div>
  );
}
