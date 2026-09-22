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
	// maxBodyBytes 是我们愿意读进内存的 HTML 上限。
	//
	// 🚨 2026-09-22 实测：2 MiB 太小，而且失败的样子是整篇文章抓不到。
	// `https://en.wikipedia.org/wiki/Artificial_intelligence` 直接
	// too_large —— 一个中学生会粘的、最普通不过的链接。现在的新闻页面
	// 光内联脚本就能到几 MiB，HTML 的大小和正文的长度早就不是一回事了。
	//
	// 8 MiB 装得下实测过的每一个页面，同时仍然挡得住把一整个站点塞进
	// 一个响应里的那种东西。真正的正文长度由下游的 bodyRuneCap 管。
	maxBodyBytes = 8 << 20 // 8 MiB
	maxRedirects = 3
)

// 🚨 抓回来一个字都没有，不是「成功」。
//
// 2026-09-22 实测：`mp.weixin.qq.com` 那类页面返回 200、Content-Type 是
// text/html，而抽出来的正文是**空字符串**。当时这算成功，于是空正文一路流到
// 调用点，她看到的是「先把文章正文放进来。」—— 一句在说她没粘东西的话，
// 而她粘的是一个链接。
//
// 这里只判「一个字都没有」这个客观事实。「抓到的太短、不像一篇文章」是产品
// 判断，归调用点（reading_source.go 的 readableEnoughToRead）—— 抽取器的
// 单测用的是三行的样例页，把那条产品线画在这里会把它们一起判死。

// FetchError carries a stable machine reason for a failed fetch. For a DOI whose
// body couldn't be fetched, Meta carries the Crossref metadata we DID recover
// (#4) — so the caller can still show the student the title/authors/abstract and
// ask her to paste the full text.
type FetchError struct {
	Reason string
	Err    error
	Meta   *DOIMeta
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

// browserUA mimics a real browser so publisher/CDN bot-walls (a common source of
// 403 "取不到正文") don't reject us outright, while still self-identifying (#4).
const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 MindImprint/1.0"

// setBrowserHeaders sends a realistic UA + Accept headers. Many sites 403 the
// default Go/bot UA or serve a stub without an Accept header.
func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
}

// FetchReadable fetches rawURL and returns an extracted title + text + any DOI
// metadata Crossref resolved (#4 — nil when the URL isn't a DOI or resolution
// failed), or a *FetchError (which, for a DOI, also carries the recovered
// Crossref metadata on its .Meta so a failed fetch can still surface it). The
// success-path meta lets the caller persist abstract/journal/author/year even
// when the full text WAS fetched, not just on the 422 fallback.
func (f *HTTPFetcher) FetchReadable(ctx context.Context, rawURL string) (title string, text string, meta *DOIMeta, err error) {
	u, perr := url.Parse(rawURL)
	if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", nil, &FetchError{Reason: "blocked", Err: errors.New("unsupported url")}
	}
	// #4 · if this is a DOI, resolve it to the publisher's landing page + full
	// metadata via Crossref first (best-effort — failure leaves the original
	// URL). Fetch the landing page directly: more often real HTML than the
	// doi.org redirect chain, and the title/abstract are recovered even if body
	// extraction fails.
	fetchURL, fallbackTitle := rawURL, ""
	var doiMeta *DOIMeta
	if doi, ok := extractDOI(u); ok {
		doiMeta = f.resolveDOI(ctx, doi)
		if doiMeta != nil {
			fallbackTitle = doiMeta.Title
			// Only adopt a resolved URL that is a well-formed http(s) URL — never
			// let Crossref's response steer us to a non-web scheme (defense-in-depth;
			// the SSRF guard already blocks private IPs at dial).
			if ru, uerr := url.Parse(doiMeta.URL); uerr == nil && (ru.Scheme == "http" || ru.Scheme == "https") && ru.Host != "" {
				fetchURL = doiMeta.URL
			}
		}
	}
	// On any FetchError, attach the DOI metadata we recovered so the caller can
	// still show it and invite a paste of the full text.
	defer func() {
		if err != nil && doiMeta != nil {
			var fe *FetchError
			if errors.As(err, &fe) {
				fe.Meta = doiMeta
			}
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return "", "", nil, &FetchError{Reason: "blocked", Err: err}
	}
	setBrowserHeaders(req)
	resp, err := f.client.Do(req)
	if err != nil {
		var fe *FetchError
		if errors.As(err, &fe) {
			return "", "", nil, fe
		}
		return "", "", nil, &FetchError{Reason: "unreachable", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", nil, &FetchError{Reason: "bad_status", Err: fmt.Errorf("status %d", resp.StatusCode)}
	}
	ct := resp.Header.Get("Content-Type")
	isHTML := strings.HasPrefix(ct, "text/html")
	isText := strings.HasPrefix(ct, "text/plain")
	if !isHTML && !isText {
		return "", "", nil, &FetchError{Reason: "unsupported_content", Err: fmt.Errorf("content-type %q", ct)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return "", "", nil, &FetchError{Reason: "unreachable", Err: err}
	}
	if len(body) > maxBodyBytes {
		return "", "", nil, &FetchError{Reason: "too_large", Err: nil}
	}
	if isText {
		if strings.TrimSpace(string(body)) == "" {
			return "", "", nil, &FetchError{Reason: "no_text", Err: errors.New("the page came back with no readable body")}
		}
		return fallbackTitle, string(body), doiMeta, nil
	}
	title, text = extractHTML(body)
	if title == "" {
		title = fallbackTitle
	}
	// 200 + text/html 不等于抓到了正文：见上面那段注释。
	if strings.TrimSpace(text) == "" {
		return "", "", nil, &FetchError{Reason: "no_text", Err: errors.New("the page came back with no readable body")}
	}
	return title, text, doiMeta, nil
}
