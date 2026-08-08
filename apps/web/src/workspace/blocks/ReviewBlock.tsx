import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Mirror, Proposal, ProjectStatus } from "@mind-imprint/contracts";
import { useStudioAiSlot } from "@/studio/ai/StudioAiSlot";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
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
import type { AIUseRecord, CardTurnRef } from "@mind-imprint/contracts";
import { CoachCardPanel } from "./CoachCardPanel";
import { CardTurnChip } from "./CardTurnChip";

// #21 · the 回顾 card shelf — review/reflection thinking cards the STUDENT may
// summon to look back on her own thinking (印记 supports, never writes her
// reflection). Mirrors FORMING_DECK / READING_DECK; server allowlist in
// card_persist.go's reflectionDeckCards.
const REFLECTION_DECK = ["learning-report", "metacognition", "knower-perspective"];

// The Review block: a mirror, not a report card. The student's own reflection
// leads (left, in <main>); the AI-assembled "你的思维印记" narrative sits
// alongside as support — portaled into the constant AiPanel (Task 4/6),
// same slot the active coach uses before she's done. The rubric/assessment
// runs quietly on 定稿并评估 and feeds the teacher/parent reports — it is NOT
// shown here as a grade.
//
// #20 · view-only lock: the room only unlocks once writing is finished
// (完成写作 in the writing room). Before that the prompts show as a read-only
// outline — nothing is enterable or generable.
//
// #21 · mutual reflection ordering: the AI's mirror comes AFTER the student
// finishes her OWN reflection. Two explicit steps replace the old single
// 完成回顾: (a) 「我写完了我的反思」 marks her reflection done (done=true) and only
// THEN composes the mirror; (b) 「定稿并开始评估」 (enabled once the mirror shows)
// finishes the project → assessment. While she fills the form, review/reflection
// cards support her (克制, 打开由学生确认) — 印记 never writes her reflection.
export function ReviewBlock({
  projectId,
  proposal,
  status,
  writingFinished,
  onFinished,
}: {
  projectId: string;
  proposal: Proposal;
  status: ProjectStatus;
  // #20 · gates the whole room. False → view-only outline; nothing composes.
  writingFinished: boolean;
  onFinished?: () => void;
}) {
  const [answers, setAnswers] = useState<string[]>(reflectionPrompts.map(() => ""));
  // done = the student marked HER reflection finished (reflection.done). It is
  // NOT "archived" — the point of no return is 定稿并评估 (finalize below).
  const [done, setDone] = useState(false);
  const [markingDone, setMarkingDone] = useState(false);
  const [markDoneError, setMarkDoneError] = useState<string | null>(null);
  // #21 · the mirror is READY (composed + on screen) — gates 定稿并评估 so the AI
  // reflects only after the student did, and she sees it before finalizing.
  const [mirrorReady, setMirrorReady] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState<string | null>(null);
  const [showModal, setShowModal] = useState(false);
  // #5 · 定稿并评估 is the point of no return: archiving locks 正文与回顾 and
  // generates the assessment. Confirm before running it.
  const [confirmFinish, setConfirmFinish] = useState(false);
  // archived = the finalize path has begun (evaluating) or completed (done).
  const archived = status === "evaluating" || status === "done";
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

  // #21 · step (a) 「我写完了我的反思」: persist answers with done=true. This does
  // NOT finish the project — it unlocks the mirror (the AI reflects only after
  // the student did). MirrorPane composes once canCompose flips true.
  async function finishReflection() {
    if (markingDone || done) return;
    setMarkingDone(true);
    setMarkDoneError(null);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    try {
      await putReflection(projectId, { answers: answersRef.current, done: true });
      setDone(true);
    } catch (e) {
      setMarkDoneError(
        e instanceof ApiError ? e.message || "还不能标记完成，请稍后再试。" : "刚才没接上，稍等再试一次。",
      );
    } finally {
      setMarkingDone(false);
    }
  }

  // #21 · step (b) 定稿并开始评估: the point of no return. finish is async (202) —
  // it kicks off the flagship process assessment in the background and returns
  // status "evaluating" immediately; we open a modal that sends the student back
  // to 全部项目 (where the report shows up later as "评估中" → "已完成"). Enabled
  // only after the mirror is on screen. A 422 means a server gate still isn't
  // satisfied (writing not finished / reflection not done) — surface it inline.
  async function finalizeAndEvaluate() {
    if (finishing) return;
    setFinishing(true);
    setFinishError(null);
    try {
      await finishProject(projectId);
      setShowModal(true);
    } catch (e) {
      setFinishError(
        e instanceof ApiError ? e.message || "还不能定稿，请稍后再试。" : "刚才没接上，稍等再试一次。",
      );
    } finally {
      setFinishing(false);
    }
  }

  // The room→panel contract (Task 4/6, spec §17): this room's WORK — the
  // reflection prompts + AI-use retrospective + finish actions — renders
  // directly below, in <main>; its COACH (chat, before she's done) and its
  // MIRROR (read-only "你的思维印记", after) both portal into the constant
  // AiPanel via `useStudioAiSlot`. `slot` is null when the panel is
  // collapsed or this component renders outside a studio shell (e.g. some
  // tests) — in either case the panel content simply doesn't render, never
  // crashes.
  const slot = useStudioAiSlot();

  return (
    <>
      {/* WORK — the student's own reflection + AI-use retrospective. */}
      <div className="h-full overflow-y-auto px-10 py-9">
        <div className="mx-auto max-w-2xl">
          <header className="mb-6">
            <p className="text-[12px] font-semibold uppercase tracking-[0.18em] text-mk-faint">项目收尾</p>
            <h1 className="mt-1 font-sans text-[26px] font-bold leading-tight text-mk-ink">回过头看看这一程</h1>
            <p className="mt-1.5 text-[14px] text-mk-muted">用你自己的话回答几个问题。右边是印记帮你整理的过程，卡壳时可以看看——但话得你自己说。</p>
          </header>

          {/* #20 · view-only lock — the room only unlocks once writing is finished.
              A calm state label, no nav: 印记 cues 完成写作 in the chat. */}
          {!writingFinished && (
            <div className="mb-6 flex items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-paper px-4 py-3">
              <Icon name="writing" size={16} />
              <p className="flex-1 text-[14px] font-semibold text-mk-muted">写完初稿后，这里会解锁回顾。</p>
            </div>
          )}

          <div className="flex flex-col gap-6">
            {reflectionPrompts.map((p, i) => (
              <div key={i}>
                <div className="flex items-center gap-2">
                  <span className="flex h-6 w-6 flex-none items-center justify-center rounded-full bg-mk-accent-50 text-[12px] font-bold text-mk-accent">{i + 1}</span>
                  <span className="rounded-full bg-mk-paper px-2 py-0.5 text-[12px] font-bold text-mk-muted">{p.label}</span>
                </div>
                <p className="mt-1.5 text-[14.5px] font-bold leading-snug text-mk-ink">{p.q}</p>
                {p.anchor === "goal" && proposal.objective && (
                  <p className="mt-1.5 rounded-mk-md border-l-2 border-mk-butter bg-mk-butter-bg px-3 py-2 text-[14px] leading-relaxed text-mk-muted">
                    <span className="font-bold text-mk-butter-fg">开题时你写的目标 · </span>{proposal.objective}
                  </p>
                )}
                <textarea
                  value={answers[i]}
                  onChange={(e) => editAnswer(i, e.target.value)}
                  disabled={!writingFinished || done}
                  rows={4}
                  placeholder={writingFinished ? "写下你的想法……" : "完成写作后在这里回顾"}
                  className="mt-2 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3 text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent disabled:opacity-60"
                />
              </div>
            ))}
          </div>

          {/* Everything below only lives once the room is unlocked — no AI-use
              seed, no coach thread, no cards, no finish path before 完成写作. */}
          {writingFinished && (
            <>
              {/* S5 · 复盘我与 AI 的互动 — the objective record + the student's own statement */}
              <AIUsePanel projectId={projectId} done={done} />

              {/* Q4 followup (2026-08): 「印记陪你把回顾写完」moved OUT of this bottom
                  spot and INTO the AI panel (see the portal below) — it's the
                  ACTIVE coach while she's filling the form, not a buried footnote. */}

              <div className="mt-7 flex flex-wrap items-center gap-4">
                {archived ? (
                  <>
                    <span className="text-[14px] font-semibold text-mk-success">已归档 · 过程评估正在生成，稍后可在「全部项目」里点开查看</span>
                    {onFinished && (
                      <button
                        type="button"
                        onClick={onFinished}
                        className="rounded-mk-md bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-600"
                      >
                        回到全部项目 →
                      </button>
                    )}
                  </>
                ) : !done ? (
                  // #21 · step (a): the student marks HER reflection done → THEN the
                  // AI reflects (mirror composes). No assessment yet.
                  <>
                    <button
                      type="button"
                      onClick={() => void finishReflection()}
                      disabled={markingDone}
                      className="rounded-mk-md bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-600 disabled:opacity-50"
                    >
                      {markingDone ? "记录中……" : "我写完了我的反思"}
                    </button>
                    {markDoneError ? (
                      <span className="text-[14px] font-semibold text-mk-danger">{markDoneError}</span>
                    ) : (
                      <span className="text-[14px] text-mk-faint">写完后，印记才会照着你的全过程给你一面镜子——然后你再定稿评估。</span>
                    )}
                  </>
                ) : !mirrorReady ? (
                  <span className="text-[14px] font-semibold text-mk-accent">印记正在照镜子……看看右侧的思维印记，然后就可以定稿评估。</span>
                ) : (
                  // #21 · step (b): the mirror is on screen → finalize & assess.
                  <>
                    <button
                      type="button"
                      onClick={() => setConfirmFinish(true)}
                      disabled={finishing}
                      className="rounded-mk-md bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-600 disabled:opacity-50"
                    >
                      {finishing ? "定稿中……" : "定稿并开始评估"}
                    </button>
                    {finishError ? (
                      <span className="text-[14px] font-semibold text-mk-danger">{finishError}</span>
                    ) : (
                      <span className="text-[14px] text-mk-faint">定稿后会生成过程评估，记入成长报告（老师 / 家长可见），这里不打分。</span>
                    )}
                  </>
                )}
              </div>
            </>
          )}
        </div>
      </div>

      {/* AI PANEL — Q4 followup (2026-08) + Task 6: portaled into the constant
          AiPanel (Task 4), on the shared `ChatLog`/`Composer` (Task 5's
          pattern). While she's editing, the panel is the ACTIVE coach
          (「印记陪你把回顾写完」+ the reflection card shelf + 问印记 chat), so
          help is visible the whole time. Once she marks her OWN reflection
          done (#21 mutual-reflection order — unchanged), the panel becomes
          the mirror ("你的思维印记"), which composes only then. Before 完成写作
          unlocks the room, the mirror's own locked placeholder shows instead
          (nothing to coach yet). */}
      {slot &&
        createPortal(
          writingFinished && !done && !archived ? (
            <ReviewCoachThread projectId={projectId} locked={done} />
          ) : (
            <MirrorPane
              projectId={projectId}
              canCompose={writingFinished && (done || archived)}
              onReady={setMirrorReady}
            />
          ),
          slot,
        )}

      {/* #5 · 定稿并评估 confirm — the point of no return. Archiving locks 正文与回顾
          and generates the process assessment. */}
      {confirmFinish && !archived && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-mk-lg">
            <h2 className="font-sans text-[18px] font-bold text-mk-ink">定稿并开始评估？</h2>
            <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
              定稿后，<span className="font-bold text-mk-ink">正文与回顾都会锁定、无法再修改</span>，印记会据此生成过程评估（记入成长报告，这里不打分）。确定定稿吗？
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setConfirmFinish(false)}
                className="rounded-mk-md border border-mk-border px-4 py-2 text-[14px] font-semibold text-mk-muted hover:text-mk-ink"
              >
                再看看
              </button>
              <button
                type="button"
                onClick={() => { setConfirmFinish(false); void finalizeAndEvaluate(); }}
                className="rounded-mk-md bg-mk-accent px-5 py-2 text-[14px] font-bold text-white transition hover:bg-mk-accent-600"
              >
                定稿并评估
              </button>
            </div>
          </div>
        </div>
      )}

      {/* finish modal · the assessment runs in the background; the student is
          free to leave. One action returns to 全部项目, where the row shows
          "评估中" and later "已完成 · 查看评估报告". */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-mk-lg">
            <div className="flex items-center gap-2 text-mk-accent">
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
                className="rounded-mk-md bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-600"
              >
                回到全部项目
              </button>
            </div>
          </div>
        </div>
      )}
    </>
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
      <p className="mt-1 text-[14px] leading-relaxed text-mk-muted">
        印记根据记录整理了你和 AI 的真实互动。下面两段是你的 AI 使用声明——初稿是印记帮你起的，话得你自己改，这部分不能让 AI 代写。
      </p>
      {record && (
        <p className="mt-3 rounded-mk-md border-l-2 border-mk-accent/40 bg-mk-paper px-3 py-2 text-[14px] leading-relaxed text-mk-muted">
          {recordLine(record)}
        </p>
      )}
      <div className="mt-4 flex flex-col gap-3">
        <label className="text-[14px] font-bold text-mk-ink">
          我用 AI 做了什么
          <textarea
            value={usedFor}
            onChange={(e) => setUsedFor(e.target.value)}
            disabled={done}
            rows={3}
            placeholder="例如：澄清检索词、核对来源功能、追问论证、检查过度概括……"
            className="mt-1.5 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-2 text-[14px] leading-relaxed text-mk-ink outline-none focus:border-mk-accent disabled:opacity-70"
          />
        </label>
        <label className="text-[14px] font-bold text-mk-ink">
          我明确没有用 AI 做什么
          <textarea
            value={notUsedFor}
            onChange={(e) => setNotUsedFor(e.target.value)}
            disabled={done}
            rows={3}
            placeholder="例如：代写正文、编造材料细节、预测分数、替我写反思……"
            className="mt-1.5 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-2 text-[14px] leading-relaxed text-mk-ink outline-none focus:border-mk-accent disabled:opacity-70"
          />
        </label>
      </div>
      {!done && (
        <div className="mt-3 flex items-center gap-3">
          <button
            type="button"
            onClick={() => void save()}
            className="rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white transition hover:bg-mk-accent-600"
          >
            保存声明
          </button>
          {saved && <span className="text-[12px] font-semibold text-mk-success">已保存</span>}
        </div>
      )}
    </section>
  );
}

