import type { RosterReportEntry } from "../api";
import { badgeColor } from "./badgeColor";

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "12px", borderBottom: "1px solid #F2F3F7" };

function Badge({ text }: { text: string }) {
  const { fg, bg } = badgeColor(text);
  return (
    <span style={{ display: "inline-flex", minWidth: 34, justifyContent: "center", background: bg, color: fg, fontSize: 12, fontWeight: 800, padding: "3px 9px", borderRadius: 8 }}>
      {text}
    </span>
  );
}

const backBtn: React.CSSProperties = { background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 };
const dangerBtn: React.CSSProperties = { background: "#C76B6B", border: "none", color: "#fff", fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", padding: "5px 11px", borderRadius: 9 };

export function ClassRosterTable({
  roster,
  onOpenStudent,
  onRemove,
  confirmRemove,
  setConfirmRemove,
  busy,
}: {
  roster: RosterReportEntry[];
  onOpenStudent: (userId: string) => void;
  onRemove: (studentId: string) => void | Promise<void>;
  confirmRemove: string | null;
  setConfirmRemove: (id: string | null) => void;
  busy: boolean;
}) {
  return (
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
        {roster.map((s) => (
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
                  <button onClick={() => void onRemove(s.id)} disabled={busy} style={dangerBtn}>确认移除</button>
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
  );
}
