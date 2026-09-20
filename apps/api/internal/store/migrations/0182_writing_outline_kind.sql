-- +goose Up
-- 图上一个节点「是什么」。取值是闭表，见 apps/api/internal/api/writing_kind.go。
--
-- 为什么加这一列：在这之前模型自己挑 parentId、自己用散文写 role，下游拿关键词
-- 把意思猜回来。同事 2026-09-20 截到的那张图里，结尾挂在中心论点底下（深度 1），
-- 而 slots.ts 只在 depth = 0 时认结尾，于是它被印成「分论点 3」。
-- 产品负责人转来的原话：「这个是总结，不是分论点」。
--
-- 不写 CHECK 约束：同 course.category / course.audience 的理由 —— 词表在 Go 和
-- TS 两处，加 CHECK 会让加一个取值变成一次迁移。校验在 writingKindValid。
ALTER TABLE writing_outline ADD COLUMN kind text NOT NULL DEFAULT '';

-- 回填。规则与 writingKindFromRole 逐条一致：先认最具体的（待补、回应、反方），
-- 再认开篇 / 结尾，再认论据，最后按深度兜底。
--
-- 🚨 **只写 kind，不动 depth 和 position。** 新的深度强制只作用于此后新增和
-- 她拖动的节点；在这里重排，会把一份她已经在写的稿子的卡片顺序当场换掉 ——
-- 而卡片顺序决定她写下的每一段挂在哪儿。
UPDATE writing_outline SET kind = CASE
  WHEN role ~ '还没找到|没找到|待补|暂时没有|缺一份' THEN 'gap'
  WHEN role ~ '回应|反驳' OR lower(role) ~ 'rebuttal|response' THEN 'rebuttal'
  WHEN role ~ '反方|对方|反对|质疑' OR lower(role) ~ 'counter|objection' THEN 'counter'
  WHEN role ~ '开头|引言|开篇|钩子|导入' OR lower(role) ~ 'opening|hook|introduction|intro' THEN 'opening'
  WHEN role ~ '结尾|结论|总结|收尾|落点|结语' OR lower(role) ~ 'closing|conclusion|ending' THEN 'closing'
  WHEN role ~ '例|经历|的事|事件|故事|材料|数据|研究|报道|访谈|调查|案例|引用|名言|人物|史实|素材|证据|新闻|实验|统计|场景|现象'
    OR lower(role) ~ 'example|experience|evidence|data|study|research|report|story|quote|case|survey|statistic|source'
    THEN 'evidence'
  WHEN depth = 0 THEN 'thesis'
  WHEN depth = 1 THEN 'point'
  ELSE 'evidence'
END;

-- +goose Down
ALTER TABLE writing_outline DROP COLUMN kind;
