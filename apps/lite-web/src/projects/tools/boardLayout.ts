/**
 * 便签在板上的尺寸与落位。
 *
 * 单独一份，因为**两条路**都会往板上贴便签：她在头脑风暴板里手写一条
 * （Board.add），以及她从「观察日记」带回来一批（Observe.finish）。两边各写
 * 一份网格公式，迟早会错开——而错开的样子就是便签互相压住。
 */

export const NOTE_W = 148;
export const NOTE_H = 88;

/** 第 n 张便签落在哪：沿网格铺开，之后她自己挪。 */
export function boardSpot(n: number): { x: number; y: number } {
  const perRow = 2;
  return {
    x: 12 + (n % perRow) * (NOTE_W + 14),
    y: 12 + Math.floor(n / perRow) * (NOTE_H + 14),
  };
}
