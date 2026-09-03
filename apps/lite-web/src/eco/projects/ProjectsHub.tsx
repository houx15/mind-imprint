import { useState } from "react";
import { ArrowRight, ArrowUp, Globe, Lightbulb, RotateCcw } from "lucide-react";
import { useEco } from "../store";
import { EXAMPLE_PROBLEM, TRACKS, trackById } from "../data/projects";
import { activeSteps } from "../data/plan";
import { motiveOf } from "../data/cards";
import { PBL_COVENANT } from "../data/method";
import { go } from "../route";
import type { Project, TrackId } from "../data/types";
import { Btn, Panel, Sys, cx } from "../ui";
import { CoverArt } from "./CoverPicker";

/**
 * 项目 · the PBL hub.
 *
 * ## Two modes, one switch
 * **开始新项目** — a single large composer. **我的项目** — the shelf.
 *
 * They are genuinely different jobs and putting them on one screen made both
 * worse: the composer got squeezed under a list, and the list got read as "the
 * thing this page is for" when the thing this page is for, most of the time,
 * is starting something.
 *
 * ## The composer
 * One big box. She types what she wants to make, in her own words, and the
 * category bubbles under it are a way to *steer* rather than a form to fill —
 * tapping one drops a starting sentence in, which she then rewrites. A picker
 * of five kinds asks her to classify her idea before she has said it; a box
 * asks her to say it.
 *
 * ## The empty screen is still one button
 * With zero projects there is no shelf to switch to and no reason to make a
 * thirteen-year-old invent a project from nothing, so the first visit is one
 * sentence and one button: her own page. The switcher appears once she has
 * something to switch to.
 */
export function ProjectsHub() {
  const { state } = useEco();
  const [tab, setTab] = useState<"new" | "mine">("new");

  if (state.projects.length === 0) return <FirstRun />;

  return (
    <div className="mx-auto max-w-[1000px] px-8 py-8">
      <div className="mb-7 flex flex-wrap items-center justify-between gap-4">
        <div>
          <Sys>项目 · PROJECT BASED LEARNING</Sys>
          <h1 className="mt-1 text-mk-display text-mk-ink">做一个真的东西</h1>
        </div>
        <Switcher tab={tab} onTab={setTab} count={state.projects.length} />
      </div>

      {tab === "new" ? <Composer /> : <Shelf />}
    </div>
  );
}

function Switcher({
  tab,
  onTab,
  count,
}: {
  tab: "new" | "mine";
  onTab: (t: "new" | "mine") => void;
  count: number;
}) {
  return (
    <div className="flex gap-1 rounded-mk-full p-1" style={{ background: "var(--mk-paper)" }}>
      {(
        [
          ["new", "开始新项目", null],
          ["mine", "我的项目", count],
        ] as const
      ).map(([k, label, n]) => (
        <button
          key={k}
          type="button"
          onClick={() => onTab(k)}
          className={cx(
            "flex items-center gap-2 rounded-mk-full px-4 py-2 text-mk-body transition-colors",
            "duration-[120ms] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
            tab === k ? "bg-mk-surface font-semibold text-mk-ink shadow-mk-xs" : "text-mk-muted",
          )}
        >
          {label}
          {n !== null ? (
            <span className="font-mono text-mk-small tabular-nums text-mk-faint">{n}</span>
          ) : null}
        </button>
      ))}
    </div>
  );
}

/* ── 开始新项目 ────────────────────────────────────────────────────────── */

/** What tapping a bubble puts in the box. A first sentence to rewrite, not a
 *  category to be filed under. */
const SEEDS: Record<TrackId, string> = {
  website: "我想做一个网页，关于",
  design: "我想给一个具体的人设计一个东西，因为",
  game: "我想做一个游戏，让玩的人明白",
  survey: "我想去问一些人，搞清楚",
  other: "我想做的是",
};

