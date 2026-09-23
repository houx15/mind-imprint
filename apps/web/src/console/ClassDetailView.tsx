import { useEffect, useState } from "react";
import type { ApiClient, ClassDetail, ClassRoster, Teacher } from "../api";
import { ApiError, CLASS_GRADE_OPTIONS } from "../api";
import { ClassRosterTable } from "./ClassRosterTable";
import { ClassWeeklyView } from "./ClassWeeklyView";
import { Button, Select } from "@/ui";

type Client = Pick<
  ApiClient,
  | "getClass"
  | "renameClass"
  | "regenerateJoinCode"
  | "setClassGrade"
  | "removeEnrollment"
  | "listTeachers"
  | "assignTeacher"
  | "removeTeacher"
  | "getClassRosterReport"
  | "getClassWeeklyReport"
  | "generateClassWeeklyProse"
>;

const LIVE_HEADER_STATS: { key: keyof ClassRoster["header"]; label: string }[] = [
  { key: "activeStudents", label: "活跃学生" },
  { key: "activeProjects", label: "进行中项目" },
  { key: "turns", label: "对话轮次" },
  { key: "reports", label: "能力报告" },
];

function LiveHeaderStrip({ header }: { header: ClassRoster["header"] }) {
  return (
    <div style={{ marginTop: 22, display: "flex", background: "var(--mk-paper)", border: "1px solid var(--mk-border)", borderRadius: "var(--mk-radius-lg)", overflow: "hidden" }}>
      {LIVE_HEADER_STATS.map((s, i) => (
        <div key={s.key} style={{ flex: 1, padding: "14px 20px", borderLeft: i > 0 ? "1px solid var(--mk-border)" : "none" }}>
          <div style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 600 }}>{s.label}</div>
          <div style={{ marginTop: 6, fontSize: 22, fontWeight: 800, color: "var(--mk-ink)" }}>
            {s.key === "activeStudents" ? `${header.activeStudents} / ${header.classSize}` : header[s.key]}
          </div>
        </div>
      ))}
    </div>
  );
}

