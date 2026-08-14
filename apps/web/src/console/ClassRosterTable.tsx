import type { RosterEntry } from "../api";
import { Button } from "@/ui";

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "var(--mk-muted)", padding: "10px 12px", borderBottom: "1px solid var(--mk-border)" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "var(--mk-ink)", padding: "12px", borderBottom: "1px solid var(--mk-border)" };

export function ClassRosterTable({
  roster,
  onOpenStudent,
  onRemove,
  confirmRemove,
  setConfirmRemove,
  busy,
}: {
  roster: RosterEntry[];
  onOpenStudent: (userId: string) => void;
  onRemove: (studentId: string) => void | Promise<void>;
  confirmRemove: string | null;
  setConfirmRemove: (id: string | null) => void;
  busy: boolean;
}) {
  return (
    <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 26, background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: "var(--mk-radius-lg)", overflow: "hidden" }}>
      <thead>
        <tr>
          <th style={TH}>学生</th>
          <th style={TH}>进行中项目</th>
          <th style={TH}>能力报告</th>
          <th style={TH}>完成课程</th>
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
                    background: s.avatarColor,
                    color: "var(--mk-surface)",
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
            <td style={TD}>{s.activeProjects}</td>
            <td style={TD}>{s.reportCount}</td>
            <td style={TD}>{s.coursesFinished}</td>
            <td style={{ ...TD, textAlign: "right" }} onClick={(e) => e.stopPropagation()}>
              {confirmRemove === s.id ? (
                <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 12.5, color: "var(--mk-danger)", fontWeight: 600 }}>
                  将 {s.displayName} 移出班级？仅解除关联，不删除其账号或作品。
                  <Button variant="danger" size="sm" onClick={() => void onRemove(s.id)} disabled={busy}>确认移除</Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirmRemove(null)}>取消</Button>
                </span>
              ) : (
                <button
                  aria-label={`移除 ${s.displayName}`}
                  onClick={() => setConfirmRemove(s.id)}
                  style={{ background: "transparent", border: "none", color: "var(--mk-faint)", fontSize: 16, cursor: "pointer", fontFamily: "inherit", lineHeight: 1 }}
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
