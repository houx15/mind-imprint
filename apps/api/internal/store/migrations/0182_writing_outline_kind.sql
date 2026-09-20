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
  -- 深度 ≥ 2 上的「道理 / 解释 / 理由」是撑着分论点的推理，不是材料
  -- （深度 1 上的「一条理由」是分论点，所以这一条必须带着深度判）。
  WHEN depth >= 2 AND (role ~ '道理|解释|推理|分析|原因|理由'
    OR lower(role) ~ 'reasoning|explanation|analysis|reason') THEN 'reasoning'
  -- 先认 reference（她找来的：研究、报道、数据、史实），剩下的才是 evidence
  -- （她自己见过的事）。两者分开的理由见 writing_kind.go 的 writingKindReference：
  -- 一个司马迁的例子没有链接可填，但它不是「她见过的事」，而
  -- writingPlanShape.Wider 那条判据（至少要有一条不是个人经历）靠的正是这个区分。
  -- 有 source 的一定是她找来的。
  WHEN source <> '' THEN 'reference'
  WHEN role ~ '材料|数据|研究|报道|访谈|调查|引用|名言|史实|素材|新闻|实验|统计|文献|论文'
    OR lower(role) ~ 'data|study|research|report|quote|survey|statistic|source|paper'
    THEN 'reference'
  -- 只有 role 明说是她的，才算她见过的事；其余的材料算她找来的。
  -- 🚨 这个方向是老 writingRoleIsPersonal 的方向，不是随手挑的：
  -- 那个数只用来提醒「还缺一条更有说服力的例子」，少提醒一次比冤枉她强。
  WHEN role ~ '你|自己|亲身|个人|身边|经历过|见过'
    OR lower(role) ~ 'your own|personal|my own|you saw|you did'
    THEN 'evidence'
  WHEN role ~ '例|经历|的事|事件|故事|案例|人物|证据|场景|现象|材料|数据|研究|报道|访谈|调查|引用|名言|史实|素材|新闻|实验|统计|文献|论文'
    OR lower(role) ~ 'example|experience|evidence|story|case|data|study|research|report|quote|survey|statistic|source|paper'
    THEN 'reference'
  WHEN depth = 0 THEN 'thesis'
  WHEN depth = 1 THEN 'point'
  ELSE 'reference'
END;

-- +goose Down
ALTER TABLE writing_outline DROP COLUMN kind;
