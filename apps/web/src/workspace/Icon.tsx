// Minimal geometric line-icons for the prototype shell + blocks. Stroke-based,
// inherit `currentColor`, sized by `size`. Kept tiny on purpose — refined, not
// emoji-slop.
import type { BlockKey } from "./blocks/mockData";

export function Icon({ name, size = 18 }: { name: string; size?: number }) {
  const common = {
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.7,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (name) {
    case "forming":
      return (
        <svg {...common}>
          <path d="M6 3h9l4 4v14H6z" />
          <path d="M14 3v5h5M9 13h6M9 17h4" />
        </svg>
      );
    case "plan":
      return (
        <svg {...common}>
          <rect x="4" y="4" width="16" height="16" rx="2.5" />
          <path d="M9 4v16M4 9h5M4 15h5" />
        </svg>
      );
    case "reading":
      return (
        <svg {...common}>
          <path d="M12 6c-2-1.4-4.5-1.6-7-1v12c2.5-.6 5-.4 7 1 2-1.4 4.5-1.6 7-1V5c-2.5-.6-5-.4-7 1z" />
          <path d="M12 6v13" />
        </svg>
      );
    case "writing":
      return (
        <svg {...common}>
          <path d="M4 20h16" />
          <path d="M15.5 4.5l4 4L8 20l-4 1 1-4L15.5 4.5z" />
        </svg>
      );
    case "reflection":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="8" />
          <path d="M12 8a4 4 0 000 8" />
        </svg>
      );
    case "back":
      return (
        <svg {...common}>
          <path d="M15 6l-6 6 6 6" />
        </svg>
      );
    case "send":
      return (
        <svg {...common}>
          <path d="M4 12l16-8-6 16-3-7-7-1z" />
        </svg>
      );
    case "arrow":
      return (
        <svg {...common}>
          <path d="M5 12h14M13 6l6 6-6 6" />
        </svg>
      );
    case "spark":
      return (
        <svg {...common}>
          <path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5L18 18M18 6l-2.5 2.5M8.5 15.5L6 18" />
        </svg>
      );
    case "library":
      return (
        <svg {...common}>
          <path d="M4 4h4v16H4zM10 4h4v16h-4z" />
          <path d="M16.5 4.6l3.3 15.6-3.9.9L12.5 5.6z" />
        </svg>
      );
    case "explore":
      return (
        <svg {...common}>
          <circle cx="6" cy="7" r="2" />
          <circle cx="18" cy="7" r="2" />
          <circle cx="12" cy="18" r="2" />
          <path d="M7.7 8.3L10.4 16.4M16.3 8.3L13.6 16.4M8 7h8" />
        </svg>
      );
    default:
      return null;
  }
}

// `label` is the single source of truth for the room's short name — the
// studio top-bar room switcher (WorkspaceContainer's `TopBar`, spec §17)
// renders these labels directly, so there's no second hardcoded copy to
// drift. 立项 (P2a) splits into two segments sharing one component
// (PlanBlock): 提案 is the forming coach chat, 管理 is the persisted board
// (still internally headed "项目管理" inside PlanBlock's board view).
export const BLOCK_META: { key: BlockKey; label: string; sub: string }[] = [
  { key: "forming", label: "立题", sub: "Proposal" },
  { key: "plan", label: "管理", sub: "Plan" },
  { key: "reading", label: "阅读", sub: "Read" },
  { key: "writing", label: "写作", sub: "Write" },
  { key: "reflection", label: "回顾", sub: "Review" },
];
