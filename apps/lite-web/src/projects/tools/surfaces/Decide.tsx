import { useCallback, useEffect, useState } from "react";
import { Plus, X } from "lucide-react";
import { Icon } from "@/ui";
import {
  addCriterion,
  addOption,
  decisionTodo,
  listDecisions,
  openDecision,
  removeCriterion,
  settleDecision,
  updateOption,
  type Decision,
} from "../../../api/decide";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Decide —— 做一个决定。
 *
 * ⚠️ 产品负责人 2026-09-01 的这一节是空的，只有标题。这是我的设计，等她审。
 *
 * 不做打分矩阵。四步，每一步一句话：
 *
 *   摆开选项 → 说清这里什么重要 → 每个选项赢在哪、疼在哪 → 选，说为什么
 *
 * 最后多一格：**什么会让你改主意**。写得出这一句，这个决定就成了一个可以被
 * 后来的事实推翻的判断，而不是一次表态——复盘的时候也才有东西可以回头看。
 *
 * 界面按这个顺序**一段一段解锁**，因为反过来做（先选好、再补理由）正是这件
 * 工具想拦住的那种"决定"。
 */
export function Decide({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [decision, setDecision] = useState<Decision | null>(null);
  const [subject, setSubject] = useState("");
  const [optDraft, setOptDraft] = useState("");
  const [critDraft, setCritDraft] = useState("");
  const [choice, setChoice] = useState("");
  const [why, setWhy] = useState("");
  const [gaveUp, setGaveUp] = useState("");
  const [flip, setFlip] = useState("");
  const [error, setError] = useState<string | null>(null);

  const boot = useCallback(async () => {
    const all = await listDecisions(projectId);
    const live = all.filter((d) => !d.settledAt);
    setDecision(live[live.length - 1] ?? null);
  }, [projectId]);

  useEffect(() => {
    void boot();
  }, [boot]);

  function guard<T>(p: Promise<T>, msg: string): Promise<T | null> {
    return p.catch((e) => {
      setError(e instanceof Error ? e.message : msg);
      return null;
    });
  }

  async function start() {
    const s = subject.trim();
    if (!s) return;
    const d = await guard(openDecision(projectId, { subject: s }), "没开起来。");
    if (d) setDecision(d);
  }

  async function pushOption() {
    if (!decision || !optDraft.trim()) return;
    const d = await guard(addOption(projectId, decision.id, { label: optDraft.trim() }), "没加上。");
    if (d) {
      setDecision(d);
      setOptDraft("");
    }
  }

  async function pushCriterion() {
    if (!decision || !critDraft.trim()) return;
    const d = await guard(addCriterion(projectId, decision.id, critDraft.trim()), "没加上。");
    if (d) {
      setDecision(d);
      setCritDraft("");
    }
  }

  async function finish() {
    if (!decision) return;
    const d = await guard(
      settleDecision(projectId, decision.id, {
        choice: choice.trim(),
        why: why.trim(),
        gaveUp: gaveUp.trim(),
        flip: flip.trim(),
      }),
      "还差点东西。",
    );
    if (d) {
      // 她写的"为什么选它"是她的话；其余几栏印记自己去读。
      onFinish({ decisionId: d.id }, d.why);
    }
  }

  const todo = decisionTodo(decision, { choice, why, flip });
  const hasOptions = (decision?.options.length ?? 0) >= 2;
  const hasCriteria = (decision?.criteria.length ?? 0) > 0;

  return (
    <ToolFrame
      title={tool.label}
      task="针对每个选项的原因及可能后果进行深入思考，再做出决定"
      why={tool.reason}
      todo={todo}
      finishLabel="确认选择"
      onFinish={() => void finish()}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {!decision ? (
        <div>
          <label className="text-mk-body text-mk-ink">你在定什么？</label>
          <textarea
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            rows={2}
            placeholder="比如：这个建议先给食堂还是先发在班群里"
            className="mt-2 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
          <button
            type="button"
            onClick={() => void start()}
            disabled={!subject.trim()}
            className="mt-2 rounded-mk-full px-4 py-1.5 text-mk-small text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            开始
          </button>
        </div>
      ) : (
        <>
          <p className="text-mk-body text-mk-ink">{decision.subject}</p>

          {/* ① 有哪些选择 */}
          <Section n={1} title="有哪些选择">
            <div className="space-y-1.5">
              {decision.options.map((o) => (
                <div key={o.id} className="rounded-mk-md border border-mk-border px-3 py-2">
                  <p className="text-mk-small text-mk-ink">{o.label}</p>
                  {o.author === "yinji" && (
                    <p className="mt-0.5 text-mk-small text-mk-faint">印记提的</p>
                  )}
                </div>
              ))}
            </div>
            <AddRow
              value={optDraft}
              onChange={setOptDraft}
              onAdd={() => void pushOption()}
              placeholder="再写一个选择"
            />
          </Section>

          {/* ② 什么最重要 —— 最常被跳过的一步 */}
          {hasOptions && (
            <Section n={2} title="这件事上，什么最重要">
              <p className="mb-1.5 text-mk-small text-mk-muted">
                用你自己的话。想不出来，就想想哪个结果你最不能接受。
              </p>
              <div className="flex flex-wrap gap-1.5">
                {decision.criteria.map((c) => (
                  <span
                    key={c.id}
                    className="flex items-center gap-1 rounded-mk-full border border-mk-border px-2.5 py-1 text-mk-small text-mk-ink"
                  >
                    {c.label}
                    <button
                      type="button"
                      aria-label="去掉"
                      onClick={() =>
                        void guard(removeCriterion(projectId, c.id), "没去掉。").then(
                          (d) => d && setDecision(d),
                        )
                      }
                      className="text-mk-faint"
                    >
                      <Icon icon={X} size={12} />
                    </button>
                  </span>
                ))}
              </div>
              <AddRow
                value={critDraft}
                onChange={setCritDraft}
                onAdd={() => void pushCriterion()}
                placeholder="比如：这周之内能做完"
              />
            </Section>
          )}

          {/* ③ 各自赢在哪、疼在哪 */}
          {hasOptions && hasCriteria && (
            <Section n={3} title="照这几条看，各自怎么样">
              <div className="space-y-2">
                {decision.options.map((o) => (
                  <div key={o.id} className="rounded-mk-md border border-mk-border px-3 py-2">
                    <p className="text-mk-small font-semibold text-mk-ink">{o.label}</p>
                    <Line
                      label="赢在"
                      value={o.wins}
                      onSave={(v) =>
                        void guard(updateOption(projectId, o.id, { wins: v }), "没记下。").then(
                          (d) => d && setDecision(d),
                        )
                      }
                    />
                    <Line
                      label="疼在"
                      value={o.hurts}
                      onSave={(v) =>
                        void guard(updateOption(projectId, o.id, { hurts: v }), "没记下。").then(
                          (d) => d && setDecision(d),
                        )
                      }
                    />
                  </div>
                ))}
              </div>
            </Section>
          )}

          {/* ④ 选 */}
          {hasOptions && hasCriteria && (
            <Section n={4} title="那就选一个">
              <div className="flex flex-wrap gap-1.5">
                {decision.options.map((o) => (
                  <button
                    key={o.id}
                    type="button"
                    onClick={() => setChoice(o.label)}
                    className="rounded-mk-full px-3 py-1 text-mk-small"
                    style={
                      choice === o.label
                        ? { background: "var(--mk-accent-500)", color: "#fff" }
                        : { border: "1px solid var(--mk-border)", color: "var(--mk-secondary)" }
                    }
                  >
                    {o.label}
                  </button>
                ))}
              </div>
              <Field label="为什么选它" value={why} onChange={setWhy} placeholder="一句就够" />
              <Field
                label="放弃了什么"
                value={gaveUp}
                onChange={setGaveUp}
                placeholder="选了这个，就没得到的那个东西"
              />
              {/* 整件事的关键格。 */}
              <Field
                label="什么会让你改主意"
                value={flip}
                onChange={setFlip}
                placeholder="比如：如果食堂说他们早就试过了"
              />
              {flip.trim() && (
                <p className="mt-1.5 text-mk-small text-mk-muted">
                  记下来了。以后真碰上这件事，回头看这里。
                </p>
              )}
            </Section>
          )}
        </>
      )}
    </ToolFrame>
  );
}

function Section({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <section className="mt-4 border-t border-mk-border pt-3">
      <p className="mb-2 text-mk-small text-mk-secondary">
        <span className="text-mk-faint">{n}.</span> {title}
      </p>
      {children}
    </section>
  );
}

function AddRow({
  value,
  onChange,
  onAdd,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  onAdd: () => void;
  placeholder: string;
}) {
  return (
    <div className="mt-2 flex items-end gap-2">
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && onAdd()}
        placeholder={placeholder}
        className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
      <button
        type="button"
        onClick={onAdd}
        disabled={!value.trim()}
        aria-label="加上"
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
        style={{ background: "var(--mk-accent-500)" }}
      >
        <Icon icon={Plus} size={15} />
      </button>
    </div>
  );
}

function Line({
  label,
  value,
  onSave,
}: {
  label: string;
  value: string;
  onSave: (v: string) => void;
}) {
  const [text, setText] = useState(value);
  useEffect(() => setText(value), [value]);
  return (
    <div className="mt-1.5 flex items-baseline gap-2">
      <span className="shrink-0 text-mk-small text-mk-muted">{label}</span>
      <input
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text.trim() !== value.trim() && onSave(text.trim())}
        placeholder="一句"
        className="flex-1 border-b border-mk-border bg-transparent pb-0.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <div className="mt-2.5">
      <label className="text-mk-small text-mk-secondary">{label}</label>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={2}
        placeholder={placeholder}
        className="mt-1 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}
