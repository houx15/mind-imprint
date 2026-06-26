// No React import needed — `React.CSSProperties` / `React.ReactNode` resolve via the
// global namespace from @types/react, matching shell/LeftRail.tsx.

export type ConsoleTab = "overview" | "classes" | "teachers" | "import" | "settings";

interface NavItem {
  key: ConsoleTab;
  label: string;
  icon: (stroke: string) => React.ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  {
    key: "overview",
    label: "概览",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="9" rx="1.5" />
        <rect x="14" y="3" width="7" height="5" rx="1.5" />
        <rect x="14" y="12" width="7" height="9" rx="1.5" />
        <rect x="3" y="16" width="7" height="5" rx="1.5" />
      </svg>
    ),
  },
  {
    key: "classes",
    label: "班级",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
      </svg>
    ),
  },
  {
    key: "teachers",
    label: "教师",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M16 21v-2a4 4 0 00-4-4H6a4 4 0 00-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M22 21v-2a4 4 0 00-3-3.87" />
      </svg>
    ),
  },
  {
    key: "import",
    label: "导入",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
        <path d="M7 10l5 5 5-5M12 15V3" />
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

const TEACHER_TABS: ConsoleTab[] = ["classes", "settings"];

const BOX_BASE: React.CSSProperties = { width: 42, height: 42, borderRadius: 12, display: "flex", alignItems: "center", justifyContent: "center" };
const ACTIVE_BOX: React.CSSProperties = { ...BOX_BASE, background: "#EDEFF9" };
const INACTIVE_BOX: React.CSSProperties = { ...BOX_BASE, background: "transparent" };
const ACTIVE_ICON_STROKE = "#2A3B7A";
const INACTIVE_ICON_STROKE = "#9AA1B0";
const ACTIVE_LABEL: React.CSSProperties = { color: "#2A3B7A", fontWeight: 700, fontSize: 10 };
const INACTIVE_LABEL: React.CSSProperties = { color: "#9AA1B0", fontSize: 10 };

export function ConsoleRail({ role, tab, onTab }: { role: string; tab: ConsoleTab; onTab: (t: ConsoleTab) => void }) {
  const items = role === "admin" ? NAV_ITEMS : NAV_ITEMS.filter((i) => TEACHER_TABS.includes(i.key));
  return (
    <div style={{ width: 74, flexShrink: 0, background: "#FFFFFF", borderRight: "1px solid #EAECF2", display: "flex", flexDirection: "column", alignItems: "center", padding: "16px 0", gap: 4 }}>
      {/* logo SVG — unchanged */}
      <svg viewBox="0 0 48 48" width="38" height="38" style={{ display: "block", marginBottom: 16 }}>
        <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A" />
        <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10" />
        <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
        <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
        <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round" />
        <circle cx="39" cy="9" r="4.5" fill="#E8A33D" />
      </svg>

      {items.map(({ key, label, icon }) => {
        const isActive = tab === key;
        return (
          <div
            key={key}
            role="tab"
            aria-selected={isActive}
            onClick={() => onTab(key)}
            style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 5, cursor: "pointer", padding: "4px 0" }}
          >
            <div style={isActive ? ACTIVE_BOX : INACTIVE_BOX}>{icon(isActive ? ACTIVE_ICON_STROKE : INACTIVE_ICON_STROKE)}</div>
            <span style={isActive ? ACTIVE_LABEL : INACTIVE_LABEL}>{label}</span>
          </div>
        );
      })}
    </div>
  );
}
