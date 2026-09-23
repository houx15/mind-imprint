package liteassign

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/library"
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
	// Server-cached page title for URL assignments. Client input is discarded
	// by ValidatePayload; publication fills it together with Text.
	ArticleTitle string `json:"articleTitle,omitempty"`
	// FileName is set when the teacher uploaded a document for a text source.
	FileName string `json:"fileName,omitempty"`
	// Disciplines and Picks belong to the personalized source. Picks is keyed
	// by the student's user id; a student with no pick is recommended at start.
	Disciplines []string                `json:"disciplines,omitempty"`
	Picks       map[string]PersonalPick `json:"picks,omitempty"`
}

// PersonalPick is the article the teacher confirmed for one student. A nil
// Tier means the student's suggested tier when she starts.
type PersonalPick struct {
	Slug string `json:"slug"`
	Tier *int   `json:"tier"`
}

type WritingPayload struct {
	Prompt      string  `json:"prompt"`
	TargetWords int     `json:"targetWords"`
	Lang        string  `json:"lang"`
	Rubric      *Rubric `json:"rubric,omitempty"`
}

type ProjectPayload struct {
	DrivingQuestion string `json:"drivingQuestion"`
	Description     string `json:"description"`
}

const (
	maxPromptRunes       = 2000
	maxTextRunes         = 50000
	maxInstructionsRunes = 2000
	maxFileNameRunes     = 200
)

func validTier(t *int) bool { return t == nil || (*t >= 1 && *t <= 5) }

// hasLevel reports whether art has the given tier — checked separately from
// validTier's 1..5 range so a future article published with fewer levels
// still rejects a pick at publish time, not at her start (picks lock once
// she starts, so she could not recover from a bad one then).
func hasLevel(art library.Article, tier int) bool {
	_, ok := art.LevelAt(tier)
	return ok
}

