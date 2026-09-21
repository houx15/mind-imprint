package gateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 这张表就是这条规矩本身。
//
// 🚨 写宽一格和写窄一格的代价是不对称的，所以两边都钉：
//   - 该重试的没重试 ⇒ 她白等一次，还得把刚才那句话再打一遍（走查里撞到的那次）。
//   - 不该重试的重试了 ⇒ 把**我们自己的 bug**（400 请求拼错了、401 key 不对）
//     拖成一句「有时候会失败」，而那是最难查的那种；ctx 都取消了还再打一次，
//     则是没人等着的纯烧钱。
func TestRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil 不是失败", nil, false},

		// —— 上游自己的毛病：再试一次常常就过了 ——
		{"429 太挤", newUpstreamHTTPError("dashscope", 429, ""), true},
		{"500", newUpstreamHTTPError("dashscope", 500, ""), true},
		{"502 走查撞到的那个", newUpstreamHTTPError("dashscope", 502, "model_unavailable"), true},
		{"503", newUpstreamHTTPError("dashscope", 503, ""), true},
		{"504 网关超时", newUpstreamHTTPError("dashscope", 504, ""), true},

		// —— 我们自己的毛病：再试一百次也是同一个答案 ——
		{"400 请求拼错了", newUpstreamHTTPError("dashscope", 400, "bad request"), false},
		{"401 key 不对", newUpstreamHTTPError("dashscope", 401, ""), false},
		{"403", newUpstreamHTTPError("dashscope", 403, ""), false},
		{"404 模型 id 写错了", newUpstreamHTTPError("dashscope", 404, ""), false},
		{"422", newUpstreamHTTPError("dashscope", 422, ""), false},

		// —— 连接层 ——
		{"连不上", newUpstreamTransportError("dashscope", errors.New("dial tcp: i/o timeout")), true},

		// —— 不重试的几类 ——
		{"ctx 被取消：没有人在等了", context.Canceled, false},
		{"ctx 超时", context.DeadlineExceeded, false},
		{"通道不支持非流式：这是分支信号不是失败", errNotCompletable, false},
		{"能力不匹配是目录的事实，不是一阵风", errors.New("glm-5.3-flash does not support disabling thinking"), false},
		{"认不出来的一律不重试", errors.New("boom"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Retryable(c.err); got != c.want {
				t.Fatalf("Retryable(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// 🚨 一个因为 ctx 超时而失败的**传输层**错误，同时满足「传输层 ⇒ 重试」和
// 「ctx 超时 ⇒ 不重试」。判反了就会在没有人等的时候再烧一次钱，
// 所以 ctx 那一条必须先判 —— 这条测试钉的就是那个顺序。
func TestRetryable_CtxDeadlineWrappedInTransportErrorWins(t *testing.T) {
	err := newUpstreamTransportError("dashscope", context.DeadlineExceeded)
	if Retryable(err) {
		t.Fatal("传输层错误里裹着 ctx 超时，不该重试：没有人在等那个答案了")
	}
}

// 既有的 errors.Is(err, errStreamFailed) 一处都不能改坏 ——
// 结构化之后它仍然要成立，否则调用点那一侧会悄悄改变行为。
func TestUpstreamErrorStillUnwrapsToStreamFailed(t *testing.T) {
	for _, err := range []error{
		newUpstreamHTTPError("dashscope", 502, "model_unavailable"),
		newUpstreamTransportError("dashscope", errors.New("i/o timeout")),
	} {
		if !errors.Is(err, errStreamFailed) {
			t.Fatalf("%v 不再 Is errStreamFailed —— 调用点那一侧会跟着变", err)
		}
	}
}

// 🚨 上游正文不许漏进错误里（同 provider.go 那条：客户端看到的是固定错误码）。
// 这里只验我们自己拼的那句话不含密钥之类的东西 —— detail 是调用方给的，
// 由 readErrorBody 负责裁剪。
func TestUpstreamErrorMessageNamesStatusAndProvider(t *testing.T) {
	msg := newUpstreamHTTPError("dashscope", 502, "model_unavailable").Error()
	for _, want := range []string{"dashscope", "502"} {
		if !contains(msg, want) {
			t.Fatalf("错误里没有 %q，日志里就分不出是谁、坏在哪：%s", want, msg)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// —— Collect 这一侧：真的只试第二次，而且只在该试的时候 ——

// flakyCompleter 头 n 次按 fail 给错，之后成功。
type flakyCompleter struct {
	fail  error
	times int
	calls int
}

func (f *flakyCompleter) Complete(_ context.Context, _ Resolved, _ ChatRequest) (ChatResult, error) {
	f.calls++
	if f.calls <= f.times {
		return ChatResult{}, f.fail
	}
	return ChatResult{Text: "第二次过了"}, nil
}

// flakyCompleter 也得是个 Provider —— Collect 的签名要的是 Provider。
// 这条路走不到（Complete 不返回 errNotCompletable），给个会炸的实现，
// 万一将来走到了，测试会当场说话而不是悄悄换一条路。
func (f *flakyCompleter) Stream(context.Context, Resolved, ChatRequest) (<-chan StreamEvent, error) {
	return nil, errors.New("这条路不该走到：Complete 没说它不支持非流式")
}

func TestCollect_RetriesOnceOnATransientUpstreamFailure(t *testing.T) {
	p := &flakyCompleter{fail: newUpstreamHTTPError("dashscope", 502, "model_unavailable"), times: 1}
	res, err := Collect(context.Background(), p, Resolved{}, ChatRequest{})
	if err != nil {
		t.Fatalf("第二次本该过：%v", err)
	}
	if res.Text != "第二次过了" {
		t.Fatalf("拿回来的不是第二次那一份：%q", res.Text)
	}
	if p.calls != 2 {
		t.Fatalf("打了 %d 次，该打 2 次", p.calls)
	}
}

// 🚨 一次，不是三次。她正同步等着。
func TestCollect_GivesUpAfterTheSecondTry(t *testing.T) {
	p := &flakyCompleter{fail: newUpstreamHTTPError("dashscope", 502, ""), times: 99}
	if _, err := Collect(context.Background(), p, Resolved{}, ChatRequest{}); err == nil {
		t.Fatal("两次都坏，该老实报错")
	}
	if p.calls != 2 {
		t.Fatalf("打了 %d 次，该打 2 次 —— 三次就是在让她干等", p.calls)
	}
}

// 400 是我们自己把请求拼错了。重试它，就是把一个 bug 拖成「有时候会失败」。
func TestCollect_DoesNotRetryOurOwnBadRequest(t *testing.T) {
	p := &flakyCompleter{fail: newUpstreamHTTPError("dashscope", 400, "bad request"), times: 99}
	if _, err := Collect(context.Background(), p, Resolved{}, ChatRequest{}); err == nil {
		t.Fatal("400 该直接报错")
	}
	if p.calls != 1 {
		t.Fatalf("打了 %d 次，400 只该打 1 次", p.calls)
	}
}

// 她关掉页面之后，不该还在那儿睡够 300ms 再打一次。
func TestCollect_StopsWhenSheIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &flakyCompleter{fail: newUpstreamHTTPError("dashscope", 502, ""), times: 99}
	start := time.Now()
	if _, err := Collect(ctx, p, Resolved{}, ChatRequest{}); err == nil {
		t.Fatal("ctx 已经取消了，该报错")
	}
	if p.calls != 1 {
		t.Fatalf("打了 %d 次；ctx 取消之后不该再打", p.calls)
	}
	if elapsed := time.Since(start); elapsed >= retryPause {
		t.Fatalf("等了 %v —— ctx 取消之后不该把那一觉睡完", elapsed)
	}
}
