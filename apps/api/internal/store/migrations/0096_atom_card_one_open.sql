-- +goose Up
-- 「一次只开一副透镜」从约定变成约束。
--
-- 在这条索引之前，这条规则只活在两个 handler 里：postLiteReadingTurn 靠
-- PacingState.OpenCard，liteSummonCard 先扫一遍 ListAtomCards——两者都是
-- 「先读后写、中间隔着一次几秒钟的模型调用」。两个并发请求可以双双通过检查、
-- 各自插入一行 proposed；而 getOpenCard 只返回第一行，于是第二张卡对学生完全
-- 不可见，却仍然在此后每一次 summon 里把她挡回去——她被告知「先完成当前这副
-- 透镜」，而那副透镜她刷新之前根本看不到。
--
-- 部分唯一索引把它变成数据库层面的事实：**任何**未来的写入方，哪怕忘了检查，
-- 也不可能让一个 atom 同时有两张未结束的卡。已提交/已跳过的卡不在索引谓词里，
-- 所以一次阅读仍然可以按顺序用很多副透镜。
--
-- 注意：若现有数据里已存在「一个 atom 两张未结束的卡」，这条迁移会失败并中止
-- 部署。这是刻意的——那种情况下正确的做法是有人去看一眼，而不是让迁移替学生
-- 编造一次她没做过的「跳过」。
CREATE UNIQUE INDEX atom_card_one_open_idx
  ON atom_card (atom_id)
  WHERE status IN ('proposed', 'active');

-- +goose Down
DROP INDEX IF EXISTS atom_card_one_open_idx;
