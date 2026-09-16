package pbl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AudienceBoard describes the student's intended audience, not verified research.
// IDs are stable within a document so changing a role label preserves its work.
type AudienceBoard struct {
	ID            string                          `json:"id"`
	Role          string                          `json:"role"`
	Person        string                          `json:"person"`
	AgeRange      string                          `json:"ageRange"`
	Hobbies       []string                        `json:"hobbies,omitempty"`
	Interests     []string                        `json:"interests"`
	Offerings     []string                        `json:"offerings"`
	KeywordDrafts map[string]AudienceKeywordDraft `json:"keywordDrafts,omitempty"`
}

type AudienceKeywordDraft struct {
	Text    string  `json:"text"`
	Editing *string `json:"editing"`
}

type AudienceDocument struct {
	Boards         []AudienceBoard     `json:"boards"`
	ArchivedBoards []AudienceBoard     `json:"archivedBoards,omitempty"`
	ActiveBoardID  string              `json:"activeBoardId"`
	Step           string              `json:"step"`
	Summary        *AudienceSummary    `json:"summary,omitempty"`
	Keywords       map[string][]string `json:"keywords,omitempty"`
}

// NormalizeAudienceDocument accepts incomplete drafts, but never silent truncation
// or duplicate board identities. Confirmation has stronger completeness rules.
func NormalizeAudienceDocument(in AudienceDocument, complete bool) (AudienceDocument, error) {
	if len(in.ArchivedBoards) > 0 {
		archived, err := NormalizeAudienceDocument(AudienceDocument{Boards: in.ArchivedBoards, Step: "roles"}, false)
		if err != nil {
			return in, err
		}
		in.ArchivedBoards = archived.Boards
		for _, inactive := range in.ArchivedBoards {
			for _, active := range in.Boards {
				if inactive.ID == strings.TrimSpace(active.ID) || inactive.Role == strings.TrimSpace(active.Role) {
					return in, fmt.Errorf("已选与未选人物板重复")
				}
			}
		}
	}
	if len(in.Boards) > 12 {
		return in, fmt.Errorf("人物板不能超过12个")
	}
	switch in.Step {
	case "roles", "person", "age", "interests", "offerings", "summary":
	default:
		return in, fmt.Errorf("人物板步骤无效")
	}
	seen := map[string]bool{}
	for i := range in.Boards {
		b := &in.Boards[i]
		b.ID, b.Role, b.Person, b.AgeRange = strings.TrimSpace(b.ID), strings.TrimSpace(b.Role), strings.TrimSpace(b.Person), strings.TrimSpace(b.AgeRange)
		if b.ID == "" || len(b.ID) > 80 || seen[b.ID] {
			return in, fmt.Errorf("人物板标识无效或重复")
		}
		seen[b.ID] = true
		if b.Role == "" || len([]rune(b.Role)) > 100 || len([]rune(b.Person)) > 200 || len([]rune(b.AgeRange)) > 100 {
			return in, fmt.Errorf("人物板称呼或年龄长度无效")
		}
		var err error
		for field, draft := range b.KeywordDrafts {
			if field != "hobbies" && field != "interests" && field != "offerings" {
				return in, fmt.Errorf("关键词草稿字段无效")
			}
			if len([]rune(draft.Text)) > 300 || (draft.Editing != nil && len([]rune(*draft.Editing)) > 300) {
				return in, fmt.Errorf("关键词草稿过长")
			}
			if complete && (strings.TrimSpace(draft.Text) != "" || draft.Editing != nil) {
				return in, fmt.Errorf("请先添加、保存或取消%s人物板中尚未完成的关键词", b.Role)
			}
		}
		if b.Hobbies, err = normalizeAudienceItems(b.Hobbies); err != nil {
			return in, err
		}
		if b.Interests, err = normalizeAudienceItems(b.Interests); err != nil {
			return in, err
		}
		if b.Offerings, err = normalizeAudienceItems(b.Offerings); err != nil {
			return in, err
		}
		if complete && (b.Person == "" || b.AgeRange == "" || len(b.Interests) == 0 || len(b.Offerings) == 0) {
			return in, fmt.Errorf("请完善每个人物板的具体人物、年龄、关注内容和展示内容")
		}
	}
	if in.ActiveBoardID != "" && !seen[in.ActiveBoardID] {
		return in, fmt.Errorf("当前人物板不存在")
	}
	if complete && len(in.Boards) == 0 {
		return in, fmt.Errorf("请至少选择一类读者")
	}
	if in.Boards == nil {
		in.Boards = []AudienceBoard{}
	}
	if in.Summary != nil {
		data, _ := json.Marshal(in.Summary)
		if _, err := ParseAudienceSummary(string(data), in); err != nil {
			return in, err
		}
	}
	for id, words := range in.Keywords {
		if !seen[id] || len(words) > 6 {
			return in, fmt.Errorf("关键词人物板或数量无效")
		}
		unique := map[string]bool{}
		for _, word := range words {
			if strings.TrimSpace(word) == "" || len([]rune(word)) > 40 || unique[word] {
				return in, fmt.Errorf("关键词为空、重复或过长")
			}
			unique[word] = true
		}
	}
	return in, nil
}

func normalizeAudienceItems(items []string) ([]string, error) {
	if len(items) > 12 {
		return nil, fmt.Errorf("每个人物板的每类关键词不能超过12项")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if len([]rune(item)) > 300 {
			return nil, fmt.Errorf("每项内容不能超过300字")
		}
		if item != "" && !seen[item] {
			out = append(out, item)
			seen[item] = true
		}
	}
	return out, nil
}
