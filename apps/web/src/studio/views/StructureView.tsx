import type { StructureCardFx } from "../state";
import { Bean } from "../Bean";

export type StructureViewProps = {
  cards: StructureCardFx[];
};

const WRAP: React.CSSProperties = { flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" };
const COL: React.CSSProperties = { maxWidth: 760, margin: "0 auto" };
// Same neutral deferred-shell placeholder as OnboardingView's ShellView
// (structure deep view is deferred to Slice 7 — StudioContainer stubs
// `views.structure: []`, so this guards against a vacuously-true
// `cards.every(...)` rendering a false green "门禁通过" banner on 0 cards).
const DEFERRED_CARD: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px" };

function GateBanner({ allClean }: { allClean: boolean }) {
  if (allClean) {
    return (
      <div
        style={{
          display: "flex",
          alignItems: "flex-start",
          gap: 11,
          background: "#E7F3EE",
          border: "1px solid #CDE7DC",
          borderRadius: 13,
          padding: "13px 16px",
          marginBottom: 20,
        }}
      >
        <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }} aria-hidden="true">
          <path d="M20 6L9 17l-5-5" />
        </svg>
        <div style={{ fontSize: 13, lineHeight: 1.65, color: "#2B4A3E" }}>
          五张卡片都写成了句子、该接素材的都接上了——本环节门禁通过。可以进成稿打磨了。
        </div>
      </div>
    );
  }
  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 11,
        background: "#FBEEE7",
        border: "1px solid #F1D6C8",
        borderRadius: 13,
        padding: "13px 16px",
        marginBottom: 20,
      }}
    >
      <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="#C96F4F" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }} aria-hidden="true">
        <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
        <path d="M12 9v4M12 17h.01" />
      </svg>
      <div style={{ fontSize: 13, lineHeight: 1.65, color: "#8A4A32" }}>
        还有卡片没完成——每张都要写成句子，需要素材的卡片至少选一条。全部完成，本环节门禁就过了。
      </div>
    </div>
  );
}

function RoleCard({ card }: { card: StructureCardFx }) {
  const done = card.status === "done";
  const active = card.status === "active";
  const empty = card.status === "empty";

  const statusLabel = done ? "已完成" : active ? "进行中" : "待开始";
  const statusColor = done ? "#4C9A82" : active ? "#2A3B7A" : "#AEB4C2";
  const statusBg = done ? "#E7F3EE" : active ? "#EDEFF9" : "#F1F2F5";

  return (
    <div
      style={{
        background: "#fff",
        border: active ? "1px solid #D7DCF3" : "1px solid #ECEEF3",
        borderRadius: 14,
        padding: "14px 16px",
        marginBottom: 12,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          style={{
            fontSize: 12,
            fontWeight: 700,
            color: "#5B6373",
            background: "#F3F4F7",
            padding: "3px 10px",
            borderRadius: 999,
          }}
        >
          {card.role}
        </span>
        <span
          style={{
            fontSize: 11.5,
            fontWeight: 700,
            color: statusColor,
            background: statusBg,
            padding: "3px 10px",
            borderRadius: 999,
          }}
        >
          {statusLabel}
        </span>
      </div>

      {done && card.preview && (
        <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.55, color: "#2B3346" }}>{card.preview}</div>
      )}

      {empty && (
        <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.55, color: "#AEB4C2" }}>还没开始——点开这张卡片开始。</div>
      )}

      {active && (
        <>
          <div style={{ display: "flex", gap: 9, alignItems: "flex-start", margin: "13px 0 14px" }}>
            <Bean size={26} />
            <div
              style={{
                flex: 1,
                fontSize: 13,
                lineHeight: 1.6,
                color: "#5B6373",
                background: "#F7F8FB",
                border: "1px solid #EEF0F5",
                borderRadius: 10,
                padding: "9px 12px",
              }}
            >
              {card.question}
            </div>
          </div>
          <div style={{ fontSize: 11.5, fontWeight: 700, color: "#9AA1B0", marginBottom: 8 }}>
            ① 选择相关素材（信源评估里已锁定的）
          </div>
          <div
            aria-disabled="true"
            style={{
              display: "flex",
              gap: 8,
              flexWrap: "wrap",
              marginBottom: 14,
              fontSize: 12,
              color: "#C2C8D6",
              border: "1px dashed #E1E4ED",
              borderRadius: 10,
              padding: "10px 12px",
            }}
          >
            素材选择器（Slice 7 接入）
          </div>
          <div style={{ fontSize: 11.5, fontWeight: 700, color: "#9AA1B0", margin: "6px 0 6px" }}>
            ② 基于素材，把这一步写成句子
          </div>
          <textarea
            disabled
            rows={3}
            placeholder="用你自己的话写……"
            style={{
              width: "100%",
              border: "1px solid #E1E4ED",
              borderRadius: 10,
              padding: "10px 12px",
              fontSize: 13.5,
              lineHeight: 1.65,
              color: "#1C2333",
              background: "#F7F8FB",
              outline: "none",
              resize: "vertical",
            }}
          />
        </>
      )}
    </div>
  );
}

export function StructureView({ cards }: StructureViewProps) {
  if (cards.length === 0) {
    return (
      <div style={WRAP}>
        <div style={COL}>
          <div style={{ ...DEFERRED_CARD, textAlign: "center", color: "#8A92A3", fontSize: 13.5, fontWeight: 600 }}>
            此环节的深入交互将在后续切片接入
          </div>
        </div>
      </div>
    );
  }

  const allClean = cards.every((c) => c.status !== "empty");

  return (
    <div style={WRAP}>
      <div style={COL}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginBottom: 14 }}>
          <span
            style={{
              fontSize: 12.5,
              fontWeight: 700,
              color: "#2A3B7A",
              background: "#EDEFF9",
              padding: "4px 11px",
              borderRadius: 999,
            }}
          >
            论证构建 · 已完成 {cards.filter((c) => c.status !== "empty").length}/{cards.length}
          </span>
        </div>

        <GateBanner allClean={allClean} />

        <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, marginBottom: 16 }}>
          逐张卡片来：点开一张，先选相关素材（你在信源评估里锁定的），再基于素材把这一步写成句子。印记只提问、不代笔。
        </div>

        {cards.map((c) => (
          <RoleCard key={c.id} card={c} />
        ))}
      </div>
    </div>
  );
}
