import type { ProcessNode } from "./processTree";
import { nodeView } from "./nodeView";

export function TreeBody({ nodes = [] }: { nodes?: ProcessNode[] }) {
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "18px 20px 28px" }}>
      {nodes.length > 1
        ? nodes.map((n) => {
            const v = nodeView(n);
            return (
              <div key={n.id} style={v.rowStyle}>
                <div style={{ flex: "none", display: "flex", flexDirection: "column", alignItems: "center", paddingTop: "3px" }}>
                  <div style={v.markerStyle} />
                  <div style={{ width: "2px", flex: 1, background: "#EDEEF3", marginTop: "4px", minHeight: "8px" }} />
                </div>
                <div style={{ flex: 1, paddingBottom: "10px" }}>
                  <span style={v.tagStyle}>{v.tag}</span>
                  <div style={{ fontSize: "13.5px", fontWeight: 600, color: "#2B3346", lineHeight: 1.5, marginTop: "6px" }}>
                    {n.title}
                  </div>
                  {n.sub && (
                    <div style={{ fontSize: "12px", color: "#9AA1B0", marginTop: "3px" }}>{n.sub}</div>
                  )}
                </div>
              </div>
            );
          })
        : null}
      <div style={{ textAlign: "center", fontSize: "11.5px", color: "#C2C8D6", marginTop: "10px", fontWeight: 500 }}>
        边做边长 · 随评估归并枝节
      </div>
    </div>
  );
}
