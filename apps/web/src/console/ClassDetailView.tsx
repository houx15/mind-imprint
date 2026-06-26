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

  function load() {
    setError(null);
    client.getClass(classId).then(setDetail).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client, classId]);

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
          <div style={{ fontSize: 24, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>{c.name}</div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
          <span style={{ background: "#EDEFF9", color: "#2A3B7A", fontWeight: 700, fontSize: 13, padding: "6px 12px", borderRadius: 10 }}>邀请码 {c.join_code}</span>
          <button onClick={() => void navigator.clipboard?.writeText(c.join_code)} style={chipBtn}>复制</button>
        </div>

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
                    <td style={TD} />
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
