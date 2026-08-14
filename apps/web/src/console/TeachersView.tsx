import { useEffect, useState } from "react";
import type { ApiClient, Teacher, TeacherInvite } from "../api";
import { ApiError } from "../api";
import { shortDate } from "./time";
import { Button, Input } from "@/ui";

type Client = Pick<ApiClient, "listTeachers" | "listTeacherInvites" | "createTeacherInvite">;

const card: React.CSSProperties = { background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: "var(--mk-radius-lg)", padding: "18px 20px", boxShadow: "var(--mk-shadow-sm)" };
const sectionTitle: React.CSSProperties = { fontSize: 15, fontWeight: 700, color: "var(--mk-ink)", margin: "30px 0 12px" };

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
        <div style={{ fontSize: 26, fontWeight: 800, color: "var(--mk-ink)", letterSpacing: "-.01em" }}>教师</div>
        {error && (
          <div style={{ marginTop: 14, color: "var(--mk-danger)", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        <div style={sectionTitle}>生成教师邀请码</div>
        <div style={{ ...card, display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
          <div style={{ flex: 1, minWidth: 200 }}>
            <Input placeholder="绑定邮箱（可选）" value={email} onChange={setEmail} />
          </div>
          <div style={{ width: 160 }}>
            <Input placeholder="有效天数（默认 14）" value={days} onChange={setDays} />
          </div>
          <Button onClick={() => void mint()} disabled={busy}>生成邀请码</Button>
        </div>
        {minted && (
          <div style={{ marginTop: 12, background: "var(--mk-accent-50)", border: "1px solid var(--mk-accent-100)", borderRadius: "var(--mk-radius-lg)", padding: "12px 16px", fontSize: 13.5, color: "var(--mk-accent-600)", fontWeight: 600 }}>
            新邀请码 {minted.code} · 有效期至 {shortDate(minted.expires_at)}（发给教师注册）
          </div>
        )}

        <div style={sectionTitle}>本校教师</div>
        {teachers && teachers.length === 0 && <div style={{ color: "var(--mk-muted)", fontSize: 14 }}>暂无教师。生成邀请码邀请第一位。</div>}
        {teachers && teachers.length > 0 && (
          <div style={{ ...card, padding: 0 }}>
            {teachers.map((t, i) => (
              <div key={t.id} style={{ display: "flex", justifyContent: "space-between", padding: "14px 18px", borderTop: i === 0 ? "none" : "1px solid var(--mk-border)" }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--mk-ink)" }}>{t.display_name}</span>
                <span style={{ fontSize: 13, color: "var(--mk-muted)" }}>{t.email}</span>
              </div>
            ))}
          </div>
        )}

        <div style={sectionTitle}>待使用的邀请码</div>
        {invites && invites.length === 0 && <div style={{ color: "var(--mk-muted)", fontSize: 14 }}>暂无待使用的邀请码。</div>}
        {invites && invites.length > 0 && (
          <div style={{ ...card, padding: 0 }}>
            {invites.map((inv, i) => (
              <div key={inv.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "14px 18px", borderTop: i === 0 ? "none" : "1px solid var(--mk-border)" }}>
                <span style={{ fontSize: 14, fontWeight: 700, color: "var(--mk-accent-600)" }}>{inv.code}</span>
                <span style={{ fontSize: 13, color: "var(--mk-muted)" }}>{inv.email ? `${inv.email} · ` : ""}有效期至 {shortDate(inv.expires_at)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
