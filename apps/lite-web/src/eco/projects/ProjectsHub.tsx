import { useState } from "react";
import { ArrowRight, Globe, Lightbulb, Plus } from "lucide-react";
import { useEco } from "../store";
import { EXAMPLE_PROBLEM, TRACKS, trackById } from "../data/projects";
import { activeSteps } from "../data/plan";
import { motiveOf } from "../data/cards";
import { PBL_COVENANT } from "../data/method";
import { go } from "../route";
import type { Project } from "../data/types";
import { Btn, Panel, SectionHead, Sys, cx } from "../ui";

/**
 * 项目 · the PBL hub.
 *
 * ## The empty screen is the design (2026-08-31)
 * A student opening 项目 for the first time has no projects, no idea what a
 * project here is, and no reason to trust that finishing one is possible. The
 * worst thing to show her is a menu of five kinds. So the empty state is **one
 * sentence and one button** — a specific first project, described concretely
 * enough that she can picture the thing that will exist at the end of it.
 *
 * The personal page is the right first project for a reason that is not
 * sentimental: it is the only one whose output every later project feeds into.
 * Everything she publishes afterwards lands on the page she built here.
 *
 * Under the button, quieter, is the other door — 我发现了一个真问题 — which is
 * the one that goes through 澄清 → 选路 → 计划 instead of straight to a plan.
 * It is second because a student who already has a real problem will find it,
 * and a student who does not should not be asked to invent one.
 */
