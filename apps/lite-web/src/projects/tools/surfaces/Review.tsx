import { useCallback, useEffect, useMemo, useState } from "react";
import { MessageSquareQuote } from "lucide-react";
import { Icon } from "@/ui";
import {
  isDocument,
  listArtifacts,
  pendingArtifacts,
  settleArtifact,
  type Artifact,
} from "../../../api/artifacts";
import {
  answerDimension,
  answerMark,
  askAbout,
  getReview,
  reviewTodo,
  splitByMarks,
  type ReviewMark,
  type ReviewPlan,
} from "../../../api/review";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Review —— 审一遍印记交出来的东西。
 *
 * 产品负责人 2026-09-01：能审的前提是**看得懂**。所以原文照原样铺开，划出来的
 * 句子就高亮在它本来的位置上——一句话离开上下文就没法判断对不对，而判断对不对
 * 正是审阅这件事本身。
 *
 * 她选中任何一段都能就地问一句，那一问会开一条独立的会话线。这是「click on any
 * word to have a new chat line」，也是这块界面上最该顺手的动作：选中，点一下，
 * 完了。
 *
 * 印记自己写的"我猜了什么 / 哪里还不对"钉在最上面。她要审的是一份**承认了自己
 * 弱点**的东西，不是一份看起来什么都对的东西。
 */
