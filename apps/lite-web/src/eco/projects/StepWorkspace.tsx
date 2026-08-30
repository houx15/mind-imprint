import { useState } from "react";
import { ArrowLeft, ArrowRight, Check, MessageCircle, Plus, Trash2 } from "lucide-react";
import { useEco } from "../store";
import { STEP_META } from "../data/projects";
import { go } from "../route";
import type { Project, ProjectStep } from "../data/types";
import { Btn, Field, Panel, Sys, cx } from "../ui";

/**
 * 一步的工作台 — the screen where student and AI actually work together.
 *
 * ## Why each step type gets its own instrument
 * A plan whose every step opens the same empty textarea is a to-do list
 * wearing a costume. 学 gives her a small lesson and one question; 查 gives her
 * a questionnaire builder and a (clearly labelled) mock return; 设计 forces
 * three directions instead of one; 试 records what a real person got stuck on.
 * The instrument is what makes the step teach something.
 *
 * ## 印记 sits beside, not above
 * The right column is one prompt at a time, in the step's own terms. It never
 * produces the artifact — it asks the question that makes the artifact better.
 */
export function StepWorkspace({ project, step }: { project: Project; step: ProjectStep }) {
  const { toggleStep, openCoach } = useEco();
  const m = STEP_META[step.kind];
  const idx = project.steps.findIndex((s) => s.id === step.id);
  const next = project.steps[idx + 1];

  return (
    <div className="mx-auto max-w-[1080px] px-8 py-8">
      <Btn
        variant="quiet"
        size="sm"
        iconStart={<ArrowLeft size={15} />}
        onClick={() => go({ name: "project", id: project.id })}
      >
        {project.title}
      </Btn>

      <div className="mt-4 flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="flex h-7 w-7 items-center justify-center rounded-mk-md font-mono text-[15px]"
              style={{ background: `color-mix(in srgb, ${m.hue} 28%, var(--mk-surface))` }}
            >
              {m.glyph}
            </span>
            <Sys>
              第 {idx + 1} 步 / 共 {project.steps.length} · {m.label} · {step.minutes} 分钟
            </Sys>
          </div>
          <h1 className="mt-2 text-mk-h1 text-mk-ink">{step.title}</h1>
          <p className="mt-1.5 max-w-[62ch] text-mk-body leading-[1.85] text-mk-secondary">
            <span className="text-mk-muted">为什么有这一步：</span>
            {step.why}
          </p>
        </div>
        <Btn
          variant={step.done ? "quiet" : "primary"}
          iconStart={step.done ? <Check size={16} strokeWidth={2.4} /> : undefined}
          onClick={() => toggleStep(project.id, step.id)}
        >
          {step.done ? "已完成（点一下取消）" : "标记这一步完成"}
        </Btn>
      </div>

      <hr className="eco-hair my-6" />

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div>
          <Instrument kind={step.tool ?? step.kind} title={step.title} />
        </div>

        <aside className="space-y-4">
          <Panel className="p-5">
            <div className="flex items-center gap-2">
              <span
                className="flex h-6 w-6 items-center justify-center rounded-mk-full"
                style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
              >
                <span className="text-[11px] font-bold text-white">印</span>
              </span>
              <Sys>印记 · 这一步</Sys>
            </div>
            <p className="mt-2.5 text-mk-body-lg leading-[1.9] text-mk-ink">{COACH_LINE[step.kind]}</p>
            <Btn
              variant="outline"
              size="sm"
              className="mt-3 w-full"
              iconStart={<MessageCircle size={15} />}
              onClick={() => openCoach("step")}
            >
              继续问
            </Btn>
          </Panel>

          {next ? (
            <Panel className="p-5">
              <Sys>做完这一步之后</Sys>
              <p className="mt-1.5 text-mk-h3 text-mk-ink">{next.title}</p>
              <p className="mt-1 text-mk-small leading-relaxed text-mk-muted">{next.why}</p>
              <Btn
                variant="quiet"
                size="sm"
                className="mt-2"
                onClick={() => go({ name: "project", id: project.id, stepId: next.id })}
              >
                去下一步
                <ArrowRight size={15} />
              </Btn>
            </Panel>
          ) : null}
        </aside>
      </div>
    </div>
  );
}

