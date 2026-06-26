import { useEffect, useState } from "react";
import type { ApiClient, Overview } from "../api";
import { ApiError } from "../api";

type Client = Pick<ApiClient, "getOverview">;

const STATS: { key: keyof Overview["counts"]; label: string }[] = [
  { key: "student", label: "学生" },
  { key: "teacher", label: "教师" },
  { key: "class", label: "班级" },
  { key: "task", label: "任务" },
  { key: "evaluation", label: "评估" },
  { key: "active_student", label: "活跃学生" },
];

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "12px", borderBottom: "1px solid #F2F3F7" };

export function OverviewView({ client }: { client: Client }) {
  const [data, setData] = useState<Overview | null>(null);
  const [error, setError] = useState<string | null>(null);

  function load() {
    setError(null);
    client.getOverview().then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client]);

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>概览</div>
        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}
        {data && (
          <>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 16, marginTop: 24 }}>
              {STATS.map(({ key, label }) => (
                <div key={key} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}>
                  <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>{label}</div>
                  <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6 }}>{data.counts[key]}</div>
                </div>
              ))}
            </div>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "34px 0 14px" }}>用量（按档位）</div>
            {data.usage_by_tier.length === 0 ? (
              <div style={{ color: "#8A92A3", fontSize: 14 }}>暂无用量。</div>
            ) : (
              <table style={{ width: "100%", borderCollapse: "collapse", background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
                <thead>
                  <tr>
                    <th style={TH}>档位</th>
                    <th style={TH}>输入 token</th>
                    <th style={TH}>输出 token</th>
                    <th style={TH}>成本</th>
                  </tr>
                </thead>
                <tbody>
                  {data.usage_by_tier.map((u) => (
                    <tr key={u.tier}>
                      <td style={{ ...TD, fontWeight: 600 }}>{u.tier}</td>
                      <td style={TD}>{u.prompt_tokens}</td>
                      <td style={TD}>{u.completion_tokens}</td>
                      <td style={TD}>{u.cost}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </>
        )}
      </div>
    </div>
  );
}
