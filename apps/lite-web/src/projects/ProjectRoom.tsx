import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { AwayCard, ToolInvite } from "./tools/ToolInvite";
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
  // 开场那一轮发过没有。见下面 seeded.current 那一处。
  const seeded = useRef(false);
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

  /**
   * 对话滚到最新的一句。
   *
   * 🚨 房间原来根本没有滚动这回事：一个聊了十几轮的项目打开时停在 scrollTop=0，
   * 她看到的是自己最开始那句话，得往下拖一千多像素才找得到进度——连印记刚递
   * 给她的那张邀请卡也在那下面（2026-09-02 线上实测：2111px 的对话停在顶部）。
   */
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [thread, pending, thinking, tools]);

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
        // 🚨 开场那一轮只发一次。
        //
        // 判空条件是"线程是空的"，可两个请求可以同时看到空线程：StrictMode
        // 会把挂载跑两遍，而 cancelled 只拦得住 setState，拦不住已经飞出去的
        // POST。两个都写一条"她说的话"+一条印记的回话，她就会在屏幕上看见
        // 自己那句话出现两遍、三遍——2026-09-02 的手机截图上正是三遍。
        //
        // 和复盘那条竞态是同一回事，也用同一个办法：一个同步的 ref 闸。
        if (msgs.length === 0 && mine?.idea.trim() && !seeded.current) {
          seeded.current = true;
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
      // 🚨 印记也可能在这一轮**出了一份计划**。不拉一遍，右边那栏会一直写着
      // 「计划待生成」，而计划其实已经存好了——2026-09-02 线上实测：印记在
      // 对话里说「就按你定下来的问题来安排」，面板纹丝不动，她只有刷新整页
      // 才看得见。计划、决定、结构、分工都从这一条路上来。
      setPlan(await getPlan(projectId));
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
      // 🚨 支线由印记先开口（产品负责人 2026-09-02）。空文本的一轮：她还没说
      // 话，是这条支线刚开，印记要接住她上面说的那件事，把这一层要看什么讲
      // 清楚。让她一进来面对一个空房间，等于把"深挖"变成了又一个输入框。
      setThinking(true);
      await postTurn(projectId, "", s.id);
      await refreshThread(s.id);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
    }
  }

  /**
   * 进入一条**已经存在**的支线（服务端开的），印记先开口。
   *
   * 和 dig 的区别只有一个：那边由前端开支线，这边支线已经开好了——审核的
   * 「问问这一句」和长期迭代的「深入讨论」都是服务端在一个请求里连支线一起
   * 建好的，前端要做的只是把她带过去。
   */
  async function enterSession(sessionId: string) {
    setBusy(true);
    setError(null);
    try {
      // 支线是服务端刚建的，本地这份列表里还没有它，面包屑会找不到路。
      setSessions(await listSessions(projectId));
      setActiveSession(sessionId);
      // 面板让开：她要去聊了，不是还在填这一屏。
      setOpenTool(null);
      await refreshThread(sessionId);
      setThinking(true);
      await postTurn(projectId, "", sessionId);
      await refreshThread(sessionId);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
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
  // 她答应了、人出门去做的那几件。留在对话里她当初答应的那个位置，因为那就是
  // 她记得的地方；右侧面板不为它单开一档（产品负责人 2026-09-02）。
  const away = tools.filter((t) => t.kind === "world" && t.status === "accepted");

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

        <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 py-4">
          <div className="mx-auto flex max-w-[640px] flex-col gap-4">
            {thread.length === 0 && !thinking && (
              <div className="flex flex-col items-center gap-3 py-10 text-center">
                <Pebble state="idle" size={44} />
                {/* 印记永远先开口（D1），所以这一屏只在那一轮没成功时才出现。
                    上面的红字会说明原因，这里不再假装是在邀请她开始。 */}
                <p className="text-mk-body text-mk-secondary">对话还没有开始</p>
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
            {away.map((t) => (
              <AwayCard
                key={t.id}
                tool={t}
                busy={busy}
                onBack={() => setOpenTool(t.id)}
              />
            ))}
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
              // 🚨 回车原来只是插一个换行：她打完一句按回车，什么也没发生，
              // 光标掉到第二行。Shift+回车 留给真的要换行的时候。
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  void send();
                }
              }}
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
      {/* 🚨 窄屏上工具面板要盖在对话上，不能直接消失。
          原来只有 `hidden … lg:block` 一条规则，没有任何兜底：在 1024px 以下
          按「开始任务」，服务端把工具接受了，openTool 也设上了，而屏幕上什么
          都不会出现——没有面板，也没有一句话说明。她只会认为这东西坏了。
          现在窄屏是一层浮层（工具打开时才铺上来），宽屏还是右边那一栏。 */}
      <aside
        className={`${
          openTool
            ? "fixed inset-0 z-40 w-full border-l-0 bg-mk-surface"
            : "hidden"
        } shrink-0 border-mk-border lg:static lg:z-auto lg:block lg:w-[360px] lg:border-l lg:bg-transparent`}
      >
        <WorkPanel
          projectId={projectId}
          projectKind={project?.kind ?? ""}
          plan={plan}
          tools={tools}
          openTool={openTool}
          busy={busy}
          onSelectTool={setOpenTool}
          onFinishTool={(t, result, summary) => void finishToolInstance(t, result, summary)}
          onOpenSession={(sid) => void enterSession(sid)}
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
