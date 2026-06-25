import { useState } from "react";
import type { SessionStore } from "../session";
import { useSession } from "../session";

const avatarOptions = ["#2A3B7A", "#D98263", "#4C9A82", "#E8A33D"];

const togglesDefault = [
  { label: "自动触发工具卡", desc: "分析你输入的内容，在合适时机弹出对应工具卡。", on: true },
  { label: "过程记录", desc: "将每次工具卡填写和对话节点保存到过程树。", on: true },
  { label: "使用统计", desc: "帮助改进工具推荐与陪练策略（数据不出设备）。", on: false },
];

export function SettingsView({
  session,
  onLogout,
}: {
  session: SessionStore;
  onLogout: () => void;
}) {
  const { aiAvatar } = useSession(session);

  const [toggles, setToggles] = useState(togglesDefault);

  function flipToggle(index: number) {
    setToggles((prev) =>
      prev.map((t, i) => (i === index ? { ...t, on: !t.on } : t))
    );
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 680, margin: "0 auto", padding: "40px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>
          设置
        </div>

        {/* === 个人 profile === */}
        <div style={{ fontSize: 13, fontWeight: 700, color: "#8A92A3", letterSpacing: ".04em", margin: "28px 0 12px" }}>
          个人
        </div>
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 16, marginBottom: 20 }}>
            <div style={{ width: 56, height: 56, borderRadius: 16, background: "#E8A33D", color: "#fff", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 22, fontWeight: 700 }}>
              P
            </div>
            <div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#1C2333" }}>Phoebe Chen</div>
              <div style={{ fontSize: 13, color: "#8A92A3", marginTop: 2 }}>IB DP1 · A 班 · TOK</div>
            </div>
          </div>
          <div style={{ fontSize: 12.5, fontWeight: 600, color: "#3A4256", marginBottom: 6 }}>姓名</div>
          <input
            defaultValue="Phoebe Chen"
            style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", background: "#FCFCFD", outline: "none", marginBottom: 14, boxSizing: "border-box" }}
          />
          <div style={{ fontSize: 12.5, fontWeight: 600, color: "#3A4256", marginBottom: 6 }}>邮箱</div>
          <input
            defaultValue="phoebe@ibschool.edu"
            style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", background: "#FCFCFD", outline: "none", boxSizing: "border-box" }}
          />
        </div>

        {/* === AI 形象 === */}
        <div style={{ fontSize: 13, fontWeight: 700, color: "#8A92A3", letterSpacing: ".04em", margin: "28px 0 12px" }}>
          AI 形象
        </div>
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            {/* Preview avatar (lines 775–784) */}
            <svg viewBox="0 0 48 48" width={58} height={58} style={{ display: "block", flex: "none" }}>
              <rect x="5" y="6" width="38" height="36" rx="13" fill={aiAvatar} />
              <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.12" />
              <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
              <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
              <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
              <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
              <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round" />
              <circle cx="39" cy="9" r="4.5" fill="#E8A33D" />
            </svg>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>你的陪练 · 印记</div>
              <div style={{ fontSize: 13, color: "#8A92A3", lineHeight: 1.6, marginTop: 4 }}>
                它克制、安静，一次只问你一个问题。选一个你看着舒服的颜色。
              </div>
            </div>
          </div>
          {/* Avatar options (lines 790–804) */}
          <div style={{ display: "flex", gap: 12, marginTop: 18 }}>
            {avatarOptions.map((color) => {
              const selected = color === aiAvatar;
              return (
                <div
                  key={color}
                  data-testid="avatar-option"
                  onClick={() => session.setAvatar(color)}
                  style={{
                    cursor: "pointer",
                    borderRadius: 14,
                    padding: 3,
                    border: selected ? "2.5px solid #3B5BDB" : "2.5px solid transparent",
                    boxSizing: "border-box",
                  }}
                >
                  <svg viewBox="0 0 48 48" width={40} height={40} style={{ display: "block" }}>
                    <rect x="5" y="6" width="38" height="36" rx="13" fill={color} />
                    <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.12" />
                    <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
                    <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
                    <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
                    <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
                    <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round" />
                  </svg>
                </div>
              );
            })}
          </div>
        </div>

        {/* === 其他 toggles === */}
        <div style={{ fontSize: 13, fontWeight: 700, color: "#8A92A3", letterSpacing: ".04em", margin: "28px 0 12px" }}>
          其他
        </div>
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, overflow: "hidden", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}>
          {toggles.map((t, i) => {
            const trackStyle: React.CSSProperties = {
              width: 44,
              height: 24,
              borderRadius: 12,
              background: t.on ? "#3B5BDB" : "#D1D5E0",
              position: "relative",
              cursor: "pointer",
              transition: "background .2s",
              flexShrink: 0,
            };
            const knobStyle: React.CSSProperties = {
              position: "absolute",
              top: 3,
              left: t.on ? 23 : 3,
              width: 18,
              height: 18,
              borderRadius: "50%",
              background: "#fff",
              boxShadow: "0 1px 3px rgba(0,0,0,.18)",
              transition: "left .2s",
            };
            return (
              <div
                key={t.label}
                style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 14, padding: "16px 20px", borderBottom: "1px solid #F2F3F7" }}
              >
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 600, color: "#1C2333" }}>{t.label}</div>
                  <div style={{ fontSize: 12.5, color: "#9AA1B0", lineHeight: 1.5, marginTop: 3 }}>{t.desc}</div>
                </div>
                <div onClick={() => flipToggle(i)} style={trackStyle}>
                  <div style={knobStyle} />
                </div>
              </div>
            );
          })}

          {/* 退出登录 (lines 821–824) */}
          <div
            onClick={onLogout}
            style={{ display: "flex", alignItems: "center", gap: 10, padding: "16px 20px", cursor: "pointer", color: "#C76B6B", fontSize: 14, fontWeight: 600 }}
          >
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#C76B6B" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4M16 17l5-5-5-5M21 12H9" />
            </svg>
            退出登录
          </div>
        </div>
      </div>
    </div>
  );
}
