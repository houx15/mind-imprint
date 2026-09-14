package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteweekly"
	"mindimprint/api/internal/store/sqlc"
)

// loadLiteClassWeek gathers the facts of one completed Beijing week for every
// student currently enrolled in the class. Read-only: it never generates a
// report and never calls a model.
func (a *API) loadLiteClassWeek(ctx context.Context, classID uuid.UUID, weekStart time.Time) ([]liteweekly.StudentWeek, error) {
	members, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return nil, err
	}
	return a.loadLiteWeeks(ctx, classID, members, weekStart)
}

// loadLiteStudentWeek is loadLiteClassWeek for one student. A user who is not
// a current student member of the class gets pgx.ErrNoRows.
func (a *API) loadLiteStudentWeek(ctx context.Context, classID, userID uuid.UUID, weekStart time.Time) (liteweekly.StudentWeek, error) {
	members, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return liteweekly.StudentWeek{}, err
	}
	for _, m := range members {
		if m.ID != userID {
			continue
		}
		weeks, err := a.loadLiteWeeks(ctx, classID, []sqlc.ListLiteWeekClassStudentsRow{m}, weekStart)
		if err != nil {
			return liteweekly.StudentWeek{}, err
		}
		return weeks[0], nil
	}
	return liteweekly.StudentWeek{}, pgx.ErrNoRows
}

// loadLiteWeeks runs each fact query once for all members and groups the rows
// by user id. The result keeps the members' order.
func (a *API) loadLiteWeeks(ctx context.Context, classID uuid.UUID, members []sqlc.ListLiteWeekClassStudentsRow, weekStart time.Time) ([]liteweekly.StudentWeek, error) {
	ws := weekStart.In(liteweek.Beijing)
	we := ws.AddDate(0, 0, 7)
	prev := ws.AddDate(0, 0, -7)

	ids := make([]uuid.UUID, 0, len(members))
	byID := make(map[uuid.UUID]*liteweekly.StudentWeek, len(members))
	out := make([]liteweekly.StudentWeek, len(members))
	for i, m := range members {
		ids = append(ids, m.ID)
		out[i] = liteweekly.StudentWeek{
			UserID: m.ID.String(), Name: m.DisplayName, Minutes: -1,
			Finished: []liteweekly.Item{}, Stalled: []liteweekly.Item{},
			NewKeywords: []string{}, Moments: []liteweekly.Moment{},
		}
		byID[m.ID] = &out[i]
	}
	if len(ids) == 0 {
		return out, nil
	}
	q := a.d.Queries

	activity, err := q.ListLiteWeekActivity(ctx, sqlc.ListLiteWeekActivityParams{
		UserIds: ids, WeekStart: ws, WeekEnd: we, PrevStart: prev,
		WeekStartDay: pgDate(ws), WeekEndDay: pgDate(we), PrevStartDay: pgDate(prev),
	})
	if err != nil {
		return nil, err
	}
	for _, r := range activity {
		s := byID[r.UserID]
		if s == nil {
			continue
		}
		s.ActiveDays = int(r.ActiveDays)
		s.PrevActiveDays = int(r.PrevActiveDays)
		s.Turns = int(r.Turns)
		if r.HasAnyBucketByWeekEnd {
			s.Minutes = int(r.Seconds) / 60
		}
	}

	finished, err := q.ListLiteWeekFinished(ctx, sqlc.ListLiteWeekFinishedParams{UserIds: ids, WeekStart: ws, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range finished {
		if s := byID[r.UserID]; s != nil {
			s.Finished = append(s.Finished, liteweekly.Item{Kind: r.Kind, Title: r.Title})
		}
	}

	states, err := q.ListLiteWeekAssignmentStates(ctx, sqlc.ListLiteWeekAssignmentStatesParams{
		ClassID: classID, UserIds: ids, WeekStart: ws, WeekEnd: we,
	})
	if err != nil {
		return nil, err
	}
	for _, r := range states {
		s := byID[r.UserID]
		if s == nil {
			continue
		}
		// Work finished after the week ended does not count as finished for
		// this week: at week end it was still overdue.
		fin := tsPtr(r.FinishedAt)
		if fin != nil && !fin.Before(we) {
			fin = nil
		}
		switch liteassign.Status(r.StartedAt.Valid, fin, r.DueAt, we) {
		case "done":
			s.AssignmentsDone++
		case "done_late":
			s.AssignmentsLate++
		case "overdue":
			s.AssignmentsOverdue++
		}
	}

	stalled, err := q.ListLiteWeekStalled(ctx, sqlc.ListLiteWeekStalledParams{UserIds: ids, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range stalled {
		if s := byID[r.UserID]; s != nil {
			s.Stalled = append(s.Stalled, liteweekly.Item{Kind: r.Kind, Title: r.Title})
		}
	}

	keywords, err := q.ListLiteWeekNewKeywords(ctx, sqlc.ListLiteWeekNewKeywordsParams{UserIds: ids, WeekStart: ws, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range keywords {
		if s := byID[r.UserID]; s != nil {
			s.NewKeywords = append(s.NewKeywords, r.TextZh)
		}
	}

	moments, err := q.ListLiteWeekMoments(ctx, sqlc.ListLiteWeekMomentsParams{UserIds: ids, WeekStart: ws, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range moments {
		s := byID[r.UserID]
		if s == nil || r.ProsePending {
			continue
		}
		ms, err := weekMoments(r.Moments, r.AssignedPrompt)
		if err != nil {
			return nil, fmt.Errorf("lite weekly: report moments for %s: %w", r.UserID, err)
		}
		for _, quote := range ms {
			s.Moments = append(s.Moments, liteweekly.Moment{Quote: quote, ItemTitle: r.Title})
		}
	}
	return out, nil
}

// weekMoments reads a report's stored moments. The quotes were checked against
// her own words when the report was built, so they are taken as-is; an empty
// quote is skipped, and so is any quote that is part of the teacher's assigned
// prompt, which is never her words.
func weekMoments(raw []byte, assignedPrompt *string) ([]string, error) {
	var ms []reportMoment
	if err := json.Unmarshal(raw, &ms); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		q := strings.TrimSpace(m.Quote)
		if q == "" {
			continue
		}
		if assignedPrompt != nil && strings.Contains(*assignedPrompt, q) {
			continue
		}
		out = append(out, m.Quote)
	}
	return out, nil
}
