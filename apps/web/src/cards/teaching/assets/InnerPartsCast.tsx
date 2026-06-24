/**
 * InnerPartsCast — the six 内在小人 avatars with metadata.
 * SVG artwork ported from .superpowers/brainstorm/55230-1782285283/content/inner-parts-cast.html
 * Names/quotes from packages/contracts/cards/emotional-alignment.json inner_part_choice options.
 * Pure presentational — no local state, no envelope writes.
 */
import type { ComponentType } from "react";

function ProtectorAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M16 28c0-8 6-13 14-13s14 5 14 13v8c0 5-3 8-8 8H24c-5 0-8-3-8-8z" fill="#A9D6C6" stroke="#2C5B4C" strokeWidth="2.2" />
      <path d="M30 9l8 3v5c0 5-3.4 8.6-8 10-4.6-1.4-8-5-8-10v-5z" fill="#fff" stroke="#2C5B4C" strokeWidth="1.8" />
      <path d="M27 17l2 2 4-4.4" stroke="#3F7F6B" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="25" cy="33" r="2.3" fill="#2C5B4C" />
      <circle cx="35" cy="33" r="2.3" fill="#2C5B4C" />
      <path d="M26 40h8" stroke="#2C5B4C" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

function PerfectAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M16 27c0-8 6-13 14-13s14 5 14 13v9c0 5-3 8-8 8H24c-5 0-8-3-8-8z" fill="#C3CCF2" stroke="#3A47A0" strokeWidth="2.2" />
      <rect x="20" y="40" width="20" height="5" rx="1.4" fill="#fff" stroke="#3A47A0" strokeWidth="1.4" />
      <path d="M24 42v3M28 42v3M32 42v3M36 42v3" stroke="#3A47A0" strokeWidth="1" />
      <circle cx="25" cy="30" r="2.3" fill="#2A3270" />
      <circle cx="35" cy="30" r="2.3" fill="#2A3270" />
      <path d="M22 24l5 1.5M38 24l-5 1.5" stroke="#3A47A0" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M27 36h6" stroke="#2A3270" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

function ProcrastinatorAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M16 30c0-8 6-13 14-13s14 5 14 13v6c0 5-3 8-8 8H24c-5 0-8-3-8-8z" fill="#EAD7AE" stroke="#9C7A3A" strokeWidth="2.2" />
      <path d="M22 31c1.4-1.4 4-1.4 5.4 0M32.6 31c1.4-1.4 4-1.4 5.4 0" stroke="#7A5C24" strokeWidth="2" strokeLinecap="round" />
      <path d="M26 39c2 1.6 6 1.6 8 0" stroke="#7A5C24" strokeWidth="2" strokeLinecap="round" />
      <path d="M40 14h7l-7 7h7" stroke="#9C7A3A" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function AnxiousAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M16 29c0-8 6-13 14-13s14 5 14 13v7c0 5-3 8-8 8H24c-5 0-8-3-8-8z" fill="#F3C9CF" stroke="#B05068" strokeWidth="2.2" />
      <circle cx="25" cy="31" r="3" fill="#fff" stroke="#7A2E42" strokeWidth="1.6" />
      <circle cx="35" cy="31" r="3" fill="#fff" stroke="#7A2E42" strokeWidth="1.6" />
      <circle cx="25" cy="31" r="1.2" fill="#7A2E42" />
      <circle cx="35" cy="31" r="1.2" fill="#7A2E42" />
      <path d="M26 40c1.5-1 6-1 8 0" stroke="#7A2E42" strokeWidth="2" strokeLinecap="round" />
      <path d="M40 16c2-2 4 0 2 2M44 21c2-2 4 0 2 2" stroke="#B05068" strokeWidth="1.6" strokeLinecap="round" />
      <circle cx="44" cy="14" r="1.5" fill="#B05068" />
    </svg>
  );
}

function PleaserAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M16 28c0-8 6-13 14-13s14 5 14 13v8c0 5-3 8-8 8H24c-5 0-8-3-8-8z" fill="#F6CBB6" stroke="#C0633C" strokeWidth="2.2" />
      <circle cx="25" cy="31" r="2.3" fill="#8A3E1E" />
      <circle cx="35" cy="31" r="2.3" fill="#8A3E1E" />
      <path d="M23 38c3 3.5 11 3.5 14 0" stroke="#8A3E1E" strokeWidth="2.2" strokeLinecap="round" />
      <path d="M44 30c2.4-2.2 5 .4 0 3.4-5-3-2.4-5.6 0-3.4z" fill="#E0738C" />
      <path d="M19 33c-1 .5-2 1.5-1.5 3" stroke="#C0633C" strokeWidth="1.4" strokeLinecap="round" />
    </svg>
  );
}

function LittleAdultAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M16 28c0-8 6-13 14-13s14 5 14 13v8c0 5-3 8-8 8H24c-5 0-8-3-8-8z" fill="#A9CBD0" stroke="#356068" strokeWidth="2.2" />
      <path d="M30 38l-3 7h6z" fill="#fff" stroke="#356068" strokeWidth="1.6" strokeLinejoin="round" />
      <circle cx="25" cy="31" r="2.3" fill="#23464C" />
      <circle cx="35" cy="31" r="2.3" fill="#23464C" />
      <path d="M26 36h8" stroke="#23464C" strokeWidth="2" strokeLinecap="round" />
      <path d="M21 24c1.5-1.5 4-1.5 5.5 0" stroke="#356068" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

export const INNER_PARTS: {
  key: string;
  name: string;
  quote: string;
  protects: string;
  Avatar: ComponentType<{ size?: number }>;
}[] = [
  {
    key: "protector",
    name: "保护者",
    quote: "别去冒险，可能会受伤",
    protects: "它在替你挡住可能的受伤",
    Avatar: ProtectorAvatar,
  },
  {
    key: "perfect",
    name: "完美小人",
    quote: "做不好还不如不做",
    protects: "它在替你挡住可能的难堪与失败",
    Avatar: PerfectAvatar,
  },
  {
    key: "procrastinator",
    name: "拖延小人",
    quote: "等会儿再说，现在还不是时候",
    protects: "它在替你挡住开始时的那份压力",
    Avatar: ProcrastinatorAvatar,
  },
  {
    key: "anxious",
    name: "焦虑小人",
    quote: "这么多事根本做不完",
    protects: "它在替你挡住被压垮的失控感",
    Avatar: AnxiousAvatar,
  },
  {
    key: "pleaser",
    name: "讨好小人",
    quote: "我得做对，不然别人怎么看我",
    protects: "它在替你挡住被否定与不被接纳的恐惧",
    Avatar: PleaserAvatar,
  },
  {
    key: "littleadult",
    name: "小大人",
    quote: "我得自己扛，不能麻烦别人",
    protects: "它在替你挡住依赖别人却被辜负的脆弱",
    Avatar: LittleAdultAvatar,
  },
];
