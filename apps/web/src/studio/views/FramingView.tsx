import { useState } from "react";
import type { FramingFx } from "../state";

// N3d Task 10: S1 立题.
//
// Binding design: docs/design/思维印记_工作区.dc.html:874-928 (markup) +
// :2144-2157 (the JS that computes its data). Every panel below —
// copy, colours, radii, spacing — is that design verbatim, with ONE
// deliberate departure documented at the 关键概念 panel: the design hard-codes
// three key terms computed from the DEMO's own project title ("sustainable" /
// "China's role" / "the world"). A real project has no such list — the only
// ways to produce one are an LLM call (banned this task) or a fixture that
// cannot know her actual research question — so SHE names her own key terms.
// That is why this panel, alone among the three, has an add/remove control
// the design does not draw.
export type FramingViewProps = {
  data: FramingFx;
  onSubmit?: (body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] }) => Promise<void>;
};

const WRAP: React.CSSProperties = { flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" };
const COL: React.CSSProperties = { maxWidth: 720, margin: "0 auto" };
const CARD: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "18px 20px" };

// The binding design's own rule (dc.html:2149-2153): rune count on the
// trimmed definition. [...s] not s.length — a Chinese definition must not be
// judged by its byte or UTF-16 length (the backend gate itself counts runes
// via utf8.RuneCountInString).
function defChip(def: string): { label: string; color: string; background: string } {
  const n = [...def.trim()].length;
  if (n === 0) return { label: "待定义", color: "#9AA1B0", background: "#F1F2F5" };
  if (n < 15) return { label: "偏模糊，再具体点", color: "#B8892F", background: "#FBF4E2" };
  return { label: "✓ 可检验", color: "#4C9A82", background: "#E7F3EE" };
}

function PlusIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function XIcon({ size = 13 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  );
}

const ADD_LINK: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  marginTop: 4,
  fontSize: 12.5,
  fontWeight: 700,
  color: "#2A3B7A",
  background: "none",
  border: "none",
  padding: 0,
  cursor: "pointer",
};

const REMOVE_BTN: React.CSSProperties = {
  flex: "none",
  width: 24,
  height: 24,
  border: "none",
  background: "none",
  borderRadius: 7,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  cursor: "pointer",
  color: "#C2C8D6",
  padding: 0,
};

