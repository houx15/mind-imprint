package docextract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DOCX extracts readable plain text from a .docx (Office Open XML) using only
// the standard library. Body paragraphs (<w:p>) become blocks; tables (<w:tbl>)
// are rendered as one cohesive block with each row's cells joined by " | "
// (mirroring the HTML table handling in materialize/extract.go). title comes
// from docProps/core.xml's <dc:title>, else the first non-empty paragraph.
func DOCX(b []byte) (title, text string, err error) {
	if len(b) == 0 {
		return "", "", fmt.Errorf("docextract: empty docx")
	}
	zr, zerr := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if zerr != nil {
		return "", "", fmt.Errorf("docextract: open docx: %w", zerr)
	}
	var docFile, coreFile *zip.File
	for _, f := range zr.File {
		switch f.Name {
		case "word/document.xml":
			docFile = f
		case "docProps/core.xml":
			coreFile = f
		}
	}
	if docFile == nil {
		return "", "", fmt.Errorf("docextract: not a docx (missing word/document.xml)")
	}

	body, berr := extractDocumentXML(docFile)
	if berr != nil {
		return "", "", berr
	}
	title = docxCoreTitle(coreFile)
	if title == "" {
		title = firstNonEmptyLine(body)
	}
	return title, normalizeText(body), nil
}

// extractDocumentXML token-streams word/document.xml, emitting body paragraphs
// as blocks and tables as pipe-joined rows. Paragraphs inside table cells are
// folded into their cell text, not emitted as standalone blocks.
func extractDocumentXML(f *zip.File) (string, error) {
	rc, oerr := f.Open()
	if oerr != nil {
		return "", fmt.Errorf("docextract: read document.xml: %w", oerr)
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var blocks []string

	var para strings.Builder // current body <w:p> text
	// Table state (depth-counted for the rare nested table).
	inTable := 0
	inCell := false
	var tableLines []string
	var rowCells []string
	var cell strings.Builder

	for {
		tok, terr := dec.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return "", fmt.Errorf("docextract: parse document.xml: %w", terr)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tbl":
				inTable++
				tableLines = nil
			case "tr":
				if inTable > 0 {
					rowCells = nil
				}
			case "tc":
				if inTable > 0 {
					inCell = true
					cell.Reset()
				}
			case "tab":
				if inCell {
					cell.WriteString(" ")
				} else if inTable == 0 {
					para.WriteString(" ")
				}
			}
		case xml.CharData:
			// Text nodes: <w:t> content arrives as CharData between start/end.
			if inCell {
				cell.Write(t)
			} else if inTable == 0 {
				para.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inTable == 0 && !inCell {
					if s := strings.TrimSpace(para.String()); s != "" {
						blocks = append(blocks, s)
					}
					para.Reset()
				} else if inCell {
					cell.WriteString(" ") // separate multi-paragraph cells
				}
			case "tc":
				if inTable > 0 {
					rowCells = append(rowCells, strings.TrimSpace(cell.String()))
					inCell = false
				}
			case "tr":
				if inTable > 0 {
					if line := strings.Trim(strings.Join(rowCells, " | "), " |"); line != "" {
						tableLines = append(tableLines, strings.Join(rowCells, " | "))
					}
				}
			case "tbl":
				if inTable > 0 {
					inTable--
					if len(tableLines) > 0 {
						blocks = append(blocks, strings.Join(tableLines, "\n"))
					}
				}
			}
		}
	}
	return strings.Join(blocks, "\n\n"), nil
}

// docxCoreTitle reads docProps/core.xml's dc:title, "" when absent/unreadable.
func docxCoreTitle(f *zip.File) string {
	if f == nil {
		return ""
	}
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		tok, terr := dec.Token()
		if terr != nil {
			return ""
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "title" {
			var s string
			if dec.DecodeElement(&s, &se) == nil {
				return strings.TrimSpace(s)
			}
			return ""
		}
	}
}

func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}
