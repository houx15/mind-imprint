package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// 上游抖一下，不该让她把刚才那句话再打一遍。
//
// # 这条是怎么被发现的
//
// 2026-09-21「真学生走查」（`apps/lite-web/e2e/real-student-walk.spec.ts`）第二趟，
// 规划的第 5 轮回了 **HTTP 502 `model_unavailable`**。产品那一侧做得是对的 ——
// 屏幕上是「后台错误：AI 响应错误（model_unavailable）」，没有编一句话糊弄她
// （[[ai-errors-must-surface-never-fake]]）。可她那一轮说的话就这么没了，
// 要自己再打一遍。
//
// 查下去发现一处**反着的**不对称：`writing_plan.go` 里本来就有一次重试，
// 但它挂在**解析失败**那一支（模型答了，JSON 读不出来）；而**调用本身失败**
// 那一支（`cerr != nil`）是直接报错返回的。两者里更像一阵风的恰恰是后者。
// 那段解析重试的理由，原样适用于这里，而且更强 ——
//
//	「这一轮的钱已经花掉了，直接报错等于让她白等一次，
//	  还得自己把刚才那句话再说一遍。」
//
// 传输层挂掉的时候，我们常常连账都还没记上。
//
// # 为什么写在网关里，而不是各个 handler 里
//
// `model_unavailable` 在 `internal/api` 下有二十来个调用点，形状一模一样。
// 各写各的，等于这条规矩有二十份实现、二十个走样的机会，而下一个人还得把它
// 重新发现一遍。网关是所有模型调用的唯一入口，判「这个错该不该再试一次」
// 需要的东西（HTTP 状态码、是不是传输层、ctx 有没有被取消）也只有这里齐全。
//
// # 🚨 只重试真的会自己好的那几类
//
// 「重试」不是「再试一次总没坏处」。分类写窄，是因为每一类写宽都有代价：
//
//   - **4xx（除 429）不重试。** 400 是我们自己把请求拼错了，401 是 key 不对，
//     404 是模型 id 写错了。这些再试一百次也是同一个答案，只是把一个
//     **我们的 bug** 拖成一句「有时候会失败」—— 而那正是最难查的那种。
//     `models.json` 写错 id 要的是**启动即失败**，不是悄悄重试一周。
//   - **ctx 取消 / 超时不重试。** 她关掉页面、或者这一轮已经超时了，
//     再打一次是纯烧钱，而且没有人在等那个答案了。
//   - **能力不匹配不重试**（「这个模型不会画图」「关不掉思考」）——
//     那是目录的事实，不是一阵风。
//   - **一次，不是三次。** 她正同步等着。第二次还坏就老实报错 ——
//     同 `writing_plan.go` 那条解析重试的判断。
//
// # 流式的那一路故意不在这里
//
// 只有 `Collect`（非流式）走这条路。SSE 那一路一旦开始吐字节，重试就意味着
// 她屏幕上那半句话要被另一份回复覆盖掉；而且真正要解析 JSON 的调用早就
// 不走流了（见 complete.go 的文件头）。

// upstreamError 带着上游这一次到底怎么坏的。
//
// 它仍然 `Unwrap` 到 errStreamFailed，所以既有的
// `errors.Is(err, errStreamFailed)` 一处都不用改。
//
// 🚨 `Status` 只在真的收到了一个 HTTP 响应时才有意义；连不上的时候是 0，
// 用 `Transport` 区分 —— 两者都该重试，但把「连不上」和「上游回了 500」
// 记成同一件事，日志里就分不出是我们这边的网络还是他们那边的服务。
type upstreamError struct {
	Provider  string
	Status    int  // 上游的 HTTP 状态码；连不上时为 0
	Transport bool // 连接层就没成（拨号、读 body、超时）
	err       error
}

func (e *upstreamError) Error() string {
	if e.Transport {
		return fmt.Sprintf("%v: %s transport: %v", errStreamFailed, e.Provider, e.err)
	}
	return fmt.Sprintf("%v: %s http %d: %v", errStreamFailed, e.Provider, e.Status, e.err)
}

// Unwrap 交出**两条**链：errStreamFailed 和上游那一次的真实错因。
//
// 🚨 两条都要，而且这正是 `TestRetryable_CtxDeadlineWrappedInTransportErrorWins`
// 抓到的那个 bug：一开始这里只交出 errStreamFailed，于是一个裹着
// `context.DeadlineExceeded` 的传输层错误，`errors.Is(err, context.DeadlineExceeded)`
// 看不穿 —— `Retryable` 只看见「传输层」就判了重试，而那时候已经没有人在等答案了。
//
//   - errStreamFailed：既有的 `errors.Is(err, errStreamFailed)` 一处都不用改。
//   - e.err：ctx 那两种收场看得穿，判得出「这不是上游抖了，是她走了」。
func (e *upstreamError) Unwrap() []error { return []error{errStreamFailed, e.err} }

// newUpstreamHTTPError 上游回了一个非 200。
func newUpstreamHTTPError(provider string, status int, detail string) error {
	return &upstreamError{Provider: provider, Status: status, err: errors.New(detail)}
}

// newUpstreamTransportError 连接层就没成。
func newUpstreamTransportError(provider string, err error) error {
	return &upstreamError{Provider: provider, Transport: true, err: err}
}

// retryableStatus 哪些状态码值得再试一次。
//
// 429 收在里面是因为它说的是「现在太挤」，不是「你这个请求不对」——
// 隔一下再来常常就过了。其余 4xx 一律不重试，理由见文件头。
func retryableStatus(status int) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	return status >= 500 && status <= 599
}

// Retryable 判这个错该不该再试一次。
//
// 导出是为了让调用点也能问同一个问题（而不是各自照着状态码再写一遍），
// 并且让测试能直接钉住这张表。
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	// 🚨 ctx 的两种收场都不重试，而且要**先**判 —— 一个因为 ctx 超时而
	// 失败的传输层错误，同时满足下面那一条，判反了就会在没有人等的时候
	// 再烧一次钱。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// 通道不支持非流式不是失败，是 Collect 自己的分支信号。
	if errors.Is(err, errNotCompletable) {
		return false
	}
	var up *upstreamError
	if errors.As(err, &up) {
		if up.Transport {
			return true
		}
		return retryableStatus(up.Status)
	}
	// 认不出来的就不重试。能力不匹配、拼请求拼错了都落在这里 ——
	// 它们再试一次还是同一个答案。
	return false
}
