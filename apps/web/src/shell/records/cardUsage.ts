import type { CardInstance, CardSpec } from "@mind-imprint/contracts";

export interface CardUsageView {
  name: string; purpose: string; usage: string;
  badge: string; badgeStyle: string; boxStyle: string;
}
export interface CardGroup {
  name: string; dotStyle: string; countLabel: string; cards: CardUsageView[];
}

const DOT = "width:9px; height:9px; border-radius:50%; background:#2A3B7A; display:inline-block;";
const BOX = "background:#fff; border:1px solid #EAECF2; border-radius:14px; padding:14px 16px;";
const BADGE_COMMON = "font-size:11px; font-weight:700; color:#2A3B7A; background:#EDEFF9; padding:2px 9px; border-radius:999px;";
const BADGE_USED = "font-size:11px; font-weight:700; color:#6B7384; background:#F1F2F5; padding:2px 9px; border-radius:999px;";

export function deriveCardUsage(
  cards: CardInstance[],
  registry: Record<string, CardSpec>,
): CardGroup[] {
  const counts = new Map<string, number>();
  for (const c of cards) counts.set(c.card_id, (counts.get(c.card_id) ?? 0) + 1);

  const groups = new Map<string, CardUsageView[]>();
  const order: string[] = [];
  for (const [cardId, n] of counts) {
    const spec = registry[cardId];
    if (!spec) continue;
    const category = spec.category;
    if (!groups.has(category)) { groups.set(category, []); order.push(category); }
    const purpose = (spec as unknown as { purpose?: string; one_liner?: string }).purpose
      ?? (spec as unknown as { one_liner?: string }).one_liner ?? "";
    groups.get(category)!.push({
      name: spec.name, purpose, usage: `用过 ${n} 次`,
      badge: n >= 3 ? "常用" : "已用",
      badgeStyle: n >= 3 ? BADGE_COMMON : BADGE_USED,
      boxStyle: BOX,
    });
  }
  return order.map((category) => {
    const cs = groups.get(category)!;
    return { name: category, dotStyle: DOT, countLabel: `${cs.length} 张`, cards: cs };
  });
}
