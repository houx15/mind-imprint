import { useState } from "react";
import { api, ApiError, type ApiClient, type MeUser } from "../../api";

type AuthClient = Pick<ApiClient, "signin" | "signup">;

const CARD = { background: "#fff", border: "1px solid #EAECF2", borderRadius: "18px", padding: "26px 26px 24px", boxShadow: "0 8px 30px rgba(20,30,60,.06)" } as const;
const LABEL = { fontSize: "12.5px", fontWeight: 600, color: "#3A4256", marginBottom: "6px" } as const;
const INPUT = { width: "100%", border: "1px solid #E1E4ED", borderRadius: "11px", padding: "12px 14px", fontSize: "14px", color: "#1C2333", background: "#FCFCFD", outline: "none", marginBottom: "14px", boxSizing: "border-box" } as const;
const BTN = { width: "100%", background: "#2A3B7A", color: "#fff", border: "none", padding: "13px", borderRadius: "12px", fontSize: "15px", fontWeight: 700, cursor: "pointer", fontFamily: "inherit" } as const;
const ERR = { color: "#C0392B", fontSize: "13px", marginBottom: "12px", textAlign: "center" } as const;

function messageFor(e: unknown): string {
  if (e instanceof ApiError) return e.message;
  return "网络错误，请稍后再试";
}

export function AuthScreen({
  onAuthed,
  client = api,
}: {
  onAuthed: (user: MeUser) => void;
  client?: AuthClient;
}) {
  const [step, setStep] = useState<"login" | "register" | "bind">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [joinCode, setJoinCode] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function doLogin() {
    setErr(null); setBusy(true);
    try {
      onAuthed(await client.signin({ email, password }));
    } catch (e) {
      setErr(messageFor(e));
    } finally {
      setBusy(false);
    }
  }

  async function doRegister() {
    setErr(null); setBusy(true);
    try {
      await client.signup({ email, password, display_name: displayName, join_code: joinCode });
      onAuthed(await client.signin({ email, password }));
    } catch (e) {
      setErr(messageFor(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ width: "100%", height: "100%", display: "flex", alignItems: "center", justifyContent: "center", background: "radial-gradient(1200px 600px at 50% -10%, #EAEDF8 0%, #F3F4F8 60%)", padding: "24px" }}>
      <div style={{ width: "100%", maxWidth: "420px" }}>
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", marginBottom: "26px" }}>
          <svg viewBox="0 0 48 48" width="60" height="60" style={{ display: "block", filter: "drop-shadow(0 8px 18px rgba(42,59,122,.22))" }}>
            <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A"></rect>
            <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10"></rect>
            <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff"></ellipse>
            <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff"></ellipse>
            <circle cx="19.3" cy="25" r="1.5" fill="#1C2333"></circle>
            <circle cx="30.3" cy="25" r="1.5" fill="#1C2333"></circle>
            <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round"></path>
            <circle cx="39" cy="9" r="4.5" fill="#E8A33D"></circle>
          </svg>
          <div style={{ fontSize: "22px", fontWeight: 800, color: "#1C2333", marginTop: "14px", letterSpacing: ".01em" }}>思维印记</div>
          <div style={{ fontSize: "13.5px", color: "#8A92A3", marginTop: "6px" }}>带着真实的问题来，和 AI 一起把思考走深</div>
        </div>

        <div style={CARD}>
          {step === "login" && (
            <>
              <div style={{ fontSize: "17px", fontWeight: 700, color: "#1C2333", marginBottom: "18px" }}>登录</div>
              <div style={LABEL}>邮箱 / 手机号</div>
              <input value={email} onChange={(e) => setEmail(e.target.value)} style={INPUT} />
              <div style={LABEL}>密码</div>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} style={{ ...INPUT, marginBottom: "20px" }} />
              {err && <div style={ERR}>{err}</div>}
              <button onClick={doLogin} disabled={busy} style={{ ...BTN, opacity: busy ? 0.6 : 1 }}>登录</button>
              <div style={{ textAlign: "center", marginTop: "16px", fontSize: "13px", color: "#8A92A3" }}>
                还没有账号？
                <span onClick={() => { setStep("register"); setErr(null); }} style={{ color: "#2A3B7A", fontWeight: 700, cursor: "pointer" }}>注册</span>
              </div>
            </>
          )}

          {step === "register" && (
            <>
              <div style={{ fontSize: "17px", fontWeight: 700, color: "#1C2333", marginBottom: "18px" }}>创建账号</div>
              <div style={LABEL}>姓名</div>
              <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} style={INPUT} />
              <div style={LABEL}>邮箱</div>
              <input value={email} onChange={(e) => setEmail(e.target.value)} style={INPUT} />
              <div style={LABEL}>设置密码</div>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} style={{ ...INPUT, marginBottom: "20px" }} />
              <button onClick={() => { setErr(null); setStep("bind"); }} style={BTN}>下一步 · 绑定班级</button>
              <div style={{ textAlign: "center", marginTop: "16px", fontSize: "13px", color: "#8A92A3" }}>
                已有账号？
                <span onClick={() => { setStep("login"); setErr(null); }} style={{ color: "#2A3B7A", fontWeight: 700, cursor: "pointer" }}>登录</span>
              </div>
            </>
          )}

          {step === "bind" && (
            <>
              <div style={{ fontSize: "17px", fontWeight: 700, color: "#1C2333", marginBottom: "6px" }}>绑定你的班级</div>
              <div style={{ fontSize: "13px", color: "#8A92A3", lineHeight: "1.6", marginBottom: "18px" }}>
                绑定后，老师布置的探究任务会出现在你的任务列表里。你的思考过程只属于你，老师看不到对话本身。
              </div>
              <div style={LABEL}>班级邀请码</div>
              <input value={joinCode} onChange={(e) => setJoinCode(e.target.value)} style={INPUT} />
              {err && <div style={ERR}>{err}</div>}
              <button onClick={doRegister} disabled={busy} style={{ ...BTN, opacity: busy ? 0.6 : 1 }}>完成，进入思维印记</button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
