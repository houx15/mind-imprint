// Package oss signs short-lived presigned Aliyun OSS URLs. It is the only unit
// that holds the OSS AccessKey; the rest of the platform receives only signed
// URLs. Uploads are signed against the OSS origin endpoint; reads are signed
// against the CDN domain so they are cache-accelerated. The bucket is private —
// a presigned read URL carries an OSS signature the CDN forwards to the origin,
// which validates it.
package oss

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	alioss "github.com/aliyun/aliyun-oss-go-sdk/oss"

	"mindimprint/api/internal/config"
)

// Service signs presigned upload (PUT) and download (GET) URLs.
type Service struct {
	origin        *alioss.Bucket // client bound to the OSS origin endpoint (PUT)
	cdn           *alioss.Bucket // client bound to the CDN domain, UseCname (GET)
	cdnDomain     string         // custom CDN host, no scheme (e.g. mind-oss.uni-robot.cn)
	cdnAuthKey    string         // URL鉴权 Type A 主KEY; "" ⇒ presigned fallback
	cdnAuthWindow time.Duration  // mirror of console 验证时长
}

// New returns a Service, or nil (not an error) when OSS is unconfigured — file
// storage is optional and the platform must still boot without it. It errors
// only when a key is present but the endpoints/bucket are misconfigured.
func New(cfg config.Config) (*Service, error) {
	if cfg.OSSAccessKeyID == "" || cfg.OSSAccessSecret == "" {
		return nil, nil // disabled
	}
	if cfg.OSSEndpoint == "" || cfg.OSSCDNDomain == "" || cfg.OSSBucket == "" {
		return nil, fmt.Errorf("oss: OSS_ACCESS_KEY_ID set but OSS_ENDPOINT/OSS_CDN_DOMAIN/OSS_BUCKET missing")
	}

	// UseCname treats the given endpoint as the final host (no bucket prefix),
	// while the signature still binds the bucket in its canonicalized resource.
	// That lets us sign against the bucket's origin host for uploads and the
	// custom CDN domain for reads with the same code path.
	originClient, err := alioss.New(withScheme(cfg.OSSEndpoint), cfg.OSSAccessKeyID, cfg.OSSAccessSecret, alioss.UseCname(true))
	if err != nil {
		return nil, fmt.Errorf("oss: origin client: %w", err)
	}
	originBucket, err := originClient.Bucket(cfg.OSSBucket)
	if err != nil {
		return nil, fmt.Errorf("oss: origin bucket: %w", err)
	}

	cdnClient, err := alioss.New(withScheme(cfg.OSSCDNDomain), cfg.OSSAccessKeyID, cfg.OSSAccessSecret, alioss.UseCname(true))
	if err != nil {
		return nil, fmt.Errorf("oss: cdn client: %w", err)
	}
	cdnBucket, err := cdnClient.Bucket(cfg.OSSBucket)
	if err != nil {
		return nil, fmt.Errorf("oss: cdn bucket: %w", err)
	}

	window := time.Duration(cfg.OSSCDNAuthWindow) * time.Second
	return &Service{
		origin:        originBucket,
		cdn:           cdnBucket,
		cdnDomain:     cfg.OSSCDNDomain,
		cdnAuthKey:    cfg.OSSCDNAuthKey,
		cdnAuthWindow: window,
	}, nil
}

// SignUpload returns a presigned PUT URL (OSS origin host) that requires the
// client to send exactly the given Content-Type header — it is bound into the
// signature.
func (s *Service) SignUpload(objectKey, contentType string, ttl time.Duration) (string, error) {
	return s.origin.SignURL(objectKey, alioss.HTTPPut, int64(ttl.Seconds()), alioss.ContentType(contentType))
}

// presignFallbackTTL bounds the OSS presigned GET used before URL鉴权 is
// configured (cdnAuthKey == ""). Generous enough for large audio/video reads.
const presignFallbackTTL = 15 * time.Minute

// NewSigner builds a Service that only signs URL鉴权 download links (no OSS
// origin/CDN clients). Used by tests and any caller that needs signing without
// network access; SignUpload/PutObject/GetObject/Exists must not be called on it.
func NewSigner(cdnDomain, cdnAuthKey string, window time.Duration) *Service {
	return &Service{cdnDomain: cdnDomain, cdnAuthKey: cdnAuthKey, cdnAuthWindow: window}
}