function Composer() {
  const { createProject, draftTrack, draftTitle, draftIntent, startProblemProject } = useEco();
  const [text, setText] = useState("");
  const [track, setTrack] = useState<TrackId | null>(null);

  function pick(t: TrackId) {
    setTrack(t);
    if (!text.trim()) setText(SEEDS[t]);
  }

  function start() {
    const said = text.trim();
    if (!said) return;
    const t = track ?? "other";
    draftTrack(t);
    // The first line is the name until she renames it; the whole thing is kept
    // verbatim as what she said at the start.
    draftTitle(said.length > 24 ? `${said.slice(0, 24)}…` : said);
    draftIntent(said);
    go({ name: "project", id: createProject() });
  }

  return (
    <>
      <div
        className="rounded-mk-lg border p-5 transition-colors duration-[140ms]"
        style={{ borderColor: "var(--mk-border)", background: "var(--mk-surface)" }}
      >
        <div className="flex items-center gap-2">
          <span className="eco-mono text-mk-faint">你想做什么</span>
        </div>
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) start();
          }}
          rows={5}
          placeholder="用你自己的话说。做什么、为谁做、为什么是这件事——想到哪儿写到哪儿，印记会追问。"
          className="mt-2 w-full resize-none border-0 bg-transparent p-0 text-[17px] leading-[1.85]
                     text-mk-ink outline-none placeholder:text-mk-faint"
          style={{ fontFamily: "inherit" }}
        />

        {/* the bubbles */}
        <div className="mt-4 flex flex-wrap items-center gap-1.5 border-t border-mk-border pt-4">
          {TRACKS.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => pick(t.id)}
              className={cx(
                "inline-flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-small",
                "transition-colors duration-[120ms] focus-visible:outline-none",
                "focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                track === t.id
                  ? "border-mk-accent bg-mk-accent-50 text-mk-ink"
                  : "border-mk-border text-mk-secondary hover:border-mk-accent-200",
              )}
            >
              <span style={{ color: t.hue }}>{t.glyph}</span>
              {t.label}
            </button>
          ))}

          <div className="ml-auto flex items-center gap-3">
            <span className="eco-mono hidden text-mk-faint sm:block">⌘ + ↵</span>
            <Btn disabled={!text.trim()} iconStart={<ArrowUp size={16} strokeWidth={2} />} onClick={start}>
              开始
            </Btn>
          </div>
        </div>
      </div>

      <p className="mt-3 text-mk-small leading-[1.8] text-mk-muted">
        {track
          ? trackById(track).ends
            ? `做完你会有：${trackById(track).ends}`
            : ""
          : "先说事，类型可以不选——印记会问清楚再排计划。"}
      </p>

      {/* the other door */}
      <ProblemDoor onStart={(p) => go({ name: "project", id: startProblemProject(p) })} />

      <Panel className="mt-8 p-6">
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
    </>
  );
}

/* ── 我的项目 ──────────────────────────────────────────────────────────── */

