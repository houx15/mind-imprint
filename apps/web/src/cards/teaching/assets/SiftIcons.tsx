/**
 * SiftIcons — inline SVG icons for each SIFT×CRAAP step.
 * Ported from .superpowers/brainstorm/55230-1782285283/content/sift-interactions.html
 * Pure presentational — no local state, no envelope writes.
 */

export function StopIcon({ size = 48 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="24" cy="24" r="16" fill="#F2C9A6" />
      <circle cx="24" cy="24" r="16" stroke="#B5632F" strokeWidth="2" />
      <rect x="18" y="17" width="4.4" height="14" rx="2.2" fill="#8A4A22" />
      <rect x="25.6" y="17" width="4.4" height="14" rx="2.2" fill="#8A4A22" />
      <path d="M14 11c2 2 4 2 6 0" stroke="#B5632F" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function InvestigateIcon({ size = 48 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="20" cy="20" r="12" fill="#DCE6FF" stroke="#2A3B7A" strokeWidth="2" />
      <circle cx="20" cy="20" r="7" fill="#fff" stroke="#2A3B7A" strokeWidth="1.6" />
      <path d="M29 29l9 9" stroke="#2A3B7A" strokeWidth="3.4" strokeLinecap="round" />
      <path d="M15 19c1-2 4-2.5 6-1" stroke="#7E96D8" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export function FindIcon({ size = 48 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect x="9" y="14" width="14" height="19" rx="2" fill="#E7CFA0" stroke="#9C7A3A" strokeWidth="1.8" />
      <rect x="25" y="10" width="15" height="23" rx="2" fill="#AFD8C8" stroke="#2C5B4C" strokeWidth="1.8" />
      <path d="M29 21l3 3 5-6" stroke="#2C5B4C" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M13 20h6M13 24h6" stroke="#9C7A3A" strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  );
}

export function TraceIcon({ size = 48 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M14 14l8 8M22 22l-8 8" stroke="#C2557A" strokeWidth="2.4" strokeLinecap="round" />
      <circle cx="14" cy="14" r="4" fill="#F4C6D8" stroke="#C2557A" strokeWidth="2" />
      <circle cx="14" cy="30" r="4" fill="#F4C6D8" stroke="#C2557A" strokeWidth="2" />
      <circle cx="34" cy="22" r="6" fill="#fff" stroke="#C2557A" strokeWidth="2.4" />
      <path d="M31.5 22l1.6 1.6 3-3.4" stroke="#C2557A" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function CraapIcon({ size = 48 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <polygon points="30,8 50,22 43,46 17,46 10,22" fill="none" stroke="#C7D0E6" strokeWidth="1.4" />
      <polygon points="30,16 44,25 39,42 22,41 17,26" fill="#C7D6FF" stroke="#2A3B7A" strokeWidth="2" />
      <circle cx="30" cy="8" r="2.2" fill="#2A3B7A" />
      <circle cx="50" cy="22" r="2.2" fill="#2A3B7A" />
      <circle cx="43" cy="46" r="2.2" fill="#2A3B7A" />
      <circle cx="17" cy="46" r="2.2" fill="#2A3B7A" />
      <circle cx="10" cy="22" r="2.2" fill="#2A3B7A" />
    </svg>
  );
}