const COACH_LINE: Record<ProjectStep["kind"], string> = {
  learn: "先别记笔记。看完之后合上，用自己的话说一遍——说不出来的那部分，才是你要回去看的。",
  research: "问卷最容易犯的错是问「你觉得重要吗」。没有人会说不重要。改成问他上一次真的做了什么。",
  design: "只做一个方案的人会爱上它，然后看不见问题。做三个，你才有得比。",
  make: "做的过程会推翻一部分设计，这很正常。把被推翻的地方记下来——那是你最值钱的部分。",
  document: "写给一个没来过现场的人看。你觉得「这还用说吗」的地方，正是要说的地方。",
  test: "别解释怎么用。他卡住的地方就是你要改的地方，你一开口，这次测试就废了。",
  publish: "写你真的做出了什么，不要写你本来想做什么。半成品也可以发布——说清楚它是半成品就行。",
};

/** One instrument per step. Keyed off `tool` when the step names one, else its
 *  kind — see `ProjectStep.tool`. */
function Instrument({ kind, title }: { kind: NonNullable<ProjectStep["tool"]>; title: string }) {
  switch (kind) {
    case "learn":
      return <LearnCard title={title} />;
    case "survey":
      return <SurveyBuilder />;
    case "design":
      return <ThreeDirections />;
    case "test":
      return <TestLog />;
    case "research":
    case "make":
    case "document":
    case "publish":
    default:
      return <WorkNotes kind={kind} />;
  }
}

/** 学 — a small lesson with one question at the end. Three points, because a
 *  fourth is where a mini-lesson turns into a lecture. */
function LearnCard({ title }: { title: string }) {
  const [answer, setAnswer] = useState("");
  const [read, setRead] = useState<number[]>([]);
  const points = [
    {
      t: "别问「你觉得重要吗」",
      d: "没有人会回答不重要。问他上一次真的做了什么：「上个月你扔掉过还能用的东西吗？是什么？」",
    },
    {
      t: "别让人回忆超过一个月",
      d: "超过一个月的记忆基本是编的。把范围收到「最近两周」，数据会难看，但会是真的。",
    },
    {
      t: "留一个开放题，只留一个",
      d: "开放题最费填写者的力气，但最值钱——你没想到的东西只会出现在那里。放在最后。",
    },
  ];

  return (
    <div>
      <Panel className="p-6">
        <Sys>迷你课 · {title}</Sys>
        <ol className="mt-4 space-y-3">
          {points.map((p, i) => {
            const on = read.includes(i);
            return (
              <li key={p.t}>
                <button
                  type="button"
                  onClick={() => setRead((r) => (r.includes(i) ? r.filter((x) => x !== i) : [...r, i]))}
                  className={cx(
                    "flex w-full items-start gap-3 rounded-mk-md border p-4 text-left transition-colors duration-[140ms]",
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                    on ? "border-mk-success bg-mk-success-bg" : "border-mk-border bg-mk-surface hover:border-mk-accent-200",
                  )}
                >
                  <span className="font-mono text-mk-h2 leading-none text-mk-accent-300">{i + 1}</span>
                  <span className="min-w-0">
                    <span className="block text-mk-h3 text-mk-ink">{p.t}</span>
                    <span className="mt-1 block text-mk-body leading-[1.85] text-mk-secondary">{p.d}</span>
                  </span>
                  {on ? <Check size={16} strokeWidth={2.6} className="ml-auto shrink-0 text-mk-success" /> : null}
                </button>
              </li>
            );
          })}
        </ol>
      </Panel>

      <Panel className="mt-4 p-6">
        <Sys>合上再说一遍</Sys>
        <p className="mt-1.5 text-mk-report-quote leading-[1.6] text-mk-ink">
          不看上面，用你自己的话写一条你会怎么改你的问卷。
        </p>
        <Field label="" hint="" value={answer} onChange={setAnswer} rows={4} placeholder="我会把……改成……" />
        {answer.trim().length > 10 ? (
          <p className="mt-2 text-mk-small text-mk-success">
            写下来了。这一条比上面三条都有用，因为它是你的。
          </p>
        ) : null}
      </Panel>
    </div>
  );
}