function Shelf() {
  const { state, resetPrototype } = useEco();
  const running = state.projects.filter((p) => p.phase !== "published");
  const published = state.projects.filter((p) => p.phase === "published");

  return (
    <>
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

      {published.length > 0 ? (
        <section className="mb-9">
          <Sys className="mb-2.5 block">已发布</Sys>
          <ul className="grid gap-3 md:grid-cols-2">
            {published.map((p) => (
              <li key={p.id}>
                <ProjectCard project={p} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <button
        type="button"
        onClick={resetPrototype}
        className="flex items-center gap-1 text-mk-small text-mk-faint underline decoration-dotted
                   underline-offset-4 transition-colors hover:text-mk-muted focus-visible:outline-none"
        title="清空这个原型的全部数据，恢复初始状态"
      >
        <RotateCcw size={12} strokeWidth={1.9} />
        原型：清空数据，恢复初始状态
      </button>
    </>
  );
}

/** Progress reads off the PLAN she approved, not off the track's card list. */
function ProjectCard({ project }: { project: Project }) {
  const t = trackById(project.track);
  const steps = activeSteps(project.plan);
  const done = Math.max(0, Math.min(project.at, steps.length));
  const motive = motiveOf(project);
  const now = steps[Math.min(project.at, steps.length - 1)];

  const stage =
    project.phase === "published"
      ? "已发布"
      : project.phase === "frame"
        ? "在把问题问准"
        : project.phase === "choose"
          ? "在选走哪条路"
          : project.phase === "plan"
            ? "计划等你确认"
            : (now?.title ?? "在收尾");

  return (
    <button
      type="button"
      onClick={() => go({ name: "project", id: project.id })}
      className="flex h-full w-full gap-4 rounded-mk-lg border border-mk-border bg-mk-surface p-4 text-left
                 transition-colors duration-[140ms] ease-mk hover:border-mk-accent-200
                 hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2
                 focus-visible:ring-mk-accent-200"
    >
      <CoverArt cover={project.cover} />
      <span className="min-w-0 flex-1">
        <span className="flex items-center justify-between gap-3">
          <Sys>{t.label}</Sys>
          {project.phase === "run" ? (
            <span className="font-mono text-mk-small tabular-nums text-mk-muted">
              {done}/{steps.length} 步
            </span>
          ) : project.phase === "published" ? (
            <Globe size={13} strokeWidth={1.9} color="var(--mk-success)" />
          ) : null}
        </span>
        <span className="mt-1 block truncate text-mk-h3 text-mk-ink">{project.title}</span>
        {motive ? (
          <span className="mt-1 block truncate text-mk-small text-mk-muted">为{motive.who}</span>
        ) : null}
        {steps.length > 0 && project.phase === "run" ? (
          <span
            className="mt-2.5 block h-1.5 overflow-hidden rounded-mk-full"
            style={{ background: "var(--mk-border)" }}
          >
            <span
              className="block h-full rounded-mk-full"
              style={{ width: `${(done / steps.length) * 100}%`, background: t.hue }}
            />
          </span>
        ) : null}
        <span className="mt-2 flex items-center gap-1.5 text-mk-small text-mk-accent-700">
          <ArrowRight size={13} strokeWidth={2} />
          {stage}
        </span>
      </span>
    </button>
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
            <ProblemBox value={problem} onChange={setProblem} />
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

/** The 我发现了一个真问题 door, collapsed until she asks for it. */
function ProblemDoor({ onStart }: { onStart: (problem: string) => void }) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState(EXAMPLE_PROBLEM);

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="group mt-6 flex items-start gap-3 text-left focus-visible:outline-none"
      >
        <Lightbulb
          size={16}
          strokeWidth={1.9}
          className="mt-0.5 shrink-0 text-mk-muted group-hover:text-mk-accent-700"
        />
        <span>
          <span className="block text-mk-body font-semibold text-mk-ink group-hover:text-mk-accent-700">
            我发现了一个真问题，不知道该怎么解
          </span>
          <span className="mt-0.5 block text-mk-small leading-[1.8] text-mk-muted">
            那走另一条路：先把问题问准，我给你几条不同的办法，选哪条由你定。
          </span>
        </span>
      </button>
    );
  }
  return (
    <div className="mt-6">
      <ProblemBox value={text} onChange={setText} onStart={onStart} />
    </div>
  );
}

/**
 * 🚨 Prefilled with the worked example, and it says so. In the real product
 * 印记 reads whatever she typed and proposes roads for it; in this prototype
 * the roads are hand-written for one problem. A free box that silently returns
 * the garden's two roads whatever she typed would be the prototype lying about
 * what it can do.
 */
function ProblemBox({
  value,
  onChange,
  onStart,
}: {
  value: string;
  onChange: (v: string) => void;
  onStart?: (problem: string) => void;
}) {
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
        onClick={() =>
          onStart
            ? onStart(value.trim())
            : go({ name: "project", id: startProblemProject(value.trim()) })
        }
      >
        跟印记说这个问题
      </Btn>
    </div>
  );
}
