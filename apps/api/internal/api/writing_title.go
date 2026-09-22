package api

// writing_title.go — giving the finished piece a NAME.
//
// ## The bug this exists for
//
// A writing's title is born as her raw idea sentence: createWriting
// (writings.go) takes the one line she typed into 「我想写：…」 and uses it,
// truncated to 200 runes, as `writing.title`. That was always meant to be a
// placeholder — EditableTitle exists precisely so she can replace it — but
// nothing ever ASKED her to, so most pieces are still carrying it at 完成这篇.
//
// It stops being a private placeholder at that moment. The report puts the
// title in display type at the top of the hero, the exported poster prints
// it, and the share link opens it for someone with no account at all. So a
// parent scanning a QR code was reading 「我想写中国是不是真的让地球更可持续
// 了，因为我们地理课上…」 where the piece's name should be.
//
// ## The shape: asked at 完成这篇, and she names it herself
//
// `POST /title-ideas` answers ONE question, with no model call: is the title
// still the placeholder (`needsName`)? If not, 完成这篇 goes straight through.
// If so, the client opens a small dialog with a text box.
//
// 🚨 2026-09-18 产品负责人：「ai不要直接生成题目文本，我觉得可以在学生点击
// 『需要提示』之后，给出一些关键词，但是不能直接给取名字。」
//
// The first version offered four finished titles as tappable chips — she
// tapped one and the piece was named by 印记. Now:
//
//   - nothing is suggested until she asks (「需要提示」);
//   - what she gets is **keywords** (`POST /title-keywords`), and every one of
//     them must appear VERBATIM in her draft (validateTitleKeywords). A title
//     cannot be validated as a substring of her writing — naming is finding
//     words that are not already there — but a keyword can, so the rule
//     「不能直接给取名字」 is enforced by the checker, not by the prompt's manners;
//   - the chips are inert: tapping one does not fill the box. She composes the
//     name from them herself.
//
// The server never writes a title here; she saves hers through PATCH
// /writings/{id}.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// titleRuneCap is createWriting's own truncation, repeated here because
// `needsName` compares against exactly what that function stored. If one
// changes, both must — a mismatch would make every writing look renamed
// (and silently switch this whole feature off), which is the failure mode
// worth naming rather than the one worth being clever about.
const titleRuneCap = 200

// 关键词的长度：2–12 个字符（英文一两个词）。再长就是一句话，
// 一句话离一个标题只差一步。
const (
	titleKeywordMinRunes = 2
	titleKeywordMaxRunes = 12
	titleKeywordMax      = 6
)

// titleIsStillTheIdea reports whether `title` is untouched since
// createWriting — i.e. it is still her idea sentence, truncated the same way
// that function truncates it.
//
// Equality, not a heuristic. A length threshold ("titles are short") would
// pester a student who deliberately chose a long title and would miss a
// short idea sentence entirely; comparing against the actual stored opening
// turn is exact, and it costs nothing because the caller has already loaded
// the messages.
//
// Whitespace is trimmed on both sides before comparing: createWriting stores
// the idea already trimmed, and renameWriting trims too, so a difference in
// surrounding space alone never means she renamed anything.
func titleIsStillTheIdea(title, idea string) bool {
	t := strings.TrimSpace(title)
	i := strings.TrimSpace(idea)
	if i == "" {
		// No opening turn to compare against (should not happen — createWriting
		// writes it in the same transaction as the writing row). Treat the
		// title as hers rather than interrupting her over a row we cannot
		// account for.
		return false
	}
	if len([]rune(i)) > titleRuneCap {
		i = string([]rune(i)[:titleRuneCap])
	}
	return t == strings.TrimSpace(i)
}

// firstStudentMessage returns the verbatim opening turn — her idea sentence,
// untruncated. createWriting writes it as seq=1, role='student', in the same
// transaction as the writing itself, so it is always the first row;
// ListAtomMessages already orders by seq and already filters out block
// threads in SQL (see writing_deepen.go), so no caller of this can pick up a
// 深入一层 message by accident.
func firstStudentMessage(msgs []sqlc.AtomMessage) string {
	for _, m := range msgs {
		if m.Role == "student" {
			return m.Content
		}
	}
	return ""
}

// validateTitleKeywords keeps the keywords that are really HERS: each must
// appear verbatim in her draft (after stripping the quotes/书名号 a model
// likes to wrap things in), be 2–12 characters, not repeat, and not be the
// placeholder title. At most titleKeywordMax.
//
// 🚨 The substring check is the whole point (see the file header): a model
// that returns 「苦乐在心」 when her draft never says it has handed her a
// title, not a keyword — and it is dropped.
func validateTitleKeywords(keywords []string, draft, current string) []string {
	out := make([]string, 0, titleKeywordMax)
	seen := map[string]bool{strings.TrimSpace(current): true}
	for _, raw := range keywords {
		k := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), `"'“”‘’「」《》『』`))
		n := len([]rune(k))
		if n < titleKeywordMinRunes || n > titleKeywordMaxRunes || seen[k] {
			continue
		}
		if !strings.Contains(draft, k) {
			continue
		}
		seen[k] = true
		out = append(out, k)
		if len(out) == titleKeywordMax {
			break
		}
	}
	return out
}

type writingTitleKeywordsResult struct {
	Keywords []string `json:"keywords"`
}

func parseWritingTitleKeywords(text string) (writingTitleKeywordsResult, bool) {
	c := extractWritingJSONObject(text)
	if c == "" {
		return writingTitleKeywordsResult{}, false
	}
	var got writingTitleKeywordsResult
	if err := json.Unmarshal([]byte(c), &got); err != nil {
		return writingTitleKeywordsResult{}, false
	}
	return got, true
}

// suggestWritingTitles is POST /api/v1/writings/{id}/title-ideas — despite the
// route's old name it no longer suggests anything: it answers `needsName`
// with no model call. `ideas` stays in the wire shape, always empty, so an
// older client in a stale tab keeps working. See the file header.
func (a *API) suggestWritingTitles(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"needsName": titleIsStillTheIdea(wr.Title, firstStudentMessage(msgs)),
		"ideas":     []string{},
	})
}

// suggestWritingTitleKeywords is POST /api/v1/writings/{id}/title-keywords —
// 「需要提示」. A spend endpoint (one model call, metered as
// purpose="title_keywords"); persists nothing.
func (a *API) suggestWritingTitleKeywords(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	draft, derr := a.d.Queries.GetWritingDraft(r.Context(), at.ID)
	body := ""
	if derr == nil {
		body = strings.TrimSpace(draft.Body)
	}
	if body == "" {
		// Nothing written, nothing to take words from.
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"keywords": []string{}})
		return
	}

	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// §model-routing · digest：长输入（整篇）、短输出（几个词），而且产出是
	// 从她的原文里摘的，不需要推理预算。
	resolved, ok2 := a.route(turnCtx, gateway.ClassDigest)
	if !ok2 {
		slog.Warn("writing title keywords: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingTitleKeywordsSystem},
			{Role: gateway.RoleUser, Content: buildWritingTitleKeywordsPrompt(wr, body)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "title_keywords", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing title keywords: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	parsed, okParse := parseWritingTitleKeywords(res.Text)
	if !okParse {
		slog.Warn("writing title keywords: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
			"reply_head", headRunes(res.Text, 120))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"keywords": validateTitleKeywords(parsed.Keywords, body, wr.Title),
	})
}
