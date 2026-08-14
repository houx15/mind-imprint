import { useEffect, useState } from "react";
import type { ApiClient, StudentDetail, StudentRecord } from "../api";
import { ApiError } from "../api";
import { ArrowLeft, Button, Card, EmptyState, Icon, Segmented } from "@/ui";

type Client = Pick<ApiClient, "getStudentDetail">;

type RecordTab = "all" | "course" | "chat" | "project";

const TABS: { key: RecordTab; label: string }[] = [
  { key: "all", label: "全部" },
  { key: "course", label: "课程" },
  { key: "chat", label: "对话" },
  { key: "project", label: "项目" },
];

const TYPE_META: Record<StudentRecord["surface"], { label: string; fg: string; bg: string }> = {
  course: { label: "课程", fg: "var(--mk-info)", bg: "var(--mk-info-bg)" },
  chat: { label: "对话", fg: "var(--mk-warning)", bg: "var(--mk-warning-bg)" },
  project: { label: "项目", fg: "var(--mk-success)", bg: "var(--mk-success-bg)" },
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
        <Button variant="ghost" size="sm" iconStart={<Icon icon={ArrowLeft} size={16} />} onClick={onBack}>全部学生</Button>
        <div style={{ marginTop: 20 }}>
          <EmptyState illustration="completed" title="加载失败" body={error} action={{ label: "重试", onClick: load }} />
        </div>
      </div>
    );
  }
  if (!detail) {
    return <div style={{ flex: 1 }} />;
  }

  const { student, usage } = detail;

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
        <Button variant="ghost" size="sm" iconStart={<Icon icon={ArrowLeft} size={16} />} onClick={onBack}>全部学生</Button>

        <Card className="mt-4 p-6">
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <div
              style={{
                width: 58,
                height: 58,
                borderRadius: "50%",
                background: student.avatarColor,
                color: "var(--mk-surface)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontWeight: 800,
                fontSize: 24,
                flex: "none",
              }}
            >
              {student.displayName.slice(0, 1)}
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 24, fontWeight: 800, color: "var(--mk-ink)" }}>{student.displayName}</div>
            </div>
          </div>

          <div style={{ marginTop: 20, display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 12 }}>
            {stats.map((s) => (
              <div key={s.label} style={{ background: "var(--mk-paper)", border: "1px solid var(--mk-border)", borderRadius: "var(--mk-radius-md)", padding: "14px 16px" }}>
                <div style={{ fontSize: 11.5, color: "var(--mk-muted)", fontWeight: 600 }}>{s.label}</div>
                <div style={{ marginTop: 6, display: "flex", alignItems: "baseline", gap: 4 }}>
                  <span style={{ fontSize: 22, fontWeight: 800, color: "var(--mk-ink)" }}>{s.value}</span>
                  <span style={{ fontSize: 12, color: "var(--mk-faint)", fontWeight: 600 }}>{s.unit}</span>
                </div>
              </div>
            ))}
          </div>

          {primaryReport && (
            <div style={{ marginTop: 20, display: "flex", gap: 10, flexWrap: "wrap" }}>
              <Button
                variant="primary"
                onClick={() => onOpenReport(primaryReport.surface, primaryReport.scopeId, student.displayName)}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M12 20V10M18 20V4M6 20v-4" /></svg>
                查看完整能力报告
              </Button>
            </div>
          )}
        </Card>

        <div style={{ marginTop: 26, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 18, fontWeight: 800, color: "var(--mk-ink)" }}>平台使用记录</div>
          <Segmented options={TABS.map((t) => ({ value: t.key, label: t.label }))} value={tab} onChange={(v) => setTab(v as RecordTab)} />
        </div>
        <div style={{ fontSize: 12, color: "var(--mk-faint)", marginTop: 6 }}>对话（chat）只有生成了能力报告才可查看，否则仅显示标题。</div>

        <div style={{ marginTop: 14, display: "flex", flexDirection: "column", gap: 10 }}>
          {records.map((r) => {
            const tm = TYPE_META[r.surface];
            return (
              <Card key={`${r.surface}:${r.scopeId}`} className="flex items-center gap-4 p-4">
                <span style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 52, flex: "none", background: tm.bg, color: tm.fg, fontSize: 12, fontWeight: 800, padding: "5px 0", borderRadius: "var(--mk-radius-sm)" }}>
                  {tm.label}
                </span>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 700, color: "var(--mk-ink)", lineHeight: 1.4 }}>{r.title}</div>
                  {/* r.date is an RFC3339 timestamp from the server; render only
                      the date part (no locale formatting needed — keep it simple). */}
                  <div style={{ fontSize: 12, color: "var(--mk-faint)", marginTop: 3 }}>{r.date.slice(0, 10)} · {r.status}</div>
                </div>
                {r.hasReport && (
                  <div
                    onClick={() => onOpenReport(r.surface, r.scopeId, student.displayName)}
                    style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 700, color: "var(--mk-accent-600)", cursor: "pointer", flex: "none" }}
                  >
                    查看报告
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--mk-accent-600)" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      </div>
    </div>
  );
}
