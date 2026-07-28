import { useEffect, useRef, useState } from "react";
import type { Mirror, Proposal } from "@mind-imprint/contracts";
import { ApiError } from "../../api/client";
import { finishProject } from "../../api/projects";
import { Icon } from "../Icon";
import { reflectionPrompts } from "./mockData";
import { getReflection, putReflection, getMirror, postMirror } from "../api/workspace";

// The Review block: a mirror, not a report card. The student's own reflection
// leads (left); the AI-assembled "你的思维印记" narrative sits alongside as
// support (right). The rubric/assessment runs quietly on 完成回顾 and feeds the
// teacher/parent reports — it is NOT shown here as a grade.
//
// API-backed (slice 5): reflection answers debounce to PUT /reflection-doc, the
// mirror loads via GET /mirror (composing once via POST /mirror when absent),
// and 完成回顾 flips done=true then finishes the project (assessment generated
// silently into the growth report).
export function ReviewBlock({
  projectId,
  proposal,
  onFinished,
}: {
  projectId: string;
  proposal: Proposal;
  onFinished?: () => void;
}) {
  const [answers, setAnswers] = useState<string[]>(reflectionPrompts.map(() => ""));
  const [done, setDone] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState<string | null>(null);
  const answersRef = useRef<string[]>(reflectionPrompts.map(() => ""));
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load the stored reflection on mount, padded/truncated to exactly the 5
  // prompts so a shorter/longer stored array still lines up with the UI.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const doc = await getReflection(projectId);
        if (cancelled) return;
        const padded = reflectionPrompts.map((_, i) => doc.answers[i] ?? "");
        answersRef.current = padded;
        setAnswers(padded);
        setDone(doc.done);
      } catch {
        /* leave the blank prompts; the placeholders show */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // Flush a pending save on unmount so a last edit inside the debounce window
  // isn't lost when the student leaves the room.
  useEffect(
    () => () => {
      if (saveTimer.current) {
        clearTimeout(saveTimer.current);
        void putReflection(projectId, { answers: answersRef.current }).catch(() => {});
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [projectId],
  );

  function editAnswer(i: number, value: string) {
    const next = answersRef.current.map((x, j) => (j === i ? value : x));
    answersRef.current = next;
    setAnswers(next);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      void putReflection(projectId, { answers: answersRef.current }).catch(() => {
        /* retries on the next keystroke */
      });
    }, 600);
  }

  // 完成回顾: persist answers with done=true, then finish the project (which
  // generates the process assessment into the growth report). A 422 means the
  // server still considers the reflection unfinished — surface its message.
  async function finish() {
    if (finishing || done) return;
    setFinishing(true);
    setFinishError(null);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    try {
      await putReflection(projectId, { answers: answersRef.current, done: true });
      await finishProject(projectId);
      setDone(true);
    } catch (e) {
      if (e instanceof ApiError) {
        setFinishError(e.message || "还不能归档，请稍后再试。");
      } else {
        setFinishError("刚才没接上，稍等再试一次。");
      }
    } finally {
      setFinishing(false);
    }
  }

  return (
    <div className="grid h-full grid-cols-[1fr,380px]">
      {/* main · the student's reflection */}
      <div className="min-h-0 overflow-y-auto px-10 py-9">
        <div className="mx-auto max-w-2xl">
          <header className="mb-6">
            <p className="text-[12px] font-semibold uppercase tracking-[0.18em] text-mk-muted-2">项目收尾</p>
            <h1 className="mt-1 font-sans text-[26px] font-bold leading-tight text-mk-ink">回过头看看这一程</h1>
            <p className="mt-1.5 text-[14px] text-mk-muted">用你自己的话回答几个问题。右边是印记帮你整理的过程，卡壳时可以看看——但话得你自己说。</p>
          </header>

          <div className="flex flex-col gap-6">
            {reflectionPrompts.map((p, i) => (
              <div key={i}>
                <div className="flex items-center gap-2">
                  <span className="flex h-6 w-6 flex-none items-center justify-center rounded-full bg-mk-primary-tint text-[12px] font-bold text-mk-primary">{i + 1}</span>
                  <span className="rounded-full bg-mk-bg px-2 py-0.5 text-[11px] font-bold text-mk-muted">{p.label}</span>
                </div>
                <p className="mt-1.5 text-[14.5px] font-bold leading-snug text-mk-ink">{p.q}</p>
                {p.anchor === "goal" && proposal.objective && (
                  <p className="mt-1.5 rounded-mk border-l-2 border-mk-accent bg-mk-accent-tint/40 px-3 py-2 text-[12.5px] leading-relaxed text-mk-muted">
                    <span className="font-bold text-mk-accent">开题时你写的目标 · </span>{proposal.objective}
                  </p>
                )}
                <textarea
                  value={answers[i]}
                  onChange={(e) => editAnswer(i, e.target.value)}
                  disabled={done}
                  rows={4}
                  placeholder="写下你的想法……"
                  className="mt-2 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3 text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary disabled:opacity-70"
                />
              </div>
            ))}
          </div>

          <div className="mt-7 flex flex-wrap items-center gap-4">
            {done ? (
              <>
                <span className="text-[13px] font-semibold text-mk-green">已归档 · 这次的过程评估已记入你的成长报告</span>
                {onFinished && (
                  <button
                    type="button"
                    onClick={onFinished}
                    className="rounded-mk bg-mk-primary px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-primary-hover"
                  >
                    查看成长报告 →
                  </button>
                )}
              </>
            ) : (
              <>
                <button
                  type="button"
                  onClick={() => void finish()}
                  disabled={finishing}
                  className="rounded-mk bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-hover disabled:opacity-50"
                >
                  {finishing ? "归档中……" : "完成回顾"}
                </button>
                {finishError ? (
                  <span className="text-[12.5px] font-semibold text-mk-accent">{finishError}</span>
                ) : (
                  <span className="text-[12.5px] text-mk-muted-2">完成后会生成过程评估，记入成长报告（老师 / 家长可见），这里不打分。</span>
                )}
              </>
            )}
          </div>
        </div>
      </div>

      {/* aside · the mirror */}
      <MirrorPane projectId={projectId} />
    </div>
  );
}