export function ProjectsHub() {
  const { state } = useEco();
  const running = state.projects.filter((p) => p.phase !== "published");
  const published = state.projects.filter((p) => p.phase === "published");

  if (state.projects.length === 0) return <FirstRun />;

  return (
    <div className="mx-auto max-w-[1080px] px-8 py-8">
      <SectionHead
        index="项目 · PROJECT BASED LEARNING"
        title="做一个真的东西"
        sub="印记先给一份计划，你改完同意了才开工。它可以动手做，判断留给你。"
        right={<Doors />}
      />

      {running.length > 0 ? (
        <section className="mb-9">
          <Sys className="mb-2.5 block">在做的</Sys>
          <ul className="grid gap-3 md:grid-cols-2">
            {running.map((p, i) => (
              <li key={p.id} className="eco-in" style={{ ["--i" as string]: i }}>
                <ProjectCard project={p} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <Panel className="mb-9 p-6">
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

      <section className="mb-9">
        <Sys className="mb-2.5 block">可以做哪几类</Sys>
        <ul className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
          {TRACKS.map((t, i) => (
            <li key={t.id} className="eco-in" style={{ ["--i" as string]: i }}>
              <div
                className={cx(
                  "h-full rounded-mk-lg border border-mk-border bg-mk-surface p-5",
                )}
              >
                <span className="flex items-center gap-2">
                  <span className="text-[17px]" style={{ color: t.hue }}>
                    {t.glyph}
                  </span>
                  <span className="text-mk-h3 text-mk-ink">{t.label}</span>
                </span>
                <p className="mt-1.5 text-mk-body leading-[1.8] text-mk-secondary">{t.blurb}</p>
                <p className="mt-2.5 text-mk-small leading-[1.75] text-mk-muted">
                  <span className="text-mk-faint">做完你会有：</span>
                  {t.ends}
                </p>
              </div>
            </li>
          ))}
        </ul>
      </section>

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

/* ── the empty screen ─────────────────────────────────────────────────── */

function FirstRun() {
  const { startFirstProject, loadSamples } = useEco();
  const [problem, setProblem] = useState<string | null>(null);

  return (
    <div className="mx-auto flex min-h-full max-w-[760px] flex-col justify-center px-8 py-14">
      <div className="eco-in">
        <div className="flex items-center gap-2.5">
          <span
            className="flex h-8 w-8 items-center justify-center rounded-mk-full text-[13px] font-bold text-white"
            style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
          >
            印
          </span>
          <Sys>你还没有项目</Sys>
        </div>

        <h1 className="mt-4 text-mk-display text-mk-ink">我们从你自己的主页开始。</h1>
        <p className="mt-4 max-w-[58ch] text-mk-body-lg leading-[1.95] text-mk-secondary">
          做一个属于你的网页：它同时是你的简历和你的博客。
          <span className="text-mk-ink">你写过的东西、做完的项目，都会长在这一页上</span>，
          它有一个网址，你想发给谁就发给谁。
        </p>
        <p className="mt-3 max-w-[58ch] text-mk-body leading-[1.95] text-mk-muted">
          代码我来写。这一页上放什么、为什么是这些、别人看完记住你哪一点——这些是你的判断，
          也是这个项目真正在练的东西。
        </p>

        <Btn
          className="mt-7 !px-7 !py-4 !text-[17px]"
          iconStart={<ArrowRight size={19} strokeWidth={2.2} />}
          onClick={() => go({ name: "project", id: startFirstProject() })}
        >
          开始第一个项目
        </Btn>

        {/* ── the other door ──────────────────────────────────────────── */}
        <div className="mt-10 border-t border-mk-border pt-6">
          {problem === null ? (
            <button
              type="button"
              onClick={() => setProblem(EXAMPLE_PROBLEM)}
              className="group flex items-start gap-3 text-left focus-visible:outline-none"
            >
              <Lightbulb
                size={17}
                strokeWidth={1.9}
                className="mt-0.5 shrink-0 text-mk-muted group-hover:text-mk-accent-700"
              />
              <span>
                <span className="block text-mk-body font-semibold text-mk-ink group-hover:text-mk-accent-700">
                  我已经有一个想解决的真问题
                </span>
                <span className="mt-0.5 block text-mk-small leading-[1.8] text-mk-muted">
                  那走另一条路：先把问题问准，我给你几条不同的办法，选哪条由你定。
                </span>
              </span>
            </button>
          ) : (
            <ProblemDoor value={problem} onChange={setProblem} />
          )}
        </div>

        <button
          type="button"
          onClick={loadSamples}
          className="mt-8 text-mk-small text-mk-faint underline decoration-dotted underline-offset-4
                     transition-colors hover:text-mk-muted focus-visible:outline-none"
        >
          原型：载入两个示例项目，看看做完和做到一半长什么样
        </button>
      </div>
    </div>
  );
}

/**
 * The 我发现了一个真问题 door.
 *
 * 🚨 It opens PREFILLED with the worked example and says so. In the real
 * product 印记 would read whatever she typed and propose roads for it; in this
 * prototype the roads are hand-written for one problem. Presenting a free text
 * box that silently returns the garden's two roads no matter what she typed
 * would be the prototype lying about what it can do — see the 「原型数据」
 * convention in `data/news.ts`.
 */
function ProblemDoor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { startProblemProject } = useEco();
  return (
    <div className="eco-in">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <Sys>说说你发现了什么问题</Sys>
        <span
          className="eco-mono rounded-mk-full px-2 py-0.5 text-mk-faint"
          style={{ border: "1px solid var(--mk-border)" }}
          title="原型里只为这一个示例问题写好了印记的几条路"
        >
          原型数据
        </span>
      </div>
      <p className="mt-1 text-mk-small leading-[1.75] text-mk-muted">
        这里预填了一个示例。原型只为它写好了印记的那几条路——你可以改这段话，但下一步给出的办法还是按这个问题写的。
      </p>
      <textarea
        value={value}
        rows={4}
        onChange={(e) => onChange(e.target.value)}
        className="mt-2.5 w-full resize-y rounded-mk-md border border-mk-input-border bg-mk-surface p-3.5
                   text-mk-prose leading-[1.9] text-mk-ink outline-none transition-colors duration-[120ms]
                   focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
      />
      <Btn
        className="mt-3"
        iconStart={<ArrowRight size={16} strokeWidth={2} />}
        disabled={value.trim().length < 10}
        onClick={() => go({ name: "project", id: startProblemProject(value.trim()) })}
      >
        跟印记说这个问题
      </Btn>
    </div>
  );
}

/* ── shared bits ──────────────────────────────────────────────────────── */

function Doors() {
  const { startFirstProject, setNewProjectMode } = useEco();
  const { state } = useEco();
  const hasPage = state.projects.some((p) => p.track === "website");
  return (
    <div className="flex flex-wrap gap-2">
      {!hasPage ? (
        <Btn
          variant="outline"
          onClick={() => go({ name: "project", id: startFirstProject() })}
        >
          做我的主页
        </Btn>
      ) : null}
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
  );
}

/** Progress reads off the PLAN she approved, not off the track's card list.
 *  Those were two different lists that happened to agree; when a student
 *  deletes a step they stop agreeing, and the card on this screen is the one
 *  that would have lied. */
function ProjectCard({ project }: { project: Project }) {
  const t = trackById(project.track);
  const steps = activeSteps(project.plan);
  const done = Math.max(0, Math.min(project.at, steps.length));
  const motive = motiveOf(project);
  const now = steps[Math.min(project.at, steps.length - 1)];

  const stage =
    project.phase === "frame"
      ? "在把问题问准"
      : project.phase === "choose"
        ? "在选走哪条路"
        : project.phase === "plan"
          ? "计划等你确认"
          : now
            ? now.title
            : "在收尾";

  return (
    <button
      type="button"
      onClick={() => go({ name: "project", id: project.id })}
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
        {project.phase === "run" ? (
          <span className="font-mono text-mk-small tabular-nums text-mk-muted">
            {done}/{steps.length} 步
          </span>
        ) : null}
      </span>
      <span className="mt-1.5 block text-mk-h2 text-mk-ink">{project.title}</span>
      {motive ? (
        <span className="mt-1.5 block text-mk-small leading-[1.8] text-mk-muted">
          为{motive.who}
        </span>
      ) : null}
      {steps.length > 0 && project.phase === "run" ? (
        <span
          className="mt-3 block h-1.5 overflow-hidden rounded-mk-full"
          style={{ background: "var(--mk-border)" }}
        >
          <span
            className="block h-full rounded-mk-full"
            style={{ width: `${(done / steps.length) * 100}%`, background: t.hue }}
          />
        </span>
      ) : null}
      <span className="mt-3 flex items-center gap-1.5 text-mk-small text-mk-accent-700">
        <ArrowRight size={13} strokeWidth={2} />
        {stage}
      </span>
    </button>
  );
}
