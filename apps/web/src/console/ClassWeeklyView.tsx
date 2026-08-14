import { useEffect, useRef, useState } from "react";
import type { ApiClient, WeeklyCard, WeeklyReport } from "../api";
import { ApiError } from "../api";
import { Card, ChevronLeft, ChevronRight, EmptyState, Icon, IconButton, Loader2 } from "@/ui";

const DELTA_STYLE: Record<"up" | "down" | "flat", { fg: string; bg: string }> = {
  up: { fg: "var(--mk-success)", bg: "var(--mk-success-bg)" },
  down: { fg: "var(--mk-danger)", bg: "var(--mk-danger-bg)" },
  flat: { fg: "var(--mk-muted)", bg: "var(--mk-border)" },
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

// Fixed per-kind palette for the whole focus card (avatar, tag chip, top
// border, "怎么鼓励/怎么开口" action row) — praise is always matcha, watch is
// always warning amber, regardless of the server's per-student avatarColor.
// The design has no equivalent fixed per-tag color scheme for per-student
// variation, so `card.avatarColor` is intentionally unused here (also avoids
// the alpha-suffix-on-hex pattern the token migration forbids).
const KIND_STYLE: Record<WeeklyCard["kind"], { fg: string; bg: string }> = {
  praise: { fg: "var(--mk-matcha-fg)", bg: "var(--mk-matcha-bg)" },
  watch: { fg: "var(--mk-warning)", bg: "var(--mk-warning-bg)" },
};

const KIND_BORDER_CLASS: Record<WeeklyCard["kind"], string> = {
  praise: "border-t-mk-matcha",
  watch: "border-t-mk-warning",
};

function DeltaPill({ delta, dir }: { delta: string; dir: "up" | "down" | "flat" }) {
  const s = DELTA_STYLE[dir];
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 3, background: s.bg, color: s.fg, fontSize: 11, fontWeight: 800, padding: "3px 8px", borderRadius: "var(--mk-radius-full)" }}>
      {dir !== "flat" && (
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke={s.fg} strokeWidth={3} strokeLinecap="round" strokeLinejoin="round">
          <path d={DELTA_ARROW[dir]} />
        </svg>
      )}
      {delta}
    </span>
  );
}

function FocusCard({ card, onOpenStudent, onOpenReport }: {
  card: WeeklyCard;
  onOpenStudent: (userId: string) => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string, userId: string) => void;
}) {
  const style = KIND_STYLE[card.kind];
  const actionPrefix = card.kind === "praise" ? "怎么鼓励：" : "怎么开口：";
  return (
    <Card className={`border border-mk-border border-t-[3px] ${KIND_BORDER_CLASS[card.kind]} p-5`}>
      <div style={{ display: "flex", alignItems: "center", gap: 13 }}>
        <div style={{ width: 46, height: 46, borderRadius: "50%", background: style.bg, color: style.fg, display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800, fontSize: 19, flex: "none" }}>
          {card.displayName.slice(0, 1)}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div onClick={() => onOpenStudent(card.userId)} style={{ fontSize: 21, fontWeight: 800, color: "var(--mk-ink)", lineHeight: 1.05, cursor: "pointer" }}>
            {card.displayName}
          </div>
          <span style={{ marginTop: 7, display: "inline-flex", alignItems: "center", gap: 5, background: style.bg, color: style.fg, fontSize: 12, fontWeight: 700, padding: "3px 11px", borderRadius: "var(--mk-radius-full)" }}>
            {card.kind === "praise" ? (
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke={style.fg} strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round"><path d="M4 17 10 11l4 4 6-6" /><path d="M15 5h5v5" /></svg>
            ) : (
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke={style.fg} strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 8v4M12 16h.01" /></svg>
            )}
            {card.tagLabel}
          </span>
        </div>
      </div>
      {card.lead !== "" && (
        <div style={{ marginTop: 16, fontSize: 15, fontWeight: 700, color: "var(--mk-secondary)", lineHeight: 1.5 }}>{card.lead}</div>
      )}
      <div style={{ marginTop: 7, fontSize: 12.5, color: "var(--mk-muted)", lineHeight: 1.65 }}>{card.evidence}</div>
      {card.action !== "" && (
        <div style={{ marginTop: 14, display: "flex", alignItems: "flex-start", gap: 8, background: style.bg, borderRadius: "var(--mk-radius-md)", padding: "12px 13px" }}>
          {card.kind === "praise" ? (
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke={style.fg} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }}>
              <path d="m12 3 2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z" />
            </svg>
          ) : (
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke={style.fg} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }}>
              <path d="M8 10h8M8 14h5" /><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
          )}
          <span style={{ fontSize: 12.5, lineHeight: 1.6, color: "var(--mk-secondary)" }}>
            <b style={{ color: style.fg }}>{actionPrefix}</b>{card.action}
          </span>
        </div>
      )}
      {card.hasReport && (
        <div
          onClick={() => onOpenReport(card.reportSurface!, card.reportScopeId!, card.displayName, card.userId)}
          style={{ marginTop: 14, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 700, color: "var(--mk-accent-600)", cursor: "pointer" }}
        >
          看能力报告
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--mk-accent-600)" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
        </div>
      )}
    </Card>
  );
}

