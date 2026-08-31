import { useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowUp, Check, Globe, PanelRightClose, X } from "lucide-react";
import { useEco } from "../store";
import { asText, cardById, motiveOf } from "../data/cards";
import { artifactById } from "../data/artifacts";
import { TOOL_KINDS, activeSteps } from "../data/plan";
import { trackById } from "../data/projects";
import { go } from "../route";
import type { Project, StageRef, ThreadItem } from "../data/types";
import { Bold, Btn, Empty, Field, Panel, Sys, cx } from "../ui";
import { CardSurface } from "./CardSurface";
import { CoverArt, CoverPicker } from "./CoverPicker";
import { Planner } from "./panels/Planner";
import { Roads } from "./panels/Roads";
import { Overview } from "./panels/Overview";
import { Make } from "./panels/Make";
import { BranchTalk } from "./panels/BranchTalk";
import { StepIntro } from "./panels/StepIntro";

/**
 * 项目工作台 — the agentic room.
 *
 * ## The shape (2026-08-31, second pass)
 * Two columns, and which one is wide is the whole interaction model — the
 * Cowork/Codex shape:
 *
 *   - **Talking** → the conversation is the screen and the right panel is a
 *     narrow 全景: where in the plan we are, what has been made, who does what.
 *   - **Doing or looking** → whatever needs doing takes the panel over and it
 *     grows; the conversation shrinks to a column beside it and keeps running.
 *
 * The stage holds four things: a 工具卡 she fills, something 印记 built and
 * hands over, a branch conversation about one road, and — before any of that
 * — the planner.
 *
 * ## Nothing runs before the plan
 * A project opens in `plan` (or in `frame` → `choose` → `plan`, when she
 * arrived with a real problem instead of a known road). 印记 proposes; she
 * edits, times, approves. The first version of this workbench went straight
 * from hello to the first card, which is not how an agentic tool behaves and
 * not how a person with judgement works either.
 *
 * ## Three rules the layout enforces
 * 1. **A card never opens itself.** 印记 offers; she presses 打开 (铁律②).
 * 2. **现在不做 is a real answer.** Declining ends the subject; 印记 does not
 *    ask again.
 * 3. **The covenant is visible, not implied.** 「印记可以动手，但不能替你判断」
 *    sits in the panel for the whole project, because in a PROJECT (unlike in
 *    her writing) the AI genuinely may build things, and the line between
 *    building and deciding is the only thing holding.
 */
export function Workbench({ id, cardId }: { id: string; cardId?: string }) {
  const { state } = useEco();
  const project = state.projects.find((p) => p.id === id);

  if (!project) {
    return (
      <div className="mx-auto max-w-[720px] px-8 py-16">
        <Empty
          title="找不到这个项目"
          body="它可能是在另一个标签页里建的——这个原型的数据只活在当前标签页。"
          action={<Btn onClick={() => go({ name: "projects" })}>回项目页</Btn>}
        />
      </div>
    );
  }

  return <Room project={project} cardId={cardId} />;
}

/** What is on the stage. `null` = nothing, and the panel falls back to 全景 or
 *  to whichever pre-flight surface the phase calls for. */
type Stage = StageRef | { kind: "branch"; approachId: string } | null;

