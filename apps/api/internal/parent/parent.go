// Package parent holds pure parent-facing projections of the canonical
// assessment object. The parent surface is gentled and numberless: D levels
// become a four-step ladder, A levels a three-state signal read. Never a
// composite score (RL-5); recomputed on every read, never persisted.
package parent

// DBadge maps a canonical depth level (L1–L4, or NA/"") to the parent 四台阶
// label. Unknown/NA/empty → 暂无 (敢于空白): no real level, no reading.
func DBadge(level string) string {
	switch level {
	case "L1":
		return "起步"
	case "L2":
		return "发展"
	case "L3":
		return "熟练"
	case "L4":
		return "优秀"
	default:
		return "暂无"
	}
}

// AState maps an autonomy level (0..5) to the numberless three-state parent
// read. NO number ever reaches the parent surface (design §2「A 轴不出现任何
// 数字」). not_supplied signals arrive here as level 0 → 暂未观察到.
func AState(level int) string {
	switch {
	case level <= 1:
		return "暂未观察到"
	case level <= 3:
		return "偶有·多在引导后"
	default:
		return "观察到主动信号"
	}
}
