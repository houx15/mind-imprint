// LEFT RAIL — four pillars per binding design 思维印记 工作区.dc.html (left-rail section).
// Tabs: 课程 (courses) / 批判思维 (tasks portal) / 我的评估 (records) / 设置 (settings).

type TabKey = "courses" | "tasks" | "records" | "settings";

interface NavItem {
  key: TabKey;
  label: string;
  icon: (stroke: string) => React.ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  {
    key: "courses",
    label: "课程",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 5.5A2.5 2.5 0 016.5 3H20v15H6.5A2.5 2.5 0 004 20.5z" />
        <path d="M20 18v3H6.5A2.5 2.5 0 014 18.5" />
        <path d="M9 7.5h7M9 11h5" />
      </svg>
    ),
  },
  {
    key: "tasks",
    label: "批判思维",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z" />
      </svg>
    ),
  },
  {
    key: "records",
    label: "我的评估",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 20V10M6 20v-5M18 20V6" />
        <path d="M3 20h18" />
      </svg>
    ),
  },
  {
    key: "settings",
    label: "设置",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
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
