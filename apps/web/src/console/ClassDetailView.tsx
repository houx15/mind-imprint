import { useEffect, useState } from "react";
import type { ApiClient, ClassDetail, RosterReportEntry, Teacher } from "../api";
import { ApiError } from "../api";

type Client = Pick<ApiClient, "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment" | "listTeachers" | "assignTeacher" | "removeTeacher" | "getClassRosterReport">;

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "12px", borderBottom: "1px solid #F2F3F7" };

// Mirrors dc.html's levelColor/tint helpers verbatim (docs/design/teacher end/project/思维印记 教师端.dc.html:1513),
// including its FIXED test order: L1/0级/1级 -> red, then L2/2级 -> orange, then L4/4级/5级 -> green,
// then L3/3级 -> blue, else grey. A decimal A-axis value ("4.2") is first rounded and prefixed to "L4"
// (mirrors dc.html's `levelColor('L'+Math.round(parseFloat(a)))`) before running through the same chain.
function badgeColor(rawText: string): { fg: string; bg: string } {
  let text = rawText;
  if (!/L\d/.test(text)) {
    const dMatch = text.match(/^(\d+(?:\.\d+)?)/);
    if (dMatch) text = `L${Math.round(Number(dMatch[1]))}`;
  }
  if (/L1|(?:^|[^\d])0级|(?:^|[^\d])1级/.test(text)) return { fg: "#C4574D", bg: "#F7E6E4" };
  if (/L2|2级/.test(text)) return { fg: "#C68A3A", bg: "#F6EED9" };
  if (/L4|4级|5级/.test(text)) return { fg: "#3E8A6E", bg: "#E4F0EA" };
  if (/L3|3级/.test(text)) return { fg: "#3E7CA8", bg: "#E1EDF5" };
  return { fg: "#8A92A3", bg: "#EEF0F4" };
}

function Badge({ text }: { text: string }) {
  const { fg, bg } = badgeColor(text);
  return (
    <span style={{ display: "inline-flex", minWidth: 34, justifyContent: "center", background: bg, color: fg, fontSize: 12, fontWeight: 800, padding: "3px 9px", borderRadius: 8 }}>
      {text}
    </span>
  );
}

export function ClassDetailView({
  client,
  classId,
  onBack,
  now,
  role,
  onOpenStudent,
}: {
  client: Client;
  classId: string;
  onBack: () => void;
  now?: number;
  role?: string;
  onOpenStudent: (userId: string) => void;
}) {
  const [detail, setDetail] = useState<ClassDetail | null>(null);
  const [rosterReport, setRosterReport] = useState<RosterReportEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [renaming, setRenaming] = useState(false);
  const [draftName, setDraftName] = useState("");
  const [confirmRegen, setConfirmRegen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null); // student id
  const [busy, setBusy] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);

  function load() {
    setError(null);
    client.getClass(classId).then(setDetail).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
    client.getClassRosterReport(classId).then(setRosterReport).catch(() => setRosterReport([]));
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

        {detail.roster.length === 0 ? (
          <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
            还没有学生加入。分享邀请码 {c.join_code} 让学生加入。
          </div>
        ) : (
          <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 26, background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
            <thead>
              <tr>
                <th style={TH}>学生</th>
                <th style={TH}>本周活跃</th>
                <th style={TH}>对话轮次</th>
                <th style={TH}>D 轴</th>
                <th style={TH}>A 轴</th>
                <th style={TH}>能力报告</th>
                <th style={TH} aria-label="操作" />
              </tr>
            </thead>
            <tbody>
              {(rosterReport ?? []).map((s) => (
                <tr key={s.id} onClick={() => onOpenStudent(s.id)} style={{ cursor: "pointer" }}>
                  <td style={TD}>
                    <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                      <div
                        style={{
                          width: 32,
                          height: 32,
                          borderRadius: "50%",
                          background: badgeColor(s.dBadge).bg,
                          color: s.avatarColor,
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          fontWeight: 800,
                          fontSize: 14,
                          flexShrink: 0,
                        }}
                      >
                        {s.displayName.slice(0, 1)}
                      </div>
                      <span style={{ fontWeight: 700 }}>{s.displayName}</span>
                    </div>
                  </td>
                  <td style={TD}>{s.activeDays} 天</td>
                  <td style={TD}>{s.turns}</td>
                  <td style={TD}><Badge text={s.dBadge} /></td>
                  <td style={TD}><Badge text={s.aBadge} /></td>
                  <td style={{ ...TD, fontWeight: 700, color: s.hasReport ? "#3E8A6E" : "#8A92A3" }}>
                    {s.hasReport ? "✓ 已生成" : "—"}
                  </td>
                  <td style={{ ...TD, textAlign: "right" }} onClick={(e) => e.stopPropagation()}>
                    {confirmRemove === s.id ? (
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 12.5, color: "#C76B6B", fontWeight: 600 }}>
                        将 {s.displayName} 移出班级？仅解除关联，不删除其账号或作品。
                        <button onClick={() => void doRemove(s.id)} disabled={busy} style={dangerBtn}>确认移除</button>
                        <button onClick={() => setConfirmRemove(null)} style={backBtn}>取消</button>
                      </span>
                    ) : (
                      <button
                        aria-label={`移除 ${s.displayName}`}
                        onClick={() => setConfirmRemove(s.id)}
                        style={{ background: "transparent", border: "none", color: "#B7BECC", fontSize: 16, cursor: "pointer", fontFamily: "inherit", lineHeight: 1 }}
                      >
                        ✕
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

const backBtn: React.CSSProperties = { background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 };
const chipBtn: React.CSSProperties = { background: "transparent", border: "1px solid #D7DCF2", color: "#2A3B7A", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: "6px 12px", borderRadius: 10 };
const dangerBtn: React.CSSProperties = { background: "#C76B6B", border: "none", color: "#fff", fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", padding: "5px 11px", borderRadius: 9 };
