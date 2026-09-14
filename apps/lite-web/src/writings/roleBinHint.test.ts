import { describe, expect, it } from "vitest";
import { ROLE_BINS, ROLE_BIN_HINT } from "./RoleBoard";

/**
 * 这块板是和阅读室共用一个组件画出来的（CoachBoard），而那个组件自带的
 * BIN_HINT 是阅读室的五个格子。2026-09-12 阅读室给格子加白话说明之后，
 * 写作这块板会顺带拿到那一份 —— 而那一份：
 *
 *   - 「主张 = **作者**要你接受的那句话」：站错了位置，这一侧作者就是她自己；
 *   - 根本没有「解释」和「让步」两格：板会半边有字半边没字。
 *
 * 所以这里钉两条**人眼盯不住**的不变量：每个格子都要有话，而且不能是
 * 站在读者那一侧说的。格子将来增减时，这两条会立刻响。
 */
describe("ROLE_BIN_HINT", () => {
  it("五个格子一个都不能少说明 —— 半边有字半边没字比都没有更糟", () => {
    for (const bin of ROLE_BINS) {
      expect(ROLE_BIN_HINT[bin], `格子「${bin}」没有白话说明`).toBeTruthy();
    }
  });

  it("不留多余的说明 —— 表里有一个不存在的格子，说明格子改过而这里没跟上", () => {
    for (const bin of Object.keys(ROLE_BIN_HINT)) {
      expect(ROLE_BINS as readonly string[], `「${bin}」不是这块板上的格子`).toContain(bin);
    }
  });

  it("不许站在读者那一侧说话 —— 这一侧作者就是她自己", () => {
    for (const [bin, hint] of Object.entries(ROLE_BIN_HINT)) {
      expect(hint, `「${bin}」的说明把她当成了读者：${hint}`).not.toContain("作者");
    }
  });
});
