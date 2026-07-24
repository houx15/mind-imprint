import { useEffect, useRef, useState } from "react";
import type { ApiClient, WeeklyCard, WeeklyReport } from "../api";
import { ApiError } from "../api";
import { badgeColor } from "./badgeColor";

const DELTA_STYLE: Record<"up" | "down" | "flat", { fg: string; bg: string }> = {
  up: { fg: "#3E8A6E", bg: "#E9F2EC" },
  down: { fg: "#C4574D", bg: "#F7E6E4" },
  flat: { fg: "#8A92A3", bg: "#EEF0F4" },
};

// Fixed per-key icon paths matching the binding design (dc.html:1556-1560) —
// the icon is a rendering choice keyed by the stat's `key`, not server data.
const STAT_ICON: Record<string, string> = {
  active_students: "M17 20v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2M10 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7",
  reports: "M14 3v5h5M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8zM9 15l2 2 4-4",
  turns: "M21 15a2 2 0 0 1-2 2H8l-4 4V5a2 2 0 0 1 2-2h13a2 2 0 0 1 2 2z",
  course_steps: "M4 5.5A2.5 2.5 0 0 1 6.5 3H20v15H6.5A2.5 2.5 0 0 0 4 20.5zM20 18v3",
};

const DELTA_ARROW: Record<"up" | "down", string> = {
  up: "M12 19V5M6 11l6-6 6 6",
  down: "M12 5v14M18 13l-6 6-6-6",
};

// Fixed per-kind accent for the action row (dc.html:141-144 vs 172-175):
// the "怎么鼓励" box is always green, "怎么开口" always navy — never per-card.
const KIND_ACTION_STYLE: Record<WeeklyCard["kind"], { bg: string; stroke: string }> = {
  praise: { bg: "#F3F9F6", stroke: "#3E8A6E" },
  watch: { bg: "#F7F8FB", stroke: "#2A3B7A" },
};

// Fixed green for praise cards' avatar / tag chip / top border (dc.html:129,
// 131, 134-135 — the 值得表扬 block). The design hardcodes this regardless of
// student; only watch cards use the server's per-student avatarColor, since
// the design has no equivalent fixed per-tag color for those.
const PRAISE_GREEN = { fg: "#3E8A6E", bg: "#E4F0EA" };

function DeltaPill({ delta, dir }: { delta: string; dir: "up" | "down" | "flat" }) {
  const s = DELTA_STYLE[dir];
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 3, background: s.bg, color: s.fg, fontSize: 11, fontWeight: 800, padding: "3px 8px", borderRadius: 20 }}>
      {dir !== "flat" && (
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke={s.fg} strokeWidth={3} strokeLinecap="round" strokeLinejoin="round">
          <path d={DELTA_ARROW[dir]} />
        </svg>
      )}
      {delta}
    </span>
  );
}

