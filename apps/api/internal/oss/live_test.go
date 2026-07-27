//go:build live

// Live round-trip against real Aliyun OSS. Excluded from normal `go test` by the
// `live` build tag; run explicitly with real credentials in the environment:
//
//	OSS_ENDPOINT=... OSS_BUCKET=... OSS_CDN_DOMAIN=... \
//	OSS_ACCESS_KEY_ID=... OSS_ACCESS_KEY_SECRET=... \
//	go test -tags live -run TestLiveRoundTrip -v ./internal/oss
package oss_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/oss"
)

func TestLiveRoundTrip(t *testing.T) {
	cfg := config.Config{
		OSSEndpoint:     os.Getenv("OSS_ENDPOINT"),
		OSSBucket:       os.Getenv("OSS_BUCKET"),
		OSSCDNDomain:    os.Getenv("OSS_CDN_DOMAIN"),
		OSSAccessKeyID:  os.Getenv("OSS_ACCESS_KEY_ID"),
		OSSAccessSecret: os.Getenv("OSS_ACCESS_KEY_SECRET"),
	}
	if cfg.OSSAccessKeyID == "" {
		t.Skip("no OSS_ACCESS_KEY_ID in env; skipping live round-trip")
	}
	svc, err := oss.New(cfg)
	if err != nil {
		t.Fatalf("oss.New: %v", err)
	}

	key := fmt.Sprintf("web/livetest-%d.txt", time.Now().UnixNano())
	content := []byte("mind-imprint oss live test " + time.Now().Format(time.RFC3339Nano))

	// --- upload leg: presigned PUT to the OSS origin ---
	putURL, err := svc.SignUpload(key, "text/plain", 10*time.Minute)
	if err != nil {
		t.Fatalf("SignUpload: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPut, putURL, strings.NewReader(string(content)))
	req.Header.Set("Content-Type", "text/plain")
	putRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT request: %v", err)
	}
	defer putRes.Body.Close()
	putBody, _ := io.ReadAll(putRes.Body)
	if putRes.StatusCode != http.StatusOK {
		t.Fatalf("upload leg FAILED: PUT %d\n%s", putRes.StatusCode, putBody)
	}
	t.Logf("upload leg OK: PUT %d for %s", putRes.StatusCode, key)

	// --- download leg: presigned GET via the CDN domain ---
	getURL, err := svc.SignDownload(key, 5*time.Minute)
	if err != nil {
		t.Fatalf("SignDownload: %v", err)
	}
	getRes, err := http.Get(getURL)
	if err != nil {
		t.Fatalf("download leg FAILED (CDN unreachable — DNS/CDN not provisioned?): %v", err)
	}
	defer getRes.Body.Close()
	gotBody, _ := io.ReadAll(getRes.Body)
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("download leg FAILED: GET %d via CDN\n%s", getRes.StatusCode, gotBody)
	}
	if string(gotBody) != string(content) {
		t.Fatalf("download leg content mismatch: want %q got %q", content, gotBody)
	}
	t.Logf("download leg OK: GET %d via CDN, %d bytes round-tripped", getRes.StatusCode, len(gotBody))
}
