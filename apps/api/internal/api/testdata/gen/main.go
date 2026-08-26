// Command gen writes the tiny real .docx and .pdf fixtures used by
// reading_source_file_test.go into apps/api/internal/api/testdata/. Run once
// with `go run ./internal/api/testdata/gen` from apps/api whenever the
// fixtures need to be regenerated; the output is checked in, so this
// generator is not part of the normal build or test run.
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
)

func main() {
	if err := os.WriteFile("internal/api/testdata/reading.docx", buildDOCX(), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile("internal/api/testdata/reading.pdf", buildPDF(), 0o644); err != nil {
		panic(err)
	}
	fmt.Println("wrote internal/api/testdata/reading.docx and reading.pdf")
}

// buildDOCX zips a minimal but real Office Open XML wordprocessing document:
// two paragraphs (so SplitBlocks yields more than one block) plus a
// docProps/core.xml title.
func buildDOCX() []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	docXML := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>候鸟每年秋天离开北方,飞往更温暖的南方过冬。</w:t></w:r></w:p>
<w:p><w:r><w:t>科学家用卫星定位项圈追踪候鸟的迁徙路线,发现它们能精确地回到同一片湖泊。</w:t></w:r></w:p>
</w:body>
</w:document>`
	coreXML := `<?xml version="1.0"?><cp:coreProperties xmlns:cp="x" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>候鸟迁徙</dc:title></cp:coreProperties>`

	mustWrite(zw, "word/document.xml", docXML)
	mustWrite(zw, "docProps/core.xml", coreXML)
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func mustWrite(zw *zip.Writer, name, body string) {
	w, err := zw.Create(name)
	if err != nil {
		panic(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		panic(err)
	}
}

// buildPDF hand-assembles a minimal single-page PDF with a real text layer
// ("Migratory birds leave the north every autumn.") using Helvetica and no
// custom encoding, so ledongthuc/pdf's default byteEncoder(pdfDocEncoding)
// round-trips the ASCII content exactly. Byte offsets for the xref table are
// computed from the actual object bytes, not hand-counted, so the file is a
// structurally valid PDF.
func buildPDF() []byte {
	content := "BT /F1 18 Tf 20 150 Td (Migratory birds leave the north every autumn.) Tj ET"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /MediaBox [0 0 300 200] /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, body := range objs {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objs)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objs)+1, xrefStart)
	return buf.Bytes()
}
