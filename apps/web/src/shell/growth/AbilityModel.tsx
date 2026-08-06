import { useEffect, useState } from "react";
import type { AbilityModel as AbilityModelT } from "@mind-imprint/contracts";
import { api } from "../../api";

const RADAR_MAX = 4; // depth ordinals are 1..4 (L1..L4); insufficient (-1) → center

// N spokes evenly spaced from 12 o'clock clockwise; value 1..4 → radius fraction.
function radarPoints(levels: number[], cx: number, cy: number, r: number): string {
  const n = levels.length;
  return levels
    .map((lv, i) => {
      const frac = lv < 0 ? 0 : lv / RADAR_MAX;
      const a = ((-90 + (i * 360) / n) * Math.PI) / 180;
      return `${cx + Math.cos(a) * r * frac},${cy + Math.sin(a) * r * frac}`;
    })
    .join(" ");
}

export function AbilityModel() {
  const [model, setModel] = useState<AbilityModelT | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const m = await api.getAbilityModel();
        if (!cancelled) setModel(m);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (error) return <div style={{ padding: 24, color: "#B0432E" }}>{error}</div>;
  if (!model) return <div style={{ padding: 24, color: "var(--mk-muted)" }}>正在整理你的能力画像…</div>;
  if (model.totalSessions === 0) {
    return (
      <div style={{ background: "#fff", border: "1px solid var(--mk-border)", borderRadius: 16, padding: "34px 24px", textAlign: "center" }}>
        <div style={{ fontSize: 14.5, fontWeight: 700, color: "#3A4256" }}>还没有足够的数据</div>
        <div style={{ fontSize: 13.5, color: "#6B7384", marginTop: 8, lineHeight: 1.7 }}>完成更多任务后，你的能力画像会在这里浮现。</div>
      </div>
    );
  }

  const cx = 140, cy = 135, r = 100;
  const levels = model.depth.map((d) => d.level);

  return (
    <div>
      {/* caption (binding text, verbatim) */}
      <div style={{ background: "#fff", border: "1px solid var(--mk-border)", borderRadius: 16, padding: "22px 24px" }}>
        <div style={{ fontSize: 15, fontWeight: 700, color: "var(--mk-ink)" }}>AI 批判性思维 · 能力素养模型</div>
        <div style={{ fontSize: 12.5, color: "var(--mk-muted)", marginTop: 5, lineHeight: 1.6 }}>
          等级来自每次任务评估的归并，不是测验分数。已汇集 {model.totalSessions} 次会话。
        </div>
        {/* depth radar over the 6 depth dims */}
        <div style={{ display: "flex", justifyContent: "center", marginTop: 10 }}>
          <svg viewBox="0 0 280 270" width="100%" style={{ maxWidth: 330 }}>
            {[0.33, 0.66, 1].map((ring) => (
              <polygon key={ring} points={radarPoints(model.depth.map(() => RADAR_MAX * ring), cx, cy, r)} fill="none" stroke="#ECEEF4" strokeWidth="1" />
            ))}
            <polygon points={radarPoints(levels, cx, cy, r)} fill="rgba(42,59,122,.14)" stroke="var(--mk-accent-600)" strokeWidth="2" strokeLinejoin="round" />
            {model.depth.map((d, i) => {
              const a = ((-90 + (i * 360) / model.depth.length) * Math.PI) / 180;
              return <text key={d.code} x={cx + Math.cos(a) * (r + 16)} y={cy + Math.sin(a) * (r + 16)} textAnchor="middle" fontSize="10.5" fontWeight="600" fill="#6B7384">{d.code}</text>;
            })}
          </svg>
        </div>
      </div>

      {/* per-dim depth list */}
      <div style={{ marginTop: 12 }}>
        {model.depth.map((d) => (
          <div key={d.code} style={{ background: "#fff", border: "1px solid var(--mk-border)", borderRadius: 12, padding: "13px 16px", marginBottom: 9 }}>
            <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
              <span style={{ fontSize: 13.5, fontWeight: 700, color: "var(--mk-ink)" }}>{d.name}</span>
              {d.level < 0 ? (
                <span style={{ fontSize: 12, color: "#8A6D3B", background: "#FDF7EC", border: "1px solid #E3CFA4", borderRadius: 999, padding: "2px 9px" }}>证据不足 · 需更多任务</span>
              ) : (
                <span style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-accent-600)", background: "var(--mk-accent-50)", borderRadius: 999, padding: "2px 9px" }}>Lv{d.level} · {d.evidenceCount} 次</span>
              )}
            </div>
            {d.level >= 0 && d.levelLabel ? <div style={{ fontSize: 12.5, color: "#6B7384", marginTop: 6, lineHeight: 1.6 }}>{d.levelLabel}</div> : null}
          </div>
        ))}
      </div>

      {/* 智识自主 observation panel (no level) */}
      <div style={{ background: "#fff", border: "1px solid var(--mk-border)", borderRadius: 12, padding: "16px", marginTop: 4 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 13.5, fontWeight: 700, color: "var(--mk-ink)" }}>智识自主</span>
          <span style={{ fontSize: 11, color: "#B16A18", background: "#F5E5CE", borderRadius: 999, padding: "1px 8px" }}>观察 · 不计分</span>
        </div>
        <div style={{ fontSize: 12.5, color: "#4C5653", marginTop: 8, lineHeight: 1.7 }}>
          跨 {model.autonomy.sessions} 次会话：边界设定 ×{model.autonomy.boundarySettings} · 对手邀请 ×{model.autonomy.adversaryInvites} · 把握机会 {model.autonomy.opportunitiesTaken} / 错过机会 {model.autonomy.opportunitiesMissed}
        </div>
      </div>
    </div>
  );
}
