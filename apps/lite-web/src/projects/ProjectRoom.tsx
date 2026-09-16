import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { GrowingTextarea } from "../shared/GrowingTextarea";
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
  toolsForSession,
  resolveTool,
  summonTool,
  SELF_OPENED,
  type ToolInstance,
} from "../api/tools";
import { navigate, flushNavigationGuards } from "../routing";
import { WorkPanel } from "./WorkPanel";
import { DiscussionSource } from "./DiscussionSource";
import { setBoardAxes } from "../api/projects";
import { PaneResizer } from "./PaneResizer";
import { PANE_DEFAULT, usePaneWidth } from "./usePaneWidth";
import { AwayCard, ToolInvite } from "./tools/ToolInvite";
import { apiErrorText } from "../api/errorText";
import { Says, errorMarkdown } from "./Says";
import { useHeartbeat } from "../shared/useHeartbeat";
import { AssignmentLine } from "../inbox/AssignmentLine";
import { isAssignedProject } from "../api/projects";

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
  // 项目室的时长。只在项目载入且还没进入回顾/保留/归档时计时，与阅读、写作同一规则。
  useHeartbeat(
    "project",
    projectId,
    project !== null && !["review", "keeping", "archived"].includes(project.status),
  );
  // 🚨 把工具铺开占满整个房间（产品负责人 2026-09-03：「需要拉出来，更充分的
  // 视觉空间」）。审核助手要她读一份文档，360px 那一栏读不下去。见 tools/wide.tsx。
  const [wideTool, setWideTool] = useState(false);
  const [showMobileWork, setShowMobileWork] = useState(false);
  // 右栏宽度她自己拖（「adjustable like in cowork」）。见 usePaneWidth.ts。
  const { width: paneWidth, setWidth: setPaneWidth, desktop } = usePaneWidth();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [thread, setThread] = useState<ThreadMessage[]>([]);
  const [plan, setPlan] = useState<PlanState>({ plan: null, pending: [] });
  const [tools, setTools] = useState<ToolInstance[]>([]);
  const [openTool, setOpenTool] = useState<string | null>(null);
  const panelTransition = useRef(false);
  async function afterDraftSave(action: () => void) {
    if (panelTransition.current) return;
    panelTransition.current = true;
    try { await flushNavigationGuards(); action(); }
    catch { setShowMobileWork(true); } // The mounted editor owns save-error recovery.
    finally { panelTransition.current = false; }
  }

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
  const [failedTurn, setFailedTurn] = useState<Parameters<typeof postTurn> | null>(null);

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
        // Boot loads the project thread; a retained development/reused view
        // must not label those messages as a previously selected discussion.
        setActiveSession(null);
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
        //
        // 老师布置的项目不补这一轮：idea 是老师写的驱动问题，不是她说的话。
        // 页头上显示「驱动问题」，第一句由她自己写。判据是项目行上的
        // assigned，不查作业：作业归档或查询失败都不会让这句话变成她说的。
        if (msgs.length === 0 && mine && !isAssignedProject(mine) && mine.idea.trim() && !seeded.current) {
          seeded.current = true;
          setPending(mine.idea.trim());
          setThinking(true);
          try {
            await requestTurn(projectId, mine.idea.trim());
            const [nextThread, nextTools, nextPlan] = await Promise.all([
              getThread(projectId), listTools(projectId), getPlan(projectId),
            ]);
            if (cancelled) return;
            setThread(nextThread);
            setTools(nextTools);
            setPlan(nextPlan);
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

  async function requestTurn(...request: Parameters<typeof postTurn>) {
    setFailedTurn(null);
    try { return await postTurn(...request); }
    catch (err) {
      // Only a confirmed model failure is safe to repeat. A network failure
      // could have happened after saving, so it does not get a retry action.
      if (err instanceof ApiError && err.status === 502 && err.code === "ai_dialogue_failed") setFailedTurn(request);
      throw err;
    }
  }

  async function retryFailedTurn() {
    if (!failedTurn || busy || current?.closedAt || failedTurn[0] !== projectId || (failedTurn[2] ?? null) !== activeSession) return;
    const request = failedTurn;
    setBusy(true); setThinking(true); setPending(request[1] || null); setError(null);
    try {
      await requestTurn(...request);
      if (draft.trim() === request[1]) setDraft("");
      await refreshThread(activeSession);
      setTools(await listTools(projectId));
      setPlan(await getPlan(projectId));
    } catch (err) { setError(apiErrorText(err)); }
    finally { setBusy(false); setThinking(false); setPending(null); }
  }

  async function send() {
    const text = draft.trim();
    if (!text || busy || current?.closedAt) return;
    setBusy(true);
    setThinking(true);
    setPending(text);
    setError(null);
    try {
      const res = await requestTurn(projectId, text, activeSession ?? undefined);
      setDraft("");
      await refreshThread(activeSession);
      // 工具列表无条件拉一遍，那张邀请卡才会出现在对话末尾。
      //
      // 🚨 原来是 `if (res.toolId)`。这一轮**没递新工具**不代表工具列表没变：
      // 服务端会把一件点开是空的工具挡掉，也会有别处改了状态。少拉这一次，
      // 屏幕上留着的就是一份过期的清单。
      setTools(await listTools(projectId));
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
      await requestTurn(projectId, "", s.id);
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
  async function enterSession(sessionId: string): Promise<boolean> {
    try { await flushNavigationGuards(); } catch { return false; }
    setBusy(true);
    setError(null);
    let opened = false;
    try {
      // Load first: a failed request must not pair a new session with old messages.
      const [available, existing] = await Promise.all([listSessions(projectId), getThread(projectId, sessionId)]);
      setSessions(available);
      setActiveSession(sessionId);
      setThread(existing);
      setOpenTool(null);
      setShowMobileWork(false);
      opened = true;
      // Reopening an existing discussion must not generate another opening.
      if (existing.length === 0) {
        setThinking(true);
        await requestTurn(projectId, "", sessionId);
        await refreshThread(sessionId);
      }
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
    }
    return opened;
  }

  async function goTo(sessionId: string | null) {
    try { await flushNavigationGuards(); } catch { return; }
    setBusy(true);
    setError(null);
    try {
      const messages = await getThread(projectId, sessionId ?? undefined);
      setActiveSession(sessionId);
      setThread(messages);
      setOpenTool(null);
      setShowMobileWork(false);
    } catch (err) { setError(apiErrorText(err)); }
    finally { setBusy(false); }
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
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await approvePlan(projectId, versionId);
      setPlan(await getPlan(projectId));
      setThinking(true);
      await requestTurn(projectId, "");
      await refreshThread(activeSession);
      setTools(await listTools(projectId));
      setPlan(await getPlan(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
    }
  }

  /* ── 工具 ─────────────────────────────────────────────────────────────
   *
   * 打开一件当场做的工具，右边就切过去；打开一件出门做的，什么也不弹——她
   * 要走了，弹一个面板给她看没有意义。
   */

  async function openToolInstance(t: ToolInstance) {
    try { await flushNavigationGuards(); } catch { return; }
    setBusy(true);
    setError(null);
    try {
      const got = await acceptTool(projectId, t.id);
      setTools((prev) => prev.map((x) => (x.id === got.id ? got : x)));
      if (got.kind === "thinking") {
        // 🚨 这条路才是她最常走的那条：印记递过来一张卡，她按「开始任务」。
        // 铺开原来只加在 onSelectTool（「进行中」那一列）上，于是同一件工具，
        // 走查里是铺开的、她真用的时候是 360px 那条柱子——2026-09-04 的浏览器
        // 走查拍到受众画像挤在右边一条缝里，三个人的画像根本摆不下。
        setWideTool(true);
        setOpenTool(got.id);
      }
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
      const revision = t.tool === "observe" && result && typeof result === "object" && "observationRevision" in result ? result.observationRevision : undefined;
      const observation = typeof revision === "number" ? {toolId: t.id, revision} : undefined;
      const got = observation ? t : await resolveTool(projectId, t.id, { status: "done", result, note: note.trim() });
      setTools((prev) => prev.map((x) => (x.id === got.id ? got : x)));
      setOpenTool(null);
      await refreshThread(activeSession);
      // 空文本的一轮：她没说话，是刚做完一件事，印记该接一句。
      setThinking(true);
      await requestTurn(projectId, "", activeSession ?? undefined, observation ? undefined : got.id, observation);
      await refreshThread(activeSession);
      // 🚨 印记在这一轮递的工具也要拉一遍。
      //
      // 少了这一句，**她刚做完一件工具、印记顺势递出的下一件，是看不见的**——
      // 而那正是印记最常递工具的时刻。2026-09-03 线上实测：做完「观察日记」，
      // 印记递了「头脑风暴」，对话里什么都没出现；她只好自己打字，于是下一轮
      // 印记又递了一次「头脑风暴」，这才两张一起冒出来。做完「头脑风暴」之后
      // 递的「问题识别」同样要刷新整页才看得见。
      //
      // 无条件拉：这一轮还可能把某件工具关掉（见服务端那道空界面闸），
      // 只在"递了新的"时候拉，关掉的那件就永远留在屏幕上。
      setTools(await listTools(projectId));
      setPlan(await getPlan(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
      setThinking(false);
    }
  }

  // 刚递出来、她还没表态的。
  const discussionTools = toolsForSession(tools, current?.id ?? null);
  const invites = discussionTools.filter((t) => t.status === "summoned");
  // 她答应了、人出门去做的那几件。留在对话里她当初答应的那个位置，因为那就是
  // 她记得的地方；右侧面板不为它单开一档（产品负责人 2026-09-02）。
  const away = discussionTools.filter((t) => t.kind === "world" && t.status === "accepted");

  return (
    // relative：铺开的工具面板贴着**房间**铺开，不是贴着整个视口——
    // 视口的左边还有一条 64px 的导航栏。
    <div className="student-project-room relative flex h-full min-h-0">
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
          {/* `projectId` IS the atom id (`pbl_project`'s primary key). */}
          <div className="flex min-w-0 flex-col">
            <span className="truncate text-mk-body font-semibold text-mk-ink">
              {project ? projectTitle(project) : "项目"}
            </span>
            <AssignmentLine atomId={projectId} />
            {project && isAssignedProject(project) && project.idea.trim() && (
              <span className="line-clamp-2 text-mk-small text-mk-muted" title={project.idea}>
                驱动问题：{project.idea}
              </span>
            )}
          </div>
          <button type="button" className="ml-auto shrink-0 rounded-lg border border-mk-border px-3 py-2 text-mk-small lg:hidden" onClick={() => setShowMobileWork(true)}>计划与材料</button>

        </header>

        {trail.length > 0 && (
          <Breadcrumb trail={trail} onGo={(id) => void goTo(id)} />
        )}

        <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 py-4">
          <div className="mx-auto flex max-w-[640px] flex-col gap-4">
            {current?.kind === "keeping" && <DiscussionSource key={current.id} projectId={projectId} session={current} />}
            {thread.length === 0 && !thinking && (
              <div className="flex flex-col items-center gap-3 py-10 text-center">
                <Pebble state="idle" size={44} />
                {/* 印记永远先开口（D1），所以这一屏只在那一轮没成功时才出现。
                    上面的红字会说明原因，这里不再假装是在邀请她开始。 */}
                {project && isAssignedProject(project) ? <>
                  <h2 className="text-mk-body font-semibold text-mk-ink">开始项目</h2>
                  <p className="text-mk-small text-mk-secondary">请选择当前需要的帮助，或直接输入想法。选择后可以修改再发送。</p>
                  <div className="flex flex-wrap justify-center gap-2">
                    {[
                      ["理解问题", "我想先理解老师布置的问题，请帮我明确需要弄清楚什么。"],
                      ["设计调研", "我想设计一次观察或访谈，请结合老师的要求帮我确定从哪里开始。"],
                      ["讨论想法", "我已经有一些想法，想先讨论它是否可行。"],
                    ].map(([label, text]) => <button key={label} type="button" className="rounded-lg border border-mk-border bg-mk-surface px-4 py-3 text-mk-small" onClick={() => setDraft(text!)}>{label}</button>)}
                  </div>
                </> : <p className="text-mk-body text-mk-secondary">对话还没有开始</p>}
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
                <RequestWait />
              </div>
            )}

            {/* 递到手边的工具。放在对话末尾，因为它是印记刚说的话的一部分。 */}
            {away.map((t) => (
              <AwayCard
                key={t.id}
                projectId={projectId}
                tool={t}
                busy={busy}
                onBack={() => void afterDraftSave(() => setOpenTool(t.id))}
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
            {discussionTools.filter(t => t.kind !== "world" && t.status === "accepted" && t.id !== openTool).map(t => (
              <button key={t.id} type="button" onClick={() => void afterDraftSave(() => setOpenTool(t.id))} className="mt-3 block rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3 text-mk-body font-semibold text-mk-accent-500">
                继续：{t.label}
              </button>
            ))}
            {!current && tools.some(t => t.tool === "persona" && t.status === "done") && !tools.some(t => t.tool === "creative" && (t.status === "accepted" || t.status === "summoned")) && (
              <button type="button" disabled={busy} className="mt-3 block rounded-mk-lg border border-mk-border px-4 py-3 text-mk-body text-mk-accent-500" onClick={async () => {
                setBusy(true); setError(null);
                try {
                  await flushNavigationGuards();
                  const offered = await summonTool(projectId, { tool: "creative", reason: SELF_OPENED });
                  const accepted = await acceptTool(projectId, offered.id);
                  setTools(prev => [...prev.filter(t => t.id !== accepted.id), accepted]);
                  setWideTool(true); setOpenTool(accepted.id);
                } catch (err) { setError(apiErrorText(err)); }
                finally { setBusy(false); }
              }}>构思主页风格与意象</button>
            )}
            {!current && tools.some(t => t.tool === "persona" && t.status === "done") && !tools.some(t => t.tool === "persona" && (t.status === "accepted" || t.status === "summoned")) && (
              <button type="button" disabled={busy} className="mt-3 block rounded-mk-lg border border-mk-border px-4 py-3 text-mk-body text-mk-accent-500" onClick={async () => {
                setBusy(true); setError(null);
                try {
                  await flushNavigationGuards();
                  const offered = await summonTool(projectId, { tool: "persona", reason: SELF_OPENED });
                  const accepted = await acceptTool(projectId, offered.id);
                  setTools(prev => [...prev.filter(t => t.id !== accepted.id), accepted]);
                  setWideTool(true); setOpenTool(accepted.id);
                } catch (err) { setError(apiErrorText(err)); }
                finally { setBusy(false); }
              }}>查看与修改人物板</button>
            )}
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
          {current?.closedAt && (
            <p className="mb-2 text-mk-small text-mk-muted">讨论已结束，结论已返回上一级。请返回项目继续。</p>
          )}
          {error && (
            <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
              <Says content={errorMarkdown(error)} />
              {failedTurn && failedTurn[0] === projectId && (failedTurn[2] ?? null) === activeSession && !current?.closedAt && <button type="button" disabled={busy} onClick={() => void retryFailedTurn()} className="mt-2 rounded-full border border-mk-border px-3 py-1.5 text-mk-ink disabled:opacity-40">重试生成</button>}
            </div>
          )}
          <div className="mx-auto flex max-w-[640px] items-end gap-2">
            <GrowingTextarea
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
              disabled={busy || Boolean(current?.closedAt)}
              placeholder={current?.closedAt ? "讨论已结束" : "请输入"}
              className="flex-1 resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
            <button
              type="button"
              onClick={() => void send()}
              disabled={!draft.trim() || busy || Boolean(current?.closedAt)}
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
          (openTool || showMobileWork)
            ? "fixed inset-0 z-40 w-full border-l-0 bg-mk-surface"
            : "hidden"
        } shrink-0 border-mk-border lg:z-auto lg:block lg:border-l ${
          // 铺开时占满整个房间；对话让位，因为这时候她在读东西，不在说话。
          //
          // 🚨 用 absolute 贴房间，不用 fixed 贴视口：fixed inset-0 从 x=0 起算，
          // 整个面板滑到左边那条导航栏底下，工具自己的标题被挡掉一截。
          //
          // 🚨 背景色不能靠这串字符串的先后顺序定。lg:bg-transparent 和
          // lg:bg-mk-surface 是同一个变体下的同一条属性，谁赢由 Tailwind 生成
          // 样式表的顺序决定——线上赢的是 transparent，对话整个透过来压在正文
          // 上。同理 lg:static / lg:absolute。所以每一档只许出现一个。
          // 下面那条注释里 relative-压过-fixed 的坑，是同一个坑。
          openTool && wideTool
            ? "lg:absolute lg:inset-0 lg:z-40 lg:w-full lg:border-l-0 lg:bg-mk-surface"
            : "lg:static lg:bg-transparent"
        }`}
        // 🚨 宽度只在宽屏上按像素给。窄屏那一档是 `fixed inset-0 w-full` 的整屏
        // 浮层，行内 width 会盖过 w-full，把浮层压成一条。
        style={desktop && !(openTool && wideTool) ? { width: paneWidth } : undefined}
      >
        {/* 🚨 定位上下文放在这一层，不放 aside 上。
            aside 在「铺开」那一档要用 lg:fixed，而 Tailwind 生成的顺序里
            relative 排在 fixed 后面——给 aside 加 lg:relative 会反过来把
            lg:fixed 压掉，铺开就失效了。包一层就没有这个冲突。 */}
        <div className="relative flex h-full flex-col">
          {!openTool && <header className="shrink-0 border-b border-mk-border p-3 lg:hidden"><button type="button" className="rounded-lg border border-mk-border px-3 py-2" onClick={() => void afterDraftSave(() => setShowMobileWork(false))}>返回项目对话</button></header>}
          {/* 拖这条缝改宽度。铺开的时候没有缝可拖——那时候它已经占满了。 */}
          {!(openTool && wideTool) && (
            <PaneResizer onResize={setPaneWidth} onDoubleClick={() => setPaneWidth(PANE_DEFAULT)} />
          )}
          <div className="min-h-0 flex-1"><WorkPanel
          projectId={projectId}
          projectKind={project?.kind ?? ""}
          boardAxes={project?.boardAxes ?? false}
          onSetBoardAxes={async (on) => {
            const got = await setBoardAxes(projectId, on);
            setProject((prev) => (prev ? { ...prev, boardAxes: got.boardAxes } : prev));
          }}
          wide={wideTool}
          onToggleWide={() => setWideTool((w) => !w)}
          plan={plan}
          tools={tools}
          openTool={openTool}
          busy={busy}
          onSelectTool={(id) => {
            // 🚨 打开一件工具就铺开，不用她再点一次「铺开」。产品负责人
            // 2026-09-03：「whenever we want students interact something, make
            // it wide, because we are transiting our focus. don't be mean on
            // using the right wide interactive panel.」
            //
            // 焦点确实转移了：这一刻她在动手摆一块板，不在跟印记说话。默认给
            // 那一栏 360px、等她自己发现右上角有个按钮，等于把每件工具的第一眼
            // 都放在一条比手机还窄的柱子里。要说话时按一下就还原。
            void afterDraftSave(() => { setWideTool(Boolean(id)); setOpenTool(id); });
          }}
          onFinishTool={(t, result, summary) => void finishToolInstance(t, result, summary)}
          onOpenSession={enterSession}
          onResolve={onResolve}
            onApprove={onApprove}
            onOpenMaterial={(t) => {
              void afterDraftSave(() => {
              // 放进列表（已经在里面就替换），再选中。同步做完，右栏立刻有东西。
              setTools((prev) => {
                const has = prev.some((x) => x.id === t.id);
                return has ? prev.map((x) => (x.id === t.id ? t : x)) : [...prev, t];
              });
              // 🚨 和上面 onSelectTool 一样要铺开。少了这一行，从材料列表打开的
              // 工具就挤在 360px 那条柱子里——而「我的主页」正是从这里进去的，
              // 于是她回头看自己那一页时，整页被压成一条比手机还窄的缝，底下
              // 「复制链接」「撤回链接」直接被挤出视野。2026-09-04 的浏览器 walk
              // 拍到的就是这一张。
              setWideTool(true);
              setOpenTool(t.id);
              });
            }}
          /></div>
        </div>
      </aside>
    </div>
  );
}

/** Show elapsed waiting, without inventing server-side progress or an ETA. */
function RequestWait() {
  const [startedAt] = useState(Date.now);
  const [seconds, setSeconds] = useState(0);
  useEffect(() => {
    const timer = window.setInterval(() => setSeconds(Math.floor((Date.now() - startedAt) / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, [startedAt]);
  return <div className="rounded-[4px_13px_13px_13px] bg-mk-surface px-4 py-3 shadow-mk-xs">
    <div className="flex items-center gap-2 text-mk-small text-mk-secondary">
      <span className="mk-think-dot" aria-hidden="true" />
      <span>处理中 · {seconds} 秒</span>
    </div>
    {seconds >= 30 && <p role="status" className="mt-2 text-mk-small text-mk-muted">本次处理耗时较长，结果返回后会自动显示。</p>}
  </div>;
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
        <div className="min-w-0 text-mk-small text-mk-secondary"><Says content={m.content.includes("失败") ? errorMarkdown(m.content) : m.content} /></div>
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
          <Says content={m.content} />
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
          <Says content={m.content} />
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
