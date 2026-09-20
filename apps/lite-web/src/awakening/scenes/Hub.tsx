import { useState } from "react";

import type { AwakeningStage } from "../../api/awakening";
import { HUB } from "../content";
import { type HubCard, hubCards } from "../hub";
import { Choice, Ghost, Primary } from "../ui";

/**
 * 复访入口。她不是第一次进这个房间时，落在这一屏。
 *
 * # 为什么第一趟是一条线，第二趟是一张菜单
 *
 * 第一趟必须是线性的：剧情交代世界观，能量卡牌给她一个开口，助手在探询之前
 * 选好。一个第一次进来的人不该先面对四个按钮。
 *
 * 第二趟这条线就是错的。她已经看过剧情、已经选过助手，回来是为了再做一次
 * 兴趣探索。而 2026-09-20 学生反馈的那条更直接：**做到一半走掉的人回来只能
 * 接着那一屏往下走，到不了能量测试**。菜单解决的是这件事。
 *
 * # 保留下来的线索占两张卡，不是一张
 *
 * 她上次按了「暂时保留兴趣线索」，这一次可能想接着问，也可能想换个话题从头
 * 问。这是两件事：一件接着上次的语料走，一件把那些语料清空。合成一张卡，
 * 总有一半的人按到的是另一件。
 *
 * # 🚨 这一屏不进 run.stage
 *
 * 它是客户端的一层。把它存成一个 stage，就会把她真正的进度（比如 lens）
 * 盖掉，第二天回来那一趟就从菜单重新开始、前面答的八轮找不回来。
 * 所以 AwakeningRoom 用一个本地 `hub` 状态盖在 stage 上面，
 * 从这里点出去才 `go(...)`。
 */

export function HubScene({
  /** 第一次回来（上一趟走完了）还是接着上次没走完的那一趟。 */
  returning,
  /** 已经答完几轮。0 表示这一趟的探询还没开始。 */
  turnsDone,
  /** 「继续」那一下真正会去的那一屏。第一张卡上的字照它写。 */
  resume,
  navigator,
  hasEnergy,
  /** 清空失败时后台那句原话。空表示没失败过。 */
  freshError,
  onContinue,
  onFresh,
  onEnergy,
  onStory,
  onNavigator,
}: {
  returning: boolean;
  turnsDone: number;
  resume: AwakeningStage;
  navigator: string;
  hasEnergy: boolean;
  freshError: string;
  onContinue: () => void;
  onFresh: () => void;
  onEnergy: () => void;
  onStory: () => void;
  onNavigator: () => void;
}) {
  // 「新的探索」删的是她自己写下的字，所以先问一次。
  const [confirmFresh, setConfirmFresh] = useState(false);

  if (confirmFresh) {
    return (
      <section className="awk-screen" aria-label="确认新的探索">
        <div className="awk-wrap awk-hub">
          <div>
            <div className="awk-eyebrow">{HUB.eyebrow}</div>
            <h2 className="awk-h2">{HUB.fresh}</h2>
          </div>
          <div>
            <p className="awk-p">{HUB.freshAsk.replace("{n}", String(turnsDone))}</p>
            {freshError ? (
              <p className="awk-p" style={{ color: "var(--danger)", marginTop: 12 }}>
                {HUB.freshFailed}：{freshError}
              </p>
            ) : null}
            <div className="mt-7 flex flex-wrap items-center gap-4">
              <Primary onClick={onFresh}>{HUB.freshConfirm}</Primary>
              <Ghost onClick={() => setConfirmFresh(false)}>{HUB.cancel}</Ghost>
            </div>
          </div>
        </div>
      </section>
    );
  }

  // 摆哪几张卡、每张卡上写什么，是一个有测试的纯函数（hub.ts）——
  // 一张说错的卡不会报错，它只是让她按下去之后到了别的地方。
  const cards = hubCards({ turnsDone, resume, navigator, hasEnergy });
  const action: Record<HubCard["key"], () => void> = {
    continue: onContinue,
    fresh: () => setConfirmFresh(true),
    energy: onEnergy,
    navigator: onNavigator,
    story: onStory,
  };

  return (
    <section className="awk-screen" aria-label="兴趣测试入口">
      <div className="awk-wrap awk-hub">
        <div>
          <div className="awk-eyebrow">{HUB.eyebrow}</div>
          <h2 className="awk-h2">{HUB.title}</h2>
          <p className="awk-p" style={{ marginTop: 10 }}>
            {returning ? HUB.leadReturning : HUB.leadResume}
          </p>
        </div>

        <div className="awk-choice-stack" aria-label="这一次做什么">
          {cards.map((c, i) => {
            const n = String(i + 1).padStart(2, "0");
            return (
              <Choice
                key={c.key}
                index={n}
                hwId={`SYS-${n}`}
                title={c.title}
                body={c.body}
                onClick={action[c.key]}
              />
            );
          })}
        </div>
      </div>
    </section>
  );
}
