import { LearningSnapshot } from "./LearningSnapshot";
import { BackLink, SectionHead, StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { useEffect, useRef, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api, CLASS_GRADE_OPTIONS } from "@/api";
import { listAssignments, type AssignmentSummaryDTO } from "../api/assignments";
import { getRoster, type RosterRow } from "../api/teacher";
import { formatMinutes } from "./format";
import { errorText } from "./assignmentLogic";
import { AssignmentCard } from "./AssignmentCard";
import { TeacherPage } from "./TeacherPage";
import { Select } from "./controls/Select";

/**
 * ClassPage — the lite teacher end's one class: name + join code header,
 * then the roster table (`getRoster`). Deliberately smaller than pro's
 * `ClassDetailView`: no weekly-report sub-tab, no teacher-assignment panel
 * — those are pro console surfaces this task does not reuse. Class identity
 * (name, join code) comes from `api.getClass`, same client pro's console
 * uses (`renameClass` / `regenerateJoinCode` / `removeEnrollment`).
 */

type SortKey =
  | "displayName"
  | "lastActiveAt"
  | "activeDaysThisWeek"
  | "minutesThisWeek"
  | "minutesTotal"
  | "turns"
  | "readingsDone"
  | "writingsDone"
  | "projectsDone"
  | "overdueAssignments";

const COLUMNS: { key: SortKey; label: string }[] = [
  { key: "displayName", label: "学生" },
  { key: "lastActiveAt", label: "最近活跃" },
  { key: "activeDaysThisWeek", label: "本周活跃天数" },
  { key: "minutesThisWeek", label: "本周时长" },
  { key: "minutesTotal", label: "累计时长" },
  { key: "turns", label: "对话轮次" },
  { key: "readingsDone", label: "阅读" },
  { key: "writingsDone", label: "写作" },
  { key: "projectsDone", label: "项目" },
  { key: "overdueAssignments", label: "逾期作业" },
];

/** ISO timestamp → `M月D日`, or `—` when there is nothing to show. Lite's
 * own short form — pro's `shortDate` (YYYY-MM-DD) reads as a system log, not
 * a date a teacher scans a roster with. */
function lastActiveLabel(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return `${d.getMonth() + 1}月${d.getDate()}日`;
}

function sortRoster(roster: RosterRow[], key: SortKey, dir: "asc" | "desc"): RosterRow[] {
  const sorted = [...roster].sort((a, b) => {
    const av = a[key];
    const bv = b[key];
    if (typeof av === "string" && typeof bv === "string") return av.localeCompare(bv);
    // `lastActiveAt` can be null; treat it as older than any real timestamp
    // so students who have never been active sort to the bottom (ascending).
    if (av === null && bv === null) return 0;
    if (av === null) return -1;
    if (bv === null) return 1;
    if (typeof av === "number" && typeof bv === "number") return av - bv;
    return 0;
  });
  return dir === "asc" ? sorted : sorted.reverse();
}

export function ClassPage({
  classId,
  role,
  onBack,
  onOpenStudent,
  onNewAssignment,
  onOpenWeekly,
  onOpenChat,
  onOpenAssignment,
  onOpenAssignments,
}: {
  classId: string;
  // Accepted for parity with pro's `ConsoleShell` wiring and future
  // admin-only affordances on this page (e.g. teacher assignment); this
  // build does not yet branch on it.
  role: string;
  onBack: () => void;
  onOpenStudent: (userId: string) => void;
  /** 布置作业 in the header; opens the create form on this class. */
  onNewAssignment?: () => void;
  /** 周报 in the header, next to 布置作业; opens the class weekly page. */
  onOpenWeekly?: () => void;
  /** 班级对话 in the header: the class conversation with 印记. */
  onOpenChat?: () => void;
  /** A card in 作业 opens that homework. */
  onOpenAssignment?: (assignmentId: string) => void;
  /** 全部作业: the assignments page on this class. */
  onOpenAssignments?: () => void;
}) {
  void role;

  const [name, setName] = useState<string | null>(null);
  const [joinCode, setJoinCode] = useState<string | null>(null);
  const [grade, setGrade] = useState("");
  const [headerError, setHeaderError] = useState<string | null>(null);

  const [roster, setRoster] = useState<RosterRow[] | null>(null);
  const [rosterError, setRosterError] = useState<string | null>(null);

  const [renaming, setRenaming] = useState(false);
  const [draftName, setDraftName] = useState("");
  const [confirmRegen, setConfirmRegen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);

  const [selectedDays, setSelectedDays] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [copied, setCopied] = useState(false);
  const [emptyStateCopied, setEmptyStateCopied] = useState(false);
  const emptyStateCopyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>("displayName");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  useEffect(() => {
    return () => {
      if (emptyStateCopyTimer.current) clearTimeout(emptyStateCopyTimer.current);
    };
  }, []);

  async function copyJoinCodeFromEmptyState() {
    if (!joinCode) return;
    try {
      await navigator.clipboard.writeText(joinCode);
      setEmptyStateCopied(true);
      if (emptyStateCopyTimer.current) clearTimeout(emptyStateCopyTimer.current);
      emptyStateCopyTimer.current = setTimeout(() => setEmptyStateCopied(false), 2000);
    } catch (e) {
      setMutationError(`复制失败：${errorText(e)}`);
    }
  }

  function loadHeader() {
    setHeaderError(null);
    api
      .getClass(classId)
      .then((d) => {
        setName(d.class.name);
        setJoinCode(d.class.join_code);
        setGrade(d.class.grade);
      })
      .catch((e) => setHeaderError(errorText(e)));
  }

  function loadRoster() {
    setRosterError(null);
    getRoster(classId)
      .then(setRoster)
      .catch((e) => setRosterError(errorText(e)));
  }

  useEffect(() => {
    loadHeader();
    loadRoster();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [classId]);

  async function doRename() {
    const trimmed = draftName.trim();
    if (!trimmed) return;
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await api.renameClass(classId, trimmed);
      setName(updated.name);
      setJoinCode(updated.join_code);
      setRenaming(false);
    } catch (e) {
      setMutationError(`修改班级名称失败：${errorText(e)}`);
    } finally {
      setBusy(false);
    }
  }

  async function doRegen() {
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await api.regenerateJoinCode(classId);
      setJoinCode(updated.join_code);
      setCopied(false);
      setConfirmRegen(false);
    } catch (e) {
      setMutationError(`更换邀请码失败：${errorText(e)}`);
    } finally {
      setBusy(false);
    }
  }

  // 年级是一个七选一的下拉，改了就存，不做两步确认。
  async function doSetGrade(next: string) {
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await api.setClassGrade(classId, next);
      setGrade(updated.grade);
    } catch (e) {
      setMutationError(`修改年级失败：${errorText(e)}`);
    } finally {
      setBusy(false);
    }
  }

  async function doRemove(userId: string) {
    setMutationError(null);
    setBusy(true);
    try {
      await api.removeEnrollment(classId, userId);
      setConfirmRemove(null);
      loadRoster();
    } catch (e) {
      setMutationError(`移出失败：${errorText(e)}`);
    } finally {
      setBusy(false);
    }
  }

  function onSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  }

  const sortedRoster = roster ? sortRoster(roster.filter(s => (selectedDays === null || s.activeDaysThisWeek === selectedDays) && s.displayName.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())), sortKey, sortDir) : null;

  return (
    <TeacherPage width="wide">
      <div className="teacher-class-page">
        <BackLink label="返回班级列表" onClick={onBack} />

        {headerError ? (
          <StudioError message={headerError} onRetry={loadHeader} />
        ) : name === null ? (
          <StudioLoading />
        ) : (
          <>
            <StudioHeading
              kicker="班级"
              title={name}
              description="本周的学习情况、作业和学生名单。请从这里布置作业，或询问印记本班的情况。"
              kind="project"
              actions={
                <>
                  {onNewAssignment && (
                    <Button variant="primary" onClick={onNewAssignment}>
                      布置作业
                    </Button>
                  )}
                  {onOpenChat && (
                    <Button variant="secondary" onClick={onOpenChat}>
                      班级对话
                    </Button>
                  )}
                  {onOpenWeekly && (
                    <Button variant="secondary" onClick={onOpenWeekly}>
                      周报
                    </Button>
                  )}
                </>
              }
            />

            <div className="teacher-invite-strip">
              {renaming ? (
                <>
                  <input
                    autoFocus
                    aria-label="班级名称"
                    value={draftName}
                    onChange={(e) => setDraftName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void doRename();
                    }}
                    className="tc-input min-w-[200px] flex-1"
                  />
                  <Button variant="primary" size="sm" onClick={() => void doRename()} disabled={busy}>
                    保存
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setRenaming(false)}>
                    取消
                  </Button>
                </>
              ) : (
                <>
                  <strong>邀请码 {joinCode}</strong>
                  <span>学生凭邀请码加入本班</span>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={async () => { try { await navigator.clipboard.writeText(joinCode ?? ""); setCopied(true); } catch (e) { setMutationError(`复制失败：${String(e)}`); } }}
                  >
                    {copied ? "已复制" : "复制邀请码"}
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirmRegen(true)}>
                    更换邀请码
                  </Button>
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
                    <span>年级</span>
                    <Select
                      value={grade}
                      options={CLASS_GRADE_OPTIONS}
                      onChange={(v) => void doSetGrade(v)}
                      ariaLabel="年级"
                      size="sm"
                      disabled={busy}
                    />
                  </span>
                  <span className="teacher-invite-spacer" />
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setDraftName(name);
                      setRenaming(true);
                    }}
                  >
                    修改班级名称
                  </Button>
                </>
              )}
            </div>
            {confirmRegen && (
              <div className="teacher-confirm mt-3" role="alert">
                <p>更换后，旧邀请码将立即失效。是否继续？</p>
                <Button variant="danger" size="sm" onClick={() => void doRegen()} disabled={busy}>
                  确认更换
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setConfirmRegen(false)}>
                  取消
                </Button>
              </div>
            )}

            {mutationError && (
              <div className="mt-3 text-mk-small font-semibold text-mk-danger" role="alert">{mutationError}</div>
            )}
          </>
        )}

        {roster && <LearningSnapshot rows={roster} selectedDays={selectedDays} onSelectDays={setSelectedDays} onOpenStudent={onOpenStudent} />}

        <ClassAssignments
          classId={classId}
          onOpen={onOpenAssignment}
          onOpenAll={onOpenAssignments}
          onNew={onNewAssignment}
        />

        <div className="teacher-roster-section">
          <div className="teacher-section-heading"><div><h2>学生名单 <small className="teacher-count">{roster?.length ?? "—"}</small></h2><p>阅读、写作和项目均为「已完成 / 总数」；时长仅统计平台内的学习活动。</p></div><input className="teacher-search" type="search" aria-label="搜索学生" placeholder="搜索学生姓名" value={query} onChange={e => setQuery(e.target.value)} /></div>
          {selectedDays !== null && <button className="teacher-filter-chip" onClick={() => setSelectedDays(null)}>本周活跃 {selectedDays} 天 · 清除筛选 ×</button>}
          {confirmRemove && <div className="teacher-confirm" role="alert"><p>确认移出「{roster?.find(s => s.id === confirmRemove)?.displayName}」？移出后，该学生将无法查看本班内容。</p><Button variant="danger" size="sm" disabled={busy} onClick={() => void doRemove(confirmRemove)}>确认移出</Button><Button variant="ghost" size="sm" onClick={() => setConfirmRemove(null)}>取消</Button></div>}
          {rosterError ? (
            <StudioError message={rosterError} onRetry={loadRoster} />
          ) : sortedRoster === null ? (
            <StudioLoading />
          ) : sortedRoster.length === 0 ? (
            query.trim() || selectedDays !== null ? (
              <StudioEmpty kind="discovery" title="未找到匹配的学生" compact>
                请修改搜索条件，或清除筛选。
              </StudioEmpty>
            ) : (
              <StudioEmpty
                kind="quest"
                title="暂无学生"
                action={
                  joinCode
                    ? { label: emptyStateCopied ? "已复制" : "复制邀请码", onClick: () => void copyJoinCodeFromEmptyState() }
                    : undefined
                }
              >
                {`学生凭邀请码加入本班。请将邀请码 ${joinCode ?? "—"} 发给学生。`}
              </StudioEmpty>
            )
          ) : (
            <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface shadow-mk-xs">
              <table className="w-full min-w-[900px] border-collapse">
                <thead>
                  <tr>
                    {COLUMNS.map((col) => (
                      <th
                        key={col.key}
                        aria-sort={sortKey === col.key ? (sortDir === "asc" ? "ascending" : "descending") : "none"}
                        className="cursor-pointer whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted"
                      >
                        <button type="button" onClick={() => onSort(col.key)}>{col.label}{sortKey === col.key && (sortDir === "asc" ? " ↑" : " ↓")}</button>
                      </th>
                    ))}
                    <th className="border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted">
                      操作
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {sortedRoster.map((s) => (
                    <tr
                      key={s.id}
                      onClick={() => onOpenStudent(s.id)}
                      className="cursor-pointer hover:bg-mk-accent-50"
                    >
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">
                        <button className="teacher-student-link" onClick={e => { e.stopPropagation(); onOpenStudent(s.id); }}><span className="teacher-avatar">{Array.from(s.displayName)[0]}</span>{s.displayName}{s.activeDaysThisWeek === 0 && <span className="teacher-idle-pill">本周未活跃</span>}<span aria-hidden="true">→</span></button>
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {lastActiveLabel(s.lastActiveAt)}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {s.activeDaysThisWeek}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {formatMinutes(s.minutesThisWeek)}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {formatMinutes(s.minutesTotal)}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {s.turns}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {s.readingsDone}/{s.readingsTotal}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {s.writingsDone}/{s.writingsTotal}
                      </td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {s.projectsDone}/{s.projectsTotal}
                      </td>
                      <td className={"whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small " + (s.overdueAssignments > 0 ? "text-mk-danger font-bold" : "text-mk-ink")}>{s.overdueAssignments}</td>
                      <td
                        className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-right text-mk-small"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <Button variant="ghost" size="sm" onClick={() => setConfirmRemove(s.id)}>移出班级</Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </TeacherPage>
  );
}

/** 本班作业：最近的三份，卡片与作业页相同。自己加载，失败不影响名单。 */
function ClassAssignments({
  classId,
  onOpen,
  onOpenAll,
  onNew,
}: {
  classId: string;
  onOpen?: (assignmentId: string) => void;
  onOpenAll?: () => void;
  onNew?: () => void;
}) {
  const [rows, setRows] = useState<AssignmentSummaryDTO[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setRows(null);
    setError(null);
    listAssignments(classId)
      .then((list) => {
        if (!cancelled) setRows(list);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, nonce]);

  return (
    <section>
      <SectionHead
        title="作业"
        aside={
          rows && rows.length > 0 && onOpenAll ? (
            <button type="button" className="teacher-link" onClick={onOpenAll}>
              全部作业（{rows.length}）
              <Icon icon={ArrowRight} size={14} />
            </button>
          ) : undefined
        }
      />
      {error ? (
        <StudioError message={error} onRetry={() => setNonce((n) => n + 1)} />
      ) : rows === null ? (
        <StudioLoading />
      ) : rows.length === 0 ? (
        <StudioEmpty
          kind="writing"
          title="暂无作业"
          compact
          action={onNew ? { label: "布置作业", onClick: onNew } : undefined}
        >
          作业会显示在学生首页和收件箱。请为本班布置第一份作业。
        </StudioEmpty>
      ) : (
        <div className="teacher-assignment-list">
          {[...rows].sort((a, b) => b.issueCount - a.issueCount || b.toGrade - a.toGrade || b.needsReadingReview - a.needsReadingReview || b.counts.overdue - a.counts.overdue || Date.parse(a.dueAt) - Date.parse(b.dueAt)).slice(0, 3).map((a) => (
            <AssignmentCard key={a.id} assignment={a} compact onOpen={() => onOpen?.(a.id)} />
          ))}
        </div>
      )}
    </section>
  );
}
