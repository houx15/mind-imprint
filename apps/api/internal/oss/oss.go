// Package oss signs short-lived presigned Aliyun OSS URLs. It is the only unit
// that holds the OSS AccessKey; the rest of the platform receives only signed
// URLs. Uploads are signed against the OSS origin endpoint; reads are signed
// against the CDN domain so they are cache-accelerated. The bucket is private —
// a presigned read URL carries an OSS signature the CDN forwards to the origin,
// which validates it.
package oss

import (
	"fmt"
	"strings"
	"time"

	alioss "github.com/aliyun/aliyun-oss-go-sdk/oss"

	"mindimprint/api/internal/config"
)

// Service signs presigned upload (PUT) and download (GET) URLs.
type Service struct {
	origin *alioss.Bucket // client bound to the OSS origin endpoint (PUT)
	cdn    *alioss.Bucket // client bound to the CDN domain, UseCname (GET)
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

	return &Service{origin: originBucket, cdn: cdnBucket}, nil
}

// SignUpload returns a presigned PUT URL (OSS origin host) that requires the
// client to send exactly the given Content-Type header — it is bound into the
// signature.
func (s *Service) SignUpload(objectKey, contentType string, ttl time.Duration) (string, error) {
	return s.origin.SignURL(objectKey, alioss.HTTPPut, int64(ttl.Seconds()), alioss.ContentType(contentType))
}

// SignDownload returns a presigned GET URL on the CDN domain.
func (s *Service) SignDownload(objectKey string, ttl time.Duration) (string, error) {
	return s.cdn.SignURL(objectKey, alioss.HTTPGet, int64(ttl.Seconds()))
}

// withScheme ensures the endpoint has an https:// scheme; the SDK accepts a
// bare host but signing/host derivation is unambiguous with an explicit scheme.
func withScheme(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return "https://" + endpoint
}
