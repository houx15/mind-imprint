import { SummonCardCall } from "@mind-imprint/contracts";
import type { Message, CardInstance, CardSpec } from "@mind-imprint/contracts";

// ─── ChatItem discriminated union ─────────────────────────────────────────────

export type StudentItem = {
  kind: "student";
  text: string;
  link?: string;
};

export type AiTextItem = {
  kind: "ai_text";
  text: string;
};

/** Visual status for the proposal bubble. active and proposed both show the
 * open/skip buttons (per HTML: isProposed: status==='proposed'||status==='active'). */
export type ProposalStatus = "proposed" | "completed" | "skipped";

export type ProposalItem = {
  kind: "proposal";
  cardInstanceId: string;
  category: string;
  cardName: string;
  nudge: string;
  status: ProposalStatus;
};

export type ChatItem = StudentItem | AiTextItem | ProposalItem;

// ─── URL extraction ────────────────────────────────────────────────────────────

const URL_RE = /https?:\/\/[^\s]+/;

function extractUrl(text: string): string | undefined {
  const m = URL_RE.exec(text);
  return m ? m[0] : undefined;
}

// ─── status mapping ────────────────────────────────────────────────────────────

function toProposalStatus(status: CardInstance["status"]): ProposalStatus {
  // active shows open/skip buttons — same visual as proposed
  if (status === "active" || status === "proposed") return "proposed";
  if (status === "completed") return "completed";
  return "skipped";
}

// ─── main transform ────────────────────────────────────────────────────────────

export function messagesToItems(
  messages: Message[],
  cardById: (id: string) => CardInstance | undefined,
  specById: (cardId: string) => CardSpec | undefined,
): ChatItem[] {
  const items: ChatItem[] = [];

  for (const msg of messages) {
    if (msg.role === "system") continue;

    if (msg.role === "user") {
      const link = extractUrl(msg.content);
      const item: StudentItem = { kind: "student", text: msg.content };
      if (link !== undefined) item.link = link;
      items.push(item);
      continue;
    }

    // assistant message — check for summon_card tool call
    if (msg.tool_call !== null && msg.tool_call !== undefined) {
      const parsed = SummonCardCall.safeParse(msg.tool_call);
      if (parsed.success) {
        const { args, card_instance_id } = parsed.data;
        const ci = cardById(card_instance_id);
        const spec = specById(args.card_id);

        if (spec !== undefined) {
          const item: ProposalItem = {
            kind: "proposal",
            cardInstanceId: card_instance_id,
            category: spec.category,
            cardName: spec.name,
            nudge: args.nudge_text,
            status: ci ? toProposalStatus(ci.status) : "proposed",
          };
          items.push(item);
          continue;
        }
      }
    }

    // plain assistant text (no tool_call, or unrecognised tool_call)
    items.push({ kind: "ai_text", text: msg.content });
  }

  return items;
}
