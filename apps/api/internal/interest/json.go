package interest

import (
	"errors"
	"fmt"
	"strings"
)

// sliceJSONObject 从模型的回话里切出那个 JSON 对象。
//
// 模型很爱在 JSON 前后加一句「好的，这是结果：」或者用 ```json 围起来。与其在
// 每个 prompt 里再劝一次「不要任何解释」（劝告不可验），不如在解析这一侧把这
// 件事变成不重要的：找第一个 `{` 和最后一个 `}`。
//
// 找不到就**报错**，绝不返回一个空对象假装模型什么都没选 —— 那会让「模型答非
// 所问」和「模型认真看了但一个都没选中」变成同一件事，而这两件事该被区别对待。
//
// 原来住在 router.go 里；路由 2026-09-04 退休，这个函数被采集与深挖两处共用，
// 所以单独放一个文件，而不是塞进其中一个调用方。
func sliceJSONObject(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("empty model reply")
	}
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object in model reply (%d bytes)", len(s))
	}
	return []byte(s[start : end+1]), nil
}
