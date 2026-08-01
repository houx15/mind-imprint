import { useEffect, useRef, useState } from "react";
import type { Mirror, Proposal } from "@mind-imprint/contracts";
import { ApiError } from "../../api/client";
import { finishProject } from "../../api/projects";
import { Icon } from "../Icon";
import { reflectionPrompts } from "./mockData";
import {
  getReflection,
  putReflection,
  getMirror,
  postMirror,
  getAIUseDraft,
  postAIUse,
  coach,
  getCoachHistory,
} from "../api/workspace";
import type { AIUseRecord } from "@mind-imprint/contracts";

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
  const [showModal, setShowModal] = useState(false);
  // #5 · 完成回顾 is the second guarded moment (point of no return): archiving
  // locks 正文与回顾 and generates the assessment. Confirm before running it.
  const [confirmFinish, setConfirmFinish] = useState(false);
  const hasProposal = [proposal.objective, proposal.reason, proposal.activities, proposal.resources].some(
    (v) => v.trim() !== "",
  );
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

  // 完成回顾: persist answers with done=true, then finish the project. finish is
  // async (202) — it kicks off the flagship process assessment in the background
  // and returns status "evaluating" immediately; we don't block the UI, we open
  // a modal that sends the student back to 全部项目 (where the report shows up
  // later as "评估中" → "已完成"). A 422 means the server still considers the
  // reflection unfinished — surface its message inline.
  async function finish() {
    if (finishing || done) return;
    setFinishing(true);
    setFinishError(null);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    try {
      await putReflection(projectId, { answers: answersRef.current, done: true });
      await finishProject(projectId);
      setDone(true);
      setShowModal(true);
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

          {/* S5 · 复盘我与 AI 的互动 — the objective record + the student's own statement */}
          <AIUsePanel projectId={projectId} done={done} />

          {/* S5 · defense-readiness conversation (review不是polish) */}
          <ReviewCoachThread projectId={projectId} />

          <div className="mt-7 flex flex-wrap items-center gap-4">
            {done ? (
              <>
                <span className="text-[13px] font-semibold text-mk-green">已归档 · 过程评估正在生成，稍后可在「全部项目」里点开查看</span>
                {onFinished && (
                  <button
                    type="button"
                    onClick={onFinished}
                    className="rounded-mk bg-mk-primary px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-primary-hover"
                  >
                    回到全部项目 →
                  </button>
                )}
              </>
            ) : (
              <>
                <button
                  type="button"
                  onClick={() => setConfirmFinish(true)}
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
      <MirrorPane projectId={projectId} hasProposal={hasProposal} />

      {/* #5 · 完成回顾 confirm — the point of no return. Archiving locks 正文与回顾
          and generates the process assessment. */}
      {confirmFinish && !done && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-mk-ink/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-[0_20px_60px_rgba(28,35,51,0.25)]">
            <h2 className="font-sans text-[18px] font-bold text-mk-ink">完成回顾并归档？</h2>
            <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
              归档后，<span className="font-bold text-mk-ink">正文与回顾都会锁定、无法再修改</span>，印记会据此生成过程评估（记入成长报告，这里不打分）。确定完成吗？
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setConfirmFinish(false)}
                className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-ink"
              >
                再看看
              </button>
              <button
                type="button"
                onClick={() => { setConfirmFinish(false); void finish(); }}
                className="rounded-mk bg-mk-accent px-5 py-2 text-[13px] font-bold text-white transition hover:bg-mk-accent-hover"
              >
                完成回顾
              </button>
            </div>
          </div>
        </div>
      )}

      {/* finish modal · the assessment runs in the background; the student is
          free to leave. One action returns to 全部项目, where the row shows
          "评估中" and later "已完成 · 查看评估报告". */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-mk-ink/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-[0_20px_60px_rgba(28,35,51,0.25)]">
            <div className="flex items-center gap-2 text-mk-primary">
              <Icon name="spark" size={18} />
              <h2 className="font-sans text-[18px] font-bold text-mk-ink">评估报告生成中</h2>
            </div>
            <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
              印记正在回看你的全过程，生成过程评估——可能需要几分钟。你可以先去别处，生成好后在「全部项目」里点开查看。
            </p>
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                onClick={() => {
                  setShowModal(false);
                  onFinished?.();
                }}
                className="rounded-mk bg-mk-primary px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-primary-hover"
              >
                回到全部项目
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/* ---------- S5 · AI-use retrospective (复盘我与 AI 的互动) ---------- */

function recordLine(r: AIUseRecord): string {
  // cardsAccepted counts cards you engaged across every summon path (not only
  // AI-proposed ones), so keep the two counts separate rather than a ratio.
  return `印记陪你走的这一程：${r.coachTurns} 轮对话 · AI 提议 ${r.cardsProposed} 张卡（跳过 ${r.cardsDismissed}）· 你一共打开 ${r.cardsAccepted} 张卡 · 打开 ${r.sourcesOpened} 个来源 · 没有替你写正文、没有替你预测分数。`;
}

function AIUsePanel({ projectId, done }: { projectId: string; done: boolean }) {
  const [record, setRecord] = useState<AIUseRecord | null>(null);
  const [usedFor, setUsedFor] = useState("");
  const [notUsedFor, setNotUsedFor] = useState("");
  const [saved, setSaved] = useState(false);

  // Load the objective record + a seeded (or already-saved) draft on mount.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const d = await getAIUseDraft(projectId);
        if (cancelled) return;
        setRecord(d.record);
        setUsedFor(d.draft.usedFor);
        setNotUsedFor(d.draft.notUsedFor);
      } catch {
        /* leave empty; the section still lets the student write */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  async function save() {
    setSaved(false);
    try {
      await postAIUse(projectId, { usedFor, notUsedFor });
      setSaved(true);
    } catch {
      /* retries on next save */
    }
  }

  return (
    <section className="mt-8 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
      <h2 className="font-sans text-[15px] font-bold text-mk-ink">复盘我与 AI 的互动</h2>
      <p className="mt-1 text-[12.5px] leading-relaxed text-mk-muted">
        印记根据记录整理了你和 AI 的真实互动。下面两段是你的 AI 使用声明——初稿是印记帮你起的，话得你自己改，这部分不能让 AI 代写。
      </p>
      {record && (
        <p className="mt-3 rounded-mk border-l-2 border-mk-primary/40 bg-mk-bg px-3 py-2 text-[12.5px] leading-relaxed text-mk-muted">
          {recordLine(record)}
        </p>
      )}
      <div className="mt-4 flex flex-col gap-3">
        <label className="text-[13px] font-bold text-mk-ink">
          我用 AI 做了什么
          <textarea
            value={usedFor}
            onChange={(e) => setUsedFor(e.target.value)}
            disabled={done}
            rows={3}
            placeholder="例如：澄清检索词、核对来源功能、追问论证、检查过度概括……"
            className="mt-1.5 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-2 text-[13.5px] leading-relaxed text-mk-ink outline-none focus:border-mk-primary disabled:opacity-70"
          />
        </label>
        <label className="text-[13px] font-bold text-mk-ink">
          我明确没有用 AI 做什么
          <textarea
            value={notUsedFor}
            onChange={(e) => setNotUsedFor(e.target.value)}
            disabled={done}
            rows={3}
            placeholder="例如：代写正文、编造材料细节、预测分数、替我写反思……"
            className="mt-1.5 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-2 text-[13.5px] leading-relaxed text-mk-ink outline-none focus:border-mk-primary disabled:opacity-70"
          />
        </label>
      </div>
      {!done && (
        <div className="mt-3 flex items-center gap-3">
          <button
            type="button"
            onClick={() => void save()}
            className="rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white transition hover:bg-mk-primary-hover"
          >
            保存声明
          </button>
          {saved && <span className="text-[12px] font-semibold text-mk-green">已保存</span>}
        </div>
      )}
    </section>
  );
}

/* ---------- S5 · defense-readiness conversation (scope=reflection) ---------- */

type RevMsg = { role: "ai" | "student"; text: string };

function ReviewCoachThread({ projectId }: { projectId: string }) {
  const [chat, setChat] = useState<RevMsg[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const hist = await getCoachHistory(projectId, "reflection");
        if (!cancelled) setChat(hist.map((m) => ({ role: m.role === "ai" ? "ai" : "student", text: m.text })));
      } catch {
        /* empty thread */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  async function send() {
    const text = draft.trim();
    if (!text || sending) return;
    setChat((c) => [...c, { role: "student", text }]);
    setDraft("");
    setSending(true);
    try {
      const { reply } = await coach(projectId, "reflection", text);
      setChat((c) => [...c, { role: "ai", text: reply }]);
    } catch {
      setChat((c) => [...c, { role: "ai", text: "刚才没接上，再问我一次？" }]);
    } finally {
      setSending(false);
    }
  }

  return (
    <section className="mt-6 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
      <h2 className="font-sans text-[15px] font-bold text-mk-ink">答辩预演 · 让印记追问你</h2>
      <p className="mt-1 text-[12.5px] leading-relaxed text-mk-muted">
        回顾不是润色，是「经不经得起老师追问」。让印记像老师一样一次问一个——但答案得你自己给。
      </p>
      {chat.length > 0 && (
        <div className="mt-3 flex flex-col gap-2">
          {chat.map((m, i) => (
            <div key={i} className={`flex ${m.role === "ai" ? "justify-start" : "justify-end"}`}>
              <div
                className={`max-w-[88%] rounded-mk-lg px-3 py-2 text-[13px] leading-relaxed ${
                  m.role === "ai" ? "bg-mk-bg text-mk-ink" : "bg-mk-primary text-white"
                }`}
              >
                {m.text}
              </div>
            </div>
          ))}
        </div>
      )}
      <div className="mt-3 flex gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && void send()}
          placeholder="想让印记追问哪一处？"
          className="flex-1 rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-2 text-[13.5px] text-mk-ink outline-none focus:border-mk-primary"
        />
        <button
          type="button"
          onClick={() => void send()}
          disabled={sending}
          className="rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white transition hover:bg-mk-primary-hover disabled:opacity-50"
        >
          问印记
        </button>
      </div>
    </section>
  );
}

/* ---------- right · the "你的思维印记" mirror ---------- */

function MirrorPane({ projectId, hasProposal }: { projectId: string; hasProposal: boolean }) {
  const [mirror, setMirror] = useState<Mirror | null>(null);
  const [composing, setComposing] = useState(true);

  // GET the stored mirror; if none exists yet AND there's real work to mirror
  // (a non-empty proposal), POST once to compose it (first-open-wins on the
  // server). On a blank project we don't compose an empty mirror — an honest
  // "come back later" beats canned text.
  useEffect(() => {
    let cancelled = false;
    setMirror(null);
    setComposing(true);
    (async () => {
      try {
        let m = await getMirror(projectId);
        if (m == null && hasProposal) m = await postMirror(projectId);
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
  }, [projectId, hasProposal]);

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
          <p className="text-[12.5px] leading-relaxed text-mk-muted-2">
            {hasProposal ? "这份印记还没能整理出来——稍后重新打开回顾再看看。" : "完成一些工作后，这里会长出你的思维印记。"}
          </p>
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
