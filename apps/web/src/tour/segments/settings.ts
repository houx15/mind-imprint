import type { TourSegment } from "../types";

export const settingsSegment: TourSegment = {
  id: "settings-accent",
  name: "个性化",
  steps: [
    {
      id: "settings-accent-0",
      onEnter: (nav) => nav.setTab("me"),
      placement: "center",
      title: "最后一件小事",
      text: "这些都逛完啦！在「我」这里，你可以把整个界面换成你喜欢的主题色。",
      advance: "next",
    },
    {
      id: "settings-accent-1",
      anchor: '[data-testid="accent-swatch"]',
      placement: "top",
      text: "挑一个你喜欢的颜色，界面会立刻跟着变。祝你在思维印记玩得开心 —— 有需要随时叫我。",
      advance: "next",
    },
  ],
};
