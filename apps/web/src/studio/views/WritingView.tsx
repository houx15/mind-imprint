export type WritingViewProps = {
  draft: string;
  mode: "edit" | "preview";
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

function EditPane({ draft }: { draft: string }) {
  return (
    <>
      <div style={{ fontSize: 11.5, color: "#9AA1B0", marginBottom: 8 }}>
        直接在这里写，也可以在别处写好后粘进来。写作时印记不会打断你——想听意见，点「整稿体检」。
      </div>
      <textarea
        readOnly
        value={draft}
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
    </>
  );
}

function PreviewPane({ draft }: { draft: string }) {
  const paras = draft.split("\n\n").filter((p) => p.trim().length > 0);
  return (
    <>
      <div style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "30px 34px", minHeight: 300 }}>
        {paras.map((p, i) => (
          <p key={i} style={{ fontSize: 15.5, lineHeight: 2.05, color: "#2B3346", margin: "0 0 16px", textIndent: "2em" }}>
            {p}
          </p>
        ))}
      </div>
      <div style={{ marginTop: 18, border: "1px dashed #DDE1EB", borderRadius: 14, padding: 22, textAlign: "center" }}>
        <div style={{ fontSize: 13.5, color: "#8A92A3", lineHeight: 1.7 }}>
          还没做体检。点右上角「整稿体检」，印记会告诉你每段在向哪张评分表交证据。
          <br />
          一稿一检——想再体检一次，先提交新的快照。
        </div>
      </div>
    </>
  );
}

export function WritingView({ draft, mode }: WritingViewProps) {
  const isEdit = mode === "edit";

  return (
    <div style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
      <div style={{ flex: "none", display: "flex", alignItems: "center", gap: 12, padding: "12px 30px 0" }}>
        <div style={{ display: "flex", gap: 3, background: "#EBEDF2", borderRadius: 9, padding: 3 }}>
          <button type="button" style={tabStyle(isEdit)}>
            编辑 · 安静
          </button>
          <button type="button" style={tabStyle(!isEdit)}>
            预览 · 批注
          </button>
        </div>
        <div style={{ marginLeft: "auto" }}>
          <button
            type="button"
            disabled
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              background: "#B9C0D6",
              border: "none",
              color: "#fff",
              fontSize: 13,
              fontWeight: 700,
              padding: "9px 15px",
              borderRadius: 10,
              cursor: "not-allowed",
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
        <div style={{ maxWidth: 720, margin: "0 auto" }}>{isEdit ? <EditPane draft={draft} /> : <PreviewPane draft={draft} />}</div>
      </div>
    </div>
  );
}