/** 查 — a questionnaire builder with a clearly-labelled mock return. */
function SurveyBuilder() {
  const [qs, setQs] = useState<string[]>([
    "最近两周，你扔掉过还能用的东西吗？",
    "如果扔了，它是什么？",
    "为什么没有修它？",
  ]);
  const [draft, setDraft] = useState("");
  const [sent, setSent] = useState(false);

  const mock = [
    { label: "扔过", n: 41 },
    { label: "没扔过", n: 14 },
    { label: "不记得", n: 6 },
  ];
  const total = mock.reduce((s, m) => s + m.n, 0);

  return (
    <div>
      <Panel className="p-6">
        <div className="flex items-baseline justify-between gap-3">
          <Sys>问卷 · 8 题以内</Sys>
          <span className="font-mono text-mk-small text-mk-muted">{qs.length} / 8</span>
        </div>
        <ol className="mt-3 space-y-2">
          {qs.map((q, i) => (
            <li key={q} className="flex items-center gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-3">
              <span className="eco-mono w-5 shrink-0 text-mk-faint">{String(i + 1).padStart(2, "0")}</span>
              <span className="min-w-0 flex-1 text-mk-body text-mk-ink">{q}</span>
              <button
                type="button"
                onClick={() => setQs((x) => x.filter((_, j) => j !== i))}
                className="shrink-0 rounded p-1 text-mk-muted hover:text-mk-danger focus-visible:outline-none"
                aria-label="删除这一题"
              >
                <Trash2 size={15} strokeWidth={1.9} />
              </button>
            </li>
          ))}
        </ol>
        <div className="mt-3 flex gap-2">
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="再加一题…"
            disabled={qs.length >= 8}
            className="min-w-0 flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2.5
                       text-mk-body text-mk-ink outline-none placeholder:text-mk-faint disabled:opacity-50
                       focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
          />
          <Btn
            size="sm"
            iconStart={<Plus size={15} strokeWidth={2.2} />}
            disabled={draft.trim().length === 0 || qs.length >= 8}
            onClick={() => {
              setQs((x) => [...x, draft.trim()]);
              setDraft("");
            }}
          >
            加
          </Btn>
        </div>
        {qs.length >= 8 ? (
          <p className="mt-2 text-mk-small text-mk-muted">八题到顶了。再加下去，认真填完的人会明显变少。</p>
        ) : null}
      </Panel>

      <Panel className="mt-4 p-6">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <Sys>回收</Sys>
          <span
            className="eco-mono rounded-mk-full px-2 py-1"
            style={{ background: "var(--mk-paper)", color: "var(--mk-muted)", letterSpacing: 0 }}
            title="原型里的回收数据是造出来的示意"
          >
            原型数据
          </span>
        </div>
        {!sent ? (
          <>
            <p className="mt-2 text-mk-body leading-[1.9] text-mk-secondary">
              真的发出去之前，先想清楚发给谁：整个年级？还是你确定会认真填的三十个人？
              <strong className="font-semibold text-mk-ink">30 份是能看出趋势的最小数字。</strong>
            </p>
            <Btn className="mt-3" onClick={() => setSent(true)}>
              发出去（原型：直接模拟回收）
            </Btn>
          </>
        ) : (
          <div className="mt-3">
            <p className="text-mk-body text-mk-secondary">
              收回 <span className="font-mono font-bold text-mk-ink">{total}</span> 份。第一题的分布：
            </p>
            <ul className="mt-3 space-y-2.5">
              {mock.map((r) => (
                <li key={r.label}>
                  <div className="flex items-baseline justify-between text-mk-small">
                    <span className="text-mk-ink">{r.label}</span>
                    <span className="font-mono tabular-nums text-mk-muted">
                      {r.n} · {Math.round((r.n / total) * 100)}%
                    </span>
                  </div>
                  <div className="mt-1 h-2 overflow-hidden rounded-mk-full" style={{ background: "var(--mk-border)" }}>
                    <div
                      className="h-full rounded-mk-full transition-all duration-700 ease-mk"
                      style={{ width: `${(r.n / total) * 100}%`, background: "var(--mk-matcha)" }}
                    />
                  </div>
                </li>
              ))}
            </ul>
            <p className="mt-4 rounded-mk-md p-3 text-mk-body leading-[1.85] text-mk-ink" style={{ background: "var(--mk-accent-50)" }}>
              印记：67% 说扔过。别急着写「大家都很浪费」——先看第二题，他们扔的到底是什么。数据的意义藏在下一题里。
            </p>
          </div>
        )}
      </Panel>
    </div>
  );
}

