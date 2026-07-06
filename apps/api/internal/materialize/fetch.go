package materialize

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	fetchTimeout = 8 * time.Second
	maxBodyBytes = 2 << 20 // 2 MiB
	maxRedirects = 3
)

// FetchError carries a stable machine reason for a failed fetch.
type FetchError struct {
	Reason string
	Err    error
}

func (e *FetchError) Error() string {
	if e.Err != nil {
		return "material fetch failed (" + e.Reason + "): " + e.Err.Error()
	}
	return "material fetch failed (" + e.Reason + ")"
}
func (e *FetchError) Unwrap() error { return e.Err }

// HTTPFetcher fetches and extracts readable text. Zero value is not usable; use a constructor.
type HTTPFetcher struct{ client *http.Client }

// NewFetcher returns the production, SSRF-guarded fetcher.
func NewFetcher() *HTTPFetcher { return newFetcher(true) }

// newUnguardedFetcher (test-only) skips the IP guard so httptest loopback servers are reachable.
func newUnguardedFetcher() *HTTPFetcher { return newFetcher(false) }

// newGuardedFetcher is an alias for NewFetcher used for test readability.
func newGuardedFetcher() *HTTPFetcher { return NewFetcher() }

func newFetcher(guard bool) *HTTPFetcher {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, &FetchError{Reason: "unreachable", Err: err}
			}
			if guard {
				for _, ip := range ips {
					if isBlockedIP(ip) {
						return nil, &FetchError{Reason: "blocked", Err: fmt.Errorf("blocked ip %s", ip)}
					}
				}
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, &FetchError{Reason: "unreachable", Err: lastErr}
		},
	}
	c := &http.Client{
		Timeout:   fetchTimeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return &FetchError{Reason: "unreachable", Err: errors.New("too many redirects")}
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return &FetchError{Reason: "blocked", Err: errors.New("bad redirect scheme")}
			}
			return nil
		},
	}
	return &HTTPFetcher{client: c}
}

// FetchReadable fetches rawURL and returns an extracted title + text, or a *FetchError.
func (f *HTTPFetcher) FetchReadable(ctx context.Context, rawURL string) (string, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", &FetchError{Reason: "blocked", Err: errors.New("unsupported url")}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", &FetchError{Reason: "blocked", Err: err}
	}
	req.Header.Set("User-Agent", "MindImprintBot/1.0 (+material)")
	resp, err := f.client.Do(req)
	if err != nil {
		var fe *FetchError
		if errors.As(err, &fe) {
			return "", "", fe
		}
		return "", "", &FetchError{Reason: "unreachable", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", &FetchError{Reason: "bad_status", Err: fmt.Errorf("status %d", resp.StatusCode)}
	}
	ct := resp.Header.Get("Content-Type")
	isHTML := strings.HasPrefix(ct, "text/html")
	isText := strings.HasPrefix(ct, "text/plain")
	if !isHTML && !isText {
		return "", "", &FetchError{Reason: "unsupported_content", Err: fmt.Errorf("content-type %q", ct)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return "", "", &FetchError{Reason: "unreachable", Err: err}
	}
	if len(body) > maxBodyBytes {
		return "", "", &FetchError{Reason: "too_large", Err: nil}
	}
	if isText {
		return "", string(body), nil
	}
	title, text := extractHTML(body)
	return title, text, nil
}
