package api

// reading_routines_internal_test.go — unit test over the routine library
// itself (unexported `readingRoutines`/`taskConnect`/`taskHunt`), following
// the internal-test convention used by cards_test.go/teacher_read_internal_test.go
// for unexported symbols. reading_plan_test.go is `package api_test` (black
// box, HTTP-level) and cannot see these identifiers, so this TYPE-level
// guarantee lives here instead.

import (
	"strings"
	"testing"
)

// Every routine in the library carries one step that is hers and ends by
// sending her back into the article. This is the TYPE-level guarantee the
// spec rests on: a routine without a connect step is a routine that does not
// exist, which is a stronger promise than any line of prompt.
func TestEveryRoutineHasConnectAndEndsInHunt(t *testing.T) {
	for _, r := range readingRoutines {
		connects := 0
		hunts := 0
		for _, s := range r.Steps {
			switch s.Kind {
			case taskConnect:
				connects++
			case taskHunt:
				hunts++
			}
			if string(s.Kind) == "quiz" {
				t.Fatalf("routine %s still has a quiz step", r.Key)
			}
		}
		if connects != 1 {
			t.Errorf("routine %s: want exactly 1 connect step, got %d", r.Key, connects)
		}
		if hunts != 1 {
			t.Errorf("routine %s: want exactly 1 hunt step, got %d", r.Key, hunts)
		}
		if last := r.Steps[len(r.Steps)-1]; last.Kind != taskHunt {
			t.Errorf("routine %s: want the last step to be a hunt, got %q", r.Key, last.Kind)
		}
		if len(strings.TrimSpace(r.Steps[len(r.Steps)-1].Detail)) == 0 {
			t.Errorf("routine %s: the hunt step must say what to go find", r.Key)
		}
	}
}
