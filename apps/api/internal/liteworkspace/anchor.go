package liteworkspace

import (
	"fmt"
	"strings"
)

// anchor.go: set_material's pasted-text source. The model does not copy the
// passage into the tool call. It names the passage by two anchors, the first
// and last roughly 15 characters, and the server cuts the passage out of the
// teacher's own message.
//
// Two measured reasons for this contract (2026-09-17 live run, scenario 2):
//   - deepseek-v4-pro rewrote “” as " when it copied a passage into a JSON
//     string argument, in 6 of 6 calls. A literal substring check then
//     rejected every attempt, and each retry was byte-identical.
//   - Copying a 3,000-rune article back out costs thousands of output tokens
//     per attempt, against a 16k output cap.
//
// What is stored is always the teacher's own runes, so the stored text is
// still a literal substring of what she typed this turn.

// MinAnchorRunes is the shortest anchor accepted. A shorter anchor matches
// too many places in a real article to pin one passage.
const MinAnchorRunes = 8

// foldQuote maps typographic double and single quotes to their ASCII form.
// One rune in, one rune out, so an index into the folded text is the same
// index into the original. Corner brackets 「」『』 are left as they are: they
// are a different punctuation mark, not a different spelling of the same one.
func foldQuote(r rune) rune {
	switch r {
	case '“', '”', '„', '‟', '〝', '〞', '＂':
		return '"'
	case '‘', '’', '‚', '‛', '＇':
		return '\''
	}
	return r
}

func foldQuotes(s string) []rune {
	rs := []rune(s)
	for i, r := range rs {
		rs[i] = foldQuote(r)
	}
	return rs
}

// indexRunes returns the first index i >= from where needle occurs in
// haystack, or -1.
func indexRunes(haystack, needle []rune, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+len(needle) <= len(haystack); i++ {
		match := true
		for j, r := range needle {
			if haystack[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// AnchoredSpan cuts the passage the two anchors name out of typed and returns
// it in typed's own runes.
//
// Matching, on quote-folded text and in rune indices:
//  1. start is the first occurrence of startAnchor in typed.
//  2. end is the first occurrence of endAnchor that starts at or after start
//     AND ends at or after the end of the start anchor. The second condition
//     keeps an end anchor that lies wholly inside the start anchor from
//     cutting the passage shorter than its own first anchor. So the two
//     anchors may overlap only when the passage is shorter than both put
//     together.
//  3. The passage is typed[start : end+len(endAnchor)], both anchors included.
//
// A phrase repeated in the article resolves to its first occurrence. If that
// is the wrong one, the teacher sees the passage on the card and can correct
// it.
//
// Anchors are trimmed of surrounding whitespace before matching. The length
// floor and cap on the passage are the caller's, since they are the
// assignment's limits, not the matching rule's.
//
// The error is written for the model to act on. It says which anchor failed
// and how, so the next attempt can differ from this one.
func AnchoredSpan(typed, startAnchor, endAnchor string) (string, error) {
	startAnchor = strings.TrimSpace(startAnchor)
	endAnchor = strings.TrimSpace(endAnchor)
	if startAnchor == "" || endAnchor == "" {
		return "", fmt.Errorf("请同时给出 startAnchor 和 endAnchor：分别从老师这一轮消息里原样复制那段正文开头和结尾的约 15 个字")
	}
	if n := len([]rune(startAnchor)); n < MinAnchorRunes {
		return "", fmt.Errorf("startAnchor 太短（%d 字）：至少 %d 字，请从那段正文开头原样复制约 15 个字", n, MinAnchorRunes)
	}
	if n := len([]rune(endAnchor)); n < MinAnchorRunes {
		return "", fmt.Errorf("endAnchor 太短（%d 字）：至少 %d 字，请从那段正文结尾原样复制约 15 个字", n, MinAnchorRunes)
	}

	orig := []rune(typed)
	hay := foldQuotes(typed)
	sa := foldQuotes(startAnchor)
	ea := foldQuotes(endAnchor)

	start := indexRunes(hay, sa, 0)
	if start < 0 {
		return "", fmt.Errorf("startAnchor 在老师这一轮的消息里找不到：%s。请从她贴的正文开头原样复制一段，不要改字、不要补标点；也可以换成开头处更短的一段（不少于 %d 字）", startAnchor, MinAnchorRunes)
	}
	startEnd := start + len(sa)

	end := -1
	for from := start; ; {
		i := indexRunes(hay, ea, from)
		if i < 0 {
			break
		}
		if i+len(ea) >= startEnd {
			end = i
			break
		}
		from = i + 1
	}
	if end < 0 {
		if indexRunes(hay, ea, 0) >= 0 {
			return "", fmt.Errorf("endAnchor 只出现在 startAnchor 之前（或整个落在 startAnchor 里面）：%s。endAnchor 应取那段正文的结尾，startAnchor 取开头，请检查是否写反了", endAnchor)
		}
		return "", fmt.Errorf("endAnchor 在老师这一轮的消息里找不到：%s。请从她贴的正文结尾原样复制一段，不要改字、不要补标点；也可以换成结尾处更短的一段（不少于 %d 字）", endAnchor, MinAnchorRunes)
	}
	return string(orig[start : end+len(ea)]), nil
}
