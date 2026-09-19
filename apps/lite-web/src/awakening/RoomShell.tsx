import type { ReactNode } from "react";

import type { AwakeningStage } from "../api/awakening";

/**
 * 房间的外壳：舞台、扫描线、顶栏。
 *
 * 参考设计里这条顶栏在每一屏都在，它做三件事：说这是哪套系统（思维印记 ·
 * AWAKENING NETWORK）、说她现在在哪一节（路线 + 关卡号）、给一个声音开关。
 * 第一版实现把它换成了一个小圆角标签加「离开」，房间于是失去了「舱内」的感觉。
 *
 * 🚨 这里的路线和关卡号**不是装饰，是真数据**：第几次接入取自这一趟的
 * `attemptNo`（服务端算的），不是写死的「首次接入」。参考设计没有账号、
 * 没有历史，所以它只能写死；我们有，就该显示真的。
 */

/** 每一屏在顶栏上报出的身份。关卡号沿用设计稿的编法（E/W/A/D/R/O/N）。 */
const STAGE_BADGE: Record<AwakeningStage, { route: string; level: string }> = {
  boot: { route: "觉醒协议", level: "E-00" },
  world: { route: "觉醒协议", level: "E-00" },
  warning: { route: "认知提醒", level: "W-00" },
  archive: { route: "历史档案 · 认知让步", level: "A-01" },
  deck: { route: "觉醒训练 · 三张底牌", level: "D-00" },
  rejoin: { route: "重新决定", level: "R-00" },
  observer: { route: "观察者路线", level: "O-00" },
  energy: { route: "能量线索", level: "N-05" },
  navigator: { route: "选择印记", level: "N-05" },
  terminal: { route: "兴趣信号诊断", level: "N-01" },
  lens: { route: "三层追问", level: "N-06" },
  challenge: { route: "下一步", level: "N-06" },
  talent: { route: "兴趣 × 天赋", level: "N-06" },
  report: { route: "兴趣印记", level: "N-07" },
};

export function RoomShell({
  stage,
  attemptNo,
  muted,
  onToggleVoice,
  onLeave,
  children,
}: {
  stage: AwakeningStage;
  /** 第几趟。1 显示「首次接入」，之后显示「第 N 次接入」。 */
  attemptNo: number;
  muted: boolean;
  onToggleVoice: () => void;
  onLeave: () => void;
  children: ReactNode;
}) {
  const badge = STAGE_BADGE[stage] ?? STAGE_BADGE.boot;
  const entry = attemptNo > 1 ? `第 ${attemptNo} 次接入` : "首次接入";

  return (
    <div className="awk-shell">
      <div className="awk-noise" aria-hidden="true" />

      <header className="awk-topbar">
        <div className="awk-brand">
          <div className="awk-brand-mark" aria-hidden="true" />
          <div className="awk-brand-copy">
            思维印记
            <small>AWAKENING NETWORK</small>
          </div>
        </div>

        <div className="awk-status-group">
          {/* 这一条是气氛，不是状态 —— 设计稿里它恒亮。 */}
          <div className="awk-chip">
            <span className="awk-status-dot" aria-hidden="true" />
            神经连接稳定
          </div>
          <div className="awk-chip">
            <span>
              {entry} · {badge.route}
            </span>
            <span className="awk-level">{badge.level}</span>
          </div>
          <button
            type="button"
            className="awk-topbar-button"
            onClick={onToggleVoice}
            aria-pressed={muted ? "false" : "true"}
          >
            {muted ? "声音已关闭" : "声音已开启"}
          </button>
          <button type="button" className="awk-topbar-button" onClick={onLeave}>
            离开
          </button>
        </div>
      </header>

      {children}
    </div>
  );
}
