-- +goose Up
-- 觉醒协议 —— 兴趣测试的一次作答。
--
-- 这张表存在的理由不是「记一个分数」，而是**过程即数据**（铁律④）：她选了哪个
-- 导航员、她写下的锚点原话、她挑了哪个钩子、以及压力测试她试了几次才答对，都
-- 是关于她怎么想的信号。这里没有 score 列，也不会有 —— 这不是一场考试，没有
-- 「兴趣做得好不好」这回事。
--
-- 一次作答 = 一行。**重做是「再长几个词」，不是清空重来**：keyword_source 的
-- UNIQUE (keyword_id, kind, ref_id) 用这一行的 id 作 ref_id，于是第二次作答能给
-- 同一个词再添一条来源（强度因此上升），而不是撞进唯一约束里什么都不做。这也
-- 是 ref_id 存这一行的 id 而不是 NULL 的原因。
CREATE TABLE interest_quiz (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,

  -- 她选的 AI 导航员。存中文名，因为它是她看见的那个名字；三选一由应用层校验，
  -- 不写 CHECK —— 导航员是内容，加一个不该要一次迁移。
  navigator text NOT NULL DEFAULT '',

  -- 兴趣锚点：她自己写的两段。**anchor_reason 是这次作答里最有价值的一列** ——
  -- 树上长出来的词，evidence 就摘自这里。「很帅」和「他经历了很多痛苦，但在关
  -- 键时刻依然保持理智」是两个完全不同的信号，而区别只在这一列里。
  anchor_work   text NOT NULL DEFAULT '',
  anchor_reason text NOT NULL DEFAULT '',

  -- 兴趣钩子：character | craft | society。它决定结果页给她装上哪几片学科透镜
  -- （见 internal/interest.HookLenses），也是一条弱路由线索。
  hook text NOT NULL DEFAULT '' CHECK (hook IN ('', 'character', 'craft', 'society')),

  -- 反方压力测试。记的是**过程**：她最后选了哪个，以及在此之前错了几次。
  -- 错误次数不减分，它是摩擦被转成的信号 —— 一个一次就选中证据的学生和一个
  -- 试了三次的学生，需要的下一步不一样。
  challenge_choice   text NOT NULL DEFAULT '',
  challenge_attempts int  NOT NULL DEFAULT 0 CHECK (challenge_attempts >= 0),

  created_at  timestamptz NOT NULL DEFAULT now(),
  -- 走到结果页的那一刻。中途退出的作答留一行 finished_at IS NULL —— 那本身
  -- 也是数据（她在哪一屏走开了），并且不算「做过了」，所以邀请还会再出现。
  finished_at timestamptz
);

CREATE INDEX interest_quiz_user_idx ON interest_quiz (user_id, created_at DESC);

-- +goose Down
DROP TABLE interest_quiz;
