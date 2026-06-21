import { useState, useSyncExternalStore } from "react";
import type { Store } from "../../store/createStore";
import type { CardSpec } from "@mind-imprint/contracts";
import { FULL_RUBRIC } from "@mind-imprint/contracts";
import { deriveActivityCalendar } from "./activityCalendar";
import { deriveGrowthReviews } from "./growthReviews";
import { deriveCardUsage } from "./cardUsage";
import { deriveAbility } from "./ability";
import { radarGeometry } from "./radarGeometry";

type Tab = "learning" | "cards" | "ability";

const LEVEL_NUMBER: Record<string, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };

const ACTIVE_PILL: React.CSSProperties = {
  padding: "6px 14px",
  borderRadius: "8px",
  fontSize: "13px",
  fontWeight: 600,
  color: "#1C2333",
  background: "#fff",
  boxShadow: "0 1px 3px rgba(20,30,60,.10)",
  border: "none",
  cursor: "pointer",
};
const INACTIVE_PILL: React.CSSProperties = {
  padding: "6px 14px",
  borderRadius: "8px",
  fontSize: "13px",
  fontWeight: 600,
  color: "#6B7384",
  background: "transparent",
  border: "none",
  cursor: "pointer",
};

export function RecordsView({
  store,
  registry,
  now = () => new Date(),
}: {
  store: Store;
  registry: Record<string, CardSpec>;
  now?: () => Date;
}) {
  const [tab, setTab] = useState<Tab>("learning");
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot);

  const { messages, cards, evaluations } = snapshot;

  // Calendar events: all items with created_at
  const events = [...messages, ...cards, ...evaluations];
  const calendar = deriveActivityCalendar(events, now());
  const reviews = deriveGrowthReviews(evaluations);
  const cardGroups = deriveCardUsage(cards, registry);
  const ability = deriveAbility(evaluations, FULL_RUBRIC);
  const levels = ability.map((a) => LEVEL_NUMBER[a.level] ?? 1);
  const names = ability.map((a) => a.dim);
  const radar = radarGeometry(levels, names);

  const pillStyle = (t: Tab): React.CSSProperties =>
    tab === t ? ACTIVE_PILL : INACTIVE_PILL;

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: "920px", margin: "0 auto", padding: "40px 40px 60px" }}>
        <div style={{ fontSize: "26px", fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>
          记录
        </div>
        <div style={{ fontSize: "13.5px", color: "#8A92A3", marginTop: "5px" }}>
          你走过的思考，安静地留下印记。
        </div>

        {/* Tab pills — lines 631–635 */}
        <div
          style={{
            display: "inline-flex",
            gap: "4px",
            background: "#EBEDF2",
            borderRadius: "11px",
            padding: "4px",
            margin: "22px 0 24px",
          }}
        >
          <button onClick={() => setTab("learning")} style={pillStyle("learning")}>
            学习记录
          </button>
          <button onClick={() => setTab("cards")} style={pillStyle("cards")}>
            工具卡
          </button>
          <button onClick={() => setTab("ability")} style={pillStyle("ability")}>
            能力素养
          </button>
        </div>

        {/* Learning tab — lines 638–672 */}
        {tab === "learning" && (
          <>
            <div
              style={{
                background: "#fff",
                border: "1px solid #EAECF2",
                borderRadius: "16px",
                padding: "22px 24px",
                boxShadow: "0 1px 3px rgba(20,30,60,.04)",
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  marginBottom: "18px",
                }}
              >
                <div style={{ fontSize: "15px", fontWeight: 700, color: "#1C2333" }}>
                  活跃日历
                </div>
                <div style={{ fontSize: "12.5px", color: "#8A92A3", fontWeight: 600 }}>
                  过去 {calendar.weeks} 周 · 共 {calendar.activeDays} 天有思考
                </div>
              </div>
              {/* Calendar grid — lines 644–648 */}
              <div
                style={{
                  display: "grid",
                  gridTemplateRows: "repeat(7, 13px)",
                  gridAutoFlow: "column",
                  gridAutoColumns: "13px",
                  gap: "4px",
                }}
              >
                {calendar.cells.map((cell, i) => (
                  <div key={i} style={parseStyle(cell.style)} />
                ))}
              </div>
              {/* Legend — lines 649–657 */}
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "7px",
                  marginTop: "16px",
                  fontSize: "11.5px",
                  color: "#9AA1B0",
                }}
              >
                少
                <span style={{ width: "12px", height: "12px", borderRadius: "3px", background: "#EDEFF4" }} />
                <span style={{ width: "12px", height: "12px", borderRadius: "3px", background: "#C9D0E8" }} />
                <span style={{ width: "12px", height: "12px", borderRadius: "3px", background: "#97A3D2" }} />
                <span style={{ width: "12px", height: "12px", borderRadius: "3px", background: "#5C6CB0" }} />
                <span style={{ width: "12px", height: "12px", borderRadius: "3px", background: "#2A3B7A" }} />
                多
              </div>
            </div>

            {/* Growth reviews — lines 660–671 */}
            <div style={{ fontSize: "15px", fontWeight: 700, color: "#1C2333", margin: "28px 0 14px" }}>
              成长回顾
            </div>
            {reviews.length === 0 ? (
              <div style={{ fontSize: "13.5px", color: "#8A92A3", lineHeight: 1.7 }}>
                还没有评估记录。完成一次任务并生成思维印记后，这里会留下你的成长回顾。
              </div>
            ) : (
              reviews.map((r, i) => (
                <div
                  key={i}
                  style={{
                    background: "#fff",
                    border: "1px solid #EAECF2",
                    borderRadius: "14px",
                    padding: "18px 20px",
                    marginBottom: "12px",
                    display: "flex",
                    gap: "14px",
                    alignItems: "flex-start",
                  }}
                >
                  <div
                    style={{
                      flex: "none",
                      width: "38px",
                      height: "38px",
                      borderRadius: "10px",
                      background: "#FBF1DC",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                    }}
                  >
                    <svg
                      width="19"
                      height="19"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="#D9A23D"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M13 2L3 14h7l-1 8 10-12h-7z" />
                    </svg>
                  </div>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: "12px", color: "#9AA1B0", fontWeight: 600, marginBottom: "4px" }}>
                      {r.period}
                    </div>
                    <div style={{ fontSize: "14.5px", lineHeight: 1.7, color: "#2B3346" }}>
                      {r.text}
                    </div>
                  </div>
                </div>
              ))
            )}
          </>
        )}

        {/* Cards tab — lines 675–697 */}
        {tab === "cards" && (
          <>
            {/* Documented copy deviation: 课程库分支, NOT 五大分支 */}
            <div style={{ fontSize: "13.5px", color: "#6B7384", marginBottom: "18px", lineHeight: 1.6 }}>
              你收集到的思维工具卡，按课程库分支组织。用得越多，越成为你的本能。
            </div>
            {cardGroups.length === 0 ? (
              <div style={{ fontSize: "13.5px", color: "#8A92A3" }}>
                你还没有用过工具卡。
              </div>
            ) : (
              cardGroups.map((g, gi) => (
                <div key={gi} style={{ marginBottom: "24px" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "12px" }}>
                    <span style={parseStyle(g.dotStyle)} />
                    <span style={{ fontSize: "14px", fontWeight: 700, color: "#1C2333" }}>{g.name}</span>
                    <span style={{ fontSize: "12px", color: "#9AA1B0", fontWeight: 600 }}>{g.countLabel}</span>
                  </div>
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: "12px" }}>
                    {g.cards.map((c, ci) => (
                      <div key={ci} style={parseStyle(c.boxStyle)}>
                        <div
                          style={{
                            display: "flex",
                            alignItems: "flex-start",
                            justifyContent: "space-between",
                            gap: "10px",
                          }}
                        >
                          <div style={{ fontSize: "14.5px", fontWeight: 700, color: "#1C2333", lineHeight: 1.4 }}>
                            {c.name}
                          </div>
                          <span style={parseStyle(c.badgeStyle)}>{c.badge}</span>
                        </div>
                        <div style={{ fontSize: "12.5px", color: "#8A92A3", lineHeight: 1.55, marginTop: "6px" }}>
                          {c.purpose}
                        </div>
                        <div style={{ fontSize: "12px", color: "#9AA1B0", marginTop: "10px", fontWeight: 600 }}>
                          {c.usage}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ))
            )}
          </>
        )}

        {/* Ability tab — lines 701–742 */}
        {tab === "ability" && (
          <div style={{ display: "flex", gap: "20px", flexWrap: "wrap" }}>
            {/* Radar chart panel */}
            <div
              style={{
                flex: 1,
                minWidth: "300px",
                background: "#fff",
                border: "1px solid #EAECF2",
                borderRadius: "16px",
                padding: "22px 24px",
                boxShadow: "0 1px 3px rgba(20,30,60,.04)",
              }}
            >
              <div style={{ fontSize: "15px", fontWeight: 700, color: "#1C2333" }}>
                AI 批判性思维 · 能力素养模型
              </div>
              <div style={{ fontSize: "12.5px", color: "#8A92A3", marginTop: "5px", lineHeight: 1.6 }}>
                九个维度，跨多个课程库分支。等级来自每次任务评估的归并，不是测验分数。
              </div>
              <div style={{ display: "flex", justifyContent: "center", marginTop: "10px" }}>
                <svg viewBox="0 0 280 270" width="100%" style={{ maxWidth: "330px" }}>
                  {radar.rings.map((ring, i) => (
                    <polygon key={i} points={ring.points} fill="none" stroke="#ECEEF4" strokeWidth="1" />
                  ))}
                  {radar.axes.map((ax, i) => (
                    <line
                      key={i}
                      x1={ax.x1}
                      y1={ax.y1}
                      x2={ax.x2}
                      y2={ax.y2}
                      stroke="#ECEEF4"
                      strokeWidth="1"
                    />
                  ))}
                  <polygon
                    points={radar.polygonPoints}
                    fill="rgba(42,59,122,.14)"
                    stroke="#2A3B7A"
                    strokeWidth="2"
                    strokeLinejoin="round"
                  />
                  {radar.dots.map((dt, i) => (
                    <circle key={i} cx={dt.cx} cy={dt.cy} r="3.2" fill="#2A3B7A" />
                  ))}
                  {radar.labels.map((lb, i) => (
                    <text
                      key={i}
                      x={lb.x}
                      y={lb.y}
                      textAnchor={lb.anchor as "start" | "middle" | "end"}
                      fontSize="10.5"
                      fontWeight="600"
                      fill="#6B7384"
                    >
                      {lb.text}
                    </text>
                  ))}
                </svg>
              </div>
            </div>
            {/* Ability list */}
            <div style={{ flex: 1, minWidth: "280px" }}>
              {ability.map((a, i) => (
                <div
                  key={i}
                  style={{
                    background: "#fff",
                    border: "1px solid #EAECF2",
                    borderRadius: "12px",
                    padding: "13px 16px",
                    marginBottom: "9px",
                  }}
                >
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: "10px",
                    }}
                  >
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                      <span style={parseStyle(a.dotStyle)} />
                      <span style={{ fontSize: "13.5px", fontWeight: 700, color: "#1C2333" }}>{a.dim}</span>
                    </div>
                    <span
                      style={{
                        fontSize: "12px",
                        fontWeight: 700,
                        color: "#2A3B7A",
                        background: "#EDEFF9",
                        padding: "2px 9px",
                        borderRadius: "999px",
                      }}
                    >
                      {a.levelLabel}
                    </span>
                  </div>
                  <div style={{ display: "flex", gap: "4px", marginTop: "9px" }}>
                    {a.segs.map((seg, si) => (
                      <span key={si} style={parseStyle(seg.style)} />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Convert a CSS string (e.g. "width:13px; height:13px; ...") into a React style object.
 * This is used for styles coming from the derivation modules that emit plain CSS strings.
 */
function parseStyle(css: string): React.CSSProperties {
  const style: Record<string, string> = {};
  for (const decl of css.split(";")) {
    const idx = decl.indexOf(":");
    if (idx === -1) continue;
    const prop = decl.slice(0, idx).trim();
    const val = decl.slice(idx + 1).trim();
    if (!prop || !val) continue;
    // Convert kebab-case to camelCase
    const camel = prop.replace(/-([a-z])/g, (_, c: string) => c.toUpperCase());
    style[camel] = val;
  }
  return style as React.CSSProperties;
}
