import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft, CornerDownRight, Send } from "lucide-react";
import { Icon, Pebble } from "@/ui";
import { ApiError } from "../api/client";
import { listProjects, projectTitle, type Project } from "../api/projects";
import {
  closeSession,
  getPlan,
  getThread,
  listSessions,
  openSession,
  postTurn,
  resolveChange,
  approvePlan,
  sessionTrail,
  SESSION_KIND_LABELS,
  SESSION_REQUIRED_FIELD,
  SESSION_WRITEBACK_PROMPT,
  type PlanResolution,
  type PlanState,
  type Session,
  type SessionKind,
  type ThreadMessage,
} from "../api/projectRoom";
import {
  acceptTool,
  listTools,
  resolveTool,
  type ToolInstance,
} from "../api/tools";
import { navigate } from "../routing";
import { WorkPanel } from "./WorkPanel";
import { ToolInvite } from "./tools/ToolInvite";
import { apiErrorText } from "../api/errorText";

/**
 * ProjectRoom — the workbench.
 *
 * Cowork's shape: the conversation on the left, the thing being worked on the
 * right. The panel is the plan today; artifacts and tools land in the same slot
 * when they exist (see 占位 at the foot of this file).
 *
 * ONE DELIBERATE CHOICE, stated because it is arguable: entering a session
 * REPLACES the thread rather than splitting the view. A dig is meant to have
 * her whole attention — showing the project alongside it invites her to keep
 * half an eye on everything, which is the thing digging exists to escape. The
 * breadcrumb is how she knows where she is and how she gets back.
 */
