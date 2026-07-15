import { useState } from "react";
import type { WritingProjection } from "../state";

// WritingProjection + the two callbacks this task produces (consumed by
// Task 10, which adds review/disposition/attest props alongside these).
export type WritingViewProps = WritingProjection & {
  onBufferChange?: (text: string) => void;
  onCommit?: (text: string) => void;
};

const TAB_BASE: React.CSSProperties = {
  border: "none",
  fontFamily: "inherit",
  fontSize: 13,
  fontWeight: 700,
  padding: "7px 14px",
  borderRadius: 7,
  cursor: "pointer",
};

function tabStyle(active: boolean): React.CSSProperties {
  return {
    ...TAB_BASE,
    background: active ? "#fff" : "transparent",
    color: active ? "#1C2333" : "#6B7384",
    boxShadow: active ? "0 1px 2px rgba(28,35,51,0.08)" : "none",
  };
}

// Mirrors the backend's own paragraph unit (paragraphSpanIndex /
// snapshotParagraphs in apps/api/internal/api/writing.go): split on blank
// lines, trim, drop empties.
function paragraphsOf(text: string): string[] {
  return text
    .split("\n\n")
    .map((p) => p.trim())
    .filter((p) => p.length > 0);
}

// The binding design (docs/design/思维印记_工作区.dc.html:2334) reads
// "第 3 版快照 · 10-14 提交 · 只读" — version · MM-DD · 提交 · 只读. `committedAt`
// is an ISO datetime string (YYYY-MM-DDTHH:mm:ssZ); slice the MM-DD straight
// out of it rather than round-tripping through `Date`, which would re-derive
// the calendar day in the *local* timezone and could shift it a day off from
// what the server actually recorded as committed.
function monthDayOf(committedAt: string): string {
  return committedAt.slice(5, 10);
}

function EditPane({ buffer, onBufferChange, onCommit }: { buffer: string; onBufferChange?: (t: string) => void; onCommit?: (t: string) => void }) {
  return (
    <>
      <div style={{ fontSize: 11.5, color: "#9AA1B0", marginBottom: 8 }}>
        直接在这里写，也可以在别处写好后粘进来。写作时印记不会打断你——想听意见，点「整稿体检」。
      </div>
      <textarea
        value={buffer}
        onChange={(e) => onBufferChange?.(e.target.value)}
        style={{
          width: "100%",
          minHeight: 440,
          border: "1px solid #E7EAF1",
          borderRadius: 14,
          padding: "24px 26px",
          fontSize: 15.5,
          lineHeight: 2,
          color: "#2B3346",
          background: "#fff",
          outline: "none",
          resize: "vertical",
          fontFamily: "inherit",
        }}
      />
      <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 12 }}>
        <button
          type="button"
          onClick={() => onCommit?.(buffer)}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            background: "#EDEFF9",
            color: "#2A3B7A",
            border: "none",
            fontSize: 13,
            fontWeight: 700,
            padding: "9px 15px",
            borderRadius: 10,
            cursor: "pointer",
            fontFamily: "inherit",
          }}
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 3v12M7 8l5-5 5 5M5 21h14" />
          </svg>
          提交快照 · 定格这一稿
        </button>
      </div>
    </>
  );
}

function PreviewPane({ buffer, reviewOrdered }: { buffer: string; reviewOrdered: boolean }) {
  const paras = paragraphsOf(buffer);
  return (
    <>
      <div style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "30px 34px", minHeight: 300 }}>
        {paras.map((p, i) => (
          <p key={i} style={{ fontSize: 15.5, lineHeight: 2.05, color: "#2B3346", margin: "0 0 16px", textIndent: "2em" }}>
            {p}
          </p>
        ))}
      </div>
      {/* Task 10 fills the work-order block (段落⇄评分表) once review.ordered
          is true — for now only the design's "还没做体检" empty state ships. */}
      {!reviewOrdered && (
        <div style={{ marginTop: 18, border: "1px dashed #DDE1EB", borderRadius: 14, padding: 22, textAlign: "center" }}>
          <div style={{ fontSize: 13.5, color: "#8A92A3", lineHeight: 1.7 }}>
            还没做体检。点右上角「整稿体检」，印记会告诉你每段在向哪张评分表交证据。
            <br />
            一稿一检——想再体检一次，先提交新的快照。
          </div>
        </div>
      )}
    </>
  );
}

export function WritingView({ buffer, latestSnapshot, review, onBufferChange, onCommit }: WritingViewProps) {
  const [mode, setMode] = useState<"edit" | "preview">("edit");
  const isEdit = mode === "edit";
  const snapshotMeta = latestSnapshot
    ? `第 ${latestSnapshot.seq} 版快照 · ${monthDayOf(latestSnapshot.committedAt)} 提交 · 只读`
    : "还没有提交过快照";
  const reviewOrdered = review?.ordered ?? false;

  return (
    <div style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
      <div style={{ flex: "none", display: "flex", alignItems: "center", gap: 12, padding: "12px 30px 0" }}>
        <div style={{ display: "flex", gap: 3, background: "#EBEDF2", borderRadius: 9, padding: 3 }}>
          <button type="button" onClick={() => setMode("edit")} style={tabStyle(isEdit)}>
            编辑 · 安静
          </button>
          <button type="button" onClick={() => setMode("preview")} style={tabStyle(!isEdit)}>
            预览 · 批注
          </button>
        </div>
        <span style={{ fontSize: 11.5, color: "#AEB4C2", fontWeight: 600 }}>{snapshotMeta}</span>
        <div style={{ marginLeft: "auto" }}>
          <button
            type="button"
            disabled={!latestSnapshot}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              background: latestSnapshot ? "#2A3B7A" : "#B9C0D6",
              border: "none",
              color: "#fff",
              fontSize: 13,
              fontWeight: 700,
              padding: "9px 15px",
              borderRadius: 10,
              cursor: latestSnapshot ? "pointer" : "not-allowed",
              fontFamily: "inherit",
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M9 11l3 3L22 4M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11" />
            </svg>
            整稿体检
          </button>
        </div>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "16px 30px 40px" }}>
        <div style={{ maxWidth: 720, margin: "0 auto" }}>
          {isEdit ? (
            <EditPane buffer={buffer} onBufferChange={onBufferChange} onCommit={onCommit} />
          ) : (
            <PreviewPane buffer={buffer} reviewOrdered={reviewOrdered} />
          )}
        </div>
      </div>
    </div>
  );
}
