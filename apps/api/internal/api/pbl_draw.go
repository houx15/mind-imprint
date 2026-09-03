package api

// pbl_draw.go — 生成一张图，并且**马上把它变成我们自己的**。
//
// 上游回的那个地址是带签名、会过期的（回包里就写着 Expires，见
// internal/gateway/images.go 的实测记录）。把它存进数据库，等于给她的主页放一张
// 几天后会变成碎图的头图——而那时候没有任何人会知道为什么。
//
// 所以这一层只做一件事，做完整：生成 → 立刻取下来 → 放进我们自己的 OSS →
// 返回一个我们自己的 key。存进库的永远是那个 key。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
)

// maxGeneratedImageBytes 是一张生成图的上限。1024×1024 的 PNG 通常 1–3 MB。
const maxGeneratedImageBytes = 12 << 20

// generatedImageFetchTimeout 是把图取下来的上限。图已经生成好了，这只是一次下载。
const generatedImageFetchTimeout = 60 * time.Second

// drawAndStore 画一张图，存进我们自己的 OSS，返回 object key。
//
// purpose 只进 key 的路径，方便以后按用途清理（"persona" / "hero"）。
func (a *API) drawAndStore(
	ctx context.Context, userID, atomID uuid.UUID, purpose, prompt string,
) (string, error) {
	if a.d.OSS == nil {
		return "", fmt.Errorf("pbl draw: no object storage configured")
	}
	resolved, err := a.routeE(ctx, gateway.ClassDraw)
	if err != nil {
		return "", err
	}
	drawer := a.d.Drawer
	if drawer == nil {
		drawer = gateway.NewHTTPDrawer()
	}
	out, err := drawer.Draw(ctx, resolved, gateway.DrawRequest{Prompt: prompt})
	if err != nil {
		return "", err
	}

	// 🚨 计费：画图不走 ChatUsage（没有 token），但它**要钱**。按一次调用记一行，
	// 不然组织成本汇总里这一块是零，而它其实是这个项目里最贵的一次调用。
	a.recordLiteLLMCall(ctx, userID, atomID, "pbl_draw_"+purpose, resolved, gateway.ChatUsage{})

	blob, err := fetchGeneratedImage(ctx, out.URL)
	if err != nil {
		return "", err
	}
	key, err := generatedImageKey(userID, purpose)
	if err != nil {
		return "", err
	}
	if err := a.d.OSS.PutObject(ctx, key, "image/png", blob); err != nil {
		return "", err
	}
	return key, nil
}

// fetchGeneratedImage 把上游那张图取下来。
func fetchGeneratedImage(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("pbl draw: upstream returned no image url")
	}
	ctx, cancel := context.WithTimeout(ctx, generatedImageFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pbl draw: fetching the generated image returned %d", resp.StatusCode)
	}
	blob, err := io.ReadAll(io.LimitReader(resp.Body, maxGeneratedImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(blob) == 0 {
		return nil, fmt.Errorf("pbl draw: the generated image was empty")
	}
	if len(blob) > maxGeneratedImageBytes {
		return nil, fmt.Errorf("pbl draw: the generated image is larger than %d bytes", maxGeneratedImageBytes)
	}
	return blob, nil
}

// generatedImageKey 造一个不可猜的 object key，挂在她自己名下。
//
// 走 users/<uid>/ 前缀，和她上传的图同一个作用域（oss.go · ossKnownPrefixes），
// 所以 resolve-url 那道闸不用为它开口子。
func generatedImageKey(userID uuid.UUID, purpose string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("users/%s/generated/%s-%s.png",
		userID.String(), purpose, hex.EncodeToString(b[:])), nil
}

// signedOrEmpty 把 object key 变成她那边能显示的地址。签不出来就给空串——
// 一个签失败的地址是一张碎图，而空串至少让界面知道这里还没有图。
func (a *API) signedOrEmpty(key string) string {
	if key == "" || a.d.OSS == nil {
		return ""
	}
	url, err := a.d.OSS.SignDownload(key)
	if err != nil {
		return ""
	}
	return url
}
