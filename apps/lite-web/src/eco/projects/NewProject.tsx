import { useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  ChevronDown,
  ChevronUp,
  MessageCircle,
  Plus,
  Sparkles,
  Trash2,
} from "lucide-react";
import { useEco } from "../store";
import { STEP_META, TRACKS, TRACK_PROPOSALS, WHY_LADDER, planHours, trackById } from "../data/projects";
import { KEYWORDS } from "../data/tree";
import { newsById } from "../data/news";
import { go } from "../route";
import type { StepKind, TrackId } from "../data/types";
import { Btn, Field, Panel, SectionHead, Sys, cx } from "../ui";

/**
 * 开一个新项目 — track → why → plan.
 *
 * ## Two doors, always
 * A student who already knows what she wants shouldn't sit through a
 * conversation; a student who doesn't shouldn't have to guess from five cards.
 * So: pick a track, OR let 印记 propose from her tree (`TRACK_PROPOSALS` reads
 * real keywords, which is the payoff for having a keyword model at all).
 *
 * ## 为什么 gets the most screen time on purpose
 * Three rungs, one at a time, each with a real hint, and 印记 pushes back ONCE
 * on a thin answer (`probe`). Projects die around step three, and the only
 * thing that carries a student past that is being able to say why she started.
 * The answers become a 动机卡 pinned to the project header.
 *
 * ## The plan is generated, not negotiated
 * Per AGENTS.md, laying out steps from a track is a deterministic system
 * action — 铁律①/② govern her WRITING, not orchestration — so there is no
 * "may I plan for you?" gate. She can reorder, delete and add afterwards
 * because that is good usability, not because consent was required.
 */

type Stage = "pick" | "name" | "why" | "plan";

