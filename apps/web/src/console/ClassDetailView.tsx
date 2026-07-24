import { useEffect, useState } from "react";
import type { ApiClient, ClassDetail, RosterReportEntry, Teacher } from "../api";
import { ApiError } from "../api";
import { ClassRosterTable } from "./ClassRosterTable";
import { ClassWeeklyView } from "./ClassWeeklyView";

type Client = Pick<
  ApiClient,
  | "getClass"
  | "renameClass"
  | "regenerateJoinCode"
  | "removeEnrollment"
  | "listTeachers"
  | "assignTeacher"
  | "removeTeacher"
  | "getClassRosterReport"
  | "getClassWeeklyReport"
  | "generateClassWeeklyProse"
>;

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
  onOpenReport: (surface: string, scopeId: string, displayName: string) => void;
}) {
  const [detail, setDetail] = useState<ClassDetail | null>(null);
  const [rosterReport, setRosterReport] = useState<RosterReportEntry[] | null>(null);
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
    client.getClassRosterReport(classId).then(setRosterReport).catch((e) => {
      setRosterReport(null);
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

  async function doRemove(studentId: string) {
    setMutationError(null);
    setBusy(true);
    try {
      await client.removeEnrollment(classId, studentId);
      setDetail((d) => (d ? { ...d, roster: d.roster.filter((s) => s.id !== studentId) } : d));
      setRosterReport((r) => (r ? r.filter((s) => s.id !== studentId) : r));
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
        <button onClick={onBack} style={backBtn}>← 返回</button>
        <div style={{ marginTop: 20, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
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
        <button onClick={onBack} style={backBtn}>← 返回</button>

        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 18 }}>
          {renaming ? (
            <>
              <input
                autoFocus
                value={draftName}
                onChange={(e) => setDraftName(e.target.value)}
                style={{ fontSize: 18, fontWeight: 700, color: "#1C2333", border: "1px solid #E1E4ED", borderRadius: 10, padding: "8px 12px", outline: "none" }}
              />
              <button onClick={() => void doRename()} disabled={busy} style={chipBtn}>保存</button>
              <button onClick={() => setRenaming(false)} style={backBtn}>取消</button>
            </>
          ) : (
            <>
              <div style={{ fontSize: 24, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>{c.name}</div>
              <button onClick={() => { setDraftName(c.name); setRenaming(true); }} style={chipBtn}>改名</button>
            </>
          )}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14, flexWrap: "wrap" }}>
          <span style={{ background: "#EDEFF9", color: "#2A3B7A", fontWeight: 700, fontSize: 13, padding: "6px 12px", borderRadius: 10 }}>邀请码 {c.join_code}</span>
          <button onClick={() => void navigator.clipboard?.writeText(c.join_code)} style={chipBtn}>复制</button>
          <button onClick={() => setConfirmRegen(true)} style={chipBtn}>轮换</button>
          {confirmRegen && (
            <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 13, color: "#C76B6B", fontWeight: 600 }}>
              轮换后旧邀请码立即失效，确定？
              <button onClick={() => void doRegen()} disabled={busy} style={dangerBtn}>确认轮换</button>
              <button onClick={() => setConfirmRegen(false)} style={backBtn}>取消</button>
            </span>
          )}
        </div>

        {mutationError && (
          <div style={{ marginTop: 12, color: "#C76B6B", fontSize: 13, fontWeight: 600 }}>{mutationError}</div>
        )}

        {role === "admin" && (
          <div style={{ marginTop: 26 }}>
            <div style={{ fontSize: 14, fontWeight: 700, color: "#1C2333", marginBottom: 10 }}>任课教师</div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center" }}>
              {detail.teachers.map((t) => (
                <span key={t.id} style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "#EDEFF9", color: "#2A3B7A", fontSize: 13, fontWeight: 600, padding: "6px 10px", borderRadius: 10 }}>
                  {t.display_name}
                  <button
                    aria-label={`移除教师 ${t.display_name}`}
                    onClick={() => void doRemoveTeacher(t.id)}
                    disabled={busy}
                    style={{ background: "transparent", border: "none", color: "#2A3B7A", fontSize: 14, cursor: "pointer", fontFamily: "inherit", lineHeight: 1, padding: 0 }}
                  >
                    ✕
                  </button>
                </span>
              ))}
              {detail.teachers.length === 0 && <span style={{ color: "#8A92A3", fontSize: 13 }}>暂无任课教师</span>}
            </div>
            <div style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 12 }}>
              <select
                data-testid="assign-teacher-picker"
                value={assignId}
                onClick={loadTeacherOptions}
                onFocus={loadTeacherOptions}
                onChange={(e) => setAssignId(e.target.value)}
                style={{ minWidth: 200, border: "1px solid #E1E4ED", borderRadius: 11, padding: "9px 12px", fontSize: 13.5, color: "#1C2333", outline: "none", background: "#fff" }}
              >
                <option value="">选择教师添加…</option>
                {(teacherOptions ?? []).map((t) => (
                  <option key={t.id} value={t.id}>{t.display_name}（{t.email}）</option>
                ))}
              </select>
              <button onClick={() => void doAssignTeacher()} disabled={busy || !assignId} style={chipBtn}>添加</button>
            </div>
          </div>
        )}

        <div style={{ display: "flex", gap: 8, marginTop: 26 }}>
          <button onClick={() => setSubTab("weekly")} style={subTab === "weekly" ? activeTabBtn : chipBtn}>周报</button>
          <button onClick={() => setSubTab("roster")} style={subTab === "roster" ? activeTabBtn : chipBtn}>全部学生</button>
        </div>

        {subTab === "weekly" && (
          <ClassWeeklyView client={client} classId={classId} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />
        )}

        {subTab === "roster" && (
          detail.roster.length === 0 ? (
            <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
              还没有学生加入。分享邀请码 {c.join_code} 让学生加入。
            </div>
          ) : rosterReportError ? (
            <div style={{ marginTop: 30, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>
              学生数据加载失败：{rosterReportError} · <span onClick={loadRosterReport} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span>
            </div>
          ) : rosterReport === null ? (
            <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5 }}>加载中…</div>
          ) : (
            <ClassRosterTable
              roster={rosterReport}
              onOpenStudent={onOpenStudent}
              onRemove={doRemove}
              confirmRemove={confirmRemove}
              setConfirmRemove={setConfirmRemove}
              busy={busy}
            />
          )
        )}
      </div>
    </div>
  );
}

const backBtn: React.CSSProperties = { background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 };
const chipBtn: React.CSSProperties = { background: "transparent", border: "1px solid #D7DCF2", color: "#2A3B7A", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: "6px 12px", borderRadius: 10 };
const activeTabBtn: React.CSSProperties = { ...chipBtn, background: "#2A3B7A", borderColor: "#2A3B7A", color: "#fff" };
const dangerBtn: React.CSSProperties = { background: "#C76B6B", border: "none", color: "#fff", fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", padding: "5px 11px", borderRadius: 9 };
