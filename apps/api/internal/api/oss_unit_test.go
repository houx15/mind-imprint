package api

import "testing"

func TestValidObjectKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"courses/abc.pdf", true},
		{"web/logo.png", true},
		{"users/uid/images/x.png", true},
		{"", false},
		{"other/x.png", false},
		{"courses/../secret", false},
		{"..", false},
		{"/etc/passwd", false},
	}
	for _, c := range cases {
		if got := validObjectKey(c.key); got != c.want {
			t.Errorf("validObjectKey(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestObjectExt(t *testing.T) {
	cases := []struct {
		filename, contentType, want string
	}{
		{"cat.png", "image/png", ".png"},
		{"", "image/png", ".png"},
		{"", "image/jpeg", ".jpg"},
		{"doc.PDF", "application/pdf", ".pdf"},           // sanitized lowercase
		{"weird.name.webp", "image/webp", ".webp"},       // last ext only
		{"no-ext", "image/png", ".png"},                  // fallback to content type
		{"evil.php.long-extension", "image/png", ".png"}, // unsafe ext → fallback
	}
	for _, c := range cases {
		if got := objectExt(c.filename, c.contentType); got != c.want {
			t.Errorf("objectExt(%q,%q) = %q, want %q", c.filename, c.contentType, got, c.want)
		}
	}
}
