type Props = { visible: boolean; onOpen: () => void };

// "Offer, don't push" — a calm, dismissable-by-opening chip. No badge count, no auto-open, no streak.
export function MindImprintIndicator({ visible, onOpen }: Props) {
  if (!visible) return null;
  return (
    <button type="button" onClick={onOpen}
      style={{
        position: "absolute", bottom: "24px", right: "24px", zIndex: 40,
        display: "flex", alignItems: "center", gap: "8px",
        background: "#2A3B7A", color: "#fff", border: "none",
        padding: "11px 16px", borderRadius: "999px", cursor: "pointer",
        fontFamily: "inherit", fontSize: "13.5px", fontWeight: 700,
        boxShadow: "0 6px 20px rgba(42,59,122,.28)", animation: "mkPop .3s cubic-bezier(.22,.9,.3,1)",
      }}>
      <span aria-hidden="true">✨</span> 你的思维印记有新内容
    </button>
  );
}
