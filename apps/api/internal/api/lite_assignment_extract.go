package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// assignmentExtractSystem asks for the three writing-assignment settings the
// form has. The prompt field keeps the teacher's own words: this call is a
// compose step over text she already wrote, not a rewrite.
const assignmentExtractSystem = `你从老师粘贴的一段作业说明里提取写作作业的三项设置。
只输出 JSON：{"prompt":"","targetWords":null,"lang":"zh"}。
prompt：学生要写的题目或要求，保留老师的原话，不改写、不补充。
targetWords：老师写明的字数；写的是范围取上限；没写就填 null。
lang：作文要用的语言，中文填 zh，英文填 en。`

// maxExtractTextRunes caps the pasted text sent to the model.
const maxExtractTextRunes = 4000

var (
	extractHanRune = regexp.MustCompile(`\p{Han}`)
	extractFence   = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(.*?)\\s*```\\s*$")
)

// normalizeExtraction parses the model's reply and clamps it to what the
// form accepts. A missing, zero or negative word count means the teacher gave
// none and becomes nil; a positive count is clamped to 50–10000. A language
// other than zh/en falls back by script (any Han rune ⇒ zh, else en). An empty
// prompt or a reply that is not JSON is not ok — the caller surfaces it, never
// fills in.
func normalizeExtraction(raw string) (string, *int, string, bool) {
	if m := extractFence.FindStringSubmatch(raw); m != nil {
		raw = m[1]
	}
	var v struct {
		Prompt      string   `json:"prompt"`
		TargetWords *float64 `json:"targetWords"`
		Lang        string   `json:"lang"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &v); err != nil {
		return "", nil, "", false
	}
	prompt := strings.TrimSpace(v.Prompt)
	if prompt == "" {
		return "", nil, "", false
	}
	var words *int
	// The model may copy a 0 for "not given"; that must not become 50 words
	// the teacher never wrote.
	if v.TargetWords != nil && int(*v.TargetWords) > 0 {
		n := int(*v.TargetWords)
		if n < 50 {
			n = 50
		}
		if n > 10000 {
			n = 10000
		}
		words = &n
	}
	lang := v.Lang
	if lang != "zh" && lang != "en" {
		lang = "en"
		if extractHanRune.MatchString(prompt) {
			lang = "zh"
		}
	}
	return prompt, words, lang, true
}

// extractLiteAssignment handles POST /api/v1/lite/teacher/assignments/extract:
// one compose call turning pasted assignment text into {prompt, targetWords, lang}.
// The call is metered whether or not the reply parses; the tokens were spent.
func (a *API) extractLiteAssignment(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || utf8.RuneCountInString(text) > maxExtractTextRunes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_text", "请粘贴 1 到 4000 字的作业说明", nil))
		return
	}

	ctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(ctx, gateway.ClassCompose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: assignmentExtractSystem},
			{Role: gateway.RoleUser, Content: text},
		},
		ResponseFormat: gateway.ResponseFormatJSONObject,
	})
	a.recordLiteLLMCall(ctx, u.ID, uuid.Nil, "assignment_extract", resolved, res.Usage)
	if cerr != nil {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("extract_call: "+cerr.Error()))
		return
	}
	prompt, words, lang, ok := normalizeExtraction(res.Text)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("extract_unparsed: "+truncateRunes(res.Text, 200)))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"prompt": prompt, "targetWords": words, "lang": lang})
}
