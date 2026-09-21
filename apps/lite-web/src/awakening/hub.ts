import type { AwakeningStage } from "../api/awakening";
import { GUIDES, HUB } from "./content";
import { STAGE_BADGE } from "./RoomShell";

/**
 * 入口那一屏摆哪几张卡、每张卡上写什么。
 *
 * # 为什么这是一个纯函数
 *
 * 2026-09-20 第一版入口上线时，三张卡同时在说假话：「开始兴趣探索」按下去
 * 到的是能量卡牌那一屏；「重新选择助手」而她一个都还没选过；能量那一屏的
 * 按钮写着「选择你的印记」而它回的是入口。**一张说错的卡不会报错**，它只是
 * 让她按下去之后到了别的地方 —— 是那次靠人眼看截图才发现的。
 *
 * 所以「这一屏此刻该说什么」被摘出来单独测（hub.test.ts）。组件只负责把
 * 结果画出来、把 key 接到动作上。
 */

/** 一张卡。`key` 决定它接哪一个动作，编号由它在表里的位置现算。 */
export interface HubCard {
  key: "continue" | "library" | "energy" | "navigator" | "story";
  title: string;
  body: string;
}

export function hubCards({
  turnsDone,
  threadCount,
  resume,
  navigator,
  hasEnergy,
}: {
  /** 这一趟已经答完几轮。0 表示没有保留下来的回答。 */
  turnsDone: number;
  /** 线索库里有几条。0 表示她还没提出过线索，那张卡不摆。 */
  threadCount: number;
  /** 「继续」那一下真正会去的那一屏。 */
  resume: AwakeningStage;
  navigator: string;
  hasEnergy: boolean;
}): HubCard[] {
  const guide = GUIDES.find((g) => g.id === navigator);
  const started = turnsDone > 0;
  // 「继续」去的是探询，还是她停下的另一屏？两种说法不一样，而说错就是骗她。
  const toTerminal = resume === "terminal";

  const cards: HubCard[] = [
    {
      key: "continue",
      title: !toTerminal ? HUB.continueRun : started ? HUB.held : HUB.restart,
      body: !toTerminal
        ? HUB.continueAt.replace("{stage}", STAGE_BADGE[resume].route)
        : started
          ? HUB.progress.replace("{n}", String(turnsDone))
          : HUB.resumeBody,
    },
  ];

  // 线索库。她提出过至少一条才摆 —— 一个空的库是一扇通向空屋子的门。
  //
  // 🚨 这张卡 2026-09-21 之前是「新的探索」，而它做的事是**清空**她上次写的
  // 回答。现在每一条都留着，所以它指向库，而不是一个删除动作。
  if (threadCount > 0) {
    cards.push({
      key: "library",
      title: HUB.library,
      body: HUB.libraryBody.replace("{n}", String(threadCount)),
    });
  }

  cards.push(
    {
      key: "energy",
      title: HUB.energy,
      body: hasEnergy ? `${HUB.energyDone}${HUB.energyBody}` : HUB.energyBody,
    },
    {
      key: "navigator",
      title: guide ? HUB.navigator : HUB.navigatorFirst,
      body: guide
        ? `${HUB.navigatorNow.replace("{name}", guide.zh)}。${HUB.navigatorBody}`
        : HUB.navigatorFirstBody,
    },
    { key: "story", title: HUB.story, body: HUB.storyBody },
  );
  return cards;
}
