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
  // 🚨 details 也要带上。后台早就在往信封里放这一格（httpx.APIError.Details），
  // 只是这里一直只读 message——最能说明问题的那半截（是哪个上游、什么状态码、
  // JSON 哪里读不下去）被扔在门口。产品负责人 2026-09-02：「尽可能给出详细
  // 报错信息，方便 debug」。
  //
  // details 按约定只装不含密钥的机器细节；密钥在请求头里，不在这些字符串里。
  const more =
    err instanceof ApiError && typeof err.details === "string" && err.details.trim()
      ? `（${err.details.trim()}）`
      : "";
  return `后台错误：${detail.trim() || "没有更多信息"}${more}`;
}
