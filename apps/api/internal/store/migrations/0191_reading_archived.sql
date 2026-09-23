-- 🚨 0191 合并时从 0190 改过号。另一个会话的 0189_lite_assignment_issue
-- 已经在 origin/main 上，而 goose 碰到重复的版本号会直接起不来
-- （memory: wip-recovery-and-migration-renumber-2026-09-16 —— 这是同一类
-- 事故的第三次）。这一条只动 reading.archived_at，和那一条没有任何依赖，换号安全。
--
-- 阅读列表里，她自己收起一篇。
--
-- 产品负责人 2026-09-23 第 3 条：
--
--   「自己粘贴文本后，系统会自动分段，如果学生发现分段分错了，无法重新编辑，
--     只能再开一个新的，阅读列表里旧的也没办法删除。」
--
-- 粘错一次就多一条永远去不掉的记录 —— 而「再开一个新的」正是我们让她做的事，
-- 所以这条垃圾记录是产品自己造出来的。
--
-- # 为什么是 archived_at，不是真的删掉
--
-- 铁律④「过程即数据」：她读过的段落、批注、和印记说过的话，是过程评估的
-- 地基。真删掉就再也没有了。而教师可见性那条规矩（memory
-- teacher-visibility-rule-2026-09-14）说老师看得见她产出的一切 ——
-- 一条 DELETE 会把老师那边的东西也一起拿走。
--
-- 收起来只影响**她自己那张列表**。
--
-- # 为什么不是 status 的第三个取值
--
-- status 是 active / finished 的状态机，读完、重新打开都在上面跑。
-- 加第三个值意味着每一处 `status = 'active'` 都要重新审一遍，而「收起来」
-- 和「读没读完」本来就是两件正交的事：一篇读完的也可以收起来。
--
-- Down 直接删列：收起来这件事没有下游数据依赖它。

-- +goose Up
ALTER TABLE reading ADD COLUMN archived_at timestamptz;

-- 列表按 (user, kind, created_at) 取，再按这一列过滤；部分索引只覆盖没收起来
-- 的那些，正是列表要的那一批。
CREATE INDEX reading_not_archived_idx ON reading (atom_id) WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS reading_not_archived_idx;
ALTER TABLE reading DROP COLUMN archived_at;