export function ProjectRoom({ projectId }: { projectId: string }) {
  const [project, setProject] = useState<Project | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [thread, setThread] = useState<ThreadMessage[]>([]);
  const [plan, setPlan] = useState<PlanState>({ plan: null, pending: [] });
  const [tools, setTools] = useState<ToolInstance[]>([]);
  const [openTool, setOpenTool] = useState<string | null>(null);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  // 印记正在想。和 busy 分开：busy 只是"别重复点"，这个是要显示给她看的。
  const [thinking, setThinking] = useState(false);
  // 🚨 她刚发出去、还没落库的那句话。
  //
  // 消息是模型答完之后才和印记的回复一起写库的（一个事务，半条消息的对话是
  // 读不通的）。可那意味着等待的十几秒里，她自己说的话在屏幕上根本不存在——
  // 只有三个点在转。先在本地把它显示出来。
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const trail = useMemo(() => sessionTrail(sessions, activeSession), [sessions, activeSession]);
  const current = trail.at(-1) ?? null;

  const refreshThread = useCallback(
    async (sessionId: string | null) => {
      setThread(await getThread(projectId, sessionId ?? undefined));
    },
    [projectId],
  );

  useEffect(() => {
    let cancelled = false;
    async function boot() {
      try {
        const [projects, ss, pl, ts] = await Promise.all([
          listProjects(),
          listSessions(projectId),
          getPlan(projectId),
          listTools(projectId),
        ]);
        if (cancelled) return;
        const mine = projects.find((p) => p.id === projectId) ?? null;
        setProject(mine);
        setSessions(ss);
        setPlan(pl);
        setTools(ts);
        const msgs = await getThread(projectId);
        if (cancelled) return;
        setThread(msgs);

        // 🚨 她在大输入框里写的那句话，就是她对印记说的第一句话。
        //
        // 以前建完项目就把她扔进一个空房间，印记一声不吭——她刚说完一件事，
        // 对面没有任何反应。这里补上：线程是空的就把那句话当作第一轮发出去。
        //
        // 只在空线程时补。turn 的两条消息是在模型成功之后同一个事务里写的，
        // 所以模型失败时线程仍然是空的，刷新一次会自动再试，不会重复。
        if (msgs.length === 0 && mine?.idea.trim()) {
          setPending(mine.idea.trim());
          setThinking(true);
          try {
            await postTurn(projectId, mine.idea.trim());
            if (cancelled) return;
            setThread(await getThread(projectId));
            setTools(await listTools(projectId));
          } catch (err) {
            if (!cancelled) setError(apiErrorText(err));
          } finally {
            if (!cancelled) {
              setThinking(false);
              setPending(null);
            }
          }
        }
      } catch (err) {
        if (!cancelled) setError(apiErrorText(err));
      }
    }
    void boot();
    return () => {
      cancelled = true;
    };
  }, [projectId, refreshThread]);

  async function send() {
    const text = draft.trim();
    if (!text || busy) return;
    setBusy(true);
    setThinking(true);
    setPending(text);
    setError(null);
    try {
      const res = await postTurn(projectId, text, activeSession ?? undefined);
      setDraft("");
      await refreshThread(activeSession);
      // 印记递了一件工具就把列表拉一遍，那张邀请卡才会出现在对话末尾。
      if (res.toolId) setTools(await listTools(projectId));
    } catch (err) {
      // 印记 failing is surfaced, never smoothed into a plausible sentence.
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
      setPending(null);
    }
  }

  async function dig(kind: SessionKind, question: string, anchorRef: string) {
    setBusy(true);
    setError(null);
    try {
      const s = await openSession(projectId, {
        kind,
        question,
        parentId: activeSession ?? undefined,
        anchorKind: "hook",
        anchorRef,
      });
      setSessions((prev) => [...prev, s]);
      setActiveSession(s.id);
      await refreshThread(s.id);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }

  async function goTo(sessionId: string | null) {
    setActiveSession(sessionId);
    setError(null);
    await refreshThread(sessionId);
  }

  async function finish(takeaway: string, field: string | null) {
    if (!current) return;
    setBusy(true);
    setError(null);
    try {
      const body = field ? { writeBack: { [field]: takeaway } } : { takeaway };
      const closed = await closeSession(projectId, current.id, body);
      setSessions((prev) => prev.map((s) => (s.id === closed.id ? closed : s)));
      await goTo(current.parentId);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }

  async function onResolve(id: string, resolution: PlanResolution, reason: string) {
    await resolveChange(projectId, id, resolution, reason);
    setPlan(await getPlan(projectId));
    await refreshThread(activeSession);
  }

  async function onApprove(versionId: string) {
    await approvePlan(projectId, versionId);
    setPlan(await getPlan(projectId));
  }

  /* ── 工具 ─────────────────────────────────────────────────────────────
   *
   * 打开一件当场做的工具，右边就切过去；打开一件出门做的，什么也不弹——她
   * 要走了，弹一个面板给她看没有意义。
   */

  async function openToolInstance(t: ToolInstance) {
    setBusy(true);
    setError(null);
    try {
      const got = await acceptTool(projectId, t.id);
      setTools((prev) => prev.map((x) => (x.id === got.id ? got : x)));
      if (got.kind === "thinking") setOpenTool(got.id);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }

  async function declineToolInstance(t: ToolInstance, note: string) {
    setBusy(true);
    try {
      const got = await resolveTool(projectId, t.id, { status: "declined", note });
      setTools((prev) => prev.map((x) => (x.id === got.id ? got : x)));
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }

  /**
   * 收工。
   *
   * 结果落库，然后把她那句话当成她的发言送回对话——因为那本来就是她的话。
   * 印记接着往下说，工具就不是一个做完就沉底的表单，而是对话的一部分。
   */
  async function finishToolInstance(t: ToolInstance, result: unknown, note: string) {
    setBusy(true);
    setError(null);
    try {
      // note 只放她自己写下的那句话，一个字不改。剩下的（贴了几张便签、分了
      // 几块）印记自己去看，不用我们替她讲一遍。
      const got = await resolveTool(projectId, t.id, { status: "done", result, note: note.trim() });
      setTools((prev) => prev.map((x) => (x.id === got.id ? got : x)));
      setOpenTool(null);
      await refreshThread(activeSession);
      // 空文本的一轮：她没说话，是刚做完一件事，印记该接一句。
      setThinking(true);
      await postTurn(projectId, "", activeSession ?? undefined);
      await refreshThread(activeSession);
      setPlan(await getPlan(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
    }
  }

  // 刚递出来、她还没表态的。
  const invites = tools.filter((t) => t.status === "summoned");

  return (
    <div className="flex h-full min-h-0">
      {/* ── conversation ─────────────────────────────────────────────── */}
      <section className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center gap-3 border-b border-mk-border px-5 py-3">
          <button
            type="button"
            onClick={() => navigate("/projects")}
            aria-label="回到项目"
            className="text-mk-secondary"
          >
            <Icon icon={ArrowLeft} size={18} />
          </button>
          <span className="truncate text-mk-body font-semibold text-mk-ink">
            {project ? projectTitle(project) : "项目"}
          </span>
        </header>

        {trail.length > 0 && (
          <Breadcrumb trail={trail} onGo={(id) => void goTo(id)} />
        )}

        <div className="flex-1 overflow-y-auto px-5 py-4">
          <div className="mx-auto flex max-w-[640px] flex-col gap-4">
            {thread.length === 0 && !thinking && (
              <div className="flex flex-col items-center gap-3 py-10 text-center">
                <Pebble state="idle" size={44} />
                <p className="text-mk-body text-mk-secondary">
                  {current ? "这一层还没开始。把你想到的写下来。" : "印记在这儿。说说你想做的这件事。"}
                </p>
              </div>
            )}
            {thread.map((m) => (
              <Message key={m.seq} m={m} onDig={(k, q) => void dig(k, q, String(m.seq))} busy={busy} />
            ))}
            {/* 她刚发出去的那句。落库之前就先显示，别让她对着三个点等。 */}
            {pending && (
              <div className="flex justify-end">
                <div className="inline-block max-w-[85%] rounded-[13px_4px_13px_13px] bg-mk-accent-50 px-4 py-3 text-mk-body text-mk-ink">
                  {pending}
                </div>
              </div>
            )}

            {/* 印记正在想。她刚说完话，对面要有反应。 */}
            {thinking && (
              <div className="flex items-start gap-2" aria-label="思考中">
                <span className="mt-0.5 shrink-0">
                  <Pebble state="thinking" size={24} />
                </span>
                <div className="inline-flex items-center gap-1 rounded-[4px_13px_13px_13px] bg-mk-surface px-4 py-3 shadow-mk-xs">
                  <span className="mk-think-dot" />
                  <span className="mk-think-dot [animation-delay:0.15s]" />
                  <span className="mk-think-dot [animation-delay:0.3s]" />
                </div>
              </div>
            )}

            {/* 递到手边的工具。放在对话末尾，因为它是印记刚说的话的一部分。 */}
            {invites.map((t) => (
              <ToolInvite
                key={t.id}
                tool={t}
                busy={busy}
                onOpen={() => void openToolInstance(t)}
                onDecline={(note) => void declineToolInstance(t, note)}
              />
            ))}
          </div>
        </div>

        {current && !current.closedAt && (
          <CloseSession
            kind={current.kind}
            question={current.question}
            busy={busy}
            onFinish={(text) => void finish(text, SESSION_REQUIRED_FIELD[current.kind])}
          />
        )}

        <footer className="border-t border-mk-border px-5 py-3">
          {error && (
            <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
              {error}
            </p>
          )}
          <div className="mx-auto flex max-w-[640px] items-end gap-2">
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              rows={2}
              disabled={busy}
              placeholder="请输入"
              className="flex-1 resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
            <button
              type="button"
              onClick={() => void send()}
              disabled={!draft.trim() || busy}
              aria-label="发送"
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              <Icon icon={Send} size={16} />
            </button>
          </div>
        </footer>
      </section>

      {/* ── panel ────────────────────────────────────────────────────── */}
      <aside className="hidden w-[360px] shrink-0 border-l border-mk-border lg:block">
        <WorkPanel
          projectId={projectId}
          plan={plan}
          tools={tools}
          openTool={openTool}
          busy={busy}
          onSelectTool={setOpenTool}
          onFinishTool={(t, result, summary) => void finishToolInstance(t, result, summary)}
          onBackFromAway={(t) => setOpenTool(t.id)}
          onResolve={onResolve}
          onApprove={onApprove}
        />
      </aside>
    </div>
  );
}

function Breadcrumb({ trail, onGo }: { trail: Session[]; onGo: (id: string | null) => void }) {
  return (
    <nav className="flex flex-wrap items-center gap-1 border-b border-mk-border bg-mk-paper px-5 py-2 text-mk-small">
      <button type="button" onClick={() => onGo(null)} className="text-mk-secondary underline">
        项目
      </button>
      {trail.map((s, i) => (
        <span key={s.id} className="flex items-center gap-1">
          <span className="text-mk-faint">›</span>
          {i === trail.length - 1 ? (
            <span className="text-mk-ink">{SESSION_KIND_LABELS[s.kind] ?? s.kind}</span>
          ) : (
            <button type="button" onClick={() => onGo(s.id)} className="text-mk-secondary underline">
              {SESSION_KIND_LABELS[s.kind] ?? s.kind}
            </button>
          )}
        </span>
      ))}
    </nav>
  );
}

function Message({
  m,
  onDig,
  busy,
}: {
  m: ThreadMessage;
  onDig: (kind: SessionKind, question: string) => void;
  busy: boolean;
}) {
  if (m.role === "system") {
    // A conclusion that came back from a closed dig. It reads as a marker, not
    // as someone speaking — because nobody said it, she concluded it.
    return (
      <div className="flex items-start gap-2 rounded-mk-md bg-mk-paper px-3 py-2">
        <Icon icon={CornerDownRight} size={14} className="mt-0.5 text-mk-faint" />
        <p className="text-mk-small text-mk-secondary">{m.content}</p>
      </div>
    );
  }

  const mine = m.role === "student";
  const hook = m.payload?.kind === "hook" ? m.payload : null;

  // 🚨 形状和 token 跟阅读室一模一样（ReadingCoachPanel 的 BUBBLE / AI_RADIUS /
  // HER_RADIUS）。之前这里自己写了一套：印记说的话没有头像也没有框，看上去
  // 不像有人在说话。同一个印记在三个房间里应该长成同一个样子。
  if (mine) {
    return (
      <div className="flex justify-end">
        <div className="inline-block max-w-[85%] rounded-[13px_4px_13px_13px] bg-mk-accent-50 px-4 py-3 text-mk-body text-mk-ink">
          {m.content}
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-start justify-start gap-2">
      <span className="mt-0.5 shrink-0">
        <Pebble state="idle" size={24} />
      </span>
      <div className="min-w-0">
        <div className="inline-block max-w-[85%] rounded-[4px_13px_13px_13px] bg-mk-surface px-4 py-3 text-mk-body text-mk-ink shadow-mk-xs">
          {m.content}
        </div>
        {hook && (
          // A hook is an invitation, never an interruption: she taps it or she
          // does not, and the conversation carries on either way (铁律②).
          <button
            type="button"
            disabled={busy}
            onClick={() => onDig(hook.hookKind, hook.hook)}
            className="mt-2 flex items-start gap-1.5 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-left text-mk-small text-mk-secondary disabled:opacity-50"
          >
            <Icon icon={CornerDownRight} size={14} className="mt-0.5 shrink-0 text-mk-accent-500" />
            <span>
              <span className="font-semibold text-mk-ink">
                {SESSION_KIND_LABELS[hook.hookKind] ?? "深挖一层"}
              </span>
              <span className="ml-1">{hook.hook}</span>
            </span>
          </button>
        )}
      </div>
    </div>
  );
}

/**
 * CloseSession — the write-back, inline at the foot of the dig.
 *
 * Inline rather than a modal on purpose: this is the natural end of the
 * conversation she is having, not an interruption to it. The button stays
 * disabled until she has written something, and the server refuses an empty
 * write-back regardless — a gate that lives only in a button is decoration.
 */
function CloseSession({
  kind,
  question,
  busy,
  onFinish,
}: {
  kind: SessionKind;
  question: string;
  busy: boolean;
  onFinish: (text: string) => void;
}) {
  const [text, setText] = useState("");
  return (
    <div className="border-t border-mk-border bg-mk-paper px-5 py-3">
      <div className="mx-auto max-w-[640px]">
        {question && <p className="mb-2 text-mk-small text-mk-muted">这一层在想：{question}</p>}
        <div className="flex items-end gap-2">
          <input
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={SESSION_WRITEBACK_PROMPT[kind]}
            className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
          <button
            type="button"
            onClick={() => onFinish(text.trim())}
            disabled={!text.trim() || busy}
            className="shrink-0 rounded-mk-full border border-mk-border px-4 py-2 text-mk-body text-mk-secondary disabled:opacity-40"
          >
            收起这一层
          </button>
        </div>
      </div>
    </div>
  );
}

/* ── 占位 ──────────────────────────────────────────────────────────────────
 *
 * Not built, waiting on the product owner's interaction design:
 *
 *   · 工具箱 — the server tool `summon_tool` and `pbl_tool_instance` exist as
 *     an endpoint; what a tool LOOKS like is the open question (spec §13).
 *     The rejected thing was the form, not the card.
 *   · 成果 (artifacts) — options / draft / spec / image / site. They share the
 *     panel slot with the plan, so they need a panel switcher, not a new
 *     surface.
 *   · brainstorming room · reframe card · research hint — the three named in
 *     the 2026-09-01 brief.
 *
 * All of them arrive as new `atom_message.payload` kinds and a new panel view.
 * Nothing here has to be rebuilt to admit them. — 2026-09-01
 * ------------------------------------------------------------------------ */
