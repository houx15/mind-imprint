import type { ChatItem } from "./viewModel";

// ─── Avatar SVG (reused for AI text + proposal bubbles) ───────────────────────

const AVATAR_COLOR = "#2A3B7A";

function AiAvatar() {
  return (
    <svg
      viewBox="0 0 48 48"
      width="32"
      height="32"
      style={{ display: "block", flex: "none", marginTop: "1px" }}
    >
      <rect x="5" y="6" width="38" height="36" rx="13" fill={AVATAR_COLOR} />
      <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.12" />
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
    </svg>
  );
}

// ─── Link icon ─────────────────────────────────────────────────────────────────

function LinkIcon() {
  return (
    <svg
      width="13"
      height="13"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#C7CEF0"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M10 13a5 5 0 007 0l3-3a5 5 0 00-7-7l-1 1" />
      <path d="M14 11a5 5 0 00-7 0l-3 3a5 5 0 007 7l1-1" />
    </svg>
  );
}

// ─── Item renderers ────────────────────────────────────────────────────────────

type StudentBubbleProps = {
  text: string;
  link?: string;
};

function StudentBubble({ text, link }: StudentBubbleProps) {
  return (
    <div className="flex justify-end">
      <div
        className="max-w-[540px] rounded-[16px_16px_5px_16px] px-[17px] py-[13px] text-[14.5px] leading-[1.66]"
        style={{ background: "#2A3B7A", color: "#fff" }}
      >
        {text}
        {link !== undefined && (
          <div
            className="mt-[9px] flex items-center gap-[6px] overflow-hidden rounded-[8px] px-[10px] py-[6px] text-[12.5px]"
            style={{
              color: "#C7CEF0",
              background: "rgba(255,255,255,.10)",
            }}
          >
            <LinkIcon />
            <span className="overflow-hidden text-ellipsis whitespace-nowrap">{link}</span>
          </div>
        )}
      </div>
    </div>
  );
}

type AiTextBubbleProps = {
  text: string;
};

function AiTextBubble({ text }: AiTextBubbleProps) {
  return (
    <div className="flex items-start gap-[12px]">
      <AiAvatar />
      <div className="max-w-[560px]">
        <div
          className="rounded-[5px_16px_16px_16px] border border-mk-border px-[17px] py-[13px] text-[14.5px] leading-[1.72]"
          style={{
            background: "#fff",
            color: "#2B3346",
            boxShadow: "0 1px 2px rgba(20,30,60,.04)",
          }}
        >
          {text}
        </div>
      </div>
    </div>
  );
}

// ─── Card icon (for 建议工具卡 badge) ─────────────────────────────────────────

function CardIcon() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#D98263"
      strokeWidth="2.4"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="3" y="6" width="18" height="13" rx="2.5" />
    </svg>
  );
}

// Arrow icon for 打开卡 button
function ArrowIcon() {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#fff"
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M5 12h14M13 6l6 6-6 6" />
    </svg>
  );
}