function Room({ project, cardId }: { project: Project; cardId?: string }) {
  const { sayInProject, publishProject } = useEco();
  const [stage, setStage] = useState<Stage>(cardId ? { kind: "card", cardId } : null);
  const [draft, setDraft] = useState("");
  const [summary, setSummary] = useState("");
  const endRef = useRef<HTMLDivElement>(null);

  // The card route is the one stage state that lives in the URL, because it is
  // the one a student is likely to reload or share. Everything else is local.
  useEffect(() => {
    if (cardId) setStage({ kind: "card", cardId });
  }, [cardId]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [project.thread.length]);

  const t = trackById(project.track);
  const steps = activeSteps(project.plan);
  const motive = motiveOf(project);
  const shipped = project.cards.find((c) => c.cardId === "ship" && c.status === "done");
  const preflight = project.phase === "plan" || project.phase === "choose";
  // Wide = there is something to operate on or look at. This is the single
  // switch the whole Cowork shape hangs on.
  const wide = stage !== null || preflight;

  function open(ref: StageRef) {
    if (ref.kind === "card") {
      setStage(ref);
      go({ name: "project", id: project.id, cardId: ref.cardId });
      return;
    }
    setStage(ref);
  }

  function close() {
    setStage(null);
    if (cardId) go({ name: "project", id: project.id });
  }

  return (
    <div className="flex h-full min-h-0">
      {/* ── the conversation ────────────────────────────────────────── */}
      <section
        className={cx(
          "flex min-w-0 flex-col border-r border-mk-border transition-[flex-basis] duration-[220ms] ease-mk",
          wide ? "w-[400px] shrink-0" : "flex-1",
        )}
      >
        <header className="border-b border-mk-border px-6 pb-3.5 pt-5">
          <Btn
            variant="quiet"
            size="sm"
            iconStart={<ArrowLeft size={15} />}
            onClick={() => go({ name: "projects" })}
          >
            项目
          </Btn>
          <div className="mt-2 flex items-start gap-3">
            <CoverArt cover={project.cover} size="sm" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-2">
                <Sys>
                  {t.label} · {project.startedAt}
                </Sys>
                <CoverPicker project={project} />
              </div>
              <h1 className="mt-0.5 text-mk-h1 text-mk-ink">{project.title}</h1>
            </div>
          </div>
          {project.phase === "run" && steps.length > 0 ? (
            <p className="mt-1.5 text-mk-small text-mk-muted">
              第 <span className="font-mono tabular-nums">{Math.min(project.at + 1, steps.length)}</span>{" "}
              / {steps.length} 步 · {steps[Math.min(project.at, steps.length - 1)]?.title}
            </p>
          ) : null}
          {project.intent ? (
            <p className="mt-1.5 text-mk-small leading-[1.75] text-mk-secondary">
              <span className="text-mk-muted">你一开始说：</span>
              {project.intent}
            </p>
          ) : null}
        </header>

        {motive ? (
          <div
            className="border-b px-6 py-3"
            style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
          >
            <Sys className="!text-[#8A6320]">你当初说 · 为谁做</Sys>
            <p className="mt-1 text-mk-small leading-[1.75] text-[#6B4D14]">{motive.who}</p>
          </div>
        ) : null}

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
          <ol className={cx("space-y-4", wide ? "" : "mx-auto max-w-[720px]")}>
            {project.thread.map((item) => (
              <li key={item.id}>
                <Turn item={item} project={project} onOpen={open} />
              </li>
            ))}
          </ol>

          {/* publish, once she has filed 发布前检查 */}
          {shipped && project.phase !== "published" ? (
            <Panel className={cx("mt-6 p-5", wide ? "" : "mx-auto max-w-[720px]")}>
              <Sys>最后一步</Sys>
              <h3 className="mt-1 text-mk-h2 text-mk-ink">把它放到我的主页上</h3>
              <div className="mt-3">
                <Field
                  label="做出了什么"
                  hint="两三句。写你真的做出了什么、遇到了什么、改了什么。"
                  value={summary || asText(shipped.values.made)}
                  onChange={setSummary}
                  rows={4}
                />
              </div>
              <Btn
                className="mt-3"
                iconStart={<Globe size={16} strokeWidth={1.9} />}
                disabled={(summary || asText(shipped.values.made)).trim().length < 4}
                onClick={() =>
                  publishProject(project.id, (summary || asText(shipped.values.made)).trim())
                }
              >
                发布这个项目
              </Btn>
            </Panel>
          ) : null}

          {project.phase === "published" ? (
            <Panel className={cx("mt-6 p-5", wide ? "" : "mx-auto max-w-[720px]")}>
              <div className="flex items-center gap-2">
                <Globe size={16} strokeWidth={1.9} color="var(--mk-success)" />
                <Sys className="!text-mk-success">已发布</Sys>
              </div>
              <p className="mt-2 text-mk-body-lg leading-[1.9] text-mk-ink">{project.summary}</p>
              <Btn
                className="mt-3"
                variant="outline"
                onClick={() => go({ name: "page", handle: "zhiyao" })}
              >
                去我的主页看看
              </Btn>
            </Panel>
          ) : null}

          <div ref={endRef} />
        </div>

        <div className="border-t border-mk-border bg-mk-surface px-6 py-3.5">
          <form
            className={cx("flex items-end gap-2", wide ? "" : "mx-auto max-w-[720px]")}
            onSubmit={(e) => {
              e.preventDefault();
              const text = draft.trim();
              if (!text) return;
              sayInProject(project.id, text);
              setDraft("");
            }}
          >
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  const text = draft.trim();
                  if (!text) return;
                  sayInProject(project.id, text);
                  setDraft("");
                }
              }}
              rows={1}
              placeholder="卡住了、想反驳印记——直接说"
              className="max-h-[140px] min-h-[44px] flex-1 resize-none rounded-mk-lg border border-mk-input-border
                         bg-mk-paper px-3.5 py-3 text-mk-body text-mk-ink placeholder:text-mk-faint
                         focus:border-mk-accent focus:outline-none focus:ring-2 focus:ring-mk-accent-200"
            />
            <Btn type="submit" disabled={!draft.trim()} iconStart={<ArrowUp size={16} />}>
              说
            </Btn>
          </form>
        </div>
      </section>

      {/* ── the panel ───────────────────────────────────────────────── */}
      <aside
        className={cx(
          "flex min-w-0 flex-col bg-mk-surface transition-[flex-basis] duration-[220ms] ease-mk",
          wide ? "flex-1" : "w-[352px] shrink-0",
        )}
      >
        {stage ? (
          <>
            <div className="flex items-center justify-between gap-3 border-b border-mk-border px-5 py-2.5">
              <Sys>工作台</Sys>
              <button
                type="button"
                onClick={close}
                className="flex items-center gap-1.5 rounded-mk-sm px-2 py-1 text-mk-small text-mk-muted
                           transition-colors duration-[120ms] hover:bg-mk-paper hover:text-mk-ink
                           focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                <PanelRightClose size={14} strokeWidth={1.9} />
                收起
              </button>
            </div>
            <div className="min-h-0 flex-1">
              {stage.kind === "card" ? (
                <CardSurface project={project} cardId={stage.cardId} onDone={close} />
              ) : stage.kind === "make" ? (
                <Make project={project} artifactId={stage.artifactId} />
              ) : (
                <BranchTalk
                  project={project}
                  approachId={stage.approachId}
                  onBack={() => setStage(null)}
                />
              )}
            </div>
          </>
        ) : project.phase === "choose" ? (
          <Roads
            project={project}
            onOpenBranch={(approachId) => setStage({ kind: "branch", approachId })}
          />
        ) : project.phase === "plan" ? (
          <Planner project={project} />
        ) : (
          <Overview project={project} onOpen={open} />
        )}
      </aside>
    </div>
  );
}

