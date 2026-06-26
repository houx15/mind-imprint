import { useEffect, useState } from "react";
import type { ApiClient, Teacher, TeacherInvite } from "../api";
import { ApiError } from "../api";
import { shortDate } from "./time";

type Client = Pick<ApiClient, "listTeachers" | "listTeacherInvites" | "createTeacherInvite">;

const card: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", boxShadow: "0 1px 3px rgba(20,30,60,.04)" };
const sectionTitle: React.CSSProperties = { fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "30px 0 12px" };

export function TeachersView({ client }: { client: Client }) {
  const [teachers, setTeachers] = useState<Teacher[] | null>(null);
  const [invites, setInvites] = useState<TeacherInvite[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [days, setDays] = useState("");
  const [busy, setBusy] = useState(false);
  const [minted, setMinted] = useState<{ code: string; expires_at: string } | null>(null);

  function load() {
    setError(null);
    client.listTeachers().then(setTeachers).catch((e) => setError(e instanceof ApiError ? e.message : "加载失败"));
    client.listTeacherInvites().then(setInvites).catch((e) => setError(e instanceof ApiError ? e.message : "加载失败"));
  }
  useEffect(load, [client]);

  async function mint() {
    setBusy(true);
    setError(null);
    try {
      const input: { email?: string; expires_days?: number } = {};
      const e = email.trim();
      if (e) input.email = e;
      const d = parseInt(days, 10);
      if (!Number.isNaN(d) && d > 0) input.expires_days = d;
      const r = await client.createTeacherInvite(input);
      setMinted(r);
      setEmail("");
      setDays("");
      client.listTeacherInvites().then(setInvites).catch(() => {});
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "生成失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 880, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>教师</div>
        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        <div style={sectionTitle}>生成教师邀请码</div>
        <div style={{ ...card, display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
          <input
            placeholder="绑定邮箱（可选）"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            style={{ flex: 1, minWidth: 200, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
          />
          <input
            placeholder="有效天数（默认 14）"
            value={days}
            onChange={(e) => setDays(e.target.value)}
            style={{ width: 160, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
          />
          <button onClick={() => void mint()} disabled={busy} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 11, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>生成邀请码</button>
        </div>
        {minted && (
          <div style={{ marginTop: 12, background: "#EDEFF9", border: "1px solid #D7DCF2", borderRadius: 12, padding: "12px 16px", fontSize: 13.5, color: "#2A3B7A", fontWeight: 600 }}>
            新邀请码 {minted.code} · 有效期至 {shortDate(minted.expires_at)}（发给教师注册）
          </div>
        )}

        <div style={sectionTitle}>本校教师</div>
        {teachers && teachers.length === 0 && <div style={{ color: "#8A92A3", fontSize: 14 }}>暂无教师。生成邀请码邀请第一位。</div>}
        {teachers && teachers.length > 0 && (
          <div style={{ ...card, padding: 0 }}>
            {teachers.map((t, i) => (
              <div key={t.id} style={{ display: "flex", justifyContent: "space-between", padding: "14px 18px", borderTop: i === 0 ? "none" : "1px solid #F2F3F7" }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: "#1C2333" }}>{t.display_name}</span>
                <span style={{ fontSize: 13, color: "#8A92A3" }}>{t.email}</span>
              </div>
            ))}
          </div>
        )}

        <div style={sectionTitle}>待使用的邀请码</div>
        {invites && invites.length === 0 && <div style={{ color: "#8A92A3", fontSize: 14 }}>暂无待使用的邀请码。</div>}
        {invites && invites.length > 0 && (
          <div style={{ ...card, padding: 0 }}>
            {invites.map((inv, i) => (
              <div key={inv.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "14px 18px", borderTop: i === 0 ? "none" : "1px solid #F2F3F7" }}>
                <span style={{ fontSize: 14, fontWeight: 700, color: "#2A3B7A" }}>{inv.code}</span>
                <span style={{ fontSize: 13, color: "#8A92A3" }}>{inv.email ? `${inv.email} · ` : ""}有效期至 {shortDate(inv.expires_at)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
