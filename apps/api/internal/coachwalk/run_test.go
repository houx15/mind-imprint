package coachwalk

import (
	"context"
	"errors"
	"testing"

	"mindimprint/api/internal/gateway"
)

// 这组测试守的是「走查照生产的样子重试」。它错了的样子很隐蔽：一次生产会自己
// 救回来的失败被记成拦住上线的理由，或者反过来，一次学生真看到的 502 被当成
// 救回来了。

// fakeDriver 的 Parse 按脚本决定每一次读回复的结果。
type fakeDriver struct {
	parses []error // 第 i 次 Parse 返回的错误；nil 表示读得动
	calls  int
}

func (f *fakeDriver) Site() string                 { return "fake" }
func (f *fakeDriver) Request() gateway.ChatRequest { return gateway.ChatRequest{} }
func (f *fakeDriver) Advance(string, string)       {}
func (f *fakeDriver) HerWords() string             { return "" }
func (f *fakeDriver) Persona() string              { return "" }
func (f *fakeDriver) Screen(r string) string       { return r }
func (f *fakeDriver) Parse(raw string) (string, []Violation, error) {
	i := f.calls
	f.calls++
	if i < len(f.parses) && f.parses[i] != nil {
		return "", nil, f.parses[i]
	}
	return "你打算怎么量？", nil, nil
}

func constCall(ms int64) Call {
	return func(context.Context, gateway.ChatRequest) (CallResult, error) {
		return CallResult{Text: "{}", Ms: ms, Out: 10}, nil
	}
}

func TestRunRetriesOnceWhenProductionWouldAndCountsTheWait(t *testing.T) {
	d := &fakeDriver{parses: []error{ErrRetry}} // 第一次读不动，第二次读得动
	l, err := Run(context.Background(), d, constCall(700), constCall(1), 1)
	if err != nil {
		t.Fatal(err)
	}
	turn := l.Turns[0]
	if turn.ParseErr != "" {
		t.Fatalf("重试成功的一轮不该记失败：%q", turn.ParseErr)
	}
	if turn.Retries != 1 {
		t.Errorf("Retries = %d, want 1", turn.Retries)
	}
	// 重试她看不见，但她等得到：两次调用的时间都要算进去。
	if turn.CoachMs != 1400 {
		t.Errorf("CoachMs = %d, want 1400（两次各 700）", turn.CoachMs)
	}
	if turn.OutTokens != 20 {
		t.Errorf("OutTokens = %d, want 20", turn.OutTokens)
	}
}

func TestRunRecordsA502WhenTheRetryFailsToo(t *testing.T) {
	d := &fakeDriver{parses: []error{ErrRetry, ErrRetry}}
	l, _ := Run(context.Background(), d, constCall(1), constCall(1), 3)
	if len(l.Turns) != 1 {
		t.Fatalf("两次都读不动时走查应当停在这一轮，实际走了 %d 轮", len(l.Turns))
	}
	if l.Turns[0].ParseErr == "" || l.Count("parse") != 1 {
		t.Fatalf("两次都读不动是学生看得见的 502，必须记一条 parse：%+v", l.Turns[0])
	}
}

func TestRunDoesNotRetryWhatProductionDoesNotRetry(t *testing.T) {
	// 没包 ErrRetry 的错误（比如写作室的校验器拒收）生产直接回 502，不重试。
	d := &fakeDriver{parses: []error{errors.New("banned phrasing")}}
	l, _ := Run(context.Background(), d, constCall(1), constCall(1), 1)
	if l.Turns[0].Retries != 0 {
		t.Errorf("生产不重试的错误，走查也不该重试")
	}
	if l.Count("parse") != 1 {
		t.Errorf("这一轮学生拿不到回复，应当记一条失败")
	}
}

func TestLatencyUsesRealObservedWaits(t *testing.T) {
	l := &Log{Turns: []Turn{
		{CoachMs: 100}, {CoachMs: 200}, {CoachMs: 300}, {CoachMs: 400}, {CoachMs: 1000, Retries: 1},
	}}
	lat := LatencyOf([]*Log{l, nil})
	if lat.N != 5 || lat.P50 != 300 || lat.P90 != 1000 || lat.Max != 1000 || lat.Retries != 1 {
		t.Fatalf("latency = %+v", lat)
	}
	if got := LatencyOf(nil); got.N != 0 || got.P50 != 0 {
		t.Fatalf("空输入应当是零值：%+v", got)
	}
}
