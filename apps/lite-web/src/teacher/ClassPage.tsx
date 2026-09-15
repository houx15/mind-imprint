import { LearningSnapshot } from "./LearningSnapshot";
import { StudioEmpty, StudioHeading } from "./StudioArtwork";
import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api, ApiError } from "@/api";
import { getRoster, type RosterRow } from "../api/teacher";
import { formatMinutes } from "./format";

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
  | "projectsDone";

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
}: {
  classId: string;
  // Accepted for parity with pro's `ConsoleShell` wiring and future
  // admin-only affordances on this page (e.g. teacher assignment); this
  // build does not yet branch on it.
  role: string;
  onBack: () => void;
  onOpenStudent: (userId: string) => void;
}) {
  void role;

  const [name, setName] = useState<string | null>(null);
  const [joinCode, setJoinCode] = useState<string | null>(null);
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
  const [sortKey, setSortKey] = useState<SortKey>("displayName");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  function loadHeader() {
    setHeaderError(null);
    api
      .getClass(classId)
      .then((d) => {
        setName(d.class.name);
        setJoinCode(d.class.join_code);
      })
      .catch((e) => setHeaderError(e instanceof ApiError ? e.message : String(e)));
  }

  function loadRoster() {
    setRosterError(null);
    getRoster(classId)
      .then(setRoster)
      .catch((e) => setRosterError(e instanceof ApiError ? e.message : String(e)));
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
      setMutationError(`修改班级名称失败：${e instanceof ApiError ? e.message : String(e)}`);
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
      setMutationError(`更换邀请码失败：${e instanceof ApiError ? e.message : String(e)}`);
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
      setMutationError(`移出失败：${e instanceof ApiError ? e.message : String(e)}`);
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
    <div className="min-h-full">
      <div className="teacher-page teacher-class-page">
        <button
          type="button"
          onClick={onBack}
          className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          <Icon icon={ArrowLeft} size={15} />
          返回班级
        </button>

        {headerError ? (
          <div className="mt-4 text-mk-small font-semibold text-mk-danger">
            加载失败：{headerError}{" "}
            <button type="button" onClick={loadHeader} className="cursor-pointer underline">
              重试
            </button>
          </div>
        ) : name === null ? (
          <div className="mt-4 text-mk-body text-mk-muted">加载中…</div>
        ) : null}

        {name !== null && (
          <>
            <StudioHeading label="CLASS OVERVIEW · 班级概览" title={name} />
            <div className="teacher-class-actions">
              {renaming ? (
                <>
                  <input
                    autoFocus
                    aria-label="班级名称"
                    value={draftName}
                    onChange={(e) => setDraftName(e.target.value)}
                    className="rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-h1 text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  />
                  <Button variant="secondary" size="sm" onClick={() => void doRename()} disabled={busy}>
                    保存
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setRenaming(false)}>
                    取消
                  </Button>
                </>
              ) : (
                <>

                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => {
                      setDraftName(name);
                      setRenaming(true);
                    }}
                  >
                    修改名称
                  </Button>
                </>
              )}
            </div>

            <div className="teacher-invite flex flex-wrap items-center gap-2">
              <span
                className="rounded-mk-md px-3 py-1.5 text-mk-small font-bold text-mk-accent-700"
                style={{ background: "color-mix(in srgb, var(--mk-accent-500) 10%, var(--mk-surface))" }}
              >
                邀请码 {joinCode}
              </span>
              <Button
                variant="secondary"
                size="sm"
                onClick={async () => { try { await navigator.clipboard.writeText(joinCode ?? ""); setCopied(true); } catch (e) { setMutationError(`复制失败：${String(e)}`); } }}
              >
                {copied ? "已复制" : "复制邀请码"}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setConfirmRegen(true)}>
                更换邀请码
              </Button>
              {confirmRegen && (
                <span className="inline-flex items-center gap-2 text-mk-small font-semibold text-mk-danger">
                  更换后，旧邀请码将立即失效。是否继续？
                  <Button variant="danger" size="sm" onClick={() => void doRegen()} disabled={busy}>
                    确认更换
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirmRegen(false)}>
                    取消
                  </Button>
                </span>
              )}
            </div>

            {mutationError && (
              <div className="mt-3 text-mk-small font-semibold text-mk-danger">{mutationError}</div>
            )}
          </>
        )}

        {roster && <LearningSnapshot rows={roster} selectedDays={selectedDays} onSelectDays={setSelectedDays} />}
        <div className="teacher-roster-section">
          <div className="teacher-section-heading"><div><h2>学生名单 <small className="teacher-count">{roster?.length ?? "—"}</small></h2><p>阅读、写作和项目均为「已完成 / 总数」；时长仅统计平台内的学习活动。</p></div><input className="teacher-search" type="search" aria-label="搜索学生" placeholder="搜索学生姓名" value={query} onChange={e => setQuery(e.target.value)} /></div>
          {selectedDays !== null && <button className="teacher-filter-chip" onClick={() => setSelectedDays(null)}>本周活跃 {selectedDays} 天 · 清除筛选 ×</button>}
          {confirmRemove && <div className="teacher-confirm" role="alert"><p>确认移出「{roster?.find(s => s.id === confirmRemove)?.displayName}」？移出后，该学生将无法查看本班内容。</p><Button variant="danger" size="sm" disabled={busy} onClick={() => void doRemove(confirmRemove)}>确认移出</Button><Button variant="ghost" size="sm" onClick={() => setConfirmRemove(null)}>取消</Button></div>}
          {rosterError ? (
            <div className="text-mk-small font-semibold text-mk-danger">
              加载失败：{rosterError}{" "}
              <button type="button" onClick={loadRoster} className="cursor-pointer underline">
                重试
              </button>
            </div>
          ) : sortedRoster === null ? (
            <div className="text-mk-body text-mk-muted">加载中…</div>
          ) : sortedRoster.length === 0 ? (
            <StudioEmpty>{query.trim() || selectedDays !== null ? "未找到匹配的学生，请修改搜索条件。" : `暂无学生。请将邀请码 ${joinCode ?? "—"} 发给学生。`}</StudioEmpty>
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
                        <button className="teacher-student-link" onClick={e => { e.stopPropagation(); onOpenStudent(s.id); }}><span className="teacher-avatar">{Array.from(s.displayName)[0]}</span>{s.displayName}<span aria-hidden="true">↗</span></button>
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
    </div>
  );
}