export function Review({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [pickedId, setPickedId] = useState<string | null>(null);
  const [plan, setPlan] = useState<ReviewPlan>({ marks: [], dimensions: [] });
  const [openMark, setOpenMark] = useState<string | null>(null);
  const [selection, setSelection] = useState("");
  const [verdictWhy, setVerdictWhy] = useState("");
  const [error, setError] = useState<string | null>(null);

  const pending = pendingArtifacts(artifacts);
  const artifact = pending.find((a) => a.id === pickedId) ?? pending[0] ?? null;

  useEffect(() => {
    void (async () => {
      const all = await listArtifacts(projectId);
      setArtifacts(all);
    })();
  }, [projectId]);

  const loadPlan = useCallback(async () => {
    if (!artifact) return;
    setPlan(await getReview(projectId, artifact.id));
  }, [projectId, artifact]);

  useEffect(() => {
    void loadPlan();
  }, [loadPlan]);

  async function ask() {
    const quote = selection.trim();
    if (!artifact || !quote) return;
    try {
      await askAbout(projectId, artifact.id, quote);
      setSelection("");
      window.getSelection()?.removeAllRanges();
      await loadPlan();
    } catch {
      setError("这一句没问出去，再试一次。");
    }
  }

  async function saveMark(id: string, answer: string) {
    try {
      const got = await answerMark(projectId, id, answer);
      setPlan((p) => ({ ...p, marks: p.marks.map((m) => (m.id === got.id ? got : m)) }));
    } catch {
      setError("没记下来，再试一次。");
    }
  }

  async function saveDimension(id: string, answer: string) {
    try {
      const got = await answerDimension(projectId, id, answer);
      setPlan((p) => ({
        ...p,
        dimensions: p.dimensions.map((d) => (d.id === got.id ? got : d)),
      }));
    } catch {
      setError("没记下来，再试一次。");
    }
  }

  async function finish(verdict: "kept" | "revise" | "dropped") {
    if (!artifact) return;
    try {
      await settleArtifact(projectId, artifact.id, verdict, verdictWhy.trim());
      const word = verdict === "kept" ? "收下了" : verdict === "revise" ? "退回去改" : "不要了";
      onFinish(
        { artifactId: artifact.id, verdict },
        `我审完了《${artifact.title || "这一份"}》，${word}。因为${verdictWhy.trim()}`,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "没定下来，再试一次。");
    }
  }

  const paragraphs = useMemo(
    () => (artifact?.payload.body ?? "").split(/\n{2,}/).filter((p) => p.trim()),
    [artifact],
  );
  const marksTodo = reviewTodo(plan);
  const todo = !artifact
    ? "现在没有要审的东西"
    : marksTodo || (verdictWhy.trim() ? "" : "一句你的结论");

  return (
    <ToolFrame
      title={tool.label}
      task="一段一段看过去。不同意的地方，说出来"
      why={tool.reason}
      todo={todo}
      finishLabel="收下"
      onFinish={() => void finish("kept")}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {!artifact && (
        <p className="text-mk-small text-mk-muted">印记还没有交出什么要你审的东西。</p>
      )}

      {artifact && (
        <>
          {pending.length > 1 && (
            <div className="mb-3 flex flex-wrap gap-1">
              {pending.map((a) => (
                <button
                  key={a.id}
                  type="button"
                  onClick={() => setPickedId(a.id)}
                  className="rounded-mk-full px-2.5 py-1 text-mk-small"
                  style={
                    a.id === artifact.id
                      ? { background: "var(--mk-accent-500)", color: "#fff" }
                      : { color: "var(--mk-secondary)", border: "1px solid var(--mk-border)" }
                  }
                >
                  {a.title || "未命名"}
                </button>
              ))}
            </div>
          )}

          {/* 印记先承认自己的弱点。她要审的是这个，不是一份看起来什么都对的东西。 */}
          <div className="mb-3 rounded-mk-md px-3 py-2" style={{ background: "var(--mk-paper)" }}>
            <p className="text-mk-small text-mk-muted">印记说：</p>
            {artifact.guessed.map((g, i) => (
              <p key={`g${i}`} className="mt-0.5 text-mk-small text-mk-secondary">
                我猜了：{g}
              </p>
            ))}
            {artifact.admits.map((x, i) => (
              <p key={`a${i}`} className="mt-0.5 text-mk-small text-mk-secondary">
                这版还不对的地方：{x}
              </p>
            ))}
          </div>

          {/* 正文 */}
          {isDocument(artifact) ? (
            <div
              onMouseUp={() => setSelection(window.getSelection()?.toString() ?? "")}
              className="space-y-2"
            >
              {paragraphs.map((p, i) => (
                <p key={i} className="text-mk-small leading-relaxed text-mk-ink">
                  {splitByMarks(p, plan.marks).map((seg, j) =>
                    seg.mark ? (
                      <mark
                        key={j}
                        onClick={() => setOpenMark(openMark === seg.mark!.id ? null : seg.mark!.id)}
                        className="cursor-pointer rounded-mk-sm px-0.5"
                        style={{
                          background: seg.mark.answer.trim()
                            ? "color-mix(in srgb, #10B981 22%, transparent)"
                            : "color-mix(in srgb, #F59E0B 28%, transparent)",
                          color: "var(--mk-ink)",
                        }}
                      >
                        {seg.text}
                      </mark>
                    ) : (
                      <span key={j}>{seg.text}</span>
                    ),
                  )}
                </p>
              ))}
            </div>
          ) : artifact.payload.url ? (
            <img
              src={artifact.payload.url}
              alt={artifact.title}
              className="w-full rounded-mk-md"
            />
          ) : (
            <p className="text-mk-small text-mk-muted">这件东西没有可以直接看的内容。</p>
          )}

          {/* 选中就地问 */}
          {selection.trim() && (
            <button
              type="button"
              onClick={() => void ask()}
              className="mt-2 flex w-full items-center justify-center gap-1.5 rounded-mk-full py-2 text-mk-small text-white"
              style={{ background: "var(--mk-accent-500)" }}
            >
              <Icon icon={MessageSquareQuote} size={14} />
              问问这一句
            </button>
          )}

          {/* 划出来的句子 */}
          {plan.marks.length > 0 && (
            <div className="mt-4 space-y-2 border-t border-mk-border pt-3">
              {plan.marks.map((m) => (
                <MarkRow
                  key={m.id}
                  mark={m}
                  open={openMark === m.id}
                  onToggle={() => setOpenMark(openMark === m.id ? null : m.id)}
                  onSave={(a) => void saveMark(m.id, a)}
                />
              ))}
            </div>
          )}

          {/* 该看的几个方面 */}
          {plan.dimensions.length > 0 && (
            <div className="mt-4 space-y-2 border-t border-mk-border pt-3">
              <p className="text-mk-small text-mk-muted">看这种东西，这几点绕不开</p>
              {plan.dimensions.map((d) => (
                <div key={d.id} className="rounded-mk-md border border-mk-border px-3 py-2">
                  <p className="text-mk-small text-mk-ink">{d.prompt}</p>
                  {d.why && <p className="mt-0.5 text-mk-small text-mk-muted">{d.why}</p>}
                  <AnswerBox value={d.answer} onSave={(a) => void saveDimension(d.id, a)} />
                </div>
              ))}
            </div>
          )}

          {/* 结论 */}
          <div className="mt-4 border-t border-mk-border pt-3">
            <label className="text-mk-small text-mk-secondary">你的结论，一句话</label>
            <textarea
              value={verdictWhy}
              onChange={(e) => setVerdictWhy(e.target.value)}
              rows={2}
              placeholder="比如：第二段把我的话改成了它自己的说法"
              className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
            {/* 退回和收下一样正当，所以它们摆在一起，费的力气也一样。 */}
            <div className="mt-2 flex gap-2">
              <button
                type="button"
                disabled={!verdictWhy.trim()}
                onClick={() => void finish("revise")}
                className="flex-1 rounded-mk-full border border-mk-border py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
              >
                退回去改
              </button>
              <button
                type="button"
                disabled={!verdictWhy.trim()}
                onClick={() => void finish("dropped")}
                className="flex-1 rounded-mk-full border border-mk-border py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
              >
                不要了
              </button>
            </div>
          </div>
        </>
      )}
    </ToolFrame>
  );
}

function MarkRow({
  mark,
  open,
  onToggle,
  onSave,
}: {
  mark: ReviewMark;
  open: boolean;
  onToggle: () => void;
  onSave: (answer: string) => void;
}) {
  return (
    <div className="rounded-mk-md border border-mk-border px-3 py-2">
      <button type="button" onClick={onToggle} className="block w-full text-left">
        {mark.quote && (
          <p className="text-mk-small text-mk-muted">「{mark.quote}」</p>
        )}
        <p className="mt-0.5 text-mk-small text-mk-ink">{mark.question}</p>
        {mark.mine && <p className="mt-0.5 text-mk-small text-mk-faint">你问的</p>}
      </button>
      {open && <AnswerBox value={mark.answer} onSave={onSave} />}
      {!open && mark.answer && (
        <p className="mt-1 text-mk-small text-mk-secondary">{mark.answer}</p>
      )}
    </div>
  );
}

function AnswerBox({ value, onSave }: { value: string; onSave: (v: string) => void }) {
  const [text, setText] = useState(value);
  useEffect(() => setText(value), [value]);
  return (
    <textarea
      value={text}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => text.trim() !== value.trim() && onSave(text.trim())}
      rows={2}
      placeholder="写下你的看法"
      className="mt-1.5 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
    />
  );
}