/* ---------- S5 · supportive reflection-completion conversation (scope=reflection) ---------- */

// `card`, when set, marks a card-turn: rendered as a content-first clickable
// chip opening a read-only view of the student's answers (not raw text).
type RevMsg = { role: "ai" | "student"; text: string; card?: CardTurnRef | null };

// ReviewCoachThread's own `RevMsg[]` history → the shared ChatLog's
// `ChatMessage[]`. A card-turn (msg.card set) maps to a "system"-role message
// carrying only `node` — ChatLog's system row has no bubble
// background/padding, letting `CardTurnChip` be the whole message (its own
// content-first accent chip, never raw compiled text) instead of nesting
// inside a second bubble. Plain turns keep their text as-is (no **bold**
// markup in this thread, unlike PlanBlock's forming chat).
function toChatMessages(chat: RevMsg[]): ChatMessage[] {
  return chat.map((m, i) =>
    m.card
      ? { id: String(i), role: "system", node: <CardTurnChip card={m.card} /> }
      : { id: String(i), role: m.role === "ai" ? "assistant" : "student", text: m.text },
  );
}

function ReviewCoachThread({ projectId, locked }: { projectId: string; locked: boolean }) {
  const [chat, setChat] = useState<RevMsg[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const page = await getCoachHistory(projectId, "reflection");
        if (!cancelled) setChat(page.messages.map((m) => ({ role: m.role === "ai" ? "ai" : "student", text: m.text, card: m.card })));
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
      const result = await coach(projectId, text, "reflection");
      setChat((c) => [...c, { role: "ai", text: result.narrate }]);
    } catch {
      setChat((c) => [...c, { role: "ai", text: "刚才没接上，再问我一次？" }]);
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <header className="flex-none">
        <div className="flex items-center gap-2 text-mk-accent">
          <Icon name="spark" size={16} />
          <h2 className="font-sans text-[15px] font-bold">印记陪你把回顾写完</h2>
        </div>
        <p className="mt-1 text-[12px] text-mk-faint">
          卡在哪一块不知道怎么写，都可以跟印记说说。它一次只问一个问题，帮你想起细节、找到词——但话得你自己写。挑一张回顾卡也能帮你想清楚。
        </p>
      </header>

      <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pr-1">
        <ChatLog messages={toChatMessages(chat)} thinking={sending} />
        {/* #21 · the reflection card shelf. Reflecting on a card runs a coach turn
            (reflectProjectCard, surface="reflection") whose student turn + AI
            reply drop into this same thread. Hidden once the student marks her
            reflection done (locked).
            Task 9a (2026-08-07): this coach is now context-isolated from the
            studio orchestrator, so its coach() reply no longer carries a
            proposal — the AI-proposed card chip is retired on this path for P1;
            proposal stays null and only the self-summon shelf renders. */}
        {!locked && !sending && (
          <CoachCardPanel
            projectId={projectId}
            proposal={null}
            onProposalConsumed={() => {}}
            onReflected={(studentText, reply, card) =>
              setChat((c) => [
                ...c,
                // Card turn → content-first chip; fall back to raw text if the
                // server didn't echo a card.
                ...(card
                  ? [{ role: "student" as const, text: studentText, card }]
                  : studentText
                    ? [{ role: "student" as const, text: studentText }]
                    : []),
                ...(reply ? [{ role: "ai" as const, text: reply }] : []),
              ])
            }
            surface="reflection"
            deck={REFLECTION_DECK}
          />
        )}
      </div>

      {!locked && (
        <Composer
          value={draft}
          onChange={setDraft}
          onSend={() => void send()}
          state={sending ? "replying" : undefined}
          placeholder="卡在哪一题？说说你的初步想法，我陪你理清——但话得你自己写"
          className="flex-none"
        />
      )}
    </div>
  );
}