// ValidateDisciplines trims, drops blanks and duplicates, and refuses an id
// that is not in the discipline table.
func ValidateDisciplines(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		if _, ok := disciplines.ByID(id); !ok {
			return nil, perr("invalid_discipline", "学科不在学科表中："+id)
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// PicksOutside reports whether a personalized payload picks an article for a
// user who is not one of recipients. Other payloads have no picks.
func PicksOutside(payload json.RawMessage, recipients []uuid.UUID) bool {
	var p ReadingPayload
	if json.Unmarshal(payload, &p) != nil || p.Source != "personalized" {
		return false
	}
	in := make(map[string]bool, len(recipients))
	for _, id := range recipients {
		in[id.String()] = true
	}
	for uid := range p.Picks {
		if !in[uid] {
			return true
		}
	}
	return false
}

// PicksOutsideChanged is PicksOutside for a PATCH: it only refuses a pick
// that is new, or whose slug or tier changed from old, and whose user is not
// one of recipients. A pick left exactly as it was in old is not re-checked,
// so removing a recipient's pick from the recipient list never blocks a save
// that leaves her stale pick untouched — her pick stays in the stored
// payload rather than being stripped or refused.
func PicksOutsideChanged(payload, old json.RawMessage, recipients []uuid.UUID) bool {
	var p ReadingPayload
	if json.Unmarshal(payload, &p) != nil || p.Source != "personalized" {
		return false
	}
	var prior ReadingPayload
	_ = json.Unmarshal(old, &prior) // old may be a different kind; a failed parse just means "nothing there before"
	in := make(map[string]bool, len(recipients))
	for _, id := range recipients {
		in[id.String()] = true
	}
	for uid, pick := range p.Picks {
		if in[uid] {
			continue
		}
		if was, ok := prior.Picks[uid]; ok && was.Slug == pick.Slug && sameTier(was.Tier, pick.Tier) {
			continue
		}
		return true
	}
	return false
}

// PickOnlyChange reports whether going from (oldKind, old) to (kind, payload)
// changes nothing but personalized picks, and if so, which users' picks were
// added, removed or modified. Both payloads must be validated. Once a
// student has started, a PATCH is allowed only when onlyPicks is true and
// none of the changed users has started.
func PickOnlyChange(kind, oldKind string, payload, old json.RawMessage) (changed []string, onlyPicks bool) {
	if kind != "reading" || oldKind != "reading" {
		return nil, false
	}
	var p, prior ReadingPayload
	if json.Unmarshal(payload, &p) != nil || json.Unmarshal(old, &prior) != nil {
		return nil, false
	}
	if p.Source != "personalized" || prior.Source != "personalized" {
		return nil, false
	}
	if !sameTier(p.Tier, prior.Tier) || !sameStrings(p.Disciplines, prior.Disciplines) {
		return nil, false
	}
	if p.Slug != prior.Slug || p.URL != prior.URL || p.Text != prior.Text || p.FileName != prior.FileName {
		return nil, false
	}
	for uid, pick := range p.Picks {
		was, ok := prior.Picks[uid]
		if !ok || was.Slug != pick.Slug || !sameTier(was.Tier, pick.Tier) {
			changed = append(changed, uid)
		}
	}
	for uid := range prior.Picks {
		if _, ok := p.Picks[uid]; !ok {
			changed = append(changed, uid)
		}
	}
	return changed, true
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameTier(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

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
		p.FileName = strings.TrimSpace(p.FileName)
		p.ArticleTitle = ""
		switch p.Source {
		case "library":
			if p.Slug == "" {
				return nil, perr("invalid_slug", "请选择一篇文章")
			}
			if !validTier(p.Tier) {
				return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
			}
			p.URL, p.Text, p.FileName, p.Disciplines, p.Picks = "", "", "", nil, nil
		case "url":
			if !isHTTPURL(p.URL) {
				return nil, perr("invalid_url", "请输入以 http 或 https 开头的链接")
			}
			p.Slug, p.Tier, p.Text, p.FileName, p.Disciplines, p.Picks = "", nil, "", "", nil, nil
		case "text":
			if p.Text == "" {
				return nil, perr("empty_text", "请粘贴文章正文")
			}
			if utf8.RuneCountInString(p.Text) > maxTextRunes {
				return nil, perr("text_too_long", "文章正文不能超过 50000 字")
			}
			if utf8.RuneCountInString(p.FileName) > maxFileNameRunes {
				return nil, perr("file_name_too_long", "文件名不能超过 200 字")
			}
			p.Slug, p.Tier, p.URL, p.Disciplines, p.Picks = "", nil, "", nil, nil
		case "personalized":
			if !validTier(p.Tier) {
				return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
			}
			ds, err := ValidateDisciplines(p.Disciplines)
			if err != nil {
				return nil, err
			}
			p.Disciplines = ds
			picks := make(map[string]PersonalPick, len(p.Picks))
			for key, pick := range p.Picks {
				uid, err := uuid.Parse(strings.TrimSpace(key))
				if err != nil {
					return nil, perr("invalid_pick_user", "个性化名单中的学生无效")
				}
				canonical := uid.String()
				if _, exists := picks[canonical]; exists {
					// Two source keys (e.g. differing only by letter case)
					// normalised to the same user id. Reject rather than let
					// map iteration order pick a silent winner.
					return nil, perr("invalid_pick_user", "个性化名单中有重复的学生")
				}
				pick.Slug = strings.TrimSpace(pick.Slug)
				art, ok := library.BySlug(pick.Slug)
				if !ok {
					return nil, perr("invalid_pick_slug", "文章不在阅读库里："+pick.Slug)
				}
				if !validTier(pick.Tier) {
					return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
				}
				if pick.Tier != nil && !hasLevel(art, *pick.Tier) {
					return nil, perr("invalid_pick_tier", "这篇文章没有这一档")
				}
				picks[canonical] = pick
			}
			p.Picks = picks
			p.Slug, p.URL, p.Text, p.FileName = "", "", "", ""
		default:
			return nil, perr("invalid_source", "阅读来源只能是分级阅读库、链接、正文或个性化阅读")
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
		if p.Rubric != nil {
			r, err := ValidateRubric(*p.Rubric)
			if err != nil {
				return nil, err
			}
			p.Rubric = &r
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