/** 设计 — three directions, enforced. */
function ThreeDirections() {
  const [dirs, setDirs] = useState(["", "", ""]);
  const filled = dirs.filter((d) => d.trim().length > 0).length;

  return (
    <Panel className="p-6">
      <Sys>三个方向 · 不是一个</Sys>
      <p className="mt-1.5 max-w-[58ch] text-mk-body leading-[1.9] text-mk-secondary">
        只做一个方案的人会爱上它，然后看不见它的问题。先写三个明显不同的方向——
        <strong className="font-semibold text-mk-ink">不同到你能说出它们各自会失败在哪</strong>
        ——再挑。
      </p>
      <div className="mt-4 space-y-4">
        {dirs.map((d, i) => (
          <Field
            key={i}
            label={`方向 ${String.fromCharCode(65 + i)}`}
            hint={
              ["最保险的那个：别人做过，你知道它能成。", "最省事的那个：一个下午能做完。", "最冒险的那个：可能做不出来，但如果成了很不一样。"][i]
            }
            value={d}
            onChange={(v) => setDirs((x) => x.map((y, j) => (j === i ? v : y)))}
            rows={3}
          />
        ))}
      </div>
      <p className="mt-4 text-mk-small text-mk-muted">
        已写 <span className="font-mono font-bold text-mk-ink">{filled}</span> / 3。
        {filled === 3 ? " 三个都在了。现在问自己一句：哪一个失败了我会最后悔？" : " 三个都写完再挑。"}
      </p>
    </Panel>
  );
}

/** 试 — a log of what a real person got stuck on. */
function TestLog() {
  const [rows, setRows] = useState<{ who: string; stuck: string; fix: string }[]>([
    { who: "", stuck: "", fix: "" },
  ]);

  return (
    <Panel className="p-6">
      <Sys>试用记录</Sys>
      <p className="mt-1.5 max-w-[58ch] text-mk-body leading-[1.9] text-mk-secondary">
        找真的人来试，然后
        <strong className="font-semibold text-mk-ink">闭嘴</strong>
        。他卡住的地方就是要改的地方；你一解释，这次测试就废了。
      </p>
      <div className="mt-4 space-y-3">
        {rows.map((r, i) => (
          <div key={i} className="rounded-mk-md border border-mk-border p-4">
            <span className="eco-mono text-mk-faint">试用者 {String(i + 1).padStart(2, "0")}</span>
            <div className="mt-2 grid gap-3 sm:grid-cols-3">
              {(["who", "stuck", "fix"] as const).map((k) => (
                <label key={k} className="block">
                  <span className="block text-mk-small font-medium text-mk-secondary">
                    {{ who: "谁", stuck: "他卡在哪", fix: "我改了什么" }[k]}
                  </span>
                  <input
                    value={r[k]}
                    onChange={(e) =>
                      setRows((x) => x.map((y, j) => (j === i ? { ...y, [k]: e.target.value } : y)))
                    }
                    className="mt-1 w-full rounded-mk-sm border border-mk-input-border bg-mk-surface px-2.5 py-2
                               text-mk-body text-mk-ink outline-none focus:border-mk-accent-300
                               focus:ring-2 focus:ring-mk-accent-100"
                  />
                </label>
              ))}
            </div>
          </div>
        ))}
      </div>
      <Btn
        variant="quiet"
        size="sm"
        className="mt-3"
        iconStart={<Plus size={15} strokeWidth={2.2} />}
        onClick={() => setRows((x) => [...x, { who: "", stuck: "", fix: "" }])}
      >
        再加一个试用者
      </Btn>
      <p className="mt-3 text-mk-small text-mk-muted">
        三个人通常就够找出八成的问题。第四个人开始，你听到的多半是重复的。
      </p>
    </Panel>
  );
}

/** 做 / 记录 / 发布 — a working surface plus the one question that step needs. */
function WorkNotes({ kind }: { kind: ProjectStep["kind"] }) {
  const [text, setText] = useState("");
  const prompts: Partial<Record<ProjectStep["kind"], { label: string; hint: string }>> = {
    make: { label: "做的时候发生了什么", hint: "特别记下：哪一处和你设计的不一样，你怎么处理的。" },
    document: { label: "写给一个没来过现场的人", hint: "你觉得「这还用说吗」的地方，正是要说的地方。" },
    publish: { label: "我做出了什么", hint: "两三句。写真的做出来的东西，不写本来想做的。" },
  };
  const p = prompts[kind] ?? { label: "记录", hint: "写下来。" };

  return (
    <Panel className="p-6">
      <Sys>{STEP_META[kind].label}</Sys>
      <Field label={p.label} hint={p.hint} value={text} onChange={setText} rows={12} />
      <p className="mt-2 font-mono text-mk-small text-mk-faint">{text.trim().length} 字</p>
    </Panel>
  );
}
