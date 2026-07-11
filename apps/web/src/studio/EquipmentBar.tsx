import type { EquipCard } from "./state";

export type EquipmentBarProps = {
  cards: EquipCard[];
  open: boolean;
  onToggle: () => void;
  onOpen: (id: string) => void;
};

// 装备栏 — the popover PANEL only (design ~L1344-1364). The trigger toolbox
// button lives in the composer row and is owned by the host (CoachRail,
// design ~L1390-1392) — the design has exactly one toolbox toggle. `onToggle`
// here drives the panel's own collapse chevron. Renders nothing when closed.
function ToolboxIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M14.7 6.3a4 4 0 00-5.4 5.4l-6.6 6.6a1.5 1.5 0 002.1 2.1l6.6-6.6a4 4 0 005.4-5.4l-2.1 2.1-2.1-2.1z" />
    </svg>
  );
}

function ChevronDownIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M6 9l6 6 6-6" />
    </svg>
  );
}

export function EquipmentBar({ cards, open, onToggle, onOpen }: EquipmentBarProps) {
  if (!open) return null;
  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {
        <div
          style={{
            borderTop: "1px solid #EFF0F5",
            padding: "13px 16px 12px",
            background: "#FCFCFE",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 7, marginBottom: 10 }}>
            <span style={{ color: "#6B7384", display: "flex" }}>
              <ToolboxIcon />
            </span>
            <span style={{ fontSize: 12.5, fontWeight: 700, color: "#3A4256" }}>装备栏</span>
            <span style={{ fontSize: 11, color: "#AEB4C2", fontWeight: 500 }}>想到什么，随时取用</span>
            <div
              onClick={onToggle}
              title="收起装备栏"
              role="button"
              style={{
                marginLeft: "auto",
                width: 24,
                height: 24,
                borderRadius: 7,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                cursor: "pointer",
                color: "#AEB4C2",
              }}
            >
              <ChevronDownIcon />
            </div>
          </div>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            {cards.map((card) => {
              const isSpont = card.spont === "自发";
              return (
                <div
                  key={card.id}
                  onClick={() => onOpen(card.meth)}
                  role="button"
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 7,
                    padding: "7px 11px",
                    borderRadius: 10,
                    cursor: "pointer",
                    background: "#fff",
                    border: "1px solid #E4E8F0",
                  }}
                >
                  <span style={{ fontSize: 12.5, fontWeight: 700 }}>{card.name}</span>
                  <span
                    style={{
                      fontSize: 10,
                      fontWeight: 700,
                      padding: "1px 7px",
                      borderRadius: 999,
                      color: isSpont ? "#4C9A82" : "#8A92A3",
                      background: isSpont ? "#E7F3EE" : "#F1F2F5",
                    }}
                  >
                    {card.spont}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      }
    </div>
  );
}