// Checkmark icon for 已完成
function CheckIcon() {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#4C9A82"
      strokeWidth="2.4"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

type ProposalBubbleProps = {
  category: string;
  cardName: string;
  nudge: string;
  status: "proposed" | "completed" | "skipped";
  cardInstanceId: string;
  onOpenCard: (id: string) => void;
  onSkipCard: (id: string) => void;
};

function ProposalBubble({
  category,
  cardName,
  nudge,
  status,
  cardInstanceId,
  onOpenCard,
  onSkipCard,
}: ProposalBubbleProps) {
  return (
    <div className="flex items-start gap-[12px]">
      <AiAvatar />
      <div
        className="w-full max-w-[520px] rounded-[5px_16px_16px_16px] border border-mk-border px-[18px] py-[16px]"
        style={{
          background: "#fff",
          boxShadow: "0 2px 10px rgba(20,30,60,.05)",
        }}
      >
        {/* Header badges */}
        <div className="mb-[11px] flex items-center gap-[9px]">
          <span
            className="inline-flex items-center gap-[6px] rounded-full px-[10px] py-[4px] text-[11px] font-bold"
            style={{ color: "#D98263", background: "#FBEEE7" }}
          >
            <CardIcon />
            建议工具卡
          </span>
          <span
            className="rounded-full px-[9px] py-[3px] text-[11px] font-semibold"
            style={{ color: "#2A3B7A", background: "#EDEFF9" }}
          >
            {category}
          </span>
        </div>

        {/* Card name */}
        <div
          className="mb-[5px] text-[15px] font-bold"
          style={{ color: "#1C2333" }}
        >
          {cardName}
        </div>

        {/* Nudge text */}
        <div className="text-[14px] leading-[1.66]" style={{ color: "#5B6373" }}>
          {nudge}
        </div>

        {/* Proposed state: 打开卡 + 暂不，先继续 */}
        {status === "proposed" && (
          <div className="mt-[14px] flex items-center gap-[14px]">
            <button
              type="button"
              onClick={() => onOpenCard(cardInstanceId)}
              className="inline-flex items-center gap-[8px] rounded-[11px] border-none px-[20px] py-[11px] text-[14px] font-bold text-white"
              style={{
                background: "#D98263",
                cursor: "pointer",
                boxShadow: "0 4px 12px rgba(217,130,99,.30)",
              }}
            >
              打开卡
              <ArrowIcon />
            </button>
            <button
              type="button"
              onClick={() => onSkipCard(cardInstanceId)}
              className="border-none bg-transparent px-[4px] py-[6px] text-[13px] font-semibold"
              style={{ color: "#9AA1B0", cursor: "pointer" }}
            >
              暂不，先继续
            </button>
          </div>
        )}

        {/* Completed state */}
        {status === "completed" && (
          <div
            className="mt-[13px] inline-flex items-center gap-[7px] rounded-[9px] px-[13px] py-[7px] text-[13px] font-semibold"
            style={{ color: "#4C9A82", background: "#E7F3EE" }}
          >
            <CheckIcon />
            已完成 · 已钉到过程树
          </div>
        )}

        {/* Skipped state */}
        {status === "skipped" && (
          <div className="mt-[13px] flex items-center gap-[12px]">
            <span
              className="rounded-[9px] px-[12px] py-[7px] text-[13px] font-semibold"
              style={{ color: "#9AA1B0", background: "#F1F2F5" }}
            >
              已跳过（已记录为信号）
            </span>
            <button
              type="button"
              onClick={() => onOpenCard(cardInstanceId)}
              className="border-none bg-transparent text-[13px] font-semibold underline underline-offset-[3px]"
              style={{ color: "#2A3B7A", cursor: "pointer" }}
            >
              仍可打开
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// ─── ChatLog ──────────────────────────────────────────────────────────────────

type Props = {
  items: ChatItem[];
  onOpenCard: (cardInstanceId: string) => void;
  onSkipCard: (cardInstanceId: string) => void;
};

export function ChatLog({ items, onOpenCard, onSkipCard }: Props) {
  return (
    <div
      id="mk-chat"
      style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "32px 36px 24px" }}
    >
      <div style={{ maxWidth: "720px", margin: "0 auto" }}>
        {items.map((item, index) => (
          // eslint-disable-next-line react/no-array-index-key
          <div key={index} style={{ marginBottom: "20px" }}>
            {item.kind === "student" && (
              <StudentBubble text={item.text} link={item.link} />
            )}
            {item.kind === "ai_text" && (
              <AiTextBubble text={item.text} />
            )}
            {item.kind === "proposal" && (
              <ProposalBubble
                category={item.category}
                cardName={item.cardName}
                nudge={item.nudge}
                status={item.status}
                cardInstanceId={item.cardInstanceId}
                onOpenCard={onOpenCard}
                onSkipCard={onSkipCard}
              />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
