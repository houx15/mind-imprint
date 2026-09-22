import type { AwakeningStage } from "../../api/awakening";
import { HUB } from "../content";
import { type HubCard, hubCards } from "../hub";
import { Choice } from "../ui";

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
 * # 换一条线索是一扇门，不是一个删除动作
 *
 * 2026-09-21 之前这里有一张「新的探索」，按下去清空她上次写的回答 —— 因为
 * 那时库里一个人只能有一趟没走完的。现在每一条都留着，那张卡于是指向
 * **线索库**（scenes/Library.tsx）。
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
  /** 线索库里有几条。0 表示她还没提出过线索，那张卡不摆。 */
  threadCount,
  /** 「继续」那一下真正会去的那一屏。第一张卡上的字照它写。 */
  resume,
  navigator,
  hasEnergy,
  onContinue,
  onLibrary,
  onEnergy,
  onStory,
  onNavigator,
}: {
  returning: boolean;
  turnsDone: number;
  threadCount: number;
  resume: AwakeningStage;
  navigator: string;
  hasEnergy: boolean;
  onContinue: () => void;
  onLibrary: () => void;
  onEnergy: () => void;
  onStory: () => void;
  onNavigator: () => void;
}) {
  // 摆哪几张卡、每张卡上写什么，是一个有测试的纯函数（hub.ts）——
  // 一张说错的卡不会报错，它只是让她按下去之后到了别的地方。
  const cards = hubCards({ turnsDone, threadCount, resume, navigator, hasEnergy });
  const action: Record<HubCard["key"], () => void> = {
    continue: onContinue,
    library: onLibrary,
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
          {cards.filter(c => !["navigator", "story"].includes(c.key)).map((c, i) => (
            <div key={c.key} className={c.key === "continue" ? "awk-hub-primary" : "awk-hub-secondary"}>
              {c.key === "continue" && <p className="awk-hub-label">继续当前探索</p>}
              <Choice index={String(i + 1).padStart(2, "0")} title={c.title} body={c.body} onClick={action[c.key]} />
            </div>
          ))}
          <details className="awk-hub-more">
            <summary>助手与剧情</summary>
            {cards.filter(c => ["navigator", "story"].includes(c.key)).map(c => (
              <Choice key={c.key} title={c.title} body={c.body} onClick={action[c.key]} />
            ))}
          </details>
        </div>
      </div>
    </section>
  );
}