export function FramingView({ data, onSubmit }: FramingViewProps) {
  const [terms, setTerms] = useState(data.terms.map((t) => ({ ...t })));
  const [answers, setAnswers] = useState<string[]>([...data.answers]);
  const [searchPlan, setSearchPlan] = useState<string[]>([...data.searchPlan]);
  const [submitting, setSubmitting] = useState(false);

  const doneN = terms.filter((t) => [...t.definition.trim()].length >= 15).length;

  // 铁律 2 (不操纵): the save button is never disabled for incompleteness —
  // only while a submit is actually in flight. A half-finished S1 saves fine.
  const canSubmit = !!onSubmit && !submitting;

  function updateTerm(i: number, field: "term" | "definition", value: string) {
    setTerms((prev) => prev.map((t, idx) => (idx === i ? { ...t, [field]: value } : t)));
  }
  function addTerm() {
    setTerms((prev) => [...prev, { term: "", definition: "" }]);
  }
  function removeTerm(i: number) {
    setTerms((prev) => prev.filter((_, idx) => idx !== i));
  }

  function updateAnswer(i: number, value: string) {
    setAnswers((prev) => prev.map((a, idx) => (idx === i ? value : a)));
  }
  function addAnswer() {
    setAnswers((prev) => [...prev, ""]);
  }
  function removeAnswer(i: number) {
    setAnswers((prev) => prev.filter((_, idx) => idx !== i));
  }

  function updateSearchItem(i: number, value: string) {
    setSearchPlan((prev) => prev.map((p, idx) => (idx === i ? value : p)));
  }
  function addSearchItem() {
    setSearchPlan((prev) => [...prev, ""]);
  }
  function removeSearchItem(i: number) {
    setSearchPlan((prev) => prev.filter((_, idx) => idx !== i));
  }

  async function handleSubmit() {
    if (!canSubmit || !onSubmit) return;
    setSubmitting(true);
    try {
      await onSubmit({ terms, answers, searchPlan });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={WRAP}>
      <div style={COL}>
        {/* dc.html:878-884 */}
        <div style={{ background: "linear-gradient(135deg,#2A3B7A,#34468C)", borderRadius: 16, padding: "20px 22px", boxShadow: "0 8px 22px rgba(42,59,122,.18)" }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".06em", color: "#AEB8E4" }}>RESEARCH QUESTION</div>
            <span style={{ fontSize: 10.5, fontWeight: 700, color: "#2A3B7A", background: "#fff", padding: "2px 9px", borderRadius: 999 }}>先定义，再动笔</span>
          </div>
          <div style={{ fontSize: 18, fontWeight: 800, color: "#fff", lineHeight: 1.4, marginTop: 8 }}>{data.researchQuestion}</div>
        </div>

        {/* dc.html:885-900, with the one departure noted at the top of this file */}
        <div style={{ ...CARD, marginTop: 14 }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 6 }}>
            <div style={{ fontSize: 13, fontWeight: 800, color: "#1C2333" }}>关键概念 · 我的定义</div>
            <span style={{ fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 9px", borderRadius: 999 }}>
              本环节门禁 · 已定义 {doneN}/3
            </span>
          </div>
          <div style={{ fontSize: 12, color: "#8A92A3", marginBottom: 8 }}>
            自己写清每个关键词——别让读者把「可持续」误当成「变绿」。印记只判断你写得够不够可检验，不替你写。
          </div>
          {terms.map((t, i) => {
            const chip = defChip(t.definition);
            return (
              <div key={i} style={{ padding: "12px 0", borderBottom: "1px solid #F3F4F7" }}>
                <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 7 }}>
                  <input
                    aria-label="关键词"
                    value={t.term}
                    onChange={(e) => updateTerm(i, "term", e.target.value)}
                    placeholder="写下这个关键词/短语……"
                    style={{ flex: 1, minWidth: 0, border: "none", outline: "none", background: "transparent", fontSize: 13.5, fontWeight: 800, color: "#2A3B7A", padding: 0 }}
                  />
                  <span style={{ flex: "none", fontSize: 10.5, fontWeight: 700, padding: "2px 9px", borderRadius: 999, color: chip.color, background: chip.background }}>
                    {chip.label}
                  </span>
                  <button type="button" aria-label="删除这个关键词" onClick={() => removeTerm(i)} style={REMOVE_BTN}>
                    <XIcon />
                  </button>
                </div>
                <textarea
                  aria-label="定义"
                  value={t.definition}
                  onChange={(e) => updateTerm(i, "definition", e.target.value)}
                  rows={2}
                  placeholder="写清楚这个词在你的题目里具体指什么，做到可检验……"
                  style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 10, padding: "9px 12px", fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }}
                />
              </div>
            );
          })}
          <button type="button" onClick={addTerm} style={ADD_LINK}>
            <PlusIcon />
            添加一个关键词
          </button>
        </div>

        {/* dc.html:901-925 */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14, marginTop: 14 }}>
          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "17px 18px" }}>
            <div style={{ fontSize: 13, fontWeight: 800, color: "#1C2333", marginBottom: 4 }}>可能的核心论点</div>
            <div style={{ fontSize: 11.5, color: "#9AA1B0", marginBottom: 11 }}>你自己拟——之后在 S4 逐条验证。</div>
            {answers.map((a, i) => (
              <div key={i} style={{ display: "flex", gap: 8, alignItems: "flex-start", marginBottom: 9 }}>
                <span style={{ flex: "none", width: 6, height: 6, borderRadius: "50%", background: "#2A3B7A", marginTop: 14 }} />
                <textarea
                  aria-label="核心论点"
                  value={a}
                  onChange={(e) => updateAnswer(i, e.target.value)}
                  rows={2}
                  placeholder="写一条你打算论证的判断……"
                  style={{ flex: 1, minWidth: 0, border: "1px solid #E1E4ED", borderRadius: 9, padding: "8px 10px", fontSize: 12.5, lineHeight: 1.55, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }}
                />
                <button type="button" aria-label="删除这条论点" onClick={() => removeAnswer(i)} style={{ ...REMOVE_BTN, marginTop: 6 }}>
                  <XIcon />
                </button>
              </div>
            ))}
            <button type="button" onClick={addAnswer} style={ADD_LINK}>
              <PlusIcon />
              添加一条论点
            </button>
          </div>

          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "17px 18px" }}>
            <div style={{ fontSize: 13, fontWeight: 800, color: "#1C2333", marginBottom: 11 }}>打算去哪找证据</div>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              {searchPlan.map((p, i) => (
                <div key={i} style={{ display: "flex", alignItems: "center", gap: 4, background: "#EBEDFA", padding: "6px 6px 6px 11px", borderRadius: 9 }}>
                  <input
                    aria-label="检索方向"
                    value={p}
                    onChange={(e) => updateSearchItem(i, e.target.value)}
                    placeholder="打算去哪找……"
                    size={Math.max(6, p.length || 8)}
                    style={{ border: "none", outline: "none", background: "transparent", fontSize: 12, fontWeight: 600, color: "#5B6BB5" }}
                  />
                  <button type="button" aria-label="删除这条检索方向" onClick={() => removeSearchItem(i)} style={{ ...REMOVE_BTN, width: 18, height: 18, color: "#8B93C9" }}>
                    <XIcon size={10} />
                  </button>
                </div>
              ))}
            </div>
            <button type="button" onClick={addSearchItem} style={ADD_LINK}>
              <PlusIcon />
              添加一条检索方向
            </button>
          </div>
        </div>

        {/* mirrors S0View's 记下我的理解 exactly */}
        <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 14 }}>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit}
            style={{
              fontSize: 12.5,
              fontWeight: 700,
              color: "#fff",
              background: canSubmit ? "#2A3B7A" : "#B7BBCB",
              border: "none",
              borderRadius: 10,
              padding: "8px 16px",
              cursor: canSubmit ? "pointer" : "not-allowed",
            }}
          >
            {submitting ? "记录中…" : "记下我的立题"}
          </button>
        </div>
      </div>
    </div>
  );
}
