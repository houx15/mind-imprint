// Package liteassign holds the pure rules of a lite teacher assignment:
// how a recipient's status is derived and what a valid payload looks like.
package liteassign

import "time"

// Status derives a recipient's status. It is never stored: the inputs are
// the recipient's started flag, the item's finish time, the deadline and now.
func Status(started bool, finishedAt *time.Time, dueAt, now time.Time) string {
	if finishedAt != nil {
		if finishedAt.After(dueAt) {
			return "done_late"
		}
		return "done"
	}
	if now.After(dueAt) {
		return "overdue"
	}
	if started {
		return "in_progress"
	}
	return "not_started"
}

var labels = map[string]string{
	"not_started": "未开始",
	"in_progress": "进行中",
	"done":        "已完成",
	"done_late":   "逾期完成",
	"overdue":     "已逾期",
}

// StatusLabel is the UI word for a wire status.
func StatusLabel(status string) string { return labels[status] }