/* ---------- AI panel · the "你的思维印记" mirror ---------- */

function MirrorPane({
  projectId,
  canCompose,
  onReady,
}: {
  projectId: string;
  // #21 · the mirror composes ONLY after the student finished her own reflection.
  // While false, the pane shows a "the mirror comes after you reflect" note and
  // never fetches/composes — the AI reflects second, not first.
  canCompose: boolean;
  onReady: (ready: boolean) => void;
}) {
  const [mirror, setMirror] = useState<Mirror | null>(null);
  const [composing, setComposing] = useState(false);

  // Once canCompose flips true (the student marked her reflection done), GET the
  // stored mirror; if none exists yet, POST once to compose it (first-open-wins
  // on the server). onReady(true) once a mirror is on screen — that gates the
  // 定稿并评估 step so she reflects first and sees the mirror before finalizing.
  useEffect(() => {
    if (!canCompose) {
      setMirror(null);
      setComposing(false);
      onReady(false);
      return;
    }
    let cancelled = false;
    setComposing(true);
    (async () => {
      try {
        let m = await getMirror(projectId);
        if (m == null) m = await postMirror(projectId);
        if (!cancelled) {
          setMirror(m);
          onReady(m != null);
        }
      } catch {
        /* leave the empty state; a reopen retries */
      } finally {
        if (!cancelled) setComposing(false);
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, canCompose]);

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <header className="flex-none">
        <div className="flex items-center gap-2 text-mk-accent">
          <Icon name="spark" size={16} />
          <h2 className="font-sans text-[15px] font-bold">你的思维印记</h2>
        </div>
        <p className="mt-1 text-[12px] text-mk-faint">印记根据你的全过程整理，供你参考——不是评分。</p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        {!canCompose ? (
          <p className="text-[14px] leading-relaxed text-mk-faint">先写下你自己的反思，点「我写完了我的反思」，印记才会照着你的全过程给你一面镜子——你先说，AI 后照。</p>
        ) : composing && !mirror ? (
          <p className="text-[14px] leading-relaxed text-mk-faint">印记正在回看你的全过程，整理这份思维印记……</p>
        ) : !mirror ? (
          <p className="text-[14px] leading-relaxed text-mk-faint">
            这份印记还没能整理出来——稍后重新打开回顾再看看。
          </p>
        ) : (
          <>
            <div className="flex flex-col gap-4">
              {mirror.sections.map((s, i) => (
                <div key={i} className="border-l-2 border-mk-accent/30 pl-3">
                  <p className="text-[12px] font-bold text-mk-accent">{s.title}</p>
                  <p className="mt-1 text-[14px] leading-relaxed text-mk-ink">{s.body}</p>
                </div>
              ))}
            </div>

            {mirror.carryForwards.length > 0 && (
              <div className="mt-6 rounded-mk-lg border border-mk-butter/40 bg-mk-butter-bg p-4">
                <p className="mb-2 flex items-center gap-1.5 text-[12px] font-bold text-mk-butter-fg">
                  <Icon name="arrow" size={14} /> 带走这两点
                </p>
                <ul className="flex flex-col gap-2">
                  {mirror.carryForwards.map((c, i) => (
                    <li key={i} className="text-[14px] leading-relaxed text-mk-ink">· {c}</li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
