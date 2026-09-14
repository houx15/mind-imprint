package docextract

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// 一个入口，四种格式。
//
// 产品负责人 2026-09-12：「reading part, cannot load pdf format. we need this,
// also in writing part we would allow pdf or docx or txt formats to upload.
// so make this a general tool.」
//
// 🚨 「一件通用的工具」这句话的落点就是这个函数：认格式、挑解析器、给出**能读懂
// 的失败原因**，三件事只写一遍。阅读和写作各自去认扩展名的话，两边会慢慢长出
// 两套接受范围 —— 阅读今天收 pdf/docx，写作今天一个都不收，正是这么来的。

// Supported 是我们收的扩展名，界面上的 accept 列表照着它写。
var Supported = []string{".pdf", ".docx", ".txt", ".md"}

// SupportedAccept 给 HTML input 的 accept 属性用。
func SupportedAccept() string { return strings.Join(Supported, ",") }

// IsSupported 认不认这个文件名的扩展名。
func IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, s := range Supported {
		if ext == s {
			return true
		}
	}
	return false
}

// Any 按扩展名挑解析器，返回正文（以及 DOCX 那边偶尔能拿到的标题）。
//
// 失败的时候返回的错误是**给她看的那一句**（ErrText），不是给日志看的堆栈：
// 「这个文件没能解析出正文」对一个扫描件来说是错的描述 —— 文件没坏，它本来就
// 没有文字层，她需要知道的是「这份 PDF 是扫描件，先用能选中文字的版本」。
func Any(filename string, b []byte) (title, text string, err error) {
	if len(b) == 0 {
		return "", "", &ErrText{Msg: "这个文件是空的。"}
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		title, text, err = PDF(b)
		if err != nil {
			return "", "", &ErrText{Msg: "这个 PDF 没能解析出正文。如果它是扫描件，请换一份能选中文字的版本，或者直接把正文粘进来。", Err: err}
		}
		if strings.TrimSpace(text) == "" {
			// 🚨 解析没报错、正文却是空的 = 扫描件（没有文字层）。这两种要分开说，
			// 合成一句「解析失败」的话，她会以为是文件坏了，反复换文件。
			return "", "", &ErrText{Msg: "这份 PDF 里没有可复制的文字，它多半是扫描件。请换一份能选中文字的版本，或者直接把正文粘进来。"}
		}
		return title, text, nil
	case ".docx":
		title, text, err = DOCX(b)
		if err != nil {
			return "", "", &ErrText{Msg: "这个 DOCX 没能解析出正文。请另存一份再试，或者直接把正文粘进来。", Err: err}
		}
		return title, text, nil
	case ".txt", ".md":
		if !utf8.Valid(b) {
			return "", "", &ErrText{Msg: "这个文本文件不是 UTF-8 编码，请另存为 UTF-8 再上传。"}
		}
		return "", string(b), nil
	default:
		return "", "", &ErrText{Msg: fmt.Sprintf("只收这几种文件：%s。", strings.Join(Supported, " / "))}
	}
}

// ErrText 是一个**能直接给她看**的失败。Msg 进界面，Err 进日志。
type ErrText struct {
	Msg string
	Err error
}

func (e *ErrText) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *ErrText) Unwrap() error { return e.Err }
