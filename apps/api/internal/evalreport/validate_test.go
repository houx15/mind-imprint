package evalreport

import "testing"

func TestValidate_RejectsWrongVersion(t *testing.T) {
	_, err := Validate([]byte(`{"version":2,"reportId":"r","projectId":"p"}`))
	if err == nil {
		t.Fatal("expected error for version != 1")
	}
}

func TestValidate_AcceptsMinimalEnvelope(t *testing.T) {
	raw := []byte(`{"version":1,"reportId":"r","projectId":"p","student":{"id":"u","name":"P"},` +
		`"basics":{"title":"t","type":"EE","startDate":"x","endDate":null,` +
		`"milestones":{"started":null,"frameworkFinished":null,"proposalFinished":null,"writingFinished":null,"projectFinished":null},` +
		`"counters":{"aiTurns":0,"materialsRead":0,"wordsWritten":0,"aiCommentCount":0,"editCount":0}},` +
		`"abstract":{"overview":"","materialSentence":"","writingSentence":"","aiSentence":"","suggestionParagraph":"","suggestionSentences":[],"recommendedCourses":[]},` +
		`"events":[],"materials":[],"depth":[],"autonomy":[],"promptLens":{"summary":"","prompts":[]},"toolUsage":[],"risks":[],"generatedAt":"x"}`)
	r, err := Validate(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ReportID != "r" || r.ProjectID != "p" {
		t.Fatalf("ids not parsed: %+v", r)
	}
}
