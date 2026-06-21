import type { ProcessNode } from "./processTree";
import { nodeView } from "./nodeView";

type Props = {
  open: boolean;
  onToggle: () => void;
  nodes?: ProcessNode[];
};

export function TreePanel({ open, onToggle, nodes = [] }: Props) {
  if (open) {
    return (
      <div
        style={{
          width: "344px",
          flex: "none",
          background: "#fff",
          borderLeft: "1px solid #EAECF2",
          display: "flex",
          flexDirection: "column",
        }}
      >
        {/* Header */}
        <div
          style={{
            height: "54px",
            flex: "none",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "0 20px",
            borderBottom: "1px solid #EFF0F5",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: "9px" }}>
            <svg
              width="17"
              height="17"
              viewBox="0 0 24 24"
              fill="none"
              stroke="#2A3B7A"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <circle cx="6" cy="6" r="2.4" />
              <circle cx="6" cy="18" r="2.4" />
              <circle cx="18" cy="12" r="2.4" />
              <path d="M8 6h6a2 2 0 012 2v2M8 18h6a2 2 0 002-2v-2" />
            </svg>
            <span style={{ fontSize: "14px", fontWeight: 700, color: "#1C2333" }}>过程树</span>
            <span
              style={{
                fontSize: "11px",
                color: "#9AA1B0",
                background: "#F2F3F8",
                padding: "2px 7px",
                borderRadius: "999px",
                fontWeight: 600,
              }}
            >
              只读
            </span>
          </div>
          <button
            type="button"
            aria-label="折叠过程树"
            onClick={onToggle}
            style={{
              cursor: "pointer",
              color: "#9AA1B0",
              padding: "5px",
              borderRadius: "7px",
              display: "flex",
              background: "none",
              border: "none",
            }}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M9 18l6-6-6-6" />
            </svg>
          </button>
        </div>

        {/* Body — node list or empty state */}
        <div
          style={{
            flex: 1,
            minHeight: 0,
            overflowY: "auto",
            padding: "18px 20px 28px",
          }}
        >
          {nodes.length > 1
            ? nodes.map((n) => {
                const v = nodeView(n);
                return (
                  <div key={n.id} style={v.rowStyle}>
                    {/* Marker column with connecting line */}
                    <div
                      style={{
                        flex: "none",
                        display: "flex",
                        flexDirection: "column",
                        alignItems: "center",
                        paddingTop: "3px",
                      }}
                    >
                      <div style={v.markerStyle} />
                      <div
                        style={{
                          width: "2px",
                          flex: 1,
                          background: "#EDEEF3",
                          marginTop: "4px",
                          minHeight: "8px",
                        }}
                      />
                    </div>
                    {/* Content column */}
                    <div style={{ flex: 1, paddingBottom: "10px" }}>
                      <span style={v.tagStyle}>{v.tag}</span>
                      <div
                        style={{
                          fontSize: "13.5px",
                          fontWeight: 600,
                          color: "#2B3346",
                          lineHeight: 1.5,
                          marginTop: "6px",
                        }}
                      >
                        {n.title}
                      </div>
                      {n.sub && (
                        <div
                          style={{
                            fontSize: "12px",
                            color: "#9AA1B0",
                            marginTop: "3px",
                          }}
                        >
                          {n.sub}
                        </div>
                      )}
                    </div>
                  </div>
                );
              })
            : null}
          <div
            style={{
              textAlign: "center",
              fontSize: "11.5px",
              color: "#C2C8D6",
              marginTop: "10px",
              fontWeight: 500,
            }}
          >
            边做边长 · 随评估归并枝节
          </div>
        </div>
      </div>
    );
  }

  // Collapsed state
  return (
    <button
      type="button"
      aria-label="展开过程树"
      onClick={onToggle}
      style={{
        width: "46px",
        flex: "none",
        background: "#fff",
        borderLeft: "1px solid #EAECF2",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        paddingTop: "16px",
        gap: "14px",
        cursor: "pointer",
        border: "none",
        borderLeftWidth: "1px",
        borderLeftStyle: "solid",
        borderLeftColor: "#EAECF2",
      }}
    >
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="#6B7384"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M15 18l-6-6 6-6" />
      </svg>
      <span
        style={{
          writingMode: "vertical-rl",
          fontSize: "12.5px",
          fontWeight: 700,
          color: "#6B7384",
          letterSpacing: ".08em",
        }}
      >
        过程树
      </span>
    </button>
  );
}
