package docextract

import (
	"errors"
	"strings"
	"testing"
)

// 一个入口，四种格式。产品负责人 2026-09-12：「make this a general tool」。

func TestAnyAcceptsPlainText(t *testing.T) {
	title, text, err := Any("notes.txt", []byte("第一段。\n\n第二段。"))
	if err != nil {
		t.Fatalf("txt 应该收下：%v", err)
	}
	if title != "" {
		t.Errorf("txt 没有标题可取，拿到 %q", title)
	}
	if !strings.Contains(text, "第二段") {
		t.Errorf("正文没读全：%q", text)
	}
	if _, _, err := Any("notes.md", []byte("# 标题\n\n正文。")); err != nil {
		t.Errorf("md 应该收下：%v", err)
	}
}

func TestAnyRejectsWhatWeDoNotRead(t *testing.T) {
	_, _, err := Any("photo.png", []byte("not a document"))
	if err == nil {
		t.Fatal("png 不该被收下")
	}
	var te *ErrText
	if !errors.As(err, &te) {
		t.Fatalf("失败要带一句能给她看的话，拿到 %T", err)
	}
	if !strings.Contains(te.Msg, ".pdf") {
		t.Errorf("那句话里要说清楚收哪几种：%q", te.Msg)
	}
}

// 🚨 扫描件和「文件坏了」要分开说。
//
// 一律说「这个文件没能解析出正文」的话，她会以为文件坏了，于是反复换文件试 ——
// 而扫描件换多少次都一样：它本来就没有文字层。
func TestScannedPdfSaysWhySpecifically(t *testing.T) {
	// 一个能解析、但一个字都取不出来的 PDF：最小合法骨架，没有文字流。
	empty := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type/Pages/Kids[]/Count 0>>endobj\n" +
		"trailer<</Root 1 0 R>>\n%%EOF\n")
	_, _, err := Any("scan.pdf", empty)
	if err == nil {
		t.Fatal("一个字都取不出来的 PDF 不该算成功")
	}
	var te *ErrText
	if !errors.As(err, &te) {
		t.Fatalf("失败要带一句能给她看的话，拿到 %T", err)
	}
	// 不管它是「解析不动」还是「解析得动但没有文字」，给她的那一句都要提到
	// 「粘进来」这条退路 —— 她当场就能往下走。
	if !strings.Contains(te.Msg, "粘进来") {
		t.Errorf("那句话没给她退路：%q", te.Msg)
	}
}

func TestAnyRejectsEmpty(t *testing.T) {
	if _, _, err := Any("a.txt", nil); err == nil {
		t.Fatal("空文件不该被收下")
	}
}

func TestIsSupportedMatchesTheList(t *testing.T) {
	for _, ok := range []string{"a.pdf", "A.PDF", "b.docx", "c.txt", "d.md"} {
		if !IsSupported(ok) {
			t.Errorf("%q 应该在收的范围里", ok)
		}
	}
	for _, no := range []string{"a.doc", "b.pages", "c", "d.png"} {
		if IsSupported(no) {
			t.Errorf("%q 不该被收下", no)
		}
	}
}
