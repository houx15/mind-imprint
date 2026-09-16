package pbl

import "testing"

func TestPageCopyCannotExistOnlyInsideScript(t *testing.T) {
	page := SiteContent{About: []string{"My own words"}}
	for _, doc := range []string{"<html><body><script>My own words</script></body></html>", "<html><body>Different words</body></html>"} {
		if ValidatePageCopy(doc, page) == nil {
			t.Fatal("missing rendered copy accepted")
		}
	}
	if err := ValidatePageCopy("<html><body><p>My <b>own</b> words</p></body></html>", page); err != nil {
		t.Fatal(err)
	}
}
func TestProcessRecordPreservesStudentText(t *testing.T) {
	record := ProcessRecordSection("Keep the flowers.\nMake them larger.", "Keyboard works; touch untested.")
	if record.Body != "修改意见\nKeep the flowers.\nMake them larger.\n\n试用判断\nKeyboard works; touch untested." {
		t.Fatal(record.Body)
	}
}
