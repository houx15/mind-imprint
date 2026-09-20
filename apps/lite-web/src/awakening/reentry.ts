import type { AwakeningRun, AwakeningStage } from "../api/awakening";

/**
 * 进门时那个判断：她是第一次进来，还是回来的人。
 *
 * 单独拿出来是因为**读代码看不出对错**，而两个方向都错得很难看：
 *
 *   判成「回来的人」而她其实是第一次 → 她第一眼看见四个按钮，剧情没了
 *   判成「第一次」而她其实做到一半   → 她被丢回开场，前面答的轮数找不回来
 *
 * 这两件事在界面上都不会报错，只会安静地发生。所以它是一个纯函数，
 * 有测试（reentry.test.ts）。
 */
export interface Reentry {
  /** 落在入口那一屏（菜单），而不是 run.stage 那一屏。 */
  hub: boolean;
  /** 从入口点「继续」要去的那一屏。 */
  resume: AwakeningStage;
}

/** 重做时「继续」的落点：她已经看过剧情、也已经有助手，回来就是为了再探询一次。 */
const REDO_ENTRY: AwakeningStage = "terminal";

export function planReentry(
  run: Pick<AwakeningRun, "attemptNo" | "stage" | "finishedAt">,
): Reentry {
  const resume = run.stage === "boot" ? REDO_ENTRY : run.stage;
  // 已经走完的那一趟不进这条路：房间直接显示它的报告。
  if (run.finishedAt) return { hub: false, resume };
  // attemptNo > 1 = 上一趟走完了，这是重做。
  // stage ≠ boot   = 上一趟没走完，她是回来接着走的。
  return { hub: run.attemptNo > 1 || run.stage !== "boot", resume };
}
