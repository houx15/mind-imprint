import { useEffect, useState } from "react";
import type { ApiClient, StudentDetail, StudentRecord } from "../api";
import { ApiError } from "../api";
import { badgeColor } from "./badgeColor";

type Client = Pick<ApiClient, "getStudentDetail">;

type RecordTab = "all" | "course" | "chat" | "project";

const TABS: { key: RecordTab; label: string }[] = [
  { key: "all", label: "全部" },
  { key: "course", label: "课程" },
  { key: "chat", label: "对话" },
  { key: "project", label: "项目" },
];

const TYPE_META: Record<StudentRecord["surface"], { label: string; fg: string; bg: string }> = {
  course: { label: "课程", fg: "#3E7CA8", bg: "#E1EDF5" },
  chat: { label: "对话", fg: "#C68A3A", bg: "#F6EED9" },
  project: { label: "项目", fg: "#3E8A6E", bg: "#E4F0EA" },
};

export function StudentDetailView({
  client,
  classId,
  userId,
  onBack,
  onOpenReport,
}: {
  client: Client;
  classId: string;
  userId: string;
  onBack: () => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string) => void;
}) {
  const [detail, setDetail] = useState<StudentDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<RecordTab>("all");

  function load() {
    setError(null);
    client.getStudentDetail(classId, userId).then(setDetail).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client, classId, userId]);

  if (error) {
    return (
      <div style={{ flex: 1, padding: 40 }}>
        <button onClick={onBack} style={backBtn}>← 全部学生</button>
        <div style={{ marginTop: 20, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
      </div>
    );
  }
  if (!detail) {
    return <div style={{ flex: 1 }} />;
  }

  const { student, usage } = detail;
  const dColors = student.unrated ? { fg: "#8A92A3", bg: "#EEF0F4" } : badgeColor(student.dBadge);
  const aColors = student.unrated ? { fg: "#8A92A3", bg: "#EEF0F4" } : badgeColor(student.aBadge);

  const primaryReport = detail.records.find((r) => r.surface === "project" && r.hasReport);

  const records = tab === "all" ? detail.records : detail.records.filter((r) => r.surface === tab);

  const stats: { label: string; value: number; unit: string }[] = [
    { label: "本周活跃天数", value: usage.activeDays, unit: "天" },
    { label: "AI 对话轮次", value: usage.turns, unit: "轮" },
    { label: "生成能力报告", value: usage.reportCount, unit: "份" },
    { label: "完成课程节", value: usage.courseCount, unit: "节" },
  ];

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "26px 30px 64px" }}>
        <div onClick={onBack} style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, color: "#6C7488", fontWeight: 600, cursor: "pointer", marginBottom: 16 }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round"><path d="M19 12H5M11 18l-6-6 6-6" /></svg>
          全部学生
        </div>

        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 18, padding: 24, boxShadow: "0 4px 20px rgba(20,30,60,.04)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <div style={{ width: 58, height: 58, borderRadius: "50%", background: dColors.bg, color: student.avatarColor, display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800, fontSize: 24, flex: "none" }}>
              {student.displayName.slice(0, 1)}
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 24, fontWeight: 800, color: "#1C2333" }}>{student.displayName}</div>
            </div>
            <div style={{ display: "flex", gap: 9 }}>
              <div style={{ textAlign: "center", background: dColors.bg, borderRadius: 12, padding: "9px 16px" }}>
                <div style={{ fontSize: 10.5, color: "#6C7488", fontWeight: 700 }}>D 轴</div>
                <div style={{ fontSize: 18, fontWeight: 800, color: dColors.fg, marginTop: 2 }}>{student.dBadge}</div>
              </div>
              <div style={{ textAlign: "center", background: aColors.bg, borderRadius: 12, padding: "9px 16px" }}>
                <div style={{ fontSize: 10.5, color: "#6C7488", fontWeight: 700 }}>A 轴</div>
                <div style={{ fontSize: 18, fontWeight: 800, color: aColors.fg, marginTop: 2 }}>{student.aBadge}</div>
              </div>
            </div>
          </div>

          <div style={{ marginTop: 20, display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 12 }}>
            {stats.map((s) => (
              <div key={s.label} style={{ background: "#FAFBFD", border: "1px solid #EEF0F4", borderRadius: 12, padding: "14px 16px" }}>
                <div style={{ fontSize: 11.5, color: "#6C7488", fontWeight: 600 }}>{s.label}</div>
                <div style={{ marginTop: 6, display: "flex", alignItems: "baseline", gap: 4 }}>
                  <span style={{ fontSize: 22, fontWeight: 800, color: "#1C2333" }}>{s.value}</span>
                  <span style={{ fontSize: 12, color: "#9198A8", fontWeight: 600 }}>{s.unit}</span>
                </div>
              </div>
            ))}
          </div>

          {primaryReport && (
            <div style={{ marginTop: 20, display: "flex", gap: 10, flexWrap: "wrap" }}>
              <button
                onClick={() => onOpenReport(primaryReport.surface, primaryReport.scopeId, student.displayName)}
                style={{ display: "inline-flex", alignItems: "center", gap: 7, background: "#2A3B7A", color: "#fff", fontSize: 13.5, fontWeight: 700, padding: "11px 18px", borderRadius: 11, border: "none", cursor: "pointer", fontFamily: "inherit" }}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M12 20V10M18 20V4M6 20v-4" /></svg>
                查看完整能力报告
              </button>
            </div>
          )}
        </div>

        <div style={{ marginTop: 26, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333" }}>平台使用记录</div>
          <div style={{ display: "flex", background: "#F1F2F6", borderRadius: 10, padding: 3 }}>
            {TABS.map((t) => (
              <div
                key={t.key}
                role="tab"
                aria-selected={tab === t.key}
                onClick={() => setTab(t.key)}
                style={{
                  fontSize: 12.5,
                  fontWeight: 700,
                  padding: "6px 14px",
                  borderRadius: 8,
                  cursor: "pointer",
                  color: tab === t.key ? "#2A3B7A" : "#8A92A3",
                  background: tab === t.key ? "#fff" : "transparent",
                  boxShadow: tab === t.key ? "0 1px 4px rgba(20,30,60,.08)" : "none",
                }}
              >
                {t.label}
              </div>
            ))}
          </div>
        </div>
        <div style={{ fontSize: 12, color: "#9198A8", marginTop: 6 }}>对话（chat）只有生成了能力报告才可查看，否则仅显示标题。</div>

        <div style={{ marginTop: 14, display: "flex", flexDirection: "column", gap: 10 }}>
          {records.map((r) => {
            const tm = TYPE_META[r.surface];
            return (
              <div key={`${r.surface}:${r.scopeId}`} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "16px 18px", display: "flex", alignItems: "center", gap: 16 }}>
                <span style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 52, flex: "none", background: tm.bg, color: tm.fg, fontSize: 12, fontWeight: 800, padding: "5px 0", borderRadius: 8 }}>
                  {tm.label}
                </span>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 700, color: "#1C2333", lineHeight: 1.4 }}>{r.title}</div>
                  {/* r.date is an RFC3339 timestamp from the server; render only
                      the date part (no locale formatting needed — keep it simple). */}
                  <div style={{ fontSize: 12, color: "#9198A8", marginTop: 3 }}>{r.date.slice(0, 10)} · {r.status}</div>
                </div>
                {r.hasReport && (
                  <div
                    onClick={() => onOpenReport(r.surface, r.scopeId, student.displayName)}
                    style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 700, color: "#2A3B7A", cursor: "pointer", flex: "none" }}
                  >
                    查看报告
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

const backBtn: React.CSSProperties = { background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 };
