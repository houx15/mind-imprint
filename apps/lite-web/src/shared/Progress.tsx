/**
 * Progress —— 工具面共用的那个进度盘。
 *
 * 和阅读室那个 `ReadingPlanDial` 是同一套视觉语言（轨 + 弧 + 绕圈的高光 +
 * 呼吸的光晕），样式在 `src/index.css` 的 `.mk-progress` 一节。
 *
 * 两种用法：
 *
 *	<Progress done={2} total={4} />   已知进度：弧就是百分比，值变了补间过去
 *	<Progress />                      不知道要多久：只剩绕圈的高光
 *
 * 🚨 后一种是给「印记正在读整个项目」那一分钟用的。原来那儿只有一行不动的字，
 * 而一行不动的字和「卡死了」在屏幕上长得一模一样——我自己走查时就差点把它判成
 * 坏了（2026-09-03）。
 */
export function Progress({
  done,
  total,
  /** 直径，像素。默认 44。 */
  size = 44,
  label,
}: {
  /** 做完了几件。不给 = 不知道要多久，只转不填。 */
  done?: number;
  total?: number;
  size?: number;
  /** 无障碍读出来的那句话。 */
  label?: string;
}) {
  const known = typeof done === "number" && typeof total === "number" && total > 0;
  const pct = known ? Math.min(Math.max(done / total, 0), 1) : 0;
  const full = known && done >= total;

  // 半径按盘子大小算：ring 内缩 3px，描边 3px，所以中心线再收 1.5px。
  const r = size / 2 - 3 - 1.5;
  const c = 2 * Math.PI * r;

  return (
    <div
      className={`mk-progress${full ? " is-done" : ""}`}
      style={{ ["--mk-progress-size" as string]: `${size}px` }}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={known ? total : undefined}
      aria-valuenow={known ? done : undefined}
      aria-label={label ?? (known ? `已完成 ${done} / ${total}` : "处理中")}
    >
      {/* 全部做完就安静下来——没有还在动的东西可指了。 */}
      {!full && <span className="mk-progress__sweep" aria-hidden />}
      {!full && !known && <span className="mk-progress__halo" aria-hidden />}

      {known && (
        <svg className="mk-progress__ring" viewBox={`0 0 ${size} ${size}`} aria-hidden>
          <circle className="mk-progress__ring-track" cx={size / 2} cy={size / 2} r={r} />
          <circle
            className="mk-progress__ring-fill"
            cx={size / 2}
            cy={size / 2}
            r={r}
            strokeDasharray={c}
            strokeDashoffset={c * (1 - pct)}
          />
        </svg>
      )}

      {known && (
        <span className="mk-progress__count">
          <b>{done}</b>
          <i>/{total}</i>
        </span>
      )}
    </div>
  );
}
