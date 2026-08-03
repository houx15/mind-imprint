package oss

import "testing"

// TestPutObjectOptionsContentTypeDefault is a pure, offline test of the
// option-building branch used by PutObject. It never touches the network —
// the Aliyun SDK bucket itself needs a real OSS endpoint to PUT/Head against,
// which is exercised by live_test.go (build tag "live") instead.
func TestPutObjectOptionsContentTypeDefault(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		wantLen     int
	}{
		{name: "empty content type omits the option", contentType: "", wantLen: 0},
		{name: "given content type produces exactly one option", contentType: "audio/mpeg", wantLen: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := putObjectOptions(tc.contentType)
			if len(got) != tc.wantLen {
				t.Fatalf("putObjectOptions(%q): got %d options, want %d", tc.contentType, len(got), tc.wantLen)
			}
		})
	}
}