// signTypeA builds an Aliyun CDN URL鉴权 Type A link. Pure (ts is injected) so it
// is deterministic and unit-testable. objectKey is assumed path-safe ASCII
// (uuid/kebab/sanitized-ext by construction); the md5 is computed over the
// decoded URI, matching what the edge recomputes.
func signTypeA(cdnDomain, privateKey, objectKey string, ts int64) string {
	uri := "/" + objectKey
	const rand, uid = "0", "0"
	sum := md5.Sum([]byte(fmt.Sprintf("%s-%d-%s-%s-%s", uri, ts, rand, uid, privateKey)))
	authKey := fmt.Sprintf("%d-%s-%s-%s", ts, rand, uid, hex.EncodeToString(sum[:]))
	return fmt.Sprintf("%s%s?auth_key=%s", withScheme(cdnDomain), uri, authKey)
}

// SignDownload returns a cacheable CDN read URL. With a URL鉴权 主KEY configured it
// emits a Type A auth_key link (auth_key is excluded from the CDN cache key, so
// reads cache). Without one it falls back to the legacy OSS presigned GET, so
// this code can ship before the console is cut over to URL鉴权.
func (s *Service) SignDownload(objectKey string) (string, error) {
	if s.cdnAuthKey != "" {
		return signTypeA(s.cdnDomain, s.cdnAuthKey, objectKey, time.Now().Unix()), nil
	}
	return s.cdn.SignURL(objectKey, alioss.HTTPGet, int64(presignFallbackTTL.Seconds()))
}

// DownloadWindow is how long a SignDownload URL stays valid: the URL鉴权 window
// when configured, else the presigned fallback TTL. Callers use it to report
// expiresAt and schedule refresh.
func (s *Service) DownloadWindow() time.Duration {
	if s.cdnAuthKey != "" && s.cdnAuthWindow > 0 {
		return s.cdnAuthWindow
	}
	return presignFallbackTTL
}

// PutObject uploads data to the OSS origin under objectKey. It is used by
// server-side jobs (e.g. pre-generating course audio) that hold the data in
// memory rather than asking a client to PUT via a presigned URL. ctx is
// accepted for signature consistency with callers even though the underlying
// SDK call is synchronous.
func (s *Service) PutObject(ctx context.Context, key, contentType string, data []byte) error {
	if err := s.origin.PutObject(key, bytes.NewReader(data), putObjectOptions(contentType)...); err != nil {
		return fmt.Errorf("oss: put object %q: %w", key, err)
	}
	return nil
}

// GetObject downloads an object's bytes from the OSS origin bucket. Used
// server-side to read an uploaded document (PDF/DOCX) for text extraction.
func (s *Service) GetObject(ctx context.Context, key string) ([]byte, error) {
	rc, err := s.origin.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("oss: get object %q: %w", key, err)
	}
	defer func() { _ = rc.Close() }()
	data, rerr := io.ReadAll(rc)
	if rerr != nil {
		return nil, fmt.Errorf("oss: read object %q: %w", key, rerr)
	}
	return data, nil
}

// putObjectOptions builds the alioss.Option slice for PutObject, omitting the
// Content-Type option when contentType is empty. Factored out as a pure
// helper so the branch is unit-testable without a network call.
func putObjectOptions(contentType string) []alioss.Option {
	if contentType == "" {
		return nil
	}
	return []alioss.Option{alioss.ContentType(contentType)}
}

// Exists reports whether objectKey is already present in the bucket. It is
// used to make audio generation idempotent — skip re-synthesizing/uploading
// a piece whose object already exists. ctx is accepted for signature
// consistency with callers even though the underlying SDK call is
// synchronous.
func (s *Service) Exists(ctx context.Context, key string) (bool, error) {
	ok, err := s.origin.IsObjectExist(key)
	if err != nil {
		return false, fmt.Errorf("oss: exists %q: %w", key, err)
	}
	return ok, nil
}

// withScheme ensures the endpoint has an https:// scheme; the SDK accepts a
// bare host but signing/host derivation is unambiguous with an explicit scheme.
func withScheme(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return "https://" + endpoint
}
