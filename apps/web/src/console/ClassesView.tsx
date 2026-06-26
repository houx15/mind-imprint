import { useEffect, useState } from "react";
import type { ApiClient, ClassSummary } from "../api";
import { ApiError } from "../api";
import { shortDate } from "./time";

type Client = Pick<ApiClient, "listClasses" | "createClass">;

export function ClassesView({
  client,
  role,
  onOpenClass,
}: {
  client: Client;
  role: string;
  onOpenClass: (id: string) => void;
}) {
  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [lastCreated, setLastCreated] = useState<ClassSummary | null>(null);

  const isTeacher = role === "teacher";

  function load() {
    setError(null);
    client.listClasses().then(setClasses).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client]);

  async function submit() {
    const trimmed = name.trim();
    if (!trimmed) return;
    setBusy(true);
    setError(null);
    try {
      const c = await client.createClass({ name: trimmed });
      setLastCreated(c);
      setName("");
      setCreating(false);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>
            {isTeacher ? "我的班级" : "全校班级"}
          </div>
          {isTeacher && !creating && (
            <button
              onClick={() => { setCreating(true); setLastCreated(null); }}
              style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
            >
              + 新建班级
            </button>
          )}
        </div>

        {isTeacher && creating && (
          <div style={{ marginTop: 18, background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", display: "flex", gap: 12, alignItems: "center" }}>
            <input
              autoFocus
              placeholder="班级名称，如「11 年级 A · TOK」"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") void submit(); }}
              style={{ flex: 1, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
            />
            <button onClick={() => void submit()} disabled={busy} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 11, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>创建</button>
            <button onClick={() => { setCreating(false); setName(""); }} style={{ background: "transparent", color: "#8A92A3", border: "none", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}>取消</button>
          </div>
        )}

        {lastCreated && (
          <div style={{ marginTop: 14, background: "#EDEFF9", border: "1px solid #D7DCF2", borderRadius: 12, padding: "12px 16px", fontSize: 13.5, color: "#2A3B7A", fontWeight: 600 }}>
            已创建「{lastCreated.name}」· 邀请码 {lastCreated.join_code}（分享给学生加入）
          </div>
        )}

        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        {classes && classes.length === 0 && !creating && (
          <div style={{ marginTop: 28, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
            {isTeacher ? "还没有班级，点「+ 新建班级」创建第一个。" : "本校暂无班级。"}
          </div>
        )}

        {classes && classes.length > 0 && (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 16, marginTop: 24 }}>
            {classes.map((c) => (
              <div
                key={c.id}
                onClick={() => onOpenClass(c.id)}
                style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", cursor: "pointer", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}
              >
                <div style={{ fontSize: 16, fontWeight: 700, color: "#1C2333", lineHeight: 1.45 }}>{c.name}</div>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 14 }}>
                  <span style={{ fontSize: 12.5, color: "#8A92A3", fontWeight: 600 }}>邀请码 {c.join_code}</span>
                  <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 500 }}>创建于 {shortDate(c.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