export function ClassDetailView({
  client,
  classId,
  onBack,
  role,
  onOpenStudent,
  onOpenReport,
}: {
  client: Client;
  classId: string;
  onBack: () => void;
  role?: string;
  onOpenStudent: (userId: string) => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string, userId: string) => void;
}) {
  const [detail, setDetail] = useState<ClassDetail | null>(null);
  const [classRoster, setClassRoster] = useState<ClassRoster | null>(null);
  const [rosterReportError, setRosterReportError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [renaming, setRenaming] = useState(false);
  const [draftName, setDraftName] = useState("");
  const [confirmRegen, setConfirmRegen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null); // student id
  const [busy, setBusy] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [subTab, setSubTab] = useState<"weekly" | "roster">("weekly");

  // Tracked separately from the class-detail load/error above: roster-report
  // is a SECOND, independent request. If it fails while getClass succeeds, a
  // teacher must see an explicit error — not a headed table silently rendered
  // with zero rows, which reads as "this class has no students."
  function loadRosterReport() {
    setRosterReportError(null);
    client.getClassRosterReport(classId).then(setClassRoster).catch((e) => {
      setClassRoster(null);
      setRosterReportError(e instanceof ApiError ? e.message : "加载失败");
    });
  }

  function load() {
    setError(null);
    client.getClass(classId).then(setDetail).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
    loadRosterReport();
  }
  useEffect(load, [client, classId]);

  async function doRename() {
    const trimmed = draftName.trim();
    if (!trimmed) return;
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await client.renameClass(classId, trimmed);
      setDetail((d) => (d ? { ...d, class: updated } : d));
      setRenaming(false);
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "改名失败");
    } finally {
      setBusy(false);
    }
  }

  async function doRegen() {
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await client.regenerateJoinCode(classId);
      setDetail((d) => (d ? { ...d, class: updated } : d));
      setConfirmRegen(false);
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "轮换失败");
    } finally {
      setBusy(false);
    }
  }

  // 年级改了就直接存，不做「编辑 / 保存」两步 —— 这是一个七选一的下拉，
  // 没有可打错的中间态。失败时把后台原话摆出来（AGENTS.md 文案第 8 条）。
  async function doSetGrade(grade: string) {
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await client.setClassGrade(classId, grade);
      setDetail((d) => (d ? { ...d, class: updated } : d));
    } catch (e) {
      setMutationError(e instanceof ApiError ? `修改年级失败：${e.message}` : "修改年级失败");
    } finally {
      setBusy(false);
    }
  }

  async function doRemove(studentId: string) {
    setMutationError(null);
    setBusy(true);
    try {
      await client.removeEnrollment(classId, studentId);
      setDetail((d) => (d ? { ...d, roster: d.roster.filter((s) => s.id !== studentId) } : d));
      setClassRoster((r) => (r ? { ...r, roster: r.roster.filter((s) => s.id !== studentId) } : r));
      setConfirmRemove(null);
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "移除失败");
    } finally {
      setBusy(false);
    }
  }

  const [teacherOptions, setTeacherOptions] = useState<Teacher[] | null>(null);
  const [assignId, setAssignId] = useState("");

  function loadTeacherOptions() {
    if (teacherOptions == null) {
      client.listTeachers().then(setTeacherOptions).catch(() => setTeacherOptions([]));
    }
  }

  useEffect(() => {
    if (role === "admin") loadTeacherOptions();
  }, [role, classId]);

  async function doAssignTeacher() {
    if (!assignId) return;
    setMutationError(null);
    setBusy(true);
    try {
      const { teachers } = await client.assignTeacher(classId, assignId);
      setDetail((d) => (d ? { ...d, teachers } : d));
      setAssignId("");
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "添加教师失败");
    } finally {
      setBusy(false);
    }
  }

  async function doRemoveTeacher(userId: string) {
    setMutationError(null);
    setBusy(true);
    try {
      await client.removeTeacher(classId, userId);
      setDetail((d) => (d ? { ...d, teachers: d.teachers.filter((t) => t.id !== userId) } : d));
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "移除教师失败");
    } finally {
      setBusy(false);
    }
  }

  if (error) {
    return (
      <div style={{ flex: 1, padding: 40 }}>
        <Button variant="ghost" size="sm" onClick={onBack}>← 返回</Button>
        <div style={{ marginTop: 20, color: "var(--mk-danger)", fontSize: 14, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
      </div>
    );
  }
  if (!detail) {
    return <div style={{ flex: 1 }} />;
  }

  const c = detail.class;
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "32px 40px 60px" }}>
        <Button variant="ghost" size="sm" onClick={onBack}>← 返回</Button>

        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 18 }}>
          {renaming ? (
            <>
              <input
                autoFocus
                value={draftName}
                onChange={(e) => setDraftName(e.target.value)}
                style={{ fontSize: 18, fontWeight: 700, color: "var(--mk-ink)", border: "1px solid var(--mk-input-border)", borderRadius: "var(--mk-radius-md)", padding: "8px 12px", outline: "none" }}
              />
              <Button variant="secondary" size="sm" onClick={() => void doRename()} disabled={busy}>保存</Button>
              <Button variant="ghost" size="sm" onClick={() => setRenaming(false)}>取消</Button>
            </>
          ) : (
            <>
              <div style={{ fontSize: 24, fontWeight: 800, color: "var(--mk-ink)", letterSpacing: "-.01em" }}>{c.name}</div>
              <Button variant="secondary" size="sm" onClick={() => { setDraftName(c.name); setRenaming(true); }}>改名</Button>
            </>
          )}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14, flexWrap: "wrap" }}>
          <span style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-600)", fontWeight: 700, fontSize: 13, padding: "6px 12px", borderRadius: "var(--mk-radius-md)" }}>邀请码 {c.join_code}</span>
          <Button variant="secondary" size="sm" onClick={() => void navigator.clipboard?.writeText(c.join_code)}>复制</Button>
          <Button variant="secondary" size="sm" onClick={() => setConfirmRegen(true)}>轮换</Button>
          {confirmRegen && (
            <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 13, color: "var(--mk-danger)", fontWeight: 600 }}>
              轮换后旧邀请码立即失效，确定？
              <Button variant="danger" size="sm" onClick={() => void doRegen()} disabled={busy}>确认轮换</Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirmRegen(false)}>取消</Button>
            </span>
          )}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
          <label htmlFor="class-grade-picker" style={{ fontSize: 13, fontWeight: 700, color: "var(--mk-ink)" }}>年级</label>
          {/* 🚨 不传 placeholder：共用 Select 会把它渲染成一个 disabled 的
              value="" 选项，排在 CLASS_GRADE_OPTIONS[0]（未填写）前面，于是
              「未填写」这个状态永远显示不出来。可见的 <label> 代替它。 */}
          <Select
            id="class-grade-picker"
            data-testid="class-grade-picker"
            value={c.grade}
            onChange={(v) => void doSetGrade(v)}
            options={CLASS_GRADE_OPTIONS}
            disabled={busy}
          />
        </div>

        {mutationError && (
          <div style={{ marginTop: 12, color: "var(--mk-danger)", fontSize: 13, fontWeight: 600 }}>{mutationError}</div>
        )}

        {role === "admin" && (
          <div style={{ marginTop: 26 }}>
            <div style={{ fontSize: 14, fontWeight: 700, color: "var(--mk-ink)", marginBottom: 10 }}>任课教师</div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center" }}>
              {detail.teachers.map((t) => (
                <span key={t.id} style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "var(--mk-accent-50)", color: "var(--mk-accent-600)", fontSize: 13, fontWeight: 600, padding: "6px 10px", borderRadius: "var(--mk-radius-md)" }}>
                  {t.display_name}
                  <button
                    aria-label={`移除教师 ${t.display_name}`}
                    onClick={() => void doRemoveTeacher(t.id)}
                    disabled={busy}
                    style={{ background: "transparent", border: "none", color: "var(--mk-accent-600)", fontSize: 14, cursor: "pointer", fontFamily: "inherit", lineHeight: 1, padding: 0 }}
                  >
                    ✕
                  </button>
                </span>
              ))}
              {detail.teachers.length === 0 && <span style={{ color: "var(--mk-muted)", fontSize: 13 }}>暂无任课教师</span>}
            </div>
            <div style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 12 }}>
              <select
                data-testid="assign-teacher-picker"
                value={assignId}
                onClick={loadTeacherOptions}
                onFocus={loadTeacherOptions}
                onChange={(e) => setAssignId(e.target.value)}
                style={{ minWidth: 200, border: "1px solid var(--mk-input-border)", borderRadius: "var(--mk-radius-md)", padding: "9px 12px", fontSize: 13.5, color: "var(--mk-ink)", outline: "none", background: "var(--mk-surface)" }}
              >
                <option value="">选择教师添加…</option>
                {(teacherOptions ?? []).map((t) => (
                  <option key={t.id} value={t.id}>{t.display_name}（{t.email}）</option>
                ))}
              </select>
              <Button variant="secondary" size="sm" onClick={() => void doAssignTeacher()} disabled={busy || !assignId}>添加</Button>
            </div>
          </div>
        )}

        <div style={{ display: "flex", gap: 8, marginTop: 26 }}>
          <Button variant={subTab === "weekly" ? "primary" : "secondary"} size="sm" onClick={() => setSubTab("weekly")}>周报</Button>
          <Button variant={subTab === "roster" ? "primary" : "secondary"} size="sm" onClick={() => setSubTab("roster")}>全部学生</Button>
        </div>

        {subTab === "weekly" && (
          <ClassWeeklyView client={client} classId={classId} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />
        )}

        {subTab === "roster" && (
          detail.roster.length === 0 ? (
            <div style={{ marginTop: 30, color: "var(--mk-muted)", fontSize: 14.5, lineHeight: 1.7 }}>
              还没有学生加入。分享邀请码 {c.join_code} 让学生加入。
            </div>
          ) : rosterReportError ? (
            <div style={{ marginTop: 30, color: "var(--mk-danger)", fontSize: 14, fontWeight: 600 }}>
              学生数据加载失败：{rosterReportError} · <span onClick={loadRosterReport} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span>
            </div>
          ) : classRoster === null ? (
            <div style={{ marginTop: 30, color: "var(--mk-muted)", fontSize: 14.5 }}>加载中…</div>
          ) : (
            <>
              <LiveHeaderStrip header={classRoster.header} />
              <ClassRosterTable
                roster={classRoster.roster}
                onOpenStudent={onOpenStudent}
                onRemove={doRemove}
                confirmRemove={confirmRemove}
                setConfirmRemove={setConfirmRemove}
                busy={busy}
              />
            </>
          )
        )}
      </div>
    </div>
  );
}
