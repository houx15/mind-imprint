import { useEffect, useState } from "react";
import { CARD_REGISTRY, type CollectedCard } from "@mind-imprint/contracts";
import { api } from "../../api";

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "聊天" };

// Fixed display order of the card categories (roughly the course-library arc),
// each with a dot color. Any category not listed falls into a final 其他 group.
const CATEGORY_ORDER: { key: string; dot: string }[] = [
  { key: "探究启动", dot: "#D98263" },
  { key: "信息素养", dot: "#2A3B7A" },
  { key: "溯源与多视角", dot: "#4C9A82" },
  { key: "知识工具", dot: "#7C6BB0" },
  { key: "论证结构", dot: "#C77CA6" },
  { key: "AOK", dot: "#D9A23D" },
  { key: "AI伦理", dot: "#3E8EA8" },
  { key: "反身性与元认知", dot: "#5C6CB0" },
  { key: "成长与沉淀", dot: "#8A9A5B" },
];
const OTHER = { key: "其他", dot: "#9AA1B0" };

type Enriched = CollectedCard & { name: string; purpose: string; category: string };

function enrich(cards: CollectedCard[]): Enriched[] {
  const out: Enriched[] = [];
  for (const c of cards) {
    const spec = CARD_REGISTRY[c.cardId];
    if (!spec) continue; // drop ids absent from the registry (defensive)
    out.push({ ...c, name: spec.name, purpose: spec.purpose, category: spec.category });
  }
  return out;
}

function usageLine(c: CollectedCard): string {
  const surfaces = c.surfaces.map((s) => SURFACE_LABEL[s] ?? s).join(" · ");
  const md = c.lastUsed.slice(5, 10); // MM-DD from an RFC3339 date
  return `${surfaces} · 最近 ${md}`;
}

export function ToolkitCards() {
  const [cards, setCards] = useState<CollectedCard[] | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await api.getGrowthCards();
        if (!cancelled) setCards(list);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (error) return <div style={{ padding: 24, color: "#B0432E" }}>{error}</div>;
  if (cards === undefined) return <div style={{ padding: 24, color: "#9AA1B0" }}>正在整理你的工具卡…</div>;

  const enriched = enrich(cards);
  if (enriched.length === 0) {
    return (
      <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "34px 24px", textAlign: "center" }}>
        <div style={{ fontSize: 14.5, fontWeight: 700, color: "#3A4256" }}>还没有收集到工具卡</div>
        <div style={{ fontSize: 13.5, color: "#6B7384", marginTop: 8, lineHeight: 1.7 }}>在任务、课程或聊天里用过一张思维工具卡，它就会出现在这里。</div>
      </div>
    );
  }

  const groups = [...CATEGORY_ORDER, OTHER].map(({ key, dot }) => ({
    key, dot,
    cards: enriched.filter((c) => (key === OTHER.key
      ? !CATEGORY_ORDER.some((g) => g.key === c.category)
      : c.category === key)),
  })).filter((g) => g.cards.length > 0);

  return (
    <div>
      <div style={{ fontSize: 13.5, color: "#6B7384", marginBottom: 18, lineHeight: 1.6 }}>
        你收集到的思维工具卡，按类别组织。用得越多，越成为你的本能。
      </div>
      {groups.map((g) => (
        <div key={g.key} style={{ marginBottom: 24 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
            <span style={{ width: 9, height: 9, borderRadius: 999, background: g.dot, flex: "none" }} />
            <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{g.key}</span>
            <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 600 }}>{g.cards.length} 张</span>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 12 }}>
            {g.cards.map((c) => (
              <div key={c.cardId} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 700, color: "#1C2333", lineHeight: 1.4 }}>{c.name}</div>
                  <span style={{ flex: "none", fontSize: 12, fontWeight: 700, color: "#5B6474", background: "#F1F2F6", borderRadius: 8, padding: "2px 8px" }}>×{c.uses}</span>
                </div>
                <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.55, marginTop: 6 }}>{c.purpose}</div>
                <div style={{ fontSize: 12, color: "#9AA1B0", marginTop: 10, fontWeight: 600 }}>{usageLine(c)}</div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
