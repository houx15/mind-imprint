import { useState } from "react";
import type { ProjectListItem } from "../api/projects";

// Presentational directory for the 写作工作室 — the student's list of papers
// plus a 新建论文 create form. All data/effects live in StudioContainer; this
// component only reports intent (onOpen / onCreate) upward. Binding dc.html
// :669-708. Palette matches the sibling studio components: #2A3B7A primary,
// #EAECF2 borders, #1C2333 text, #8A92A3 muted.
export function Directory({
  projects,
  onOpen,
  onCreate,
  creating,
}: {
  projects: ProjectListItem[];
  onOpen: (id: string) => void;
  onCreate: (body: { title: string; prompt: string }) => void;
  creating: boolean;
}) {
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [prompt, setPrompt] = useState("");

  const canSubmit = !creating && prompt.trim().length > 0;

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 900, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>工作室</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6, letterSpacing: "-.01em" }}>
          选一个工作室，开始动手
        </div>

        {/* tabs — 项目工作室 is 即将上线 (disabled) */}
        <div style={{ display: "flex", gap: 26, marginTop: 24, borderBottom: "1px solid #EAECF2" }}>
          <div
            style={{
              paddingBottom: 12,
              fontSize: 14.5,
              fontWeight: 700,
              color: "#2A3B7A",
              borderBottom: "2px solid #2A3B7A",
              cursor: "pointer",
            }}
          >
            写作工作室
          </div>
          <div
            aria-disabled="true"
            style={{
              paddingBottom: 12,
              fontSize: 14.5,
              fontWeight: 600,
              color: "#B6BCC9",
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              cursor: "not-allowed",
            }}
          >
            项目工作室
            <span
              style={{
                fontSize: 10,
                fontWeight: 700,
                color: "#B08636",
                background: "#FBF1DC",
                padding: "2px 7px",
                borderRadius: 999,
              }}
            >
              即将上线
            </span>
          </div>
        </div>

        {/* 你的论文 */}
        <div style={{ display: "flex", alignItems: "flex-end", justifyContent: "space-between", gap: 16, marginTop: 26 }}>
          <div>
            <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333" }}>你的论文</div>
            <div style={{ fontSize: 13, color: "#8A92A3", marginTop: 3 }}>
              从贴题目开始，AI 陪你一站站把论证走扎实。
            </div>
          </div>
          <button
            type="button"
            onClick={() => setShowForm((v) => !v)}
            style={{
              flex: "none",
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              background: "#2A3B7A",
              color: "#fff",
              border: "none",
              padding: "11px 18px",
              borderRadius: 11,
              fontSize: 14,
              fontWeight: 700,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 5v14M5 12h14" />
            </svg>
            新建论文
          </button>
        </div>

        {/* create form (revealed by 新建论文) */}
        {showForm && (
          <div
            style={{
              marginTop: 16,
              background: "#fff",
              border: "1px solid #EAECF2",
              borderRadius: 16,
              padding: "20px 22px",
              boxShadow: "0 1px 3px rgba(20,30,60,.04)",
            }}
          >
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="给这篇论文起个名字（可选）"
              style={{
                width: "100%",
                boxSizing: "border-box",
                border: "1px solid #EAECF2",
                borderRadius: 10,
                padding: "11px 13px",
                fontSize: 14.5,
                fontWeight: 600,
                color: "#1C2333",
                outline: "none",
                fontFamily: "inherit",
              }}
            />
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={4}
              placeholder="贴上任务要求；如果你已经有思路、资料或初稿，也一起贴进来——我会据此帮你规划环节。"
              style={{
                width: "100%",
                boxSizing: "border-box",
                marginTop: 12,
                border: "1px solid #EAECF2",
                borderRadius: 10,
                padding: "11px 13px",
                fontSize: 14,
                lineHeight: 1.6,
                color: "#1C2333",
                outline: "none",
                resize: "vertical",
                fontFamily: "inherit",
              }}
            />
            <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 12 }}>
              <button
                type="button"
                aria-label="开始"
                disabled={!canSubmit}
                onClick={() => {
                  if (!canSubmit) return;
                  onCreate({ title: title.trim(), prompt: prompt.trim() });
                }}
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 8,
                  background: canSubmit ? "#2A3B7A" : "#C6CBD9",
                  color: "#fff",
                  border: "none",
                  padding: "10px 20px",
                  borderRadius: 11,
                  fontSize: 14,
                  fontWeight: 700,
                  cursor: canSubmit ? "pointer" : "not-allowed",
                  fontFamily: "inherit",
                }}
              >
                {creating ? "创建中…" : "开始"}
              </button>
            </div>
          </div>
        )}

        {/* project list */}
        <div
          style={{
            marginTop: 18,
            background: "#fff",
            border: "1px solid #EAECF2",
            borderRadius: 16,
            overflow: "hidden",
            boxShadow: "0 1px 3px rgba(20,30,60,.04)",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 16,
              padding: "12px 22px",
              borderBottom: "1px solid #F2F3F7",
              fontSize: 12,
              fontWeight: 700,
              color: "#9AA1B0",
            }}
          >
            <span style={{ flex: 1 }}>标题</span>
            <span style={{ flex: "none", width: 120, textAlign: "right" }}>当前进度</span>
          </div>
          {projects.length === 0 ? (
            <div style={{ padding: "34px 22px", textAlign: "center", fontSize: 13.5, color: "#9AA1B0", lineHeight: 1.7 }}>
              还没有论文。点「新建论文」，贴上你的任务，就能开始。
            </div>
          ) : (
            projects.map((p) => (
              <div
                key={p.id}
                data-testid="directory-project-row"
                onClick={() => onOpen(p.id)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 16,
                  padding: "17px 22px",
                  borderBottom: "1px solid #F4F5F8",
                  cursor: "pointer",
                }}
              >
                <span style={{ flex: 1, minWidth: 0 }}>
                  <span
                    style={{
                      fontSize: 15,
                      fontWeight: 700,
                      color: "#1C2333",
                      whiteSpace: "nowrap",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      display: "block",
                    }}
                  >
                    {p.title}
                  </span>
                  {p.qualLabel && (
                    <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 500 }}>{p.qualLabel}</span>
                  )}
                </span>
                <span style={{ flex: "none", width: 120, textAlign: "right", fontSize: 12.5, color: "#8A92A3", fontWeight: 600 }}>
                  {p.activeStation}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
