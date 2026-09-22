package guidance

// Pick 返回服务这一次的、最具体的那一行。
//
// 一样具体时**先登记的赢**：注册表的顺序是人排的，排在前面就是更该用的
// 那一条。所以这里用 > 而不是 >=。
func Pick[T any](k Key, rows []Row[T]) (T, bool) {
	var best T
	bestScore := -1
	for _, r := range rows {
		if !r.Scope.Matches(k) {
			continue
		}
		if s := r.Scope.specificity(k); s > bestScore {
			best, bestScore = r.Value, s
		}
	}
	return best, bestScore >= 0
}