/** One turn in the thread: a line of speech, a plan-step divider, a card on
 *  the table, or something 印记 built. */
function Turn({
  item,
  project,
  onOpen,
}: {
  item: ThreadItem;
  project: Project;
  onOpen: (ref: StageRef) => void;
}) {
  const { openCardEntry, skipCard } = useEco();

  // A step opening is 印记 briefing her: the goal, who does what, and the
  // judgement that stays hers. It used to be a hairline divider with a label,
  // which told her where she was and nothing about what to do.
  if (item.kind === "step") {
    return <StepIntro project={project} stepId={item.stepId} />;
  }

  if (item.kind === "say") {
    const coach = item.role === "coach";
    return (
      <div className={cx("flex gap-3", coach ? "" : "flex-row-reverse")}>
        <span
          className={cx(
            "mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold",
            coach ? "text-white" : "text-mk-secondary",
          )}
          style={{
            background: coach
              ? "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))"
              : "var(--mk-border)",
          }}
        >
          {coach ? "印" : "我"}
        </span>
        <div
          className="max-w-[88%] whitespace-pre-wrap rounded-mk-lg px-4 py-3 text-mk-body leading-[1.9] text-mk-ink"
          style={{
            background: coach ? "var(--mk-surface)" : "var(--mk-accent-50)",
            border: `1px solid ${coach ? "var(--mk-border)" : "var(--mk-accent-200)"}`,
          }}
        >
          {/* 印记's scripted copy carries `**bold**` to name the method in a
              sentence. Printing the asterisks is the failure mode this shared
              component exists to prevent. */}
          <Bold text={item.text} />
        </div>
      </div>
    );
  }

  if (item.kind === "make") {
    const spec = artifactById(item.artifactId);
    const st = project.artifacts[item.artifactId];
    if (!spec) return null;
    return (
      <Offer
        glyph="◗"
        hue="#4E7EA6"
        kicker="印记的产出"
        title={spec.title}
        body={spec.ask}
        done={st?.status === "settled"}
        doneLabel="已确认"
        primary={st?.status === "settled" ? "回去看看" : "打开查看"}
        onPrimary={() => onOpen({ kind: "make", artifactId: item.artifactId })}
      />
    );
  }

  const spec = cardById(item.cardId);
  const entry = project.cards.find((c) => c.cardId === item.cardId);
  if (!spec) return null;
  const done = entry?.status === "done";

  return (
    <Offer
      glyph={spec.glyph}
      hue={spec.hue}
      kicker={`${TOOL_KINDS[spec.kind].label} · 约 ${spec.minutes} 分钟`}
      title={spec.title}
      body={spec.reason}
      done={done}
      doneLabel="已提交"
      primary={done ? "回去看看我写的" : "打开这张卡"}
      onPrimary={() => {
        openCardEntry(project.id, item.cardId);
        onOpen({ kind: "card", cardId: item.cardId });
      }}
      secondary={done ? undefined : "现在不做"}
      onSecondary={() => skipCard(project.id, item.cardId)}
    />
  );
}

