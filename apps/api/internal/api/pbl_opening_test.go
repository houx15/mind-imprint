package api_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

// slowProvider 让模型那一步慢下来，好让四个请求都在"线程还是空的"时候建好
// 各自的上下文——那才是真正的竞态窗口。不加这个延迟，先跑完的那个已经提交，
// 后面的请求读到的线程就不空了，它们是**正常的第二轮**，本来就该写进去。
type slowProvider struct {
	inner gateway.Provider
	d     time.Duration
}

func (s slowProvider) Stream(
	ctx context.Context, r gateway.Resolved, req gateway.ChatRequest,
) (<-chan gateway.StreamEvent, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.d):
	}
	return s.inner.Stream(ctx, r, req)
}

// 🚨 开场那一轮只准落库一次。
//
// 前端在"线程是空的"时会把她写的那句话当第一轮发出去，而这一轮要等模型好几十
// 秒。这期间她刷新一下页面、或者 StrictMode 把挂载跑两遍，新的那次看到的线程
// **仍然是空的**（第一轮还没提交），于是又发一遍。2026-09-02 的手机截图上，
// 她那句话出现了三遍，每遍下面跟着一段不一样的回话。
//
// 客户端的闸拦不住这个——跨页面刷新的两次请求互相看不见。拦得住的只有锁里那次
// 复查：进来时线程是空的，拿到锁时已经不空了，这一轮就整个丢掉。
//
// 所以这条测试必须是并发的。串行地发两次，第二次本来就不是"开场"，测不到东西。
func TestPblTurn_ConcurrentOpeningTurnsLandOnce(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, slowProvider{
		inner: pblCoachSaying(`{"reply":"你是在哪儿看到这些剩饭的？"}`),
		d:     300 * time.Millisecond,
	})
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/turn"
	const idea = "我们学校每天剩好多饭"

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pblPost(t, h, cookie, url, `{"text":"`+idea+`"}`)
		}()
	}
	wg.Wait()

	thread := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/thread", "")
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(thread.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("decode thread: %v — %s", err, thread.Body)
	}
	var mine int
	for _, m := range msgs {
		if m.Role == "student" && m.Content == idea {
			mine++
		}
	}
	if mine != 1 {
		t.Fatalf("四个并发的开场轮之后，她那句话在主线里出现了 %d 次，want 1：%+v", mine, msgs)
	}
}
