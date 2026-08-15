package docextract

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildDOCX zips the given part contents into a .docx-shaped archive.
func buildDOCX(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

const docWithTable = `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>第一段介绍这项研究。</w:t></w:r></w:p>
<w:tbl>
  <w:tr>
    <w:tc><w:p><w:r><w:t>Outlet</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>Year</w:t></w:r></w:p></w:tc>
  </w:tr>
  <w:tr>
    <w:tc><w:p><w:r><w:t>Xinhua</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>2021</w:t></w:r></w:p></w:tc>
  </w:tr>
</w:tbl>
<w:p><w:r><w:t>最后是结论段。</w:t></w:r></w:p>
</w:body>
</w:document>`

func TestDOCX_ParagraphsAndTable(t *testing.T) {
	b := buildDOCX(t, map[string]string{
		"word/document.xml": docWithTable,
		"docProps/core.xml": `<?xml version="1.0"?><cp:coreProperties xmlns:cp="x" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>研究报告</dc:title></cp:coreProperties>`,
	})
	title, text, err := DOCX(b)
	if err != nil {
		t.Fatalf("DOCX: %v", err)
	}
	if title != "研究报告" {
		t.Fatalf("title = %q, want 研究报告", title)
	}
	for _, want := range []string{"第一段介绍这项研究。", "Outlet | Year", "Xinhua | 2021", "最后是结论段。"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q\n--- got ---\n%s", want, text)
		}
	}
	// Table cells must NOT leak out as standalone body paragraphs; the row is one
	// line, and the two body paragraphs bracket the table.
	if strings.Index(text, "第一段介绍这项研究。") > strings.Index(text, "Outlet | Year") {
		t.Fatalf("first paragraph should precede the table:\n%s", text)
	}
	if strings.Index(text, "Outlet | Year") > strings.Index(text, "最后是结论段。") {
		t.Fatalf("table should precede the last paragraph:\n%s", text)
	}
}

func TestDOCX_TitleFallsBackToFirstLine(t *testing.T) {
	b := buildDOCX(t, map[string]string{
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>开头这一句。</w:t></w:r></w:p></w:body></w:document>`,
	})
	title, _, err := DOCX(b)
	if err != nil {
		t.Fatalf("DOCX: %v", err)
	}
	if title != "开头这一句。" {
		t.Fatalf("title fallback = %q, want 开头这一句。", title)
	}
}

func TestDOCX_RejectsNonDocx(t *testing.T) {
	if _, _, err := DOCX([]byte("not a zip at all")); err == nil {
		t.Fatal("want error for non-zip bytes, got nil")
	}
	// A zip without word/document.xml is not a docx.
	b := buildDOCX(t, map[string]string{"hello.txt": "hi"})
	if _, _, err := DOCX(b); err == nil {
		t.Fatal("want error for zip lacking word/document.xml, got nil")
	}
}

func TestPDF_GarbageDoesNotPanic(t *testing.T) {
	// Malformed input must degrade to an error, never panic (the deferred
	// recover in PDF converts a library panic into an error).
	if _, _, err := PDF([]byte("%PDF-1.4 not really a pdf")); err == nil {
		t.Log("garbage pdf returned no error (empty text is acceptable) — as long as it did not panic")
	}
	if _, _, err := PDF(nil); err == nil {
		t.Fatal("want error for empty pdf, got nil")
	}
}