function Card({ card, onOpenStudent, onOpenReport }: {
  card: WeeklyCard;
  onOpenStudent: (userId: string) => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string, userId: string) => void;
}) {
  const action = KIND_ACTION_STYLE[card.kind];
  const actionPrefix = card.kind === "praise" ? "怎么鼓励：" : "怎么开口：";
  // avatarColor is the server-picked accent for this card; the light tint
  // behind it is a pure CSS rendering choice (alpha suffix on the given hex),
  // never a re-derivation of the badge/depth-level color chain. Praise cards
  // are the exception: the design hardcodes them to a fixed green regardless
  // of student (see PRAISE_GREEN), so the head color is per-kind there, not
  // per-student.
  const isPraise = card.kind === "praise";
  const headFg = isPraise ? PRAISE_GREEN.fg : card.avatarColor;
  const headBg = isPraise ? PRAISE_GREEN.bg : `${card.avatarColor}1A`;
  return (
    <div style={{ background: "#fff", border: "1px solid #EAECF2", borderTop: `3px solid ${headFg}`, borderRadius: 16, padding: "20px 20px 18px", boxShadow: "0 4px 18px rgba(20,30,60,.05)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 13 }}>
        <div style={{ width: 46, height: 46, borderRadius: "50%", background: headBg, color: headFg, display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800, fontSize: 19, flex: "none" }}>
          {card.displayName.slice(0, 1)}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div onClick={() => onOpenStudent(card.userId)} style={{ fontSize: 21, fontWeight: 800, color: "#1C2333", lineHeight: 1.05, cursor: "pointer" }}>
            {card.displayName}
          </div>
          <span style={{ marginTop: 7, display: "inline-flex", alignItems: "center", gap: 5, background: headBg, color: headFg, fontSize: 12, fontWeight: 700, padding: "3px 11px", borderRadius: 20 }}>
            {card.kind === "praise" ? (
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke={headFg} strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round"><path d="M4 17 10 11l4 4 6-6" /><path d="M15 5h5v5" /></svg>
            ) : (
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke={headFg} strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 8v4M12 16h.01" /></svg>
            )}
            {card.tagLabel}
          </span>
        </div>
      </div>
      {card.lead !== "" && (
        <div style={{ marginTop: 16, fontSize: 15, fontWeight: 700, color: "#2A3040", lineHeight: 1.5 }}>{card.lead}</div>
      )}
      <div style={{ marginTop: 7, fontSize: 12.5, color: "#7A8296", lineHeight: 1.65 }}>{card.evidence}</div>
      {card.action !== "" && (
        <div style={{ marginTop: 14, display: "flex", alignItems: "flex-start", gap: 8, background: action.bg, borderRadius: 11, padding: "12px 13px" }}>
          {card.kind === "praise" ? (
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke={action.stroke} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }}>
              <path d="m12 3 2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z" />
            </svg>
          ) : (
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke={action.stroke} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }}>
              <path d="M8 10h8M8 14h5" /><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
          )}
          <span style={{ fontSize: 12.5, lineHeight: 1.6, color: "#3A4A44" }}>
            <b style={{ color: action.stroke }}>{actionPrefix}</b>{card.action}
          </span>
        </div>
      )}
      {card.hasReport && (
        <div
          onClick={() => onOpenReport(card.reportSurface!, card.reportScopeId!, card.displayName, card.userId)}
          style={{ marginTop: 14, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 700, color: "#2A3B7A", cursor: "pointer" }}
        >
          看能力报告
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
        </div>
      )}
    </div>
  );
}

