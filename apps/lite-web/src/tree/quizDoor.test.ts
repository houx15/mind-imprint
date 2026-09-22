import { describe, expect, it } from "vitest";
import { shouldShowQuizDoor } from "./quizDoor";

const base = { readOnly: false, quizTaken: false as boolean | null, treeEmpty: true, resumable: false };

describe("shouldShowQuizDoor", () => {
  it("🚨 做到一半离开的学生，回来时那颗入口必须在", () => {
    // 2026-09-22 线上真的丢过：树还是空的、quizTaken 还是 false，
    // 于是页眉那颗不出现，屏幕上只剩空状态里写死「开始」的那颗 ——
    // 按下去是从头再来，她上一趟说过的话没有任何一条路通回去。
    expect(shouldShowQuizDoor({ ...base, resumable: true })).toBe(true);
  });

  it("空树、没做过、也没有没走完的 —— 不出现，空状态那条邀请已经在说了", () => {
    expect(shouldShowQuizDoor(base)).toBe(false);
  });

  it("空树但她做过一趟 —— 出现（空状态那条邀请这时不显示）", () => {
    expect(shouldShowQuizDoor({ ...base, quizTaken: true })).toBe(true);
  });

  it("树不空就一直在 —— 重做是再长几个词，不是清空重来", () => {
    expect(shouldShowQuizDoor({ ...base, treeEmpty: false })).toBe(true);
    expect(shouldShowQuizDoor({ ...base, treeEmpty: false, quizTaken: true })).toBe(true);
  });

  it("🚨 状态还没读回来就不显示 —— 闪一下「来做个测试」比不显示糟", () => {
    expect(shouldShowQuizDoor({ ...base, quizTaken: null })).toBe(false);
    expect(shouldShowQuizDoor({ ...base, quizTaken: null, treeEmpty: false })).toBe(false);
    // 🚨 连「有一趟没走完」都不能让它在状态未知时冒出来。
    expect(shouldShowQuizDoor({ ...base, quizTaken: null, resumable: true })).toBe(false);
  });

  it("🚨 老师视角整个收起 —— 这是学生自己的测试", () => {
    for (const treeEmpty of [true, false]) {
      for (const quizTaken of [true, false]) {
        for (const resumable of [true, false]) {
          expect(
            shouldShowQuizDoor({ readOnly: true, quizTaken, treeEmpty, resumable }),
            `readOnly 下不该出现（treeEmpty=${treeEmpty} taken=${quizTaken} resumable=${resumable}）`,
          ).toBe(false);
        }
      }
    }
  });
});
