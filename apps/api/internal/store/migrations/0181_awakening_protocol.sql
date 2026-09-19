-- +goose Up
-- 觉醒协议 —— 取代 0119 的七屏兴趣测试。
--
-- 旧表 interest_quiz 不删：历史作答是数据，树上那些词的来源还指着它的行。
-- 它不再有新写入。
--
-- 这里三张表回答三个不同的问题：
--   awakening_run     她走了哪一趟（走到哪、选了什么、第几次）
--   awakening_turn    终端里她逐轮说了什么（同时是 evidence 的语料）
--   awakening_report  跑完那一次生成的报告（只生成一次，可分享）

-- 一次作答。
--
-- 中途退出留一行 finished_at IS NULL。那一行不算「做过了」，但它记着她走到过
-- 哪一屏，下次进来可以接着走 —— 摩擦被转成信号（铁律④）。
CREATE TABLE awakening_run (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,

  -- 她的第几次。第一次走全程，第二次起走短线（跳过开场与底牌，直接进终端，
  -- 而终端这时知道她树上已经有什么）。由服务端在开一次作答时算出来并写死，
  -- 不在读取时现数 —— 读取时现数会让一次并发的重复 POST 得到两个「第 2 次」。
  attempt_no int NOT NULL DEFAULT 1 CHECK (attempt_no >= 1),

  -- 她走到哪一屏。取值是 internal/awakening.Stages 里的 id，应用层校验；
  -- 不写 CHECK，理由同 course.category —— 加一屏不该要一次迁移。
  stage text NOT NULL DEFAULT 'boot',

  -- 序章那个选择：加入联盟，还是先看 AI 的底牌。
  -- 空串 = 她还没走到那一屏。
  route text NOT NULL DEFAULT '' CHECK (route IN ('', 'joined', 'observer')),

  -- 她选的印记助手。存代号（NOVA / SAGE / KIRO），因为 prompt 和音频文件名
  -- 都按它索引。三选一由应用层校验。
  navigator text NOT NULL DEFAULT '',

  -- 能量卡牌的结果。它喂给终端第一个节点：一个刚说完「我的能量集中在动手做」
  -- 的学生，第一问就该问她最近动手做过什么。
  energy_profile jsonb NOT NULL DEFAULT '{}'::jsonb,

  -- 天赋卡牌：选中的 5 张，以及每张落在哪一堆（有能量 / 会做但消耗 / 想发展）。
  -- 报告里「怎么靠近」那一块就是它。
  talent jsonb NOT NULL DEFAULT '{}'::jsonb,

  -- 三层追问与下一步那两屏她选了什么。
  lens_choice      text NOT NULL DEFAULT '',
  challenge_choice text NOT NULL DEFAULT '',

  -- 档案确认题她试了几次才选对。不减分 —— 一次就选中证据的学生和试了三次的
  -- 学生，需要的下一步不一样。
  archive_attempts int NOT NULL DEFAULT 0 CHECK (archive_attempts >= 0),

  -- 观察者路线上她带走的那个问题（顺 / 换 / 偏）。
  observer_question text NOT NULL DEFAULT '',

  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

CREATE INDEX awakening_run_user_idx ON awakening_run (user_id, created_at DESC);

-- 未完成的那一趟每人最多一行。她在第三屏关掉页面、第二天再进来，应该接着
-- 那一趟走，而不是开第二趟把第一趟的能量卡牌丢掉。
CREATE UNIQUE INDEX awakening_run_open_idx
  ON awakening_run (user_id) WHERE finished_at IS NULL;

-- 终端里的一轮。
--
-- 这张表有两个身份。第一个是让她能接着走：刷新一次、第二天回来，前面说过的话
-- 还在。第二个更要紧 —— **它是 evidence 的语料**。树上每个词、报告里每句引文，
-- 都必须在 student_text 里逐字查得到（interest.KeepGrounded）。
--
-- 🚨 语料只含 student_text，不含 reply。幻引的来源就是模型自己的上文：
-- 把 reply 放进语料，等于允许它引用自己编的句子，然后声称那是她说的
-- （memory: prompt-twice-then-make-it-checkable-2026-09-12）。
CREATE TABLE awakening_turn (
  id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES awakening_run(id) ON DELETE CASCADE,

  -- 这一轮在这趟里的序号，从 0 起。UNIQUE 在它上面，不在 node_index 上：
  -- 一个节点可以问不止一轮（她答得太薄时会在同一节点换一个问法再问一次）。
  seq int NOT NULL CHECK (seq >= 0),

  -- 对应 9 个节点里的第几个，从 0 起。
  node_index int NOT NULL CHECK (node_index >= 0),

  -- 她敲进去的字。不截断 —— 她自己写的字一个都不要切
  -- （memory: observation-tool-is-the-bug-2026-09-12）。
  student_text text NOT NULL,

  -- 印记回的那一句。失败时为空串，界面照实说，绝不填一句像样的话进去
  -- （memory: ai-errors-must-surface-never-fake）。
  reply text NOT NULL DEFAULT '',

  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, seq)
);

CREATE INDEX awakening_turn_run_idx ON awakening_turn (run_id, seq);

-- 报告。
--
-- 一趟一份，生成一次之后只读（照 ensureAtomReport 的 advisory lock + 复查 +
-- 生成一次）。payload 存整份，因为它的形状会随版本变，而一份已经生成的报告
-- 不该因为我们改了结构就变成另一份。
CREATE TABLE awakening_report (
  id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id  uuid NOT NULL UNIQUE REFERENCES awakening_run(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,

  payload jsonb NOT NULL,

  -- 她选择公开时才有值，随时可以撤回（置 NULL，下一个请求就查不到）。
  -- 16 字节随机数的 32 位十六进制，绝不是 run_id —— 理由见 atom_report_share.go。
  share_token text UNIQUE,

  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX awakening_report_user_idx ON awakening_report (user_id, created_at DESC);

-- +goose Down
DROP TABLE awakening_report;
DROP TABLE awakening_turn;
DROP TABLE awakening_run;