export function ClassWeeklyView({ client, classId, onOpenStudent, onOpenReport }: {
  client: Pick<ApiClient, "getClassWeeklyReport" | "generateClassWeeklyProse">;
  classId: string;
  onOpenStudent: (userId: string) => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string, userId: string) => void;
}) {
  const [data, setData] = useState<WeeklyReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Generation is a SPEND. This ref is what stops a re-render from firing a
  // second flagship call; the server is idempotent per (class, week), but the
  // client must not lean on that.
  const asked = useRef<string | null>(null);

  function load() {
    setError(null);
    client.getClassWeeklyReport(classId).then((r) => {
      setData(r);
      if (!r.proseReady && asked.current !== classId) {
        asked.current = classId;
        client.generateClassWeeklyProse(classId).then(setData).catch(() => {
          /* 敢于空白: keep the numbers and the evidence; prose stays absent. */
        });
      }
    }).catch((e) => {
      setData(null);
      setError(e instanceof ApiError ? e.message : "加载失败");
    });
  }
  useEffect(load, [client, classId]);

  if (error) {
    return (
      <div style={{ marginTop: 30, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>
        周报加载失败：{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span>
      </div>
    );
  }
  if (!data) return <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5 }}>加载中…</div>;

  const noAttention = data.praise.length === 0 && data.watch.length === 0;
  const dTotal = data.depth.ratedCount;

  return (
    <div style={{ maxWidth: 1080, margin: "0 auto", padding: "32px 30px 64px" }}>
      <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: ".01em" }}>班级周报 · {data.weekLabel}</div>
      <div style={{ fontSize: 13, color: "#8A92A3", marginTop: 6, lineHeight: 1.6 }}>
        作业布置与提交在学校自己的平台完成。这里只看学生在思维印记上的使用强度，和他们思考维度的变化。
      </div>

      {/* Bean 点评 */}
      <div style={{ marginTop: 22, background: "#fff", border: "1px solid #EAECF2", borderRadius: 18, padding: "22px 24px", boxShadow: "0 4px 20px rgba(20,30,60,.04)", display: "flex", gap: 15, alignItems: "flex-start" }}>
        <div style={{ width: 38, height: 38, flex: "none", borderRadius: "50%", background: "#EDEFF9", color: "#2A3B7A", display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800 }}>阅</div>
        <div>
          <div style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", letterSpacing: ".04em" }}>本周班级点评</div>
          <div style={{ marginTop: 8, fontSize: 15, lineHeight: 1.75, color: "#2A3040", fontWeight: 500 }}>
            {data.comment ?? "本周点评暂未生成"}
          </div>
        </div>
      </div>

      {/* 使用概况 */}
      <div style={{ marginTop: 22, display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 16 }}>
        {data.stats.map((s) => (
          <div key={s.key} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 18px 16px", boxShadow: "0 4px 16px rgba(20,30,60,.04)" }}>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
                <div style={{ width: 32, height: 32, borderRadius: 9, background: "#F1F2F8", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#6C7488" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
                    <path d={STAT_ICON[s.key] ?? ""} />
                  </svg>
                </div>
                <div style={{ fontSize: 12.5, color: "#6C7488", fontWeight: 600 }}>{s.label}</div>
              </div>
              <DeltaPill delta={s.delta} dir={s.deltaDir} />
            </div>
            <div style={{ marginTop: 14, display: "flex", alignItems: "baseline", gap: 6 }}>
              <span style={{ fontSize: 30, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>{s.value}</span>
              <span style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>{s.unit}</span>
            </div>
            <div style={{ marginTop: 7, fontSize: 11.5, color: "#A2A9B8", lineHeight: 1.5 }}>{s.foot}</div>
          </div>
        ))}
      </div>

      {/* 建议关注 */}
      <div style={{ marginTop: 32 }}>
        <div style={{ fontSize: 20, fontWeight: 800, color: "#1C2333" }}>建议关注</div>
        <div style={{ fontSize: 12.5, color: "#8A92A3", marginTop: 4 }}>先看值得表扬的，再看需要给建议的 · 每张卡都带证据，具体沟通在线下进行</div>
      </div>

      {noAttention ? (
        <div style={{ marginTop: 20, color: "#8A92A3", fontSize: 14.5 }}>本周没有需要特别关注的学生</div>
      ) : (
        <>
          {data.praise.length > 0 && (
            <>
              <div style={{ marginTop: 20, display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ width: 8, height: 8, borderRadius: "50%", background: "#3E8A6E" }} />
                <div style={{ fontSize: 14, fontWeight: 800, color: "#3E8A6E" }}>值得表扬</div>
                <span style={{ fontSize: 12, color: "#9198A8" }}>{data.praise.length}</span>
              </div>
              <div style={{ marginTop: 12, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                {data.praise.map((c) => (
                  <Card key={c.userId} card={c} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />
                ))}
              </div>
            </>
          )}

          {data.watch.length > 0 && (
            <>
              <div style={{ marginTop: 26, display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ width: 8, height: 8, borderRadius: "50%", background: "#C68A3A" }} />
                <div style={{ fontSize: 14, fontWeight: 800, color: "#B0863A" }}>需要建议</div>
                <span style={{ fontSize: 12, color: "#9198A8" }}>{data.watch.length}</span>
              </div>
              <div style={{ marginTop: 12, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                {data.watch.map((c) => (
                  <Card key={c.userId} card={c} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />
                ))}
              </div>
            </>
          )}
        </>
      )}

      {/* 班级思维维度 */}
      <div style={{ marginTop: 34, background: "#fff", border: "1px solid #EAECF2", borderRadius: 18, padding: "24px 24px 26px", boxShadow: "0 4px 20px rgba(20,30,60,.04)" }}>
        <div style={{ fontSize: 20, fontWeight: 800, color: "#1C2333" }}>班级思维维度</div>
        <div style={{ fontSize: 12.5, color: "#8A92A3", marginTop: 4 }}>按能力评估体系的两根轴看：D 轴＝想得有多深，A 轴＝有多愿意自己想</div>

        {/* D 轴分布 */}
        <div style={{ marginTop: 22, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 14, fontWeight: 700, color: "#2A3040" }}>D 轴 · 认知深度分布</div>
          <div style={{ fontSize: 12, color: "#8A92A3" }}>已评估 {data.depth.ratedCount} 人</div>
        </div>
        {dTotal === 0 ? (
          <div style={{ marginTop: 12, color: "#8A92A3", fontSize: 13.5 }}>暂无可计入的证据</div>
        ) : (
          <>
            <div style={{ marginTop: 10, display: "flex", height: 16, borderRadius: 8, overflow: "hidden", background: "#F1F2F6" }}>
              {data.depth.buckets.map((b) => (
                <div key={b.code} style={{ width: `${(b.count / dTotal) * 100}%`, background: badgeColor(b.code).fg }} />
              ))}
            </div>
            <div style={{ marginTop: 12, display: "flex", flexWrap: "wrap", gap: 16 }}>
              {data.depth.buckets.map((b) => (
                <div key={b.code} style={{ display: "flex", alignItems: "center", gap: 7 }}>
                  <span style={{ width: 11, height: 11, borderRadius: 3, background: badgeColor(b.code).fg }} />
                  <span style={{ fontSize: 12.5, color: "#4A5060", fontWeight: 600 }}>{b.label}</span>
                  <span style={{ fontSize: 12.5, color: "#8A92A3" }}>{b.count} 人</span>
                </div>
              ))}
            </div>
          </>
        )}
        {data.depth.note !== "" && (
          <div style={{ marginTop: 12, fontSize: 12.5, color: "#7A8296", lineHeight: 1.65 }}>{data.depth.note}</div>
        )}

        {/* A 轴均分 */}
        <div style={{ marginTop: 22, paddingTop: 20, borderTop: "1px solid #EEF0F4", display: "grid", gridTemplateColumns: "auto 1fr", gap: 22, alignItems: "center" }}>
          <div style={{ textAlign: "center" }}>
            <div style={{ fontSize: 12.5, color: "#6C7488", fontWeight: 600 }}>A 轴 · 智识自主均分</div>
            <div style={{ marginTop: 6, display: "flex", alignItems: "baseline", gap: 6, justifyContent: "center" }}>
              <span style={{ fontSize: 40, fontWeight: 800, color: "#2A3B7A", lineHeight: 1 }}>{data.autonomy.mean}</span>
              <span style={{ fontSize: 15, color: "#9198A8", fontWeight: 700 }}>/ 5</span>
            </div>
            <div style={{ marginTop: 8 }}>
              <DeltaPill delta={data.autonomy.delta} dir={data.autonomy.deltaDir} />
            </div>
          </div>
          <div>
            {/* No progress bar here: the design's aBarW is a precomputed
                width the server does not send on WeeklyReport.autonomy, and
                the only arithmetic this component is allowed is the D
                stacked bar's count/ratedCount — so this column carries only
                the note, never a derived fill. */}
            {data.autonomy.note !== "" && (
              <div style={{ fontSize: 12.5, color: "#7A8296", lineHeight: 1.65 }}>{data.autonomy.note}</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
