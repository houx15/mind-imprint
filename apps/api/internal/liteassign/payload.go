package liteassign

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"
)

type PayloadError struct{ Code, Message string }

func (e *PayloadError) Error() string { return e.Code + ": " + e.Message }

func perr(code, msg string) error { return &PayloadError{Code: code, Message: msg} }

type ReadingPayload struct {
	Source string `json:"source"`
	Slug   string `json:"slug,omitempty"`
	Tier   *int   `json:"tier,omitempty"`
	URL    string `json:"url,omitempty"`
	Text   string `json:"text,omitempty"`
}

type WritingPayload struct {
	Prompt      string `json:"prompt"`
	TargetWords int    `json:"targetWords"`
	Lang        string `json:"lang"`
}

type ProjectPayload struct {
	DrivingQuestion string `json:"drivingQuestion"`
	Description     string `json:"description"`
}

const (
	maxPromptRunes       = 2000
	maxTextRunes         = 50000
	maxInstructionsRunes = 2000
)

// ValidateInstructions trims the teacher's 说明 and caps it at 2000 runes.
// The student sees it in her inbox, so an unbounded field would crowd the list.
func ValidateInstructions(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxInstructionsRunes {
		return "", perr("instructions_too_long", "说明不能超过 2000 字")
	}
	return s, nil
}

// isHTTPURL mirrors putReadingSourceLite's rule: only http/https may be
// stored, since a javascript: or data: URL would be unsafe to open later.
func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "http" || scheme == "https") && u.Host != ""
}

// ValidatePayload checks a payload for kind and returns it re-marshalled
// from the typed struct, so unknown fields are dropped and strings trimmed.
func ValidatePayload(kind string, raw json.RawMessage) (json.RawMessage, error) {
	switch kind {
	case "reading":
		var p ReadingPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.Slug, p.URL, p.Text = strings.TrimSpace(p.Slug), strings.TrimSpace(p.URL), strings.TrimSpace(p.Text)
		switch p.Source {
		case "library":
			if p.Slug == "" {
				return nil, perr("invalid_slug", "请选择一篇文章")
			}
			if p.Tier != nil && (*p.Tier < 1 || *p.Tier > 5) {
				return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
			}
			p.URL, p.Text = "", ""
		case "url":
			if !isHTTPURL(p.URL) {
				return nil, perr("invalid_url", "请输入以 http 或 https 开头的链接")
			}
			p.Slug, p.Tier, p.Text = "", nil, ""
		case "text":
			if p.Text == "" {
				return nil, perr("empty_text", "请粘贴文章正文")
			}
			if utf8.RuneCountInString(p.Text) > maxTextRunes {
				return nil, perr("text_too_long", "文章正文不能超过 50000 字")
			}
			p.Slug, p.Tier, p.URL = "", nil, ""
		default:
			return nil, perr("invalid_source", "阅读来源只能是分级阅读库、链接或正文")
		}
		return json.Marshal(p)
	case "writing":
		var p WritingPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.Prompt = strings.TrimSpace(p.Prompt)
		if p.Prompt == "" {
			return nil, perr("empty_prompt", "请填写写作题目")
		}
		if utf8.RuneCountInString(p.Prompt) > maxPromptRunes {
			return nil, perr("prompt_too_long", "写作题目不能超过 2000 字")
		}
		if p.TargetWords < 1 || p.TargetWords > 100000 {
			return nil, perr("invalid_target_words", "目标字数需在 1 到 100000 之间")
		}
		if p.Lang != "zh" && p.Lang != "en" {
			return nil, perr("invalid_lang", "语言只能是中文或英文")
		}
		return json.Marshal(p)
	case "project":
		var p ProjectPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.DrivingQuestion, p.Description = strings.TrimSpace(p.DrivingQuestion), strings.TrimSpace(p.Description)
		if p.DrivingQuestion == "" {
			return nil, perr("empty_driving_question", "请填写驱动问题")
		}
		if utf8.RuneCountInString(p.DrivingQuestion) > 4000 {
			return nil, perr("driving_question_too_long", "驱动问题不能超过 4000 字")
		}
		return json.Marshal(p)
	default:
		return nil, perr("invalid_kind", "作业类型只能是阅读、写作或项目")
	}
}
