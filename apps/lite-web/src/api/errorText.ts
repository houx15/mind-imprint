import { ApiError } from "./client";

/**
 * 出错时给她看的那一句。永远带上后台真正说了什么。
 *
 * 🚨 产品负责人 2026-09-02：「please treat all api error this way, be honest ok?」
 *
 * 「再试一次」会把两件完全不同的事说成同一件：一件是服务端拒绝了她的输入
 * （她看得懂、能改），另一件是我们自己的代码崩了（她再试一百次也没用）。被
 * 那句话盖掉的，恰好是唯一能查出问题的那部分——而她一个人对着屏幕，没有别的
 * 地方可以看见它。
 *
 * 和 [[ai-errors-must-surface-never-fake]] 是同一条规矩：宁可难看，不要编。
 */
export function apiErrorText(err: unknown): string {
  const detail =
    err instanceof ApiError || err instanceof Error
      ? err.message
      : typeof err === "string"
        ? err
        : "";
  return `后台错误：${detail.trim() || "没有更多信息"}`;
}
