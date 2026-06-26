import { useEffect, useState } from "react";
import type { ApiClient, ClassDetail } from "../api";
import { ApiError } from "../api";
import { relativeTime } from "./time";

type Client = Pick<ApiClient, "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment">;

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "12px", borderBottom: "1px solid #F2F3F7" };

export function ClassDetailView({
  client,
  classId,
  onBack,
  now,
}: {
  client: Client;
  classId: string;
  onBack: () => void;
  now?: number;
}) {
  const _now = now ?? Date.now();
  const [detail, setDetail] = useState<ClassDetail | null>(null);
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
      setConfirmRemove(null);
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "移除失败");
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

        {detail.roster.length === 0 ? (
          <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
            还没有学生加入。分享邀请码 {c.join_code} 让学生加入。
          </div>
        ) : (
          <>
            <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 26, background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
              <thead>
                <tr>
                  <th style={TH}>姓名</th>
                  <th style={TH}>邮箱</th>
                  <th style={TH}>最近活跃</th>
                  <th style={TH}>任务</th>
                  <th style={TH}>评估</th>
                  <th style={TH}>卡片</th>
                  <th style={TH} aria-label="操作" />
                </tr>
              </thead>
              <tbody>
                {detail.roster.map((s) => (
                  <tr key={s.id}>
                    <td style={{ ...TD, fontWeight: 600 }}>{s.display_name}</td>
                    <td style={{ ...TD, color: "#6B7384" }}>{s.email}</td>
                    <td style={TD}>{relativeTime(s.last_active_at, _now)}</td>
                    <td style={TD}>{s.task_count}</td>
                    <td style={TD}>{s.evaluation_count}</td>
                    <td style={TD}>{s.card_count}</td>
                    <td style={{ ...TD, textAlign: "right" }}>
                      {confirmRemove === s.id ? (
                        <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 12.5, color: "#C76B6B", fontWeight: 600 }}>
                          将 {s.display_name} 移出班级？仅解除关联，不删除其账号或作品。
                          <button onClick={() => void doRemove(s.id)} disabled={busy} style={dangerBtn}>确认移除</button>
                          <button onClick={() => setConfirmRemove(null)} style={backBtn}>取消</button>
                        </span>
                      ) : (
                        <button
                          aria-label={`移除 ${s.display_name}`}
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
            <div style={{ marginTop: 12, fontSize: 12, color: "#9AA1B0" }}>行不可点入 — 暂无学生作品详情页。</div>
          </>
        )}
      </div>
    </div>
  );
}

const backBtn: React.CSSProperties = { background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 };
const chipBtn: React.CSSProperties = { background: "transparent", border: "1px solid #D7DCF2", color: "#2A3B7A", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: "6px 12px", borderRadius: 10 };
const dangerBtn: React.CSSProperties = { background: "#C76B6B", border: "none", color: "#fff", fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", padding: "5px 11px", borderRadius: 9 };
