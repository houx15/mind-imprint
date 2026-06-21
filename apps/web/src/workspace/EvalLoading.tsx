export function EvalLoading() {
  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        zIndex: 50,
        background: "rgba(22,28,46,.40)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        animation: "mkScrim .2s ease",
      }}
    >
      <div
        style={{
          background: "#fff",
          borderRadius: "16px",
          padding: "26px 32px",
          display: "flex",
          alignItems: "center",
          gap: "14px",
          boxShadow: "0 16px 50px rgba(20,30,60,.22)",
        }}
      >
        <div style={{ display: "flex", gap: "5px" }}>
          <span
            style={{
              width: "8px",
              height: "8px",
              borderRadius: "50%",
              background: "#2A3B7A",
              display: "inline-block",
              animation: "mkDot 1.2s infinite ease-in-out",
            }}
          />
          <span
            style={{
              width: "8px",
              height: "8px",
              borderRadius: "50%",
              background: "#2A3B7A",
              display: "inline-block",
              animation: "mkDot 1.2s .2s infinite ease-in-out",
            }}
          />
          <span
            style={{
              width: "8px",
              height: "8px",
              borderRadius: "50%",
              background: "#2A3B7A",
              display: "inline-block",
              animation: "mkDot 1.2s .4s infinite ease-in-out",
            }}
          />
        </div>
        <span style={{ fontSize: "14px", color: "#3A4256", fontWeight: 600 }}>
          旗舰模型正在评估这一程的思维过程……
        </span>
      </div>
    </div>
  );
}
