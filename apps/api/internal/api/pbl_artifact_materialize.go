package api

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"mindimprint/api/internal/pbl"
)

// Both evidence review and persistence use this projection. The immutable
// source version, not model-supplied metadata, determines inherited fields.
type editedArtifact struct {
	PaperLayout *pbl.PaperLayout `json:"paperLayout,omitempty"`
	Title       string           `json:"title"`
	Body        string           `json:"body"`
	PrintLayout *pbl.PrintLayout `json:"printLayout,omitempty"`
	Guessed     []string         `json:"guessed"`
	Admits      []string         `json:"admits"`
}

func (a *API) materializeArtifactEdits(ctx context.Context, atomID uuid.UUID, kind, baseID string, edits []pbl.TextEdit, guessed, admits []string, paperEdits ...pbl.PaperEdit) (editedArtifact, error) {
	var out editedArtifact
	id, err := uuid.Parse(baseID)
	if err != nil {
		return out, errors.New("局部修改缺少有效的原成果")
	}
	base, err := a.d.Queries.GetPblArtifact(ctx, id)
	if err != nil || base.AtomID != atomID {
		return out, errors.New("原成果不属于当前项目")
	}
	if base.Kind != kind || (kind != "draft" && kind != "spec" && kind != "options") {
		return out, errors.New("该成果类型不支持文字局部修改")
	}
	if err := json.Unmarshal(base.Payload, &out); err != nil {
		return editedArtifact{}, err
	}
	out.Title = base.Title
	if err := json.Unmarshal(base.Guessed, &out.Guessed); err != nil {
		return editedArtifact{}, err
	}
	if err := json.Unmarshal(base.Admits, &out.Admits); err != nil {
		return editedArtifact{}, err
	}
	if len(paperEdits) > 0 {
		if out.PaperLayout == nil || len(edits) > 0 {
			return out, errors.New("paperEdits仅用于图形纸面成果，不能同时提交文字edits")
		}
		updated, err := out.PaperLayout.ApplyEdits(paperEdits)
		if err != nil {
			return out, err
		}
		out.PaperLayout = &updated
		out.Body = updated.Markdown()
	} else if out.PaperLayout != nil {
		return out, errors.New("图形局部修订请使用paperEdits；整体重做使用replacesArtifactId与完整paperLayout")
	}
	if len(edits) == 0 && len(paperEdits) == 0 && guessed == nil && admits == nil {
		return out, errors.New("局部修改必须包含正文修改或假设与局限更新")
	}
	if len(edits) > 0 {
		if out.PrintLayout != nil {
			updated, err := out.PrintLayout.ApplyEdits(edits)
			if err != nil {
				return out, err
			}
			out.PrintLayout = &updated
			out.Body = updated.Markdown()
		} else {
			updated, err := pbl.ApplyTextEdits(out.Body, edits)
			if err != nil {
				return out, err
			}
			out.Body = updated
		}
	}
	if guessed != nil {
		out.Guessed = guessed
	}
	if admits != nil {
		out.Admits = admits
	}
	return out, nil
}