/**
 * 印记 putting something on the table.
 *
 * One component for both cards and artifacts, because from the student's side
 * they are the same move: *here is a thing, here is why, it is your call
 * whether to open it*. The reason is never optional — a card that appears
 * without one is an ambush.
 */
function Offer({
  glyph,
  hue,
  kicker,
  title,
  body,
  done,
  doneLabel,
  primary,
  onPrimary,
  secondary,
  onSecondary,
}: {
  glyph: string;
  hue: string;
  kicker: string;
  title: string;
  body: string;
  done: boolean;
  doneLabel: string;
  primary: string;
  onPrimary: () => void;
  secondary?: string;
  onSecondary?: () => void;
}) {
  return (
    <div className="flex gap-3">
      <span
        className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold text-white"
        style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
      >
        印
      </span>
      <div
        className="eco-in min-w-0 flex-1 rounded-mk-lg border p-4"
        style={{
          borderColor: `color-mix(in srgb, ${hue} 48%, transparent)`,
          background: `color-mix(in srgb, ${hue} 9%, var(--mk-surface))`,
        }}
      >
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <span className="text-[15px]">{glyph}</span>
          <Sys>{kicker}</Sys>
          {done ? (
            <span className="inline-flex items-center gap-1 rounded-mk-full bg-mk-success px-2 py-0.5 text-[11px] font-semibold text-white">
              <Check size={10} strokeWidth={3} /> {doneLabel}
            </span>
          ) : null}
        </div>
        <p className="mt-1 text-mk-h3 text-mk-ink">{title}</p>
        <p className="mt-1.5 text-mk-body leading-[1.85] text-mk-ink">{body}</p>
        <div className="mt-3 flex flex-wrap gap-2">
          <Btn size="sm" onClick={onPrimary}>
            {primary}
          </Btn>
          {secondary && onSecondary ? (
            <Btn size="sm" variant="quiet" iconStart={<X size={13} />} onClick={onSecondary}>
              {secondary}
            </Btn>
          ) : null}
        </div>
      </div>
    </div>
  );
}
