package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/oss"
	"mindimprint/api/internal/store/sqlc"
)

type trialDrawer func(context.Context, gateway.Resolved, gateway.DrawRequest) (gateway.DrawResult, error)

func (f trialDrawer) Draw(ctx context.Context, r gateway.Resolved, q gateway.DrawRequest) (gateway.DrawResult, error) {
	return f(ctx, r, q)
}

func TestPblDrawValidatesAndStoresActualImageFormat(t *testing.T) {
	var pngBytes, jpegBytes bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&pngBytes, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegBytes, img, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name            string
		body            []byte
		mime, extension string
		drawError       bool
	}{
		{"png", pngBytes.Bytes(), "image/png", "png", false},
		{"jpeg", jpegBytes.Bytes(), "image/jpeg", "jpg", false},
		{"error page", []byte("<html>generation failed</html>"), "", "", false},
		{"truncated pixels", pngBytes.Bytes()[:40], "", "", false},
		{"provider failure", nil, "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := NewTestDB(t)
			uploads := 0
			contentType := ""
			var uploaded []byte
			storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uploads++
				contentType = r.Header.Get("Content-Type")
				uploaded, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			}))
			defer storage.Close()
			svc, err := oss.New(config.Config{OSSEndpoint: storage.URL, OSSCDNDomain: strings.TrimPrefix(storage.URL, "http://"), OSSBucket: "test", OSSAccessKeyID: "test", OSSAccessSecret: "test"})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(tc.body)
			}))
			defer upstream.Close()
			calls := 0
			a := &API{d: Deps{Pool: pool, Queries: sqlc.New(pool), OSS: svc, Route: func(class string) gateway.KeyResolver {
				if class != gateway.ClassDraw {
					t.Fatalf("wrong class %s", class)
				}
				return func(context.Context) (gateway.Resolved, error) {
					return gateway.Resolved{Provider: "dashscope_image", Model: "qwen-image-3.0", Tier: "chaperone"}, nil
				}
			}, Drawer: trialDrawer(func(context.Context, gateway.Resolved, gateway.DrawRequest) (gateway.DrawResult, error) {
				calls++
				if tc.drawError {
					return gateway.DrawResult{}, errors.New("provider unavailable")
				}
				return gateway.DrawResult{URL: upstream.URL}, nil
			})}}
			key, err := a.drawAndStore(t.Context(), SeedUserID, uuid.Nil, "hero", "test scene")
			valid := tc.mime != ""
			if valid {
				if err != nil || !strings.HasSuffix(key, "."+tc.extension) || contentType != tc.mime || !bytes.Equal(uploaded, tc.body) || uploads != 1 {
					t.Fatalf("key=%s type=%s uploads=%d err=%v", key, contentType, uploads, err)
				}
			} else if err == nil || key != "" || uploads != 0 {
				t.Fatalf("invalid image stored: key=%s uploads=%d err=%v", key, uploads, err)
			}
			var counted int
			if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM llm_call WHERE user_id=$1 AND purpose='pbl_draw_hero'`, SeedUserID).Scan(&counted); err != nil || counted != 1 || calls != 1 {
				t.Fatalf("draw attempt not recorded: calls=%d rows=%d err=%v", calls, counted, err)
			}
		})
	}
}

func TestPblGeneratedImageRejectsHugeDimensionsBeforeDecode(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	blob := buf.Bytes()
	binary.BigEndian.PutUint32(blob[16:20], 8193)
	binary.BigEndian.PutUint32(blob[29:33], crc32.ChecksumIEEE(blob[12:29]))
	if _, _, err := generatedImageFormat(blob); err == nil || !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("oversized image not rejected before pixel decode: %v", err)
	}
}

func TestPblFetchGeneratedImageHonorsFailureAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer server.Close()
	if _, err := fetchGeneratedImage(t.Context(), server.URL); err == nil {
		t.Fatal("HTTP error accepted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	defer cancel()
	if _, err := fetchGeneratedImage(ctx, server.URL); err == nil {
		t.Fatal("cancelled download accepted")
	}
}
