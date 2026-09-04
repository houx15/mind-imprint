package pbl

// jsonwire.go —— 从模型回的一段话里，把那个 JSON 对象取出来。
//
// ## 为什么不能只做「第一个 { 到最后一个 }」
//
// 那是原来的做法（persona.go / look.go 各写了一份）。它在两种很常见的回法上会
// 切出一段坏的：
//
//  1. 对象后面还跟着一段话，而那段话里也有花括号；
//  2. 对象前面那段话里有花括号——`strings.Index` 会从那儿开始切。
//
// 2026-09-04 的真浏览器走查上，第三关就是这么红的：
//
//	pbl: palettes are not JSON: invalid character 'ç' after object key:value pair
//
// 她那一侧看到的是「生成失败」，而其实模型给的三组配色本身是好的——坏的是我们
// 从它那段话里切出来的范围。
//
// 这里改成**数括号**：找到第一个 `{`，一路数深度直到配平，字符串里的括号和转义
// 都不算。取出来的一定是一个完整的对象，后面跟什么都不影响。
//
// 🚨 这不是"容错到什么都能过"。取出来之后照样严格 Unmarshal、照样逐项验
// （颜色必须 #RRGGBB、候选必须有称呼和关键词）。放宽的只有"从哪儿到哪儿是那段
// JSON"这一件事——那本来就不该由模型的排版决定。

// firstJSONObject 返回 s 里第一个配平的 {...}。找不到就原样返回 s，让上层的
// Unmarshal 去报那个更具体的错。
func firstJSONObject(s string) string {
	start := -1
	depth := 0
	inStr := false
	esc := false
	for i, r := range s {
		if esc {
			esc = false
			continue
		}
		switch {
		case inStr && r == '\\':
			esc = true
		case r == '"':
			inStr = !inStr
		case inStr:
			// 字符串里的括号不算数。
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return s[start : i+1]
				}
			}
		}
	}
	return s
}

// clip 截断一段模型输出，用来放进报错里。
//
// 报错会一路显示到她屏幕上（第 8 条：动词+失败，再接后台原话），所以要短；
// 但短到看不出模型写了什么就没有意义，400 字够判断它是写跑题了还是格式坏了。
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
