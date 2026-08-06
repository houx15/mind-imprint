export const NEUTRAL = {
  paper: "#FBF8F4", surface: "#FFFFFF", border: "#EFE7DD",
  muted: "#8A827A", secondary: "#5A5049", faint: "#B0A599",
  ink: "#33302E", inputBorder: "#E4DACF",
} as const;

export type MacaronName = "peach" | "butter" | "matcha" | "lake" | "mist" | "taro" | "berry";
export const MACARONS: Record<MacaronName, { base: string; bg: string; fg: string }> = {
  peach:  { base: "#F6B26B", bg: "#FDE7D3", fg: "#9A5A22" },
  butter: { base: "#F2CE63", bg: "#FAF3DE", fg: "#8A6320" },
  matcha: { base: "#A6C88F", bg: "#E7F1DD", fg: "#4D6B3A" },
  lake:   { base: "#6FBFB0", bg: "#E0F0EC", fg: "#177368" },
  mist:   { base: "#7FA8D8", bg: "#E6EEF9", fg: "#3C5A86" },
  taro:   { base: "#B49BD8", bg: "#F0EAF6", fg: "#5B4A80" },
  berry:  { base: "#E896A6", bg: "#FCE7EB", fg: "#A63A50" },
};

export const SEMANTIC = {
  success: { base: "#5FA97E", bg: "#E7F1DD" },
  warning: { base: "#E0A63A", bg: "#FBEFD6" },
  danger:  { base: "#D64541", bg: "#FBE3E1" },
  info:    { base: "#4C7AB0", bg: "#EAF0F6" },
} as const;

export const RADIUS = { xs: 6, sm: 8, md: 10, lg: 12, full: 999 } as const;

export const SHADOW = {
  xs: "0 1px 2px rgba(70,45,45,.06)",
  sm: "0 2px 8px rgba(70,45,45,.08)",
  md: "0 6px 18px rgba(70,45,45,.11)",
  lg: "0 16px 40px rgba(45,28,28,.18)",
} as const;

export const MOTION = { ease: "cubic-bezier(.2,0,0,1)", fast: 120, base: 200, slow: 320 } as const;
