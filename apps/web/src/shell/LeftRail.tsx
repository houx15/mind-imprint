// LEFT RAIL — lifted verbatim from docs/design/思维印记_工作区.dc.html lines 104–141
// Logo SVG: lines 106–115
// Nav items: 任务 117–122 (icon 119), 记录 124–129 (icon 126), 设置 131–136 (icon 133)
// Avatar dot: 138–140

type TabKey = "tasks" | "records" | "settings";

interface NavItem {
  key: TabKey;
  label: string;
  icon: (stroke: string) => React.ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  {
    key: "tasks",
    label: "任务",
    // icon lifted from HTML line 119
    icon: (stroke) => (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <rect x="3" y="3" width="7" height="7" rx="1.5" />
        <rect x="14" y="3" width="7" height="7" rx="1.5" />
        <rect x="3" y="14" width="7" height="7" rx="1.5" />
        <rect x="14" y="14" width="7" height="7" rx="1.5" />
      </svg>
    ),
  },
  {
    key: "records",
    label: "记录",
    // icon lifted from HTML line 126
    icon: (stroke) => (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M4 19.5V5a2 2 0 012-2h12a1 1 0 011 1v15a1 1 0 01-1 1H6.5A2.5 2.5 0 014 18.5" />
        <path d="M9 7h6M9 11h4" />
      </svg>
    ),
  },
  {
    key: "settings",
    label: "设置",
    // icon lifted from HTML line 133
    icon: (stroke) => (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke={stroke}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
      </svg>
    ),
  },
];

// Active/inactive style constants per brief
const BOX_BASE: React.CSSProperties = {
  width: 42,
  height: 42,
  borderRadius: 12,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
};

const ACTIVE_BOX: React.CSSProperties = { ...BOX_BASE, background: "#EDEFF9" };
const INACTIVE_BOX: React.CSSProperties = { ...BOX_BASE, background: "transparent" };

const ACTIVE_ICON_STROKE = "#2A3B7A";
const INACTIVE_ICON_STROKE = "#9AA1B0";

const ACTIVE_LABEL: React.CSSProperties = { color: "#2A3B7A", fontWeight: 700, fontSize: 10 };
const INACTIVE_LABEL: React.CSSProperties = { color: "#9AA1B0", fontSize: 10 };

export function LeftRail({
  tab,
  onTab,
}: {
  tab: TabKey;
  onTab: (t: TabKey) => void;
}) {
  return (
    /* Left rail container — HTML line 105 */
    <div
      style={{
        width: 74,
        flexShrink: 0,
        background: "#FFFFFF",
        borderRight: "1px solid #EAECF2",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        padding: "16px 0",
        gap: 4,
      }}
    >
      {/* Logo SVG — HTML lines 106–115 */}
      <svg
        viewBox="0 0 48 48"
        width="38"
        height="38"
        style={{ display: "block", marginBottom: 16 }}
      >
        <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A" />
        <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10" />
        <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
        <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
        <path
          d="M19 31.5 Q24 35 29 31.5"
          stroke="#fff"
          strokeWidth="2.2"
          fill="none"
          strokeLinecap="round"
        />
        <circle cx="39" cy="9" r="4.5" fill="#E8A33D" />
      </svg>

      {/* Nav items — HTML lines 117–136 */}
      {NAV_ITEMS.map(({ key, label, icon }) => {
        const isActive = tab === key;
        const boxStyle = isActive ? ACTIVE_BOX : INACTIVE_BOX;
        const iconStroke = isActive ? ACTIVE_ICON_STROKE : INACTIVE_ICON_STROKE;
        const labelStyle = isActive ? ACTIVE_LABEL : INACTIVE_LABEL;

        return (
          <div
            key={key}
            role="tab"
            aria-selected={isActive}
            onClick={() => onTab(key)}
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 5,
              cursor: "pointer",
              padding: "4px 0",
            }}
          >
            <div style={boxStyle}>{icon(iconStroke)}</div>
            <span style={labelStyle}>{label}</span>
          </div>
        );
      })}

      {/* Avatar dot — HTML lines 138–140 */}
      <div
        style={{
          marginTop: "auto",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: 5,
        }}
      >
        <div
          onClick={() => onTab("settings")}
          style={{
            width: 36,
            height: 36,
            borderRadius: "50%",
            background: "#E8A33D",
            color: "#fff",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 14,
            fontWeight: 700,
            cursor: "pointer",
          }}
        >
          P
        </div>
      </div>
    </div>
  );
}
