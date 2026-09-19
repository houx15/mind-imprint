package api

import (
	"errors"
	"testing"

	"mindimprint/api/internal/interest"
)

// 选词读不懂就再问一次 —— 而「再问一次」和「挑出来零个」必须分得开。
//
// 🚨 2026-09-19 线上实测：选词那一次回了
// `invalid character ',' after object key`，于是零词；报告因此对一个写了八段
// 具体经历的学生说「你写下的内容里还没有足够具体的原话可以作为根据」。
// 这一条守着两件事：读不懂要重来，以及「读懂了但零个」不能被当成故障。
func TestSelectionRetriesOnceThenTellsTheTruth(t *testing.T) {
	broken := errors.New("harvest reply is not the expected object")

	t.Run("第一次读不懂，第二次读懂了", func(t *testing.T) {
		calls := 0
		hs, ok := retryHarvest(awakeningSelectAttempts, func(int) ([]interest.Harvested, error) {
			calls++
			if calls == 1 {
				return nil, broken
			}
			return []interest.Harvested{{InterestID: "ocean", Evidence: "她的原话"}}, nil
		})
		if !ok {
			t.Fatal("第二次读懂了，却报成失败")
		}
		if calls != 2 {
			t.Errorf("打了 %d 次，want 2", calls)
		}
		if len(hs) != 1 {
			t.Errorf("挑出 %d 个，want 1", len(hs))
		}
	})

	t.Run("两次都读不懂就认账，不假装她没写够", func(t *testing.T) {
		calls := 0
		_, ok := retryHarvest(awakeningSelectAttempts, func(int) ([]interest.Harvested, error) {
			calls++
			return nil, broken
		})
		if ok {
			t.Fatal("两次都失败了，却报成成功")
		}
		if calls != awakeningSelectAttempts {
			t.Errorf("打了 %d 次，want %d", calls, awakeningSelectAttempts)
		}
	})

	t.Run("一次就读懂，不多打", func(t *testing.T) {
		calls := 0
		_, ok := retryHarvest(awakeningSelectAttempts, func(int) ([]interest.Harvested, error) {
			calls++
			return []interest.Harvested{{InterestID: "ocean"}}, nil
		})
		if !ok || calls != 1 {
			t.Errorf("ok=%v calls=%d, want true/1", ok, calls)
		}
	})

	// 🚨 这一格是那个 bug 的核心：读懂了但一个都没挑出来，是**正常结果**。
	// 判成故障的话，报告会反过来对真的没写够的学生说「关键词分析失败」。
	t.Run("读懂了但零个，算成功", func(t *testing.T) {
		hs, ok := retryHarvest(awakeningSelectAttempts, func(int) ([]interest.Harvested, error) {
			return nil, nil
		})
		if !ok {
			t.Fatal("零结果被当成了失败")
		}
		if len(hs) != 0 {
			t.Errorf("挑出 %d 个，want 0", len(hs))
		}
	})
}
