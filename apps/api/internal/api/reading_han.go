package api

// isHanRune 判断一个字符是不是汉字。
//
// 两段范围：CJK 统一汉字（4e00–9fff）和扩展 A（3400–4dbf）。扩展 A 在文言文里
// 是真的会出现的 —— 生僻字、异体字都落在那一段。
//
// 抽出来是因为它现在有两个调用点，而两个调用点的判据必须是同一条：
// readingLangOf 用它判这一篇是中文还是英文，lookupTokens 用它决定一个词
// 按整词切还是按字切。两处对「什么是汉字」的看法一旦分叉，就会出现
// 「这一篇算中文、但她点的那个字切不出 token」这种查不出来的组合。
func isHanRune(r rune) bool {
	return (r >= 0x4e00 && r <= 0x9fff) || (r >= 0x3400 && r <= 0x4dbf)
}
