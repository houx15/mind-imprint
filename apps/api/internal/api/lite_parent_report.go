package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

// loadLiteParentFacts gathers the facts of one student's parent report over
// the Beijing dates start..end, both included (start and end are Beijing
// midnights, as liteparent.ParseRange returns them).
//
// Before reading 金句 it makes sure every reading and writing finished in the
// range has a stored report, running phase 1 only (ensureAtomReportWith with
// allowProse=false): no model call. A report created here still owes its
// prose, so its moments are absent from these facts (plan 4 Ruling 2). An
// ensure error is logged and that item's moments are skipped.
//
// The facts never carry chat text, writing.assigned_prompt or an assigned
// project's idea.
func (a *API) loadLiteParentFacts(ctx context.Context, classID, userID, teacherID uuid.UUID, start, end time.Time) (liteparent.Facts, error) {
	rs := start.In(liteweek.Beijing)
	re := end.In(liteweek.Beijing).AddDate(0, 0, 1) // exclusive bound
	q := a.d.Queries

	cls, err := q.GetClassByID(ctx, classID)
	if err != nil {
		return liteparent.Facts{}, err
	}
	student, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return liteparent.Facts{}, err
	}
	teacher, err := q.GetUserByID(ctx, teacherID)
	if err != nil {
		return liteparent.Facts{}, err
	}

	f := liteparent.Facts{
		StudentName: student.DisplayName,
		ClassName:   cls.Name,
		TeacherName: teacher.DisplayName,
		RangeStart:  rs.Format("2006-01-02"),
		RangeEnd:    end.In(liteweek.Beijing).Format("2006-01-02"),
		Days:        liteparent.RangeDays(rs, end.In(liteweek.Beijing)),
		Minutes:     -1,
		Readings:    []liteparent.Item{},
		Writings:    []liteparent.Item{},
		Projects:    []liteparent.Item{},
		Moments:     []liteparent.Moment{},
		Keywords:    []liteparent.Keyword{},
	}

	activity, err := q.ParentRangeActivity(ctx, sqlc.ParentRangeActivityParams{
		UserID: userID, StartDay: pgDate(rs), EndDay: pgDate(re), RangeStart: rs, RangeEnd: re,
	})
	if err != nil {
		return liteparent.Facts{}, err
	}
	f.ActiveDays = int(activity.ActiveDays)
	f.Turns = int(activity.Turns)
	if activity.HasBucketInRange {
		f.Minutes = int(activity.Seconds) / 60
	}

	finished, err := q.ParentRangeFinished(ctx, sqlc.ParentRangeFinishedParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, err
	}
	noMoments := map[uuid.UUID]bool{}
	for _, r := range finished {
		it := liteparent.Item{Kind: r.Kind, Title: r.Title, FinishedAt: r.FinishedAt.In(liteweek.Beijing).Format("2006-01-02")}
		switch r.Kind {
		case "reading":
			f.Readings = append(f.Readings, it)
		case "writing":
			f.Writings = append(f.Writings, it)
		default:
			f.Projects = append(f.Projects, it)
			continue
		}
		if _, _, err := a.ensureAtomReportWith(ctx, userID, r.AtomID, r.Kind, false); err != nil {
			slog.Warn("lite parent report: ensure atom report failed, moments skipped", "err", err, "atom_id", r.AtomID, "kind", r.Kind)
			noMoments[r.AtomID] = true
		}
	}

	moments, err := q.ParentRangeMoments(ctx, sqlc.ParentRangeMomentsParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, err
	}
	for _, r := range moments {
		if r.ProsePending || noMoments[r.AtomID] {
			continue
		}
		ms, err := weekMoments(r.Moments, r.AssignedPrompt)
		if err != nil {
			slog.Warn("lite parent report: unreadable report moments, skipped", "err", err, "atom_id", r.AtomID)
			continue
		}
		for _, quote := range ms {
			f.Moments = append(f.Moments, liteparent.Moment{Quote: quote, ItemTitle: r.Title})
		}
	}

	states, err := q.ParentRangeAssignmentStates(ctx, sqlc.ParentRangeAssignmentStatesParams{
		ClassID: classID, UserID: userID, RangeStart: rs, RangeEnd: re,
	})
	if err != nil {
		return liteparent.Facts{}, err
	}
	for _, r := range states {
		// Work finished after the range ended was still overdue at its end.
		fin := tsPtr(r.FinishedAt)
		if fin != nil && !fin.Before(re) {
			fin = nil
		}
		f.AssignmentsTotal++
		switch liteassign.Status(r.StartedAt.Valid, fin, r.DueAt, re) {
		case "done":
			f.AssignmentsOnTime++
		case "done_late":
			f.AssignmentsLate++
		case "overdue":
			f.AssignmentsMissed++
		}
	}

	keywords, err := q.ParentRangeKeywords(ctx, sqlc.ParentRangeKeywordsParams{UserID: userID, RangeStart: rs, RangeEnd: re})
	if err != nil {
		return liteparent.Facts{}, err
	}
	for _, r := range keywords {
		f.Keywords = append(f.Keywords, liteparent.Keyword{Text: r.TextZh, Field: r.Field, FieldLabel: disciplines.FieldLabels[r.Field]})
	}
	return f, nil
}

// loadLiteParentOtherNames returns the display names of the class's other
// current students, for the prose name check.
func (a *API) loadLiteParentOtherNames(ctx context.Context, classID, userID uuid.UUID, selfName string) ([]string, error) {
	members, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return nil, err
	}
	return liteParentOtherNames(members, userID, selfName), nil
}

// liteParentOtherNames is every member except her. A blank name is skipped,
// and so is a classmate whose name is part of hers (her exact name, or 王丽
// for 王丽华): naming her would otherwise fail every attempt. A name inside a
// verified 《title》 or 「quote」 needs no skip here; CheckProse removes those
// spans before the name check.
func liteParentOtherNames(members []sqlc.ListLiteWeekClassStudentsRow, userID uuid.UUID, selfName string) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if m.ID == userID || strings.TrimSpace(m.DisplayName) == "" || strings.Contains(selfName, m.DisplayName) {
			continue
		}
		out = append(out, m.DisplayName)
	}
	return out
}