/** `data.weekStart` shifted by `days` (±7 for prev/next week), ISO string. */
function shiftWeekStart(iso: string, days: number): string {
  return new Date(new Date(iso).getTime() + days * 86400 * 1000).toISOString();
}

export function ClassWeeklyView({ client, classId, onOpenStudent, onOpenReport }: {
  client: Pick<ApiClient, "getClassWeeklyReport" | "generateClassWeeklyProse">;
  classId: string;
  onOpenStudent: (userId: string) => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string, userId: string) => void;
}) {
  const [data, setData] = useState<WeeklyReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  // undefined = server picks the last completed week; otherwise an ISO week-
  // start string set by the ◄/► nav below.
  const [weekStart, setWeekStart] = useState<string | undefined>(undefined);
  // Generation is a SPEND. This ref is what stops a re-render from firing a
  // second flagship call; the server is idempotent per (class, week), but the
  // client must not lean on that. Keyed per-week so navigating to a different
  // week generates its own prose at most once.
  const asked = useRef<string | null>(null);

  function load() {
    setError(null);
    client.getClassWeeklyReport(classId, weekStart).then((r) => {
      setData(r);
      const key = `${classId}:${weekStart ?? "latest"}`;
      if (!r.proseReady && asked.current !== key) {
        asked.current = key;
        client.generateClassWeeklyProse(classId, weekStart).then(setData).catch(() => {
          /* 敢于空白: keep the numbers and the evidence; prose stays absent. */
        });
      }
    }).catch((e) => {
      setData(null);
      setError(e instanceof ApiError ? e.message : "加载失败");
    });
  }
  useEffect(load, [client, classId, weekStart]);

  if (error) {
    return (
      <div style={{ maxWidth: 1080, margin: "32px auto 0", padding: "0 30px" }}>
        <EmptyState illustration="completed" title="周报加载失败" body={error} action={{ label: "重试", onClick: load }} />
      </div>
    );
  }
  if (!data) {
    return (
      <div style={{ marginTop: 30, display: "flex", alignItems: "center", justifyContent: "center", gap: 8, color: "var(--mk-muted)", fontSize: 14.5 }}>
        <Icon icon={Loader2} size={18} className="animate-spin" />
        加载中…
      </div>
    );
  }

  const noAttention = data.praise.length === 0 && data.watch.length === 0;

  return (
    <div style={{ maxWidth: 1080, margin: "0 auto", padding: "32px 30px 64px" }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16 }}>
        <div>
          <div style={{ fontSize: 26, fontWeight: 800, color: "var(--mk-ink)", letterSpacing: ".01em" }}>班级周报 · {data.weekLabel}</div>
          <div style={{ fontSize: 13, color: "var(--mk-muted)", marginTop: 6, lineHeight: 1.6 }}>
            作业布置与提交在学校自己的平台完成。这里只看学生在思维印记上的使用强度，和他们思考维度的变化。
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 6, flex: "none" }}>
          <IconButton icon={ChevronLeft} label="上一周" variant="secondary" size="sm" onClick={() => setWeekStart(shiftWeekStart(data.weekStart, -7))} />
          <IconButton icon={ChevronRight} label="下一周" variant="secondary" size="sm" disabled={data.isLatestWeek} onClick={() => setWeekStart(shiftWeekStart(data.weekStart, 7))} />
        </div>
      </div>

      {/* Bean 点评 */}
      <Card className="mt-6 flex items-start gap-4 p-6">
        <div style={{ width: 38, height: 38, flex: "none", borderRadius: "50%", background: "var(--mk-accent-50)", color: "var(--mk-accent-600)", display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800 }}>阅</div>
        <div>
          <div style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-accent-600)", letterSpacing: ".04em" }}>本周班级点评</div>
          <div style={{ marginTop: 8, fontSize: 15, lineHeight: 1.75, color: "var(--mk-secondary)", fontWeight: 500 }}>
            {data.comment ?? "本周点评暂未生成"}
          </div>
        </div>
      </Card>

      {/* 使用概况 */}
      <div style={{ marginTop: 22, display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 16 }}>
        {data.stats.map((s) => (
          <Card key={s.key} className="p-5">
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
                <div style={{ width: 32, height: 32, borderRadius: "var(--mk-radius-sm)", background: "var(--mk-paper)", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="var(--mk-muted)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
                    <path d={STAT_ICON[s.key] ?? ""} />
                  </svg>
                </div>
                <div style={{ fontSize: 12.5, color: "var(--mk-muted)", fontWeight: 600 }}>{s.label}</div>
              </div>
              <DeltaPill delta={s.delta} dir={s.deltaDir} />
            </div>
            <div style={{ marginTop: 14, display: "flex", alignItems: "baseline", gap: 6 }}>
              <span style={{ fontSize: 30, fontWeight: 800, color: "var(--mk-ink)", letterSpacing: "-.01em" }}>{s.value}</span>
              <span style={{ fontSize: 13, color: "var(--mk-muted)", fontWeight: 600 }}>{s.unit}</span>
            </div>
            <div style={{ marginTop: 7, fontSize: 11.5, color: "var(--mk-faint)", lineHeight: 1.5 }}>{s.foot}</div>
          </Card>
        ))}
      </div>

      {/* 建议关注 */}
      <div style={{ marginTop: 32 }}>
        <div style={{ fontSize: 20, fontWeight: 800, color: "var(--mk-ink)" }}>建议关注</div>
        <div style={{ fontSize: 12.5, color: "var(--mk-muted)", marginTop: 4 }}>先看值得表扬的，再看需要给建议的 · 每张卡都带证据，具体沟通在线下进行</div>
      </div>

      {noAttention ? (
        <div style={{ marginTop: 20, color: "var(--mk-muted)", fontSize: 14.5 }}>本周没有需要特别关注的学生</div>
      ) : (
        <>
          {data.praise.length > 0 && (
            <>
              <div style={{ marginTop: 20, display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--mk-matcha)" }} />
                <div style={{ fontSize: 14, fontWeight: 800, color: "var(--mk-matcha-fg)" }}>值得表扬</div>
                <span style={{ fontSize: 12, color: "var(--mk-faint)" }}>{data.praise.length}</span>
              </div>
              <div style={{ marginTop: 12, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                {data.praise.map((c) => (
                  <FocusCard key={c.userId} card={c} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />
                ))}
              </div>
            </>
          )}

          {data.watch.length > 0 && (
            <>
              <div style={{ marginTop: 26, display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--mk-warning)" }} />
                <div style={{ fontSize: 14, fontWeight: 800, color: "var(--mk-warning)" }}>需要建议</div>
                <span style={{ fontSize: 12, color: "var(--mk-faint)" }}>{data.watch.length}</span>
              </div>
              <div style={{ marginTop: 12, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                {data.watch.map((c) => (
                  <FocusCard key={c.userId} card={c} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />
                ))}
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
