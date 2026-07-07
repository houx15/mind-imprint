export type TeachingContent = { title: string; subtitle: string; body: string[]; foreground_asset_id: string | null };

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

export function TeachingTemplate({ content }: { content: TeachingContent }) {
  return (
    <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
      <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-0.01em", lineHeight: 1.3 }}>{content.title}</div>
      <div style={{ marginTop: 16, height: 148, borderRadius: 16, background: "linear-gradient(135deg,#EDEFF9 0%,#F3F0EC 100%)", border: "1px solid #EAECF2", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 11 }}>
        <div style={{ width: 54, height: 54, borderRadius: 16, background: "#fff", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 6px 16px rgba(42,59,122,.12)" }}>
          <svg width="27" height="27" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
        </div>
        <div style={{ fontSize: 14, color: "#5B6373", fontWeight: 600, maxWidth: 520, textAlign: "center", padding: "0 20px" }}>{content.subtitle}</div>
      </div>
      <div style={{ marginTop: 20 }}>
        {content.body.map((p, i) => (
          <div key={i} style={{ fontSize: 15.5, lineHeight: 1.85, color: "#2B3346", marginBottom: 15 }}>{p}</div>
        ))}
      </div>
    </div>
  );
}
