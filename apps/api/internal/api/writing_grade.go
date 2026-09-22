package api

// writing_grade.go —— 这一篇是几年级的学生写的。
//
// 路径：writing.atom_id → atom.user_id → enrollments → classes.grade。
// writing 表自己没有 owner 列，归属挂在 atom 上（0092_atom_substrate.sql）。
//
// # 🚨 一个学生可能在不止一个班里
//
// `enrollments` 只在 (user_id, class_id) 上唯一，`ListClassesForUser` 是
// `:many`，`GET /me` 那边早就按列表渲染。今天没有哪条路会给一个已有账号再加
// 一个学生 enrollment（只有注册时按 join code 加一次），但那是「还没做」，
// 不是数据库拦着。
//
// 所以这里的规矩是：**两个班的年级对不上就当不知道，绝不挑一个。**
// 挑错的代价是她整篇拿到另一个年级的教学内容，而屏幕上一点异常都没有；
// 退回「不知道」只是少一条线索 —— 她拿到的是今天那份不分年级的内容。
func gradeFromClasses(grades []string) string {
	found := ""
	for _, g := range grades {
		if g == "" {
			continue
		}
		if found == "" {
			found = g
			continue
		}
		if found != g {
			return "" // 对不上 —— 不猜。
		}
	}
	return found
}
