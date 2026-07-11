export type MethodologyModalProps = {
  cardId: string | null;
  onClose: () => void;
};

// 工具说明书 modal. Design: docs/design/思维印记_工作区.dc.html ~L1408-1449.
// Copy is keyed by card id (the same ids used in EQ / equipCards, e.g.
// "concession", "sift_craap"); unknown ids fall back to a generic default so
// the modal never renders blank.
type MethCopy = {
  title: string;
  intro: string;
  why: string;
  steps: Array<{ n: string; title: string; desc: string }>;
  help: string;
};

// Canonical copy is the design's own METH bank
// (docs/design/思维印记_工作区.dc.html ~L2377-2402) — verbatim, per the
// binding-design rule. Step markers are the design's (S/I/F/T for sift_craap,
// 1-4 for concession), not a positional index.
const METHODOLOGY: Record<string, MethCopy> = {
  sift_craap: {
    title: "SIFT × CRAAP 信息核查",
    intro:
      "两套互补的核查方法：SIFT 教你横向、快速判断一条网络信息可不可信；CRAAP 在你决定重点采信某个来源时，纵向把它核透。",
    why: "人最容易犯的错，是一头扎进单一材料、被它的措辞带着走。SIFT 先让你横向跳出来——看别人怎么说、找更权威的版本、溯到原始出处；只有当你确定要重点依赖某个来源时，才用 CRAAP 纵向五维细核。先广后深，省力又不容易被俘获。",
    steps: [
      { n: "S", title: "Stop", desc: "先停一下，别急着采信或转发，想清楚你要用它说明什么。" },
      { n: "I", title: "Investigate", desc: "查这条信息是谁发布的，找几个互相独立的来源对照。" },
      { n: "F", title: "Find better", desc: "去找这件事更权威、更原始的报道或研究。" },
      { n: "T", title: "Trace", desc: "顺着引用溯源，直到最初的出处。" },
    ],
    help: "让你在引用任何网络信息前都先站稳出处，不被单一来源或情绪化标题俘获——这是写研究、做 TOK、乃至日常刷手机都用得上的底层能力。",
  },
  concession: {
    title: "让步段 · 以退为进",
    intro: "一种让论证更有说服力的结构：先大方承认反方最强的那个事实，再转折反驳它。",
    why: "当证据对你不利时，绕开它只会让论证显得心虚。让步段反其道而行：先承认对方最强的事实——这让你显得诚实、可信；再说明它为什么不足以推翻你的论点——这让你的结论更稳。退一步，是为了站得更稳地进。",
    steps: [
      { n: "1", title: "立论", desc: "用一句话写清你真正想让人相信的判断。" },
      { n: "2", title: "举反方", desc: "挑出对方最难反驳的那个事实。" },
      { n: "3", title: "让步", desc: "大方承认这个事实，别躲。" },
      { n: "4", title: "反驳", desc: "说明它为什么不足以推翻你的论点。" },
    ],
    help: "专治「遇到反例就慌」。学会处理对立证据，你的议论文、TOK 展示、辩论都会明显更有分量，也更经得起追问。",
  },
};

const DEFAULT_METHODOLOGY: MethCopy = METHODOLOGY.sift_craap!;

function CloseIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  );
}

function StarIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 2l2.4 7.4H22l-6 4.6 2.3 7.4-6.3-4.6L5.7 21l2.3-7.4-6-4.6h7.6z" />
    </svg>
  );
}

export function MethodologyModal({ cardId, onClose }: MethodologyModalProps) {
  if (cardId === null) return null;

  const copy = METHODOLOGY[cardId] ?? DEFAULT_METHODOLOGY;

  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        zIndex: 55,
        background: "rgba(22,28,46,.44)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 32,
      }}
    >
      <div
        style={{
          width: "100%",
          maxWidth: 600,
          maxHeight: "88%",
          overflowY: "auto",
          background: "#fff",
          borderRadius: 20,
          boxShadow: "0 24px 64px rgba(20,30,60,.28)",
          fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif",
        }}
      >
        <div style={{ padding: "26px 30px 22px", borderBottom: "1px solid #F0F1F5", position: "relative" }}>
          <div
            onClick={onClose}
            title="关闭"
            role="button"
            style={{
              position: "absolute",
              top: 20,
              right: 20,
              width: 32,
              height: 32,
              borderRadius: 8,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              color: "#9AA1B0",
            }}
          >
            <CloseIcon />
          </div>
          <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: "0.08em", color: "#D98263", marginBottom: 8 }}>
            工具说明书 · 我不懂为什么
          </div>
          <div style={{ fontSize: 22, fontWeight: 800, color: "#1C2333" }}>{copy.title}</div>
          <div style={{ fontSize: 14, color: "#6B7384", lineHeight: 1.66, marginTop: 8 }}>{copy.intro}</div>
        </div>
        <div style={{ padding: "22px 30px 26px" }}>
          <div style={{ marginBottom: 20 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, fontWeight: 700, color: "#2A3B7A", marginBottom: 8 }}>
              <span
                style={{
                  width: 22,
                  height: 22,
                  borderRadius: 6,
                  background: "#EDEFF9",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 12,
                }}
              >
                ①
              </span>
              为什么是这几步
            </div>
            <div style={{ fontSize: 14, lineHeight: 1.75, color: "#3A4256" }}>{copy.why}</div>
          </div>
          <div style={{ marginBottom: 20 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, fontWeight: 700, color: "#2A3B7A", marginBottom: 10 }}>
              <span
                style={{
                  width: 22,
                  height: 22,
                  borderRadius: 6,
                  background: "#EDEFF9",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 12,
                }}
              >
                ②
              </span>
              怎么用
            </div>
            {copy.steps.map((st) => (
              <div key={st.title} style={{ display: "flex", gap: 11, marginBottom: 10 }}>
                <span
                  style={{
                    flex: "none",
                    width: 22,
                    height: 22,
                    borderRadius: "50%",
                    background: "#2A3B7A",
                    color: "#fff",
                    fontSize: 11,
                    fontWeight: 700,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    marginTop: 1,
                  }}
                >
                  {st.n}
                </span>
                <div>
                  <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{st.title}</span>
                  <span style={{ fontSize: 13.5, color: "#6B7384" }}> — {st.desc}</span>
                </div>
              </div>
            ))}
          </div>
          <div style={{ background: "#F7F8FB", border: "1px solid #EDEEF4", borderRadius: 14, padding: "16px 18px" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, fontWeight: 700, color: "#4C9A82", marginBottom: 8 }}>
              <StarIcon />
              它能帮到你
            </div>
            <div style={{ fontSize: 14, lineHeight: 1.72, color: "#3A4256" }}>{copy.help}</div>
          </div>
          <button
            type="button"
            onClick={onClose}
            style={{
              width: "100%",
              marginTop: 14,
              background: "#2A3B7A",
              color: "#fff",
              border: "none",
              padding: 13,
              borderRadius: 12,
              fontSize: 14.5,
              fontWeight: 700,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            明白了，回去填
          </button>
        </div>
      </div>
    </div>
  );
}
