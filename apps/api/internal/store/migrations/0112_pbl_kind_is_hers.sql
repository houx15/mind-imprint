-- +goose Up
-- 项目类别不再由 AI 判定，也不再是一张写死的枚举。
--
-- 产品负责人 2026-09-02：
--   「after I enter my questions ... neither should we decide the category of a
--    project then.」
--   「besides categories also: 网站搭建/内容设计/田野调查/。。。」
--
-- 两件事跟着变：
--
--  1. **空是合法的**。刚建出来的项目还没有类别——她自己都还没想清楚要做什么，
--     让机器先替她归好类，是把一个没答案的问题伪造成有答案。默认空串就是
--     「还没定」，不是缺数据。
--
--  2. **不再是枚举**。她给的那串以「。。。」结尾，意思是这张单子会长。写成
--     CHECK，每加一个类别就要一次迁移和一次发布；写成自由字符串，加一个类别
--     就是界面上多一行——和工具箱是同一个道理（internal/pbl/tools.go）。
--
-- 旧的五个值（website/research/design/making/investigation）原样留在库里，
-- 不改写：那是当时分类器判的，改掉就等于篡改过程记录。界面按 kindLabel 显示，
-- 认不出的就原样显示。
ALTER TABLE pbl_project DROP CONSTRAINT pbl_project_kind_check;
ALTER TABLE pbl_project ALTER COLUMN kind SET DEFAULT '';

-- +goose Down
-- 回滚前把空串和新类别归到 research，否则旧的 CHECK 加不回去。
UPDATE pbl_project
SET kind = 'research'
WHERE kind NOT IN ('website', 'research', 'design', 'making', 'investigation');
ALTER TABLE pbl_project ALTER COLUMN kind DROP DEFAULT;
ALTER TABLE pbl_project
  ADD CONSTRAINT pbl_project_kind_check
    CHECK (kind IN ('website', 'research', 'design', 'making', 'investigation'));
