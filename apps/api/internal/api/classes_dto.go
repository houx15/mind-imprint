package api

import "mindimprint/api/internal/store/sqlc"

type classDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	JoinCode  string `json:"join_code"`
	SchoolID  string `json:"school_id"`
	CreatedAt string `json:"created_at"`
}

func toClassDTO(c sqlc.Class) classDTO {
	return classDTO{
		ID:        c.ID.String(),
		Name:      c.Name,
		JoinCode:  c.JoinCode,
		SchoolID:  c.SchoolID.String(),
		CreatedAt: c.CreatedAt.Format(tsLayout),
	}
}
