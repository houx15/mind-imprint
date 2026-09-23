package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

// reading_archive.go —— 她在阅读列表里把一篇收起来。
//
// # 为什么存在（2026-09-23，产品负责人第 3 条）
//
//	「自己粘贴文本后，系统会自动分段，如果学生发现分段分错了，无法重新编辑，
//	  只能再开一个新的，阅读列表里旧的也没办法删除。」
//
// 粘错一次就多一条永远去不掉的记录 —— 而「再开一个新的」正是产品让她做的事，
// 所以那条垃圾记录是我们自己造出来的。在这之前整个 mux 里和阅读有关的
// DELETE 只有一条，撤销报告分享；`DELETE /api/v1/readings/{id}` 根本不存在。
//
// # 收起来，不是删掉
//
// 铁律④「过程即数据」：她读过的段落、批注、和 印记 说过的话，是过程评估的
// 地基。真删掉就再也没有了。教师可见性那条规矩（memory
// teacher-visibility-rule-2026-09-14）又说老师看得见她产出的一切 ——
// 一条真 DELETE 会把老师那边的东西也一并拿走，而她想做的只是让自己那张
// 列表干净一点。
//
// 所以收起来**只影响她自己那张列表**（ListReadingsByUser 过滤 archived_at）。
//
// # 收起来是可逆的
//
// `POST /api/v1/readings/{id}/unarchive` 存在，尽管今天界面上没有入口。
// 一条单向的门迟早要靠「再开一个新的」来绕过去，而那正是这条 bug 的由来。
//
// # 门是所有权，不是状态
//
// 用 loadOwnedReadingAtomRow（不带「已完成就不许改」那道闸）：一篇**读完的**
// 阅读同样该收得起来，而带闸的那个会把它拦掉。收起来和读没读完是两件正交的事
// （迁移 0190 里也写了同一条理由：所以它不是 status 的第三个取值）。

func (a *API) archiveReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtomRow(w, r)
	if !ok {
		return
	}
	if err := a.d.Queries.ArchiveReading(r.Context(), at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) unarchiveReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtomRow(w, r)
	if !ok {
		return
	}
	if err := a.d.Queries.UnarchiveReading(r.Context(), at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
