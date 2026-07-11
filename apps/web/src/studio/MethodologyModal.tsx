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
  steps: Array<{ title: string; desc: string }>;
  help: string;
};

const METHODOLOGY: Record<string, MethCopy> = {
  concession: {
    title: "让步段卡",
    intro: "一段真正的论证会先承认最强的反例，再说明它为什么不推翻你的主张。",
    why: "评分表最看重的不是「你说得对」，而是「你知道对方会怎么反驳，并且接得住」——跳过反方，分析与评估这一档很难往上走。",
    steps: [
      { title: "写出反方的最强版本", desc: "不是稻草人，是让你自己都觉得有点道理的那个说法" },
      { title: "承认它的分量", desc: "一句话说清它真实存在、不能被无视" },
      { title: "转折回你的主张", desc: "说明这个反例为什么不能推翻你更大的判断" },
      { title: "接回论证图", desc: "把让步段连到对应的主张节点，别让它悬空" },
    ],
    help: "让步段是「分析与评估」拿到 7-8 段位最常见的缺口——补上它，通常就是这一档和上一档之间的差距。",
  },
  sift_craap: {
    title: "CRAAP 五维 / SIFT 横向阅读",
    intro: "判断一条信息值不值得信，靠的不是「感觉」，而是能不能在五个维度上说清楚。",
    why: "「表F 评估」要求你对每条来源写出作用与风险，而不是简单贴一个「可信/不可信」的标签——CRAAP 把这件事拆成可执行的五步。",
    steps: [
      { title: "Currency 时效", desc: "这条信息是什么时候发布 / 更新的，跟得上话题吗" },
      { title: "Relevance 相关", desc: "它到底在回答你的问题，还是只是沾边" },
      { title: "Authority 权威", desc: "作者 / 机构在这个领域有没有资格说话" },
      { title: "Accuracy 准确", desc: "能不能在别处找到独立信源核实同一个事实" },
    ],
    help: "把 CRAAP 五维过一遍，再横向核查一次原始信源——这条来源在你论证里的「作用与风险」就有据可写了。",
  },
};

const DEFAULT_METHODOLOGY: MethCopy = {
  title: "这张工具卡",
  intro: "工具卡不是给你答案，而是在你思考卡住的地方，把结构递回给你。",
  why: "过程评估在意的是你怎么用证据、怎么处理反方、怎么组织表达——工具卡把这些拆成几个可以一步步做的动作。",
  steps: [
    { title: "读懂它在问什么", desc: "先看卡片顶部锚定的是哪个主张 / 哪段论证" },
    { title: "用自己的话回答", desc: "别急着套模板，先写出你此刻真实的想法" },
    { title: "回填到过程树", desc: "你的回答会成为过程记录的一部分" },
  ],
  help: "如果还是不确定怎么用，去对应的系统课过一遍完整流程，会更踏实。",
};

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
            {copy.steps.map((st, i) => (
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
                  {i + 1}
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
