import { useEffect, useState } from "react";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";
import { api } from "../../api";
import { DualAxisReport } from "../report/DualAxisReport";

// A2: the chat thread's report panel. Student-opt-in (铁律 2) — this
// component GETs a stored report on mount but never auto-POSTs; generation
// only ever happens from the student's explicit click on 生成/重新生成.
export function ChatReport({ threadId, onClose }: { threadId: string; onClose: () => void }) {
  const [assessment, setAssessment] = useState<DualAxisReportT | null>(null);
  const [state, setState] = useState<"loading" | "empty" | "generating" | "ready" | "error">("loading");

  // GET on mount — show a stored report if one exists, else the opt-in CTA.
  // Never auto-POST: generation is always the student's explicit click (铁律 2).
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const a = await api.getChatAssessment(threadId);
        if (cancelled) return;
        if (a) { setAssessment(a); setState("ready"); } else { setState("empty"); }
      } catch {
        if (!cancelled) setState("error");
      }
    })();
    return () => { cancelled = true; };
  }, [threadId]);

  async function generate() {
    setState("generating");
    try {
      const a = await api.generateChatAssessment(threadId);
      setAssessment(a);
      setState("ready");
    } catch {
      setState("error");
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 640, margin: "0 auto", padding: "34px 40px 56px" }}>
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "26px 28px", boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>本次对话 · 思维印记</div>
          <div style={{ fontSize: 22, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>你在这次对话里留下的思考痕迹</div>
          <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>按 SOLO 四级 · 只诊断过程，不打总分</div>
        </div>

        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 4 }}>本次对话评估</div>
          <div style={{ fontSize: 12.5, color: "#8A92A3", marginBottom: 14 }}>来自这次对话里你的追问与返工</div>
          {state === "loading" && <div style={{ fontSize: 13.5, color: "#9AA1B0" }}>正在读取…</div>}
          {state === "error" && <div style={{ fontSize: 13.5, color: "#8A92A3" }}>评估暂时没能生成，稍后再试。</div>}
          {state === "empty" && (
            <div style={{ padding: "10px 0" }}>
              <div style={{ fontSize: 13.5, color: "#6B7384", marginBottom: 14, lineHeight: 1.6 }}>
                为这次对话生成一份思维印记——看看你的问题有没有变得更锋利。
              </div>
              <button type="button" onClick={() => void generate()} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
                生成本次对话的思维印记
              </button>
            </div>
          )}
          {state === "generating" && <div style={{ fontSize: 13.5, color: "#9AA1B0" }}>正在生成本次对话的思维印记…</div>}
          {state === "ready" && assessment && (
            <DualAxisReport report={assessment} />
          )}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 22 }}>
          <button type="button" onClick={onClose} style={{ flex: "none", background: "#fff", border: "1px solid #E1E4ED", color: "#6B7384", fontSize: 14, fontWeight: 700, padding: "13px 20px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>返回对话</button>
          {state === "ready" && (
            <button type="button" onClick={() => void generate()} style={{ flex: "none", background: "none", border: "none", color: "#8A92A3", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}>重新生成</button>
          )}
        </div>
      </div>
    </div>
  );
}