/* ---------- right · the "你的思维印记" mirror ---------- */

function MirrorPane({ projectId }: { projectId: string }) {
  const [mirror, setMirror] = useState<Mirror | null>(null);
  const [composing, setComposing] = useState(true);

  // GET the stored mirror; if none exists yet, POST once to compose it
  // (first-open-wins on the server — a concurrent open won't double-spend).
  useEffect(() => {
    let cancelled = false;
    setMirror(null);
    setComposing(true);
    (async () => {
      try {
        let m = await getMirror(projectId);
        if (m == null) m = await postMirror(projectId);
        if (!cancelled) setMirror(m);
      } catch {
        /* leave the empty state; a reopen retries */
      } finally {
        if (!cancelled) setComposing(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  return (
    <aside className="flex min-h-0 flex-col border-l border-mk-border bg-mk-surface">
      <header className="border-b border-mk-border px-5 py-4">
        <div className="flex items-center gap-2 text-mk-primary">
          <Icon name="spark" size={16} />
          <h2 className="font-sans text-[15px] font-bold">你的思维印记</h2>
        </div>
        <p className="mt-1 text-[11.5px] text-mk-muted-2">印记根据你的全过程整理，供你参考——不是评分。</p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        {composing && !mirror ? (
          <p className="text-[12.5px] leading-relaxed text-mk-muted-2">印记正在回看你的全过程，整理这份思维印记……</p>
        ) : !mirror ? (
          <p className="text-[12.5px] leading-relaxed text-mk-muted-2">这份印记还没能整理出来——稍后重新打开回顾再看看。</p>
        ) : (
          <>
            <div className="flex flex-col gap-4">
              {mirror.sections.map((s, i) => (
                <div key={i} className="border-l-2 border-mk-primary/30 pl-3">
                  <p className="text-[12px] font-bold text-mk-primary">{s.title}</p>
                  <p className="mt-1 text-[13px] leading-relaxed text-mk-ink">{s.body}</p>
                </div>
              ))}
            </div>

            {mirror.carryForwards.length > 0 && (
              <div className="mt-6 rounded-mk-lg border border-mk-accent/30 bg-mk-accent-tint/50 p-4">
                <p className="mb-2 flex items-center gap-1.5 text-[12px] font-bold text-mk-accent">
                  <Icon name="arrow" size={14} /> 带走这两点
                </p>
                <ul className="flex flex-col gap-2">
                  {mirror.carryForwards.map((c, i) => (
                    <li key={i} className="text-[13px] leading-relaxed text-mk-ink">· {c}</li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </div>
    </aside>
  );
}
