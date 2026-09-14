package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

// TestLiteItemDetailNeverReturnsChat is spec DEC-1's guard: teachers see
// outputs and moments, never the chat with 印记. No code path in this
// endpoint may read atom_message.content into the response.
func TestLiteItemDetailNeverReturnsChat(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	seedAtomMessage(t, pool, atom, "student", "SECRET-CHAT-LINE-7731")
	seedAtomMessage(t, pool, atom, "ai", "SECRET-AI-LINE-7731")
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO atom_annotation (atom_id, block_id, span, quote, note) VALUES ($1, 'b1', '{}', 'article words', 'her note')`, atom); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("item = %d body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "SECRET-CHAT-LINE-7731") || strings.Contains(body, "SECRET-AI-LINE-7731") {
		t.Fatalf("chat content leaked: %s", body)
	}
	if !strings.Contains(body, "her note") {
		t.Fatalf("highlight note missing: %s", body)
	}
}

func TestLiteItemDetailAtomOfAnotherStudent404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "lt-item-stranger@demo.local")
	atom := seedLiteReadingForUser(t, pool, stranger, "active", 0)
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil); code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", code)
	}
}

func TestLiteItemDetailWritingDraft(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteWritingForUser(t, pool, studentID, "active")
	if _, err := pool.Exec(context.Background(), `INSERT INTO writing_draft (atom_id, body) VALUES ($1, 'HER-DRAFT-BODY')`, atom); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Writing struct {
			Draft string `json:"draft"`
		} `json:"writing"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), &resp); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if resp.Writing.Draft != "HER-DRAFT-BODY" {
		t.Fatalf("draft = %q", resp.Writing.Draft)
	}
}

func TestLiteItemDetailDoesNotTouchActivity(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	var before, after string
	_ = pool.QueryRow(context.Background(), `SELECT last_activity_at::text FROM atom WHERE id=$1`, atom).Scan(&before)
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil)
	_ = pool.QueryRow(context.Background(), `SELECT last_activity_at::text FROM atom WHERE id=$1`, atom).Scan(&after)
	if before != after {
		t.Fatalf("teacher read bumped last_activity_at %s → %s", before, after)
	}
}
