import { useState } from "react";
import { ChevronDown, ChevronUp, Copy, Check } from "lucide-react";
import { useEco } from "../../store";
import { PAGE_EXAMPLES } from "../../data/examples";
import { STUDENT } from "../../data/library";
import { WORK_MODES, compileInstruction, styleById } from "../../data/homepage";
import { Bold, Panel, Sys, cx } from "../../ui";

/**
 * Step 2 · 学会下清楚的指令 — the module's core.
 *
 * The claim: 「明确的指令」不是玄学，它是三样东西凑齐。The screen is literally
 * three panels, one per thing, and then it COMPILES her three choices into a
 * real instruction and shows it to her:
 *
 *   ① 直接的例子   — chosen in step 1, echoed here (and changeable)
 *   ② 内容的结构   — which blocks, in what order
 *   ③ 合作方式     — ask / propose / tidy
 *
 * The compiled text is copyable on purpose: if it works in any other AI tool,
 * she has learned to instruct rather than learned our buttons. That is the
 * whole point of the exercise, and the copy button is what proves it.
 *
 * 🚨 The last line of the compiled instruction is the 铁律: 正文是我写的。It is
 * appended by `compileInstruction`, not by anything on this screen, so it can
 * never be dropped by a UI change.
 */
export function InstructionStep() {
  const { state, hpSetSections, hpSetMode, hpGo } = useEco();
  const hp = state.homepage;
  const [copied, setCopied] = useState(false);

  const example = PAGE_EXAMPLES.find((e) => e.id === hp.exampleId) ?? PAGE_EXAMPLES[0]!;
  const enabled = hp.sections.filter((s) => s.enabled);

  function move(id: string, dir: -1 | 1) {
    const arr = [...hp.sections];
    const i = arr.findIndex((s) => s.id === id);
    const j = i + dir;
    if (i < 0 || j < 0 || j >= arr.length) return;
    const a = arr[i]!;
    const b = arr[j]!;
    arr[i] = b;
    arr[j] = a;
    hpSetSections(arr);
  }

  function toggle(id: string) {
    hpSetSections(hp.sections.map((s) => (s.id === id ? { ...s, enabled: !s.enabled } : s)));
  }

  const instruction = compileInstruction({
    exampleName: example.name,
    exampleSteal: example.steal,
    sections: hp.sections,
    mode: hp.mode ?? "ask",
    styleLabel: hp.style ? styleById(hp.style).label : "还没选（下一步选）",
    who: `${STUDENT.grade}学生 ${STUDENT.name}`,
  });

  async function copy() {
    try {
      await navigator.clipboard.writeText(instruction);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard is blocked in some contexts. The text is on screen and
      // selectable, so there is nothing to recover — and nothing worth
      // interrupting her with.
    }
  }

  return (
    <div>
      <Panel className="mb-6 p-6">
        <Sys>为什么要学这个</Sys>
        <p className="mt-2 max-w-[68ch] text-mk-body-lg leading-[1.95] text-mk-ink">
          你大概已经发现了：同样一个 AI，有人用得很好，有人用不出东西来。差别几乎不在 AI，在
          <strong className="font-semibold">指令</strong>。而「清楚的指令」是可以拆开的——它是三样东西凑齐：
        </p>
        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          {[
            { n: "①", t: "一个直接的例子", d: "你要它像哪一个。空口说「好看」没用，指一个具体的东西最快。" },
            { n: "②", t: "内容的结构", d: "哪几块、什么顺序。不说结构，它就会自己编一个。" },
            { n: "③", t: "你想怎么合作", d: "它提问你答？它提方案你定？还是你写它只整理？不说，它默认替你做主。" },
          ].map((x) => (
            <div key={x.n} className="rounded-mk-md p-4" style={{ background: "var(--mk-paper)" }}>
              <span className="font-mono text-mk-h2 text-mk-accent-300">{x.n}</span>
              <p className="mt-1 text-mk-h3 text-mk-ink">{x.t}</p>
              <p className="mt-1 text-mk-small leading-relaxed text-mk-secondary">{x.d}</p>
            </div>
          ))}
        </div>
      </Panel>

      <div className="grid gap-4 lg:grid-cols-2">
        {/* ① example */}
        <Panel className="p-5">
          <div className="flex items-center justify-between">
            <Sys>① 直接的例子</Sys>
            <button
              type="button"
              onClick={() => hpGo(0)}
              className="text-mk-small text-mk-accent-700 underline-offset-2 hover:underline focus-visible:outline-none"
            >
              换一个
            </button>
          </div>
          <p className="mt-2 text-mk-h2 text-mk-ink">{example.name}</p>
          <p className="mt-0.5 font-mono text-mk-small text-mk-faint">{example.url}</p>
          <p className="mt-3 rounded-mk-md p-3 text-mk-body leading-[1.85] text-mk-secondary"
             style={{ background: "var(--mk-paper)" }}>
            我想学的那一招：{example.steal}
          </p>
        </Panel>

        {/* ③ mode (placed second visually so the two "choices" sit side by
            side; the numbering keeps the teaching order clear) */}
        <Panel className="p-5">
          <Sys>③ 你想怎么和 AI 合作</Sys>
          <p className="mt-1.5 text-mk-small text-mk-muted">
            三种都不代写你的正文。区别是它站在哪一边。
          </p>
          <ul className="mt-3 space-y-2">
            {WORK_MODES.map((m) => {
              const on = hp.mode === m.id;
              return (
                <li key={m.id}>
                  <button
                    type="button"
                    onClick={() => hpSetMode(m.id)}
                    className={cx(
                      "w-full rounded-mk-md border p-3.5 text-left transition-all duration-[140ms] ease-mk",
                      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                      on ? "border-mk-accent bg-mk-accent-50" : "border-mk-border hover:border-mk-accent-200",
                    )}
                  >
                    <span className="flex items-center gap-2">
                      <span
                        className="flex h-6 w-6 items-center justify-center rounded-mk-full font-mono text-[13px]"
                        style={{
                          background: on ? "var(--mk-accent)" : "var(--mk-paper)",
                          color: on ? "#fff" : "var(--mk-secondary)",
                        }}
                      >
                        {m.glyph}
                      </span>
                      <span className="text-mk-h3 text-mk-ink">{m.label}</span>
                    </span>
                    <span className="mt-1.5 block text-mk-body leading-[1.75] text-mk-secondary">
                      <Bold text={m.blurb} />
                    </span>
                    <span className="mt-1.5 block text-mk-small text-mk-muted">{m.feels}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </Panel>
      </div>

      {/* ② structure */}
      <Panel className="mt-4 p-5">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <Sys>② 内容的结构</Sys>
          <span className="text-mk-small text-mk-muted">
            勾选要哪几块，用箭头排顺序。少于四块通常更好看。
            <span className="ml-2 font-mono">当前 {enabled.length} 块</span>
          </span>
        </div>
        <ul className="mt-3 space-y-1.5">
          {hp.sections.map((s, i) => (
            <li
              key={s.id}
              className={cx(
                "flex items-center gap-3 rounded-mk-md border p-3 transition-colors duration-[140ms]",
                s.enabled ? "border-mk-border bg-mk-surface" : "border-dashed border-mk-border bg-transparent",
              )}
            >
              <input
                type="checkbox"
                checked={s.enabled}
                onChange={() => toggle(s.id)}
                className="h-4 w-4 shrink-0 accent-[color:var(--mk-accent)]"
                aria-label={`包含「${s.label}」`}
              />
              <span className="eco-mono w-6 shrink-0 text-mk-faint">
                {s.enabled ? String(hp.sections.filter((x, j) => x.enabled && j <= i).length).padStart(2, "0") : "--"}
              </span>
              <span className="min-w-0 flex-1">
                <span className={cx("block text-mk-body font-medium", s.enabled ? "text-mk-ink" : "text-mk-muted")}>
                  {s.label}
                </span>
                <span className="block text-mk-small text-mk-muted">{s.hint}</span>
              </span>
              {s.picker ? (
                <span
                  className="eco-mono shrink-0 rounded-mk-full px-2 py-1"
                  style={{ background: "var(--mk-lake-bg)", color: "var(--mk-lake-fg)", letterSpacing: 0 }}
                >
                  可以从我的{s.picker === "readings" ? "阅读" : s.picker === "writings" ? "写作" : "项目"}里挑
                </span>
              ) : null}
              <span className="flex shrink-0 flex-col">
                <button
                  type="button"
                  onClick={() => move(s.id, -1)}
                  disabled={i === 0}
                  className="rounded p-0.5 text-mk-muted hover:text-mk-ink disabled:opacity-30 focus-visible:outline-none"
                  aria-label="上移"
                >
                  <ChevronUp size={14} strokeWidth={2.2} />
                </button>
                <button
                  type="button"
                  onClick={() => move(s.id, 1)}
                  disabled={i === hp.sections.length - 1}
                  className="rounded p-0.5 text-mk-muted hover:text-mk-ink disabled:opacity-30 focus-visible:outline-none"
                  aria-label="下移"
                >
                  <ChevronDown size={14} strokeWidth={2.2} />
                </button>
              </span>
            </li>
          ))}
        </ul>
      </Panel>

      {/* the payoff */}
      <div className="mt-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <Sys>三样凑齐了 · COMPILED</Sys>
            <h3 className="mt-1 text-mk-h1 text-mk-ink">这就是你给 AI 的指令</h3>
            <p className="mt-1 max-w-[64ch] text-mk-body text-mk-secondary">
              下面这段是你刚才三个选择自动拼出来的。它在任何一个 AI 工具里都能用——你学会的是下指令，不是学会我们的按钮。
            </p>
          </div>
          <button
            type="button"
            onClick={copy}
            className="inline-flex items-center gap-2 rounded-mk-sm border border-mk-accent px-4 py-2 text-mk-body
                       font-medium text-mk-accent-700 transition-colors duration-[140ms] hover:bg-mk-accent-50
                       focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            {copied ? <Check size={16} strokeWidth={2.4} /> : <Copy size={16} strokeWidth={1.9} />}
            {copied ? "已复制" : "复制这段指令"}
          </button>
        </div>

        <pre
          className="mt-4 overflow-x-auto whitespace-pre-wrap rounded-mk-lg p-6 font-mono text-[13px] leading-[1.9]"
          style={{
            background: "#1C1713",
            color: "#E6DDD2",
            border: "1px solid rgba(240,233,224,.14)",
          }}
        >
          {instruction}
        </pre>
        {!hp.mode ? (
          <p className="mt-2 text-mk-small text-mk-muted">
            还没选合作方式，上面用的是默认的「AI 提问，我来答」。选一个，这段话会跟着变。
          </p>
        ) : null}
      </div>
    </div>
  );
}
