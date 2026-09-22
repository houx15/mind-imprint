package api

// document_extract.go —— POST /api/v1/documents/extract
//
// 把一个上传的文件变成纯文本，**不落任何库**。
//
// 产品负责人 2026-09-12：「in writing part we would allow pdf or docx or txt
// formats to upload. so make this a general tool.」
//
// 🚨 为什么不是「再写一个写作版的上传接口」：阅读那一侧的上传（
// reading_source_file.go）把文本直接落成那一篇阅读材料，它和一个 reading id
// 绑死。写作要的是另一件事 —— 把文字取出来放进她正在写的那个框，落库与否由她
// 按「AI审阅」的时候决定。共用的是**取文字**这一段，不是落库那一段。
//
// 所以这个接口只做取文字：收文件、回 {title, text}。阅读那条路照旧（它还要
// 落库），但两边认的格式和失败话术都来自 docextract 那一处。

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/docextract"
	"mindimprint/api/internal/httpx"
)

type documentExtractResponse struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

func (a *API) postDocumentExtract(w http.ResponseWriter, r *http.Request) {
	// 登录才收文件。路由那一层已经挂了鉴权中间件，这里只是把它取出来确认一遍 ——
	// 这个接口不碰任何 atom，所以没有 loadOwnedAtom 那条归属校验替它守门。
	if _, ok := UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("请先登录"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, liteSourceFileMaxBytes)
	if err := r.ParseMultipartForm(liteSourceFileMultipartMemory); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("file_too_large",
			"文件太大了，请上传 30 MB 以内的文件。", nil))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_file", "请选择要上传的文件。", nil))
		return
	}
	defer file.Close()

	if !docextract.IsSupported(header.Filename) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_type",
			"支持的文件格式："+strings.Join(docextract.Supported, " / ")+"。", nil))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_file", "文件读取失败，请重新上传。", nil))
		return
	}

	title, text, err := docextract.Any(header.Filename, data)
	if err != nil {
		// 具体那一句给她 —— 扫描件和「文件坏了」是两回事。
		msg := "文件解析失败：未提取到正文，请尝试粘贴正文。"
		var te *docextract.ErrText
		if errors.As(err, &te) {
			msg = te.Msg
		}
		slog.Info("document extract: failed", "err", err, "name", header.Filename)
		httpx.WriteError(w, r, httpx.ErrBadRequest("extract_failed", msg, nil))
		return
	}

	body := strings.TrimSpace(text)
	if body == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text",
			"文件中未识别到文字，请尝试粘贴正文。", nil))
		return
	}

	httpx.WriteJSON(w, http.StatusOK, documentExtractResponse{Title: title, Text: body})
}
