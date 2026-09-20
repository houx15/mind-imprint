import { GUIDES, HUB } from "../content";
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
  navigator,
  hasEnergy,
  onContinue,
  onEnergy,
  onStory,
  onNavigator,
}: {
  returning: boolean;
  turnsDone: number;
  navigator: string;
  hasEnergy: boolean;
  onContinue: () => void;
  onEnergy: () => void;
  onStory: () => void;
  onNavigator: () => void;
}) {
  const guide = GUIDES.find((g) => g.id === navigator);
  const started = turnsDone > 0;

  return (
    <section className="awk-screen" aria-label="兴趣测试入口">
      <div className="awk-wrap awk-two-col">
        <div>
          <div className="awk-eyebrow">{HUB.eyebrow}</div>
          <h2 className="awk-h2">{HUB.title}</h2>
          <p className="awk-p" style={{ marginTop: 10 }}>
            {returning ? HUB.leadReturning : HUB.leadResume}
          </p>
        </div>

        <div className="awk-choice-stack" aria-label="这一次做什么">
          <Choice
            index="01"
            hwId="SYS-01"
            title={started ? HUB.resume : HUB.restart}
            body={started ? HUB.progress.replace("{n}", String(turnsDone)) : HUB.resumeBody}
            onClick={onContinue}
          />
          <Choice
            index="02"
            hwId="SYS-02"
            title={HUB.energy}
            body={hasEnergy ? `${HUB.energyDone} · ${HUB.energyBody}` : HUB.energyBody}
            onClick={onEnergy}
          />
          <Choice
            index="03"
            hwId="SYS-03"
            title={HUB.navigator}
            body={`${
              guide ? HUB.navigatorNow.replace("{name}", guide.zh) : HUB.navigatorNone
            } · ${HUB.navigatorBody}`}
            onClick={onNavigator}
          />
          <Choice
            index="04"
            hwId="SYS-04"
            title={HUB.story}
            body={HUB.storyBody}
            onClick={onStory}
          />
        </div>
      </div>
    </section>
  );
}