export function NewProject() {
  const { state, draftTrack, draftPlan, draftMoveStep, draftRemoveStep, draftAddStep, createProject, openCoach } =
    useEco();
  const d = state.draft;
  const [stage, setStage] = useState<Stage>(d.track ? "name" : "pick");
  // Which door she came through — the hub's 「先和印记聊聊」 sets this, so that
  // button lands on the chat entrance rather than on the track grid.
  const [talking, setTalking] = useState(state.newProjectMode === "talk");

  function choose(track: TrackId, title = "") {
    draftTrack(track, title);
    setStage("name");
  }

  return (
    <div className="mx-auto max-w-[900px] px-8 py-8">
      <Btn variant="quiet" size="sm" iconStart={<ArrowLeft size={15} />} onClick={() => go({ name: "projects" })}>
        项目
      </Btn>

      {/* ── stage: pick ────────────────────────────────────────────────── */}
      {stage === "pick" ? (
        <div className="mt-4">
          <SectionHead index="新项目 · STEP 1 / 3" title="你想做哪一种？" sub="两个入口，随便走哪个都行。" />

          <div className="mb-6 flex flex-wrap gap-2">
            <Btn variant={talking ? "quiet" : "primary"} onClick={() => setTalking(false)}>
              我知道我要做什么
            </Btn>
            <Btn
              variant={talking ? "primary" : "outline"}
              iconStart={<MessageCircle size={16} strokeWidth={1.8} />}
              onClick={() => setTalking(true)}
            >
              先和印记聊聊
            </Btn>
          </div>

          {talking ? (
            <div className="space-y-3">
              <Panel className="p-5">
                <div className="flex items-center gap-2">
                  <span
                    className="flex h-6 w-6 items-center justify-center rounded-mk-full"
                    style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
                  >
                    <span className="text-[11px] font-bold text-white">印</span>
                  </span>
                  <Sys>印记 · 读了你的树</Sys>
                </div>
                <p className="mt-2.5 text-mk-body-lg leading-[1.9] text-mk-ink">
                  我不猜你想做什么，我看你已经做过什么。你树上的词里，有三个已经走完了
                  「读到 → 写下来」这一圈。下面这三个提议就是从那三个词来的——
                  <strong className="font-semibold">选一个，或者告诉我都不对。</strong>
                </p>
                {state.kept.length > 0 ? (
                  <p className="mt-2.5 text-mk-body leading-[1.85] text-mk-secondary">
                    你还从世界收了{" "}
                    {state.kept.map((k) => `「${newsById(k)?.keywords[0] ?? ""}」`).join("、")}
                    ，也可以从那里开一个。
                  </p>
                ) : null}
              </Panel>

              <ul className="space-y-3">
                {TRACK_PROPOSALS.map((p, i) => {
                  const t = trackById(p.track);
                  const kw = KEYWORDS.find((k) => k.text === p.fromKeyword);
                  return (
                    <li key={p.title} className="eco-in" style={{ ["--i" as string]: i }}>
                      <button
                        type="button"
                        onClick={() => choose(p.track, p.title)}
                        className="w-full rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left
                                   transition-all duration-[160ms] ease-mk hover:-translate-y-0.5
                                   hover:border-mk-accent-200 hover:shadow-mk-md focus-visible:outline-none
                                   focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      >
                        <span className="flex flex-wrap items-center gap-2">
                          <span
                            className="rounded-mk-full px-2.5 py-1 text-mk-small font-medium"
                            style={{ background: `color-mix(in srgb, ${t.hue} 26%, var(--mk-surface))`, color: "var(--mk-ink)" }}
                          >
                            {t.label}
                          </span>
                          <span className="inline-flex items-center gap-1.5 text-mk-small text-mk-muted">
                            <Sparkles size={13} strokeWidth={2} color="#C9962B" />
                            从你的「{p.fromKeyword}」来的
                            {kw ? <span className="font-mono">· {kw.sources.length} 个来源</span> : null}
                          </span>
                        </span>
                        <span className="mt-2.5 block text-mk-h2 text-mk-ink">{p.title}</span>
                        <span className="mt-2 block text-mk-body leading-[1.9] text-mk-secondary">{p.pitch}</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
              <Btn variant="quiet" className="mt-2" onClick={() => openCoach("projects")}>
                都不对，我想直接跟印记说
              </Btn>
            </div>
          ) : (
            <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {TRACKS.map((t, i) => (
                <li key={t.id} className="eco-in" style={{ ["--i" as string]: i }}>
                  <button
                    type="button"
                    onClick={() => choose(t.id)}
                    className="flex h-full w-full flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5
                               text-left transition-all duration-[160ms] ease-mk hover:-translate-y-0.5
                               hover:border-mk-accent-200 hover:shadow-mk-md focus-visible:outline-none
                               focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  >
                    <span
                      className="flex h-10 w-10 items-center justify-center rounded-mk-md font-mono text-[18px]"
                      style={{ background: `color-mix(in srgb, ${t.hue} 26%, var(--mk-surface))` }}
                    >
                      {t.glyph}
                    </span>
                    <span className="mt-3 block text-mk-h2 text-mk-ink">{t.label}</span>
                    <span className="mt-1.5 block text-mk-body leading-[1.8] text-mk-secondary">{t.blurb}</span>
                    <span className="mt-3 block text-mk-small text-mk-muted">做完你会有：{t.ends}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {/* ── stage: name ────────────────────────────────────────────────── */}
      {stage === "name" && d.track ? (
        <div className="mt-4 max-w-[640px]">
          <SectionHead
            index={`新项目 · ${trackById(d.track).label}`}
            title="先给它一个名字"
            sub="不用想太久，之后可以改。写具体一点：「我们年级扔了多少还能修的东西」好过「环保调查」。"
          />
          <Field
            label="项目名"
            hint="一句话，说清楚你要做出什么。"
            value={d.title}
            rows={2}
            placeholder="例如：我们年级到底扔掉了多少还能修的东西"
            onChange={(v) => draftTrack(d.track!, v)}
          />
          <div className="mt-6 flex items-center gap-3">
            <Btn variant="quiet" onClick={() => setStage("pick")}>
              换一条赛道
            </Btn>
            <Btn disabled={d.title.trim().length < 2} onClick={() => setStage("why")}>
              下一步：为什么做它
              <ArrowRight size={16} />
            </Btn>
          </div>
        </div>
      ) : null}

      {/* ── stage: why ─────────────────────────────────────────────────── */}
      {stage === "why" && d.track ? (
        <WhyLadder
          onBack={() => setStage("name")}
          onDone={() => {
            draftPlan();
            setStage("plan");
          }}
        />
      ) : null}

      {/* ── stage: plan ────────────────────────────────────────────────── */}
      {stage === "plan" && d.track ? (
        <div className="mt-4">
          <SectionHead
            index="新项目 · STEP 3 / 3"
            title="印记给你排了一条路线"
            sub="每一步下面写着「为什么有这一步」。不同意的可以删、可以换顺序、可以加。"
            right={
              <div className="text-right">
                <Sys>预计总时长</Sys>
                <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
                  {planHours(d.steps)}
                </span>
              </div>
            }
          />

          <MotivationCard />

          <ol className="mt-5 space-y-2">
            {d.steps.map((s, i) => {
              const m = STEP_META[s.kind];
              return (
                <li
                  key={s.id}
                  className="flex items-start gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-4"
                >
                  <span className="eco-mono w-6 shrink-0 pt-1 text-mk-faint">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <span
                    className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-md font-mono text-[15px]"
                    style={{ background: `color-mix(in srgb, ${m.hue} 28%, var(--mk-surface))` }}
                  >
                    {m.glyph}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-baseline gap-2">
                      <span className="text-mk-h3 text-mk-ink">{s.title}</span>
                      <span
                        className="rounded-mk-full px-2 py-0.5 text-[11px] font-medium"
                        style={{ background: `color-mix(in srgb, ${m.hue} 24%, transparent)`, color: "var(--mk-secondary)" }}
                      >
                        {m.label}
                      </span>
                      <span className="font-mono text-[11px] text-mk-faint">{s.minutes} 分钟</span>
                    </span>
                    <span className="mt-1 block text-mk-body leading-[1.8] text-mk-secondary">
                      <span className="text-mk-muted">为什么有这一步：</span>
                      {s.why}
                    </span>
                  </span>
                  <span className="flex shrink-0 items-center gap-0.5">
                    <button
                      type="button"
                      onClick={() => draftMoveStep(s.id, -1)}
                      disabled={i === 0}
                      className="rounded p-1 text-mk-muted hover:text-mk-ink disabled:opacity-30 focus-visible:outline-none"
                      aria-label="上移"
                    >
                      <ChevronUp size={15} strokeWidth={2.2} />
                    </button>
                    <button
                      type="button"
                      onClick={() => draftMoveStep(s.id, 1)}
                      disabled={i === d.steps.length - 1}
                      className="rounded p-1 text-mk-muted hover:text-mk-ink disabled:opacity-30 focus-visible:outline-none"
                      aria-label="下移"
                    >
                      <ChevronDown size={15} strokeWidth={2.2} />
                    </button>
                    <button
                      type="button"
                      onClick={() => draftRemoveStep(s.id)}
                      className="rounded p-1 text-mk-muted hover:text-mk-danger focus-visible:outline-none"
                      aria-label="删除这一步"
                    >
                      <Trash2 size={15} strokeWidth={1.9} />
                    </button>
                  </span>
                </li>
              );
            })}
          </ol>

          <AddStep onAdd={(kind, title) => draftAddStep({ id: `x-${Date.now()}`, kind, title, why: "你自己加的一步。", minutes: 30, done: false })} />

          <div className="mt-7 flex flex-wrap items-center gap-3">
            <Btn variant="quiet" onClick={() => setStage("why")}>
              回去改动机
            </Btn>
            <Btn
              disabled={d.steps.length === 0}
              onClick={() => {
                const id = createProject();
                go({ name: "project", id });
              }}
            >
              就这么开始
              <ArrowRight size={16} />
            </Btn>
          </div>
        </div>
      ) : null}
    </div>
  );
}

/** The three-rung why ladder, one question on screen at a time. */
function WhyLadder({ onBack, onDone }: { onBack: () => void; onDone: () => void }) {
  const { state, draftWhy, draftWhyNext, draftWhyBack, draftProbe } = useEco();
  const d = state.draft;
  const rung = Math.min(d.whyStep, 2);
  // The ladder has exactly three rungs and `rung` is clamped to 0..2 above.
  const q = WHY_LADDER[rung]!;
  const value = d.why[q.id];
  const probed = d.probed.includes(q.id);
  const thin = value.trim().length > 0 && value.trim().length < 12;
  const showProbe = probed || thin;

  return (
    <div className="mt-4 max-w-[680px]">
      <SectionHead
        index="新项目 · STEP 2 / 3"
        title="为什么要做它"
        sub="这一步会花掉比你预期更多的时间。这是故意的——项目通常死在第三步，那时候唯一能撑住你的就是这三个答案。"
      />

      <div className="mb-5 flex items-center gap-2">
        {WHY_LADDER.map((r, i) => (
          <span key={r.id} className="flex items-center gap-2">
            <span
              className="h-2 w-2 rounded-mk-full"
              style={{
                background:
                  i < rung ? "var(--mk-success)" : i === rung ? "var(--mk-accent)" : "var(--mk-border)",
              }}
            />
            <span className={cx("text-mk-small", i === rung ? "font-semibold text-mk-ink" : "text-mk-muted")}>
              {["谁会用", "不做的代价", "为什么是你"][i]}
            </span>
            {i < 2 ? <span className="h-px w-6" style={{ background: "var(--mk-border)" }} /> : null}
          </span>
        ))}
      </div>

      <Panel className="p-6">
        <Sys>问题 {rung + 1} / 3</Sys>
        <h3 className="mt-2 text-mk-report-quote leading-[1.6] text-mk-ink">{q.q}</h3>
        <p className="mt-2 text-mk-body text-mk-muted">{q.hint}</p>
        <textarea
          rows={4}
          value={value}
          onChange={(e) => draftWhy(q.id, e.target.value)}
          placeholder="写下来。写得具体，之后你会感谢自己。"
          className="mt-4 w-full resize-y rounded-mk-md border border-mk-input-border bg-mk-surface p-3
                     text-mk-prose text-mk-ink outline-none transition-colors duration-[120ms] ease-mk
                     placeholder:text-mk-faint focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
        />

        {showProbe ? (
          <div
            className="mt-4 rounded-mk-md p-4"
            style={{ background: "var(--mk-accent-50)", border: "1px solid var(--mk-accent-100)" }}
          >
            <div className="flex items-center gap-2">
              <span
                className="flex h-5 w-5 items-center justify-center rounded-mk-full"
                style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
              >
                <span className="text-[10px] font-bold text-white">印</span>
              </span>
              <Sys className="!text-mk-accent-700">再追一句</Sys>
            </div>
            <p className="mt-2 text-mk-body leading-[1.9] text-mk-accent-800">{q.probe}</p>
          </div>
        ) : null}
      </Panel>

      <div className="mt-6 flex flex-wrap items-center gap-3">
        <Btn variant="quiet" onClick={() => (rung === 0 ? onBack() : draftWhyBack())}>
          上一步
        </Btn>
        <Btn
          disabled={value.trim().length === 0}
          onClick={() => {
            if (thin && !probed) {
              draftProbe(q.id);
              return;
            }
            if (rung === 2) {
              draftWhyNext();
              onDone();
            } else {
              draftWhyNext();
            }
          }}
        >
          {thin && !probed ? "让印记看看" : rung === 2 ? "生成路线" : "下一个问题"}
          <ArrowRight size={16} />
        </Btn>
        {thin && !probed ? (
          <span className="text-mk-small text-mk-muted">这个答案有点短，印记想追一句。</span>
        ) : null}
      </div>
    </div>
  );
}

function MotivationCard() {
  const { state } = useEco();
  const w = state.draft.why;
  if (!w.who && !w.cost && !w.mine) return null;
  return (
    <div
      className="rounded-mk-lg border p-5"
      style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
    >
      <div className="flex items-center gap-2">
        <Sparkles size={15} strokeWidth={2} color="#C9962B" />
        <Sys className="!text-[#8A6320]">动机卡 · 它会钉在这个项目上</Sys>
      </div>
      <dl className="mt-3 space-y-2.5">
        {[
          ["为谁做", w.who],
          ["不做的代价", w.cost],
          ["为什么是我", w.mine],
        ].map(([k, v]) =>
          v ? (
            <div key={k}>
              <dt className="text-mk-small font-semibold text-[#8A6320]">{k}</dt>
              <dd className="mt-0.5 text-mk-body leading-[1.85] text-[#6B4D14]">{v}</dd>
            </div>
          ) : null,
        )}
      </dl>
    </div>
  );
}

function AddStep({ onAdd }: { onAdd: (kind: StepKind, title: string) => void }) {
  const [open, setOpen] = useState(false);
  const [kind, setKind] = useState<StepKind>("research");
  const [title, setTitle] = useState("");

  if (!open) {
    return (
      <Btn variant="quiet" className="mt-3" iconStart={<Plus size={16} strokeWidth={2} />} onClick={() => setOpen(true)}>
        加一步
      </Btn>
    );
  }
  return (
    <Panel className="mt-3 p-4">
      <div className="flex flex-wrap gap-1.5">
        {(Object.keys(STEP_META) as StepKind[]).map((k) => (
          <button
            key={k}
            type="button"
            onClick={() => setKind(k)}
            className={cx(
              "rounded-mk-full px-3 py-1.5 text-mk-small transition-colors duration-[120ms]",
              kind === k ? "text-white" : "text-mk-secondary",
            )}
            style={
              kind === k
                ? { background: "var(--mk-ink)" }
                : { background: "var(--mk-paper)", border: "1px solid var(--mk-border)" }
            }
          >
            {STEP_META[k].glyph} {STEP_META[k].label}
          </button>
        ))}
      </div>
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="这一步要做什么？"
        className="mt-3 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2.5
                   text-mk-body text-mk-ink outline-none placeholder:text-mk-faint
                   focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
      />
      <div className="mt-3 flex gap-2">
        <Btn
          size="sm"
          disabled={title.trim().length === 0}
          onClick={() => {
            onAdd(kind, title.trim());
            setTitle("");
            setOpen(false);
          }}
        >
          加进去
        </Btn>
        <Btn size="sm" variant="quiet" onClick={() => setOpen(false)}>
          算了
        </Btn>
      </div>
    </Panel>
  );
}
