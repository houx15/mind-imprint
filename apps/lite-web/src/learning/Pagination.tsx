/**
 * Pagination —— 两座库共用的一条页码。
 *
 * 写作题库 705 道、分级阅读库 48 篇，都长到了「一屏滚不完」的地步。
 *
 * # 页码条为什么不是「加载更多」
 *
 * 她在库里找题，常常要回头：「刚才第三页那道关于攀登的」。无限滚动把位置这件
 * 事从她手里拿走了 —— 回头只能再滚一遍。页码是个地址，能记住、能回去。
 *
 * # 🚨 页数多的时候不要把一百个数字全排出来
 *
 * 705 道题分 30 页还排得下，但筛一下就变 2 页、搜一下又变 18 页，
 * 一条会变长变短的控件会把下面的内容顶来顶去。所以固定形状：
 * 头、尾、当前页左右各一，中间用省略号顶住。
 */

export function Pagination({
  page,
  pages,
  onPick,
}: {
  page: number;
  pages: number;
  onPick: (page: number) => void;
}) {
  if (pages <= 1) return null;
  const nums = pageNumbers(page, pages);
  return (
    <nav
      aria-label="分页"
      className="mt-6 flex flex-wrap items-center justify-center gap-1.5"
    >
      <Step label="上一页" disabled={page <= 1} onPick={() => onPick(page - 1)} />
      {nums.map((n, i) =>
        n === 0 ? (
          <span key={`gap${i}`} className="px-1 text-mk-label text-mk-muted">
            …
          </span>
        ) : (
          <button
            key={n}
            type="button"
            aria-current={n === page ? "page" : undefined}
            onClick={() => onPick(n)}
            className="min-w-8 rounded-mk-full border px-2.5 py-1 text-mk-label transition-colors duration-[120ms] ease-mk"
            style={
              n === page
                ? {
                    borderColor: "var(--mk-accent-500)",
                    background: "var(--mk-accent-500)",
                    color: "white",
                  }
                : { borderColor: "var(--mk-border)", color: "var(--mk-secondary)" }
            }
          >
            {n}
          </button>
        ),
      )}
      <Step
        label="下一页"
        disabled={page >= pages}
        onPick={() => onPick(page + 1)}
      />
    </nav>
  );
}

function Step({
  label,
  disabled,
  onPick,
}: {
  label: string;
  disabled: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onPick}
      className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-label text-mk-secondary transition-colors duration-[120ms] ease-mk disabled:cursor-not-allowed disabled:opacity-40"
    >
      {label}
    </button>
  );
}

/**
 * 要显示哪几个页码。0 是省略号。
 *
 * 导出是为了能直接测它：这种「首尾各留一个、当前页左右各留一个」的算法，
 * 边界（第 1 页、最后一页、只有三页）是读代码看不出对错的那一类。
 */
export function pageNumbers(page: number, pages: number): number[] {
  if (pages <= 7) return Array.from({ length: pages }, (_, i) => i + 1);
  const want = new Set([1, pages, page, page - 1, page + 1]);
  const out: number[] = [];
  let prev = 0;
  for (let n = 1; n <= pages; n++) {
    if (!want.has(n)) continue;
    if (prev && n - prev > 1) out.push(0);
    out.push(n);
    prev = n;
  }
  return out;
}
