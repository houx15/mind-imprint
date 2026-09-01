import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft, CornerDownRight, Send } from "lucide-react";
import { Icon } from "@/ui";
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
import { navigate } from "../routing";
import { PlanPanel } from "./PlanPanel";

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
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
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
        const [projects, ss, pl] = await Promise.all([
          listProjects(),
          listSessions(projectId),
          getPlan(projectId),
        ]);
        if (cancelled) return;
        setProject(projects.find((p) => p.id === projectId) ?? null);
        setSessions(ss);
        setPlan(pl);
        await refreshThread(null);
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiError ? err.message : "这个项目没打开，刷新试试。");
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
    setError(null);
    try {
      await postTurn(projectId, text, activeSession ?? undefined);
      setDraft("");
      await refreshThread(activeSession);
    } catch (err) {
      // 印记 failing is surfaced, never smoothed into a plausible sentence.
      setError(err instanceof ApiError ? err.message : "印记没接上，再试一次。");
    } finally {
      setBusy(false);
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
      setError(err instanceof ApiError ? err.message : "这一层没开起来。");
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
      setError(err instanceof ApiError ? err.message : "没收起来，再试一次。");
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
            {thread.length === 0 && (
              <p className="text-mk-small text-mk-muted">
                {current
                  ? "这一层还没说话。把你想到的写下来。"
                  : "说说你想做的这件事——现在知道什么，还不确定什么。"}
              </p>
            )}
            {thread.map((m) => (
              <Message key={m.seq} m={m} onDig={(k, q) => void dig(k, q, String(m.seq))} busy={busy} />
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
              placeholder={current ? "就想这一个问题" : "跟印记说"}
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
        <PlanPanel plan={plan.plan} pending={plan.pending} onResolve={onResolve} onApprove={onApprove} />
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

  return (
    <div className={mine ? "flex justify-end" : ""}>
      <div className={mine ? "max-w-[85%]" : ""}>
        <div
          className={
            mine
              ? "rounded-mk-lg px-3 py-2 text-mk-body text-white"
              : "text-mk-prose text-mk-ink"
          }
          style={mine ? { background: "var(--mk-accent-500)" } : undefined}
        >
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
