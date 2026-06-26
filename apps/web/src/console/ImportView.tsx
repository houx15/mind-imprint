import { useState } from "react";
import type { ApiClient, ImportRow, ImportResult } from "../api";
import { ApiError } from "../api";
import { parseCsv } from "./csv";

type Client = Pick<ApiClient, "adminImport">;

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "10px 12px", borderBottom: "1px solid #F2F3F7" };

export function ImportView({ client }: { client: Client }) {
  const [rows, setRows] = useState<ImportRow[] | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [badRow, setBadRow] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    setRows(null); setParseError(null); setError(null); setBadRow(null); setResult(null);
    if (!f) return;
    try {
      const text = await f.text();
      setRows(parseCsv(text));
    } catch {
      setParseError("无法解析文件，请检查是否包含 class 列。");
    }
  }

  async function doImport() {
    if (!rows) return;
    setBusy(true); setError(null); setBadRow(null);
    try {
      const r = await client.adminImport(rows);
      setResult(r);
      setRows(null);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
        const row = (err as ApiError & { details?: { row?: number } }).details?.row;
        if (typeof row === "number") setBadRow(row);
      } else {
        setError("导入失败");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 880, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>批量导入</div>
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6 }}>
          上传一个 CSV（列：class · teacher_email · student_email）。系统会创建班级与邀请码，所有人凭码自助注册。
        </div>
        <div style={{ marginTop: 18 }}>
          <input data-testid="csv-input" type="file" accept=".csv,text/csv" onChange={(e) => void onFile(e)} />
        </div>

        {parseError && <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{parseError}</div>}
        {error && <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error}</div>}

        {rows && (
          <>
            <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 20, background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
              <thead><tr><th style={TH}>班级</th><th style={TH}>教师邮箱</th><th style={TH}>学生邮箱</th></tr></thead>
              <tbody>
                {rows.map((r, i) => (
                  <tr key={i} style={badRow === i ? { background: "#FBECEC" } : undefined}>
                    <td style={{ ...TD, fontWeight: 600, color: r.class.trim() ? "#1C2333" : "#C76B6B" }}>{r.class.trim() || "（缺少班级名）"}</td>
                    <td style={{ ...TD, color: "#6B7384" }}>{r.teacher_email ?? ""}</td>
                    <td style={{ ...TD, color: "#6B7384" }}>{r.student_email ?? ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <button onClick={() => void doImport()} disabled={busy} style={{ marginTop: 16, background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 20px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>导入</button>
          </>
        )}

        {result && (
          <>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "30px 0 12px" }}>班级与邀请码</div>
            <table style={{ width: "100%", borderCollapse: "collapse", background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
              <thead><tr><th style={TH}>班级</th><th style={TH}>邀请码</th></tr></thead>
              <tbody>
                {result.classes.map((c) => (
                  <tr key={c.name}><td style={{ ...TD, fontWeight: 600 }}>{c.name}</td><td style={{ ...TD, fontWeight: 700, color: "#2A3B7A" }}>{c.join_code}</td></tr>
                ))}
              </tbody>
            </table>
            {result.teacher_invites.length > 0 && (
              <>
                <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "24px 0 12px" }}>教师邀请码</div>
                <table style={{ width: "100%", borderCollapse: "collapse", background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
                  <thead><tr><th style={TH}>邮箱</th><th style={TH}>邀请码</th></tr></thead>
                  <tbody>
                    {result.teacher_invites.map((t) => (
                      <tr key={t.code}><td style={{ ...TD, color: "#6B7384" }}>{t.email}</td><td style={{ ...TD, fontWeight: 700, color: "#2A3B7A" }}>{t.code}</td></tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </>
        )}
      </div>
    </div>
  );
}
