package oss_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/oss"
)

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
	svc, err := oss.New(config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Fatalf("expected nil service when unconfigured, got %v", svc)
	}
}

func TestNewErrorsWhenKeySetButEndpointsMissing(t *testing.T) {
	cfg := config.Config{OSSAccessKeyID: "AK", OSSAccessSecret: "SK"} // no endpoint/cdn/bucket
	if _, err := oss.New(cfg); err == nil {
		t.Fatalf("expected error when key set but endpoints missing")
	}
}

func TestSignUploadTargetsOriginHost(t *testing.T) {
	svc, err := oss.New(testCfg())
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

func TestSignDownloadTargetsCDNHost(t *testing.T) {
	svc, err := oss.New(testCfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	raw, err := svc.SignDownload("courses/abc.pdf", 5*time.Minute)
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
