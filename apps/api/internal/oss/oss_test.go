package oss

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/config"
)

// TestServerSideMethodsSignatures is a compile-time/reference assertion that
// *Service exposes PutObject and Exists with the signatures the
// course-audio pipeline (later tasks) depends on. It never calls them: doing
// so would hit the real Aliyun SDK over the network, which this package
// intentionally does not fake (see live_test.go for the real round-trip,
// gated behind the "live" build tag and real credentials).
func TestServerSideMethodsSignatures(t *testing.T) {
	var _ func(ctx context.Context, key, contentType string, data []byte) error = (*Service)(nil).PutObject
	var _ func(ctx context.Context, key string) (bool, error) = (*Service)(nil).Exists
}

func testCfg() config.Config {
	return config.Config{
		OSSEndpoint:     "mind-imprint.oss-cn-beijing.aliyuncs.com",
		OSSBucket:       "mind-imprint",
		OSSCDNDomain:    "mind-oss.uni-robot.cn",
		OSSAccessKeyID:  "AK-test",
		OSSAccessSecret: "SK-test",
	}
}

func TestNewDisabledWhenNoKey(t *testing.T) {
	svc, err := New(config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Fatalf("expected nil service when unconfigured, got %v", svc)
	}
}

func TestNewErrorsWhenKeySetButEndpointsMissing(t *testing.T) {
	cfg := config.Config{OSSAccessKeyID: "AK", OSSAccessSecret: "SK"} // no endpoint/cdn/bucket
	if _, err := New(cfg); err == nil {
		t.Fatalf("expected error when key set but endpoints missing")
	}
}

func TestSignUploadTargetsOriginHost(t *testing.T) {
	svc, err := New(testCfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := svc.SignUpload("users/u1/images/abc.png", "image/png", 10*time.Minute)
	if err != nil {
		t.Fatalf("SignUpload: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "mind-imprint.oss-cn-beijing.aliyuncs.com" {
		t.Fatalf("upload host: want OSS origin, got %q", u.Host)
	}
	if !strings.Contains(u.Path, "users/u1/images/abc.png") {
		t.Fatalf("upload path missing object key: %q", u.Path)
	}
	q := u.Query()
	for _, k := range []string{"OSSAccessKeyId", "Expires", "Signature"} {
		if q.Get(k) == "" {
			t.Fatalf("upload URL missing query param %q: %s", k, raw)
		}
	}
}

// TestSignDownloadTargetsCDNHost covers the presigned-fallback branch (no
// OSSCDNAuthKey configured), which SignDownload must keep unchanged.
func TestSignDownloadTargetsCDNHost(t *testing.T) {
	svc, err := New(testCfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := svc.SignDownload("courses/abc.pdf")
	if err != nil {
		t.Fatalf("SignDownload: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "mind-oss.uni-robot.cn" {
		t.Fatalf("download host: want CDN domain, got %q", u.Host)
	}
	if !strings.Contains(u.Path, "courses/abc.pdf") {
		t.Fatalf("download path missing object key: %q", u.Path)
	}
	if u.Query().Get("Signature") == "" {
		t.Fatalf("download URL missing Signature: %s", raw)
	}
}

func TestSignTypeA_KnownVector(t *testing.T) {
	const domain = "mind-oss.uni-robot.cn"
	const key = "courses/x/assets/videos/case.mp4"
	const priv = "testprivatekey"
	var ts int64 = 1_700_000_000

	got := signTypeA(domain, priv, key, ts)

	uri := "/" + key
	want := md5.Sum([]byte(fmt.Sprintf("%s-%d-%s-%s-%s", uri, ts, "0", "0", priv)))
	wantHex := hex.EncodeToString(want[:])
	wantURL := fmt.Sprintf("https://%s%s?auth_key=%d-0-0-%s", domain, uri, ts, wantHex)
	if got != wantURL {
		t.Fatalf("signTypeA = %q, want %q", got, wantURL)
	}
	if !strings.Contains(got, "?auth_key=1700000000-0-0-") {
		t.Fatalf("auth_key shape wrong: %q", got)
	}
}

func TestSignDownload_AuthKeyBranch(t *testing.T) {
	s := NewSigner("mind-oss.uni-robot.cn", "k", 2*time.Hour)
	url, err := s.SignDownload("courses/x/a.png")
	if err != nil {
		t.Fatalf("SignDownload: %v", err)
	}
	if !strings.Contains(url, "auth_key=") || strings.Contains(url, "Signature=") {
		t.Fatalf("expected URL鉴权 link, got %q", url)
	}
	if s.DownloadWindow() != 2*time.Hour {
		t.Fatalf("DownloadWindow = %v, want 2h", s.DownloadWindow())
	}
}
