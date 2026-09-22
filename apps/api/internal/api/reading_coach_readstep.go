package api

import (
	"strconv"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// reading_coach_readstep.go —— 通读那一步给 done 要有真凭据。
//
// 产品负责人 2026-09-22 报的第 4 条，附截图。她问了一句关于内容的问题：
//
//	那和选项多导致我大脑累有啥关系
//
// 印记答完那个问题，接着说「第17-20段你读过了，我们先往下走。」—— 而她那几段
// 根本还没读。一句提问被记成了一步通读。
//
// 这是[[hardcoded-thresholds-vs-user-set-scale-2026-09-12]]那条的又一次现场：
// 提示词里写着「学生明确说这几段读完了，或已回答本步的通读卡片，就给 done」，
// 而代码里**没有任何东西**在查这句软话。模型一高兴就把步走了，清单和她的真实
// 进度从此各说各的，而清单是过程评估读的那一份（铁律④）。
//
// 所以判据搬进代码：通读这一步的 done 必须有下面三样之一。
// 一样都没有就留在原地——她那几段确实还没读。
//
// 🚨 这不会把她钉住：耗满六轮那条替她推进的规则（reading_coach.go 里
// coachStepStalled 那一段）排在这条后面，照常生效。这条只拦「没凭据的当场
// 判完成」，不拦「走不动了替她往下走」。

// readStepDoneEvidence —— 这一轮有没有「她真的走完了这一部分」的凭据。
//
// 三样，任一即可：
//
//  1. 她答了一张卡（通读那一步的检验就是这张卡，答了就是走完了）；
//  2. 她自己说这几段读完了；
//  3. 印记 在这一轮已经把她领去了下一个部分（「接下来读第 12 到 15 段」）——
//     那是 alignAdvanceWithReply 认的同一个信号，话她看得见，以话为准。
func readStepDoneEvidence(answer *coachCardAnswer, studentText, reply string, tasks []sqlc.ReadingTask) bool {
	if answer != nil {
		return true
	}
	if studentSaysSheFinishedReading(studentText) {
		return true
	}
	return replyDirectsToNextReadStep(reply, tasks)
}

// guardReadStepAdvance 把没有凭据的那个 done 拿掉。
//
// 只管 read 这一种步，只管 done —— skipped 照放行（她明说跳过是她的权利），
// 别的步各有自己的判据（hunt 查真选句、label 查真摆板）。
func guardReadStepAdvance(advance string, current *sqlc.ReadingTask, answer *coachCardAnswer, studentText, reply string, tasks []sqlc.ReadingTask) string {
	if current == nil || current.Kind != string(taskRead) || advance != "done" {
		return advance
	}
	if readStepDoneEvidence(answer, studentText, reply, tasks) {
		return advance
	}
	return ""
}

// studentSaysSheFinishedReading —— 她自己说这几段读完了。
//
// 逐字查她说的那句话里有没有这几种说法。刻意写窄：这是一道**放行**的闸，
// 宽一点就等于没有闸，而它挡的正是「她什么都没说也被判完成」。
//
// 🚨 否定要先查。「还没读完」「没看完」里都含着「读完」，只按 Contains
// 判会把「我还没读完」读成「我读完了」——判错的方向正好是最伤的那个。
func studentSaysSheFinishedReading(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	for _, no := range []string{"没读", "没看", "还没", "沒讀", "沒看", "not yet", "haven't", "have not", "didn't finish", "did not finish"} {
		if strings.Contains(lower, no) {
			return false
		}
	}
	for _, yes := range []string{
		"读完", "看完", "读好", "看好", "读过了", "看过了", "都读了", "都看了", "读了一遍", "看了一遍",
		"讀完", "看完了", "讀過了",
		"finished reading", "done reading", "i've read", "i have read", "read it all", "finished it",
	} {
		if strings.Contains(lower, yes) {
			return true
		}
	}
	return false
}

// replyDirectsToNextReadStep —— 这一轮的话已经把她领去了下一个通读部分。
//
// 和 alignAdvanceWithReply 共用 directedReadStart / readStepRange：那里是拿这个
// 信号把空的 advance 提成 done，这里是拿同一个信号放行一个 done。两处必须认
// 同一件事，否则同一句话在两道闸上会得出相反的结论。
func replyDirectsToNextReadStep(reply string, tasks []sqlc.ReadingTask) bool {
	start := directedReadStart(reply)
	if start == 0 {
		return false
	}
	cur := -1
	for i := range tasks {
		if tasks[i].Status == "pending" {
			cur = i
			break
		}
	}
	if cur < 0 {
		return false
	}
	for i := cur + 1; i < len(tasks); i++ {
		if tasks[i].Status != "pending" || tasks[i].Kind != string(taskRead) {
			continue
		}
		m := readStepRange.FindStringSubmatch(tasks[i].Label)
		if m == nil {
			continue
		}
		return m[1] == strconv.Itoa(start)
	}
	return false
}
