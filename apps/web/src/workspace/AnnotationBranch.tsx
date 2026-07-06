import { useEffect, useState } from "react";
import type { Anchor, CardInstance, CardSpec } from "@mind-imprint/contracts";
import { envelopeReducer } from "../cards/envelopeReducer";

export function AnnotationBranch({
  card, spec, onSubmit, onClose,
}: {
  card: CardInstance;
  spec: CardSpec;
  onSubmit: (id: string, final: CardInstance) => void;
  onClose: (id: string) => void;
}) {
  const [anchors, setAnchors] = useState<Anchor[]>(card.anchors);
  const [asking, setAsking] = useState(false);
  const [ownQ, setOwnQ] = useState("");

  // AI anchors are generated at summon time but reach the client asynchronously
  // (via activateCard, after this component has mounted). Seed local state once
  // they land — the functional guard prevents overwriting any edits already made.
  useEffect(() => {
    if (card.anchors.length > 0) {
      setAnchors((prev) => (prev.length === 0 ? card.anchors : prev));
    }
  }, [card.anchors]);

  function setAnswer(i: number, answer: string) {
    setAnchors((prev) => prev.map((a, idx) => (idx === i ? { ...a, answer } : a)));
  }

  function addOwnQuestion() {
    const q = ownQ.trim();
    if (!q) return;
    const first = anchors[0];
    setAnchors((prev) => [
      ...prev,
      {
        id: `student_${prev.length}`,
        material_id: first?.material_id ?? "",
        block_id: "", start: 0, end: 0, quote: "",
        dimension: "我的提问", author: "student", question: q, answer: "",
      },
    ]);
    setOwnQ("");
    setAsking(false);
  }

  function submit() {
    const completed = envelopeReducer(card, { type: "submit" });
    const q = ownQ.trim();
    const finalAnchors = asking && q
      ? [
          ...anchors,
          {
            id: `student_${anchors.length}`,
            material_id: anchors[0]?.material_id ?? "",
            block_id: "", start: 0, end: 0, quote: "",
            dimension: "我的提问", author: "student" as const, question: q, answer: "",
          },
        ]
      : anchors;
    onSubmit(card.id, { ...completed, anchors: finalAnchors });
  }

  const answered = anchors.filter((a) => a.author === "ai" && a.answer.trim()).length;
  const aiCount = anchors.filter((a) => a.author === "ai").length;

  return (
    <div style={{ background: "#fff", border: "1px solid #F0DACF", borderRadius: 14, overflow: "hidden", boxShadow: "0 4px 16px rgba(217,130,99,.10)", margin: "0 0 12px" }}>
      <div style={{ height: 4, background: "#D98263" }} />
      <div style={{ padding: "15px 18px 12px", borderBottom: "1px solid #F5F0ED" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 5 }}>
          <span style={{ fontSize: 11, fontWeight: 700, color: "#D98263", letterSpacing: ".05em" }}>工具卡 · 对话分支</span>
          <span style={{ fontSize: 11, fontWeight: 600, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 9px", borderRadius: 999 }}>{spec.category}</span>
        </div>
        <div style={{ fontSize: 16, fontWeight: 800, color: "#1C2333" }}>{spec.name} · 在真实材料上核查</div>
        <div style={{ fontSize: 12.5, color: "#8A92A3", marginTop: 3 }}>文章已在右侧打开 · 点亮的句子是我圈的</div>
      </div>

      <div style={{ padding: "14px 18px 6px", background: "#FAFBFC" }}>
        {anchors.map((a, i) => (
          <div key={a.id} style={{ border: "1px solid #ECEEF3", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#fff" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11, fontWeight: 700, padding: "3px 10px", borderRadius: 999, color: "#fff", background: a.author === "student" ? "#2A3B7A" : "#7C6BB5" }}>{a.dimension}</span>
              {a.answer.trim() && (
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              )}
            </div>
            <div style={{ fontSize: 14, lineHeight: 1.7, color: "#2B3346", fontWeight: 500 }}>{a.question}</div>
            <textarea
              value={a.answer}
              onChange={(e) => setAnswer(i, e.target.value)}
              rows={2}
              placeholder="写下你的判断……"
              style={{ width: "100%", marginTop: 9, border: "1px solid #E1E4ED", borderRadius: 9, padding: "9px 11px", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }}
            />
          </div>
        ))}

        {asking ? (
          <div style={{ border: "1px dashed #2A3B7A", borderRadius: 11, padding: "11px 13px", marginBottom: 12 }}>
            <textarea
              value={ownQ}
              onChange={(e) => setOwnQ(e.target.value)}
              rows={2}
              placeholder="写下你自己的问题——从印记没覆盖的角度……"
              style={{ width: "100%", border: "none", outline: "none", resize: "vertical", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "transparent" }}
            />
            <button type="button" onClick={addOwnQuestion} style={{ marginTop: 6, background: "#2A3B7A", color: "#fff", border: "none", padding: "7px 14px", borderRadius: 9, fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>加上我的问题</button>
          </div>
        ) : (
          <div onClick={() => setAsking(true)} style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 12px", padding: "10px 13px", border: "1px dashed #D3D8E4", borderRadius: 11, color: "#8A92A3", fontSize: 12.5, cursor: "pointer" }}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 20h9" /><path d="M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4z" /></svg>
            在右侧文章里划一句，自己向印记提问
          </div>
        )}
      </div>

      <div style={{ padding: "12px 18px", borderTop: "1px solid #F0F1F5", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
        <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 600 }}>{answered} / {aiCount} 维已回应</span>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <button type="button" onClick={() => onClose(card.id)} style={{ background: "none", border: "none", color: "#9AA1B0", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: "8px 6px" }}>收起</button>
          <button type="button" onClick={submit} style={{ display: "inline-flex", alignItems: "center", gap: 7, background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 10, fontSize: 13.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            提交并钉到过程树
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
