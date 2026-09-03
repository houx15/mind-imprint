/**
 * 工具面上用的颜色，全部走设计令牌。
 *
 * 🚨 产品负责人 2026-09-03：「please also follow our design tokens. because I
 * hate left color bar designs, especially when we have a huge list of that」。
 *
 * 两件事一起纠正：
 *
 * 1. **别自己编颜色。** 之前一路写的是 #3B82F6 / #10B981 / #F59E0B 这些手挑的
 *    十六进制。它们不在马卡龙色板上，而且**不跟暗色模式走**——令牌在暗色下会被
 *    整组换掉，写死的十六进制不会，于是暗色里那几块底色会亮得刺眼。
 *
 * 2. **别用左侧色条。** 一整列卡片每张挂一条竖色带，列表一长就成了一排栅栏，
 *    颜色喧宾夺主，反而看不出内容。改用马卡龙令牌本来就配好的 `-bg` 淡底
 *    + `-fg` 深字：颜色落在整块和文字上，安静，而且天然分得开。
 *
 * 每一档三个值（来自 apps/web/src/index.css，lite 通过 @import 拿到）：
 *
 *	--mk-<c>      实色，用于小圆点、进度条、描边这种要"实"的地方
 *	--mk-<c>-bg   淡底，用于整块背景
 *	--mk-<c>-fg   深字，压在淡底上读得清
 */

export type ToneName = "mist" | "taro" | "peach" | "matcha" | "berry" | "lake" | "butter";

export interface Tone {
  /** 实色。小圆点、进度弧、描边。 */
  solid: string;
  /** 淡底。整块背景。 */
  bg: string;
  /** 深字。压在淡底上。 */
  fg: string;
}

export function tone(name: ToneName): Tone {
  return {
    solid: `var(--mk-${name})`,
    bg: `var(--mk-${name}-bg)`,
    fg: `var(--mk-${name}-fg)`,
  };
}

/**
 * 语义色：对错、成败这一类，用 semantic 那组，不用马卡龙。
 * 「答过了」是一种状态，不是一个类别。
 */
export const DONE = { solid: "var(--mk-success)", bg: "var(--mk-success-bg)", fg: "var(--mk-success)" };
export const TODO = { solid: "var(--mk-warning)", bg: "var(--mk-warning-bg)", fg: "var(--mk-warning)" };

/**
 * 一串东西按次序取色（几个选项、几层结构、几段复盘）。
 *
 * 顺序固定，循环不越界——印记给几个选项由它自己定。
 */
const CYCLE: ToneName[] = ["mist", "taro", "peach", "matcha", "berry", "lake", "butter"];

export function toneAt(index: number): Tone {
  return tone(CYCLE[((index % CYCLE.length) + CYCLE.length) % CYCLE.length] as ToneName);
}

export function toneNameAt(index: number): ToneName {
  return CYCLE[((index % CYCLE.length) + CYCLE.length) % CYCLE.length] as ToneName;
}
