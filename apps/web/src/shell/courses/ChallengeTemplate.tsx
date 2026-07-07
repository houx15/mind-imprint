import { useState } from "react";
import type { Anchor } from "@mind-imprint/contracts";

export type ChallengeContent = { title: string; prompt: string; reason_hint: string; anchors: Anchor[] };

export function ChallengeTemplate({ content }: { content: ChallengeContent }) {
  const [answers, setAnswers] = useState<string[]>(content.anchors.map((a) => a.answer));
  const [reason, setReason] = useState("");
  const [submitted, setSubmitted] = useState(false);

  return (
    <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
      <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-0.01em", lineHeight: 1.3 }}>{content.title}</div>
      <div style={{ marginTop: 16, display: "inline-flex", alignItems: "center", gap: 7, fontSize: 12, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "5px 12px", borderRadius: 999 }}>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#D98263" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M13 2L3 14h7l-1 8 10-12h-7z" /></svg>
        现在轮到你
      </div>
      <div style={{ fontSize: 15.5, lineHeight: 1.8, color: "#2B3346", marginTop: 14 }}>{content.prompt}</div>

      <div style={{ marginTop: 16 }}>
        {content.anchors.map((a, i) => (
          <div key={a.id} style={{ border: "1px solid #ECEEF3", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#fff" }}>
            <span style={{ display: "inline-flex", fontSize: 11, fontWeight: 700, padding: "3px 10px", borderRadius: 999, color: "#fff", background: "#7C6BB5" }}>{a.dimension}</span>
            <div style={{ fontSize: 14, lineHeight: 1.7, color: "#2B3346", fontWeight: 500, marginTop: 8 }}>{a.question}</div>
            <textarea value={answers[i]} onChange={(e) => setAnswers((prev) => prev.map((x, j) => (j === i ? e.target.value : x)))} rows={2}
              placeholder="写下你的判断……"
              style={{ width: "100%", marginTop: 9, border: "1px solid #E1E4ED", borderRadius: 9, padding: "9px 11px", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }} />
          </div>
        ))}
      </div>

      <div style={{ fontSize: 13, fontWeight: 600, color: "#3A4256", margin: "8px 0 9px" }}>{content.reason_hint || "写一句你的理由"}</div>
      <textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} placeholder="说说你为什么这么判断……"
        style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 12, padding: "12px 14px", fontSize: 14, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }} />

      {!submitted ? (
        <button type="button" onClick={() => setSubmitted(true)}
          style={{ marginTop: 12, background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 10, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>提交我的判断</button>
      ) : (
        <div style={{ marginTop: 16, background: "#E7F3EE", border: "1px solid #D3E9DF", borderRadius: 12, padding: "15px 17px", fontSize: 13.5, lineHeight: 1.72, color: "#2B4A3E" }}>
          很好，你已经开始像个核查者一样思考了——先分辨事实与情绪，再决定信不信。带着这份判断，继续下一步。
        </div>
      )}
    </div>
  );
}
