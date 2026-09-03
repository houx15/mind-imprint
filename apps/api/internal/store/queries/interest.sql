-- 兴趣模型（迁移 0116）。她的关键词树 + 词到学科的边。
--
-- strength 从来不由调用方传：它是 keyword_source 条数的函数
-- （internal/interest.Strength），所以这里只有 RecountKeywordStrength 一条
-- 写它的路径。一个能直接设强度的接口，迟早会有人拿它把某个词调大。

-- name: UpsertInterestKeyword :one
-- 按 (user_id, norm) 认词。已经存在就只补 text_en/note 里空着的那部分 ——
-- 后来的一次采集不该把她第一次的措辞覆盖掉，但可以把当时缺的补上。
INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, note)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, norm) DO UPDATE SET
  text_en = CASE WHEN interest_keyword.text_en = '' THEN EXCLUDED.text_en ELSE interest_keyword.text_en END,
  note    = CASE WHEN interest_keyword.note    = '' THEN EXCLUDED.note    ELSE interest_keyword.note    END
RETURNING *;

-- name: AddKeywordSource :exec
-- 同一个 (词, 类型, 来源) 只算一次：重复打开一篇阅读不该把强度刷上去。
INSERT INTO keyword_source (keyword_id, kind, ref_id, label, evidence, happened_at)
VALUES ($1, $2, $3, $4, $5, COALESCE(sqlc.narg('happened_at')::timestamptz, now()))
ON CONFLICT (keyword_id, kind, ref_id) DO NOTHING;

-- name: CountKeywordSources :one
SELECT count(*) FROM keyword_source WHERE keyword_id = $1;

-- name: RecountKeywordStrength :exec
UPDATE interest_keyword SET strength = $2 WHERE id = $1;

-- name: ListInterestKeywords :many
SELECT * FROM interest_keyword
WHERE user_id = $1
ORDER BY strength DESC, first_seen_at ASC;

-- name: ListKeywordSourcesForUser :many
-- 一次取回她全部词的来源，由 Go 侧按 keyword_id 分组 —— 每个词发一次查询会在
-- 一棵有二十个词的树上变成二十次往返。
SELECT s.* FROM keyword_source s
JOIN interest_keyword k ON k.id = s.keyword_id
WHERE k.user_id = $1
ORDER BY s.happened_at DESC;

-- name: UpsertKeywordDiscipline :exec
-- 重算一条边时，她自己改过的（how='student'）绝不被后来的模型判定覆盖掉 ——
-- 这就是 DO UPDATE 上那个 WHERE 的全部作用。
INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (keyword_id, discipline_id) DO UPDATE SET
  confidence = EXCLUDED.confidence,
  how        = EXCLUDED.how,
  rationale  = EXCLUDED.rationale
WHERE keyword_discipline.how <> 'student';

-- name: ListKeywordDisciplinesForUser :many
SELECT kd.* FROM keyword_discipline kd
JOIN interest_keyword k ON k.id = kd.keyword_id
WHERE k.user_id = $1
ORDER BY kd.confidence DESC;

-- name: ListRoutedKeywordsForUser :many
-- 路由 T2（共现继承）要的那张表：她已经有哪些词、各连到哪些学科、来源是哪几条。
SELECT
  k.id,
  k.norm,
  array_remove(array_agg(DISTINCT kd.discipline_id), NULL)::text[] AS discipline_ids,
  array_remove(array_agg(DISTINCT s.ref_id::text), NULL)::text[]   AS source_refs
FROM interest_keyword k
LEFT JOIN keyword_discipline kd ON kd.keyword_id = k.id
LEFT JOIN keyword_source     s  ON s.keyword_id  = k.id
WHERE k.user_id = $1
GROUP BY k.id, k.norm;

-- name: SetKeywordDisciplineByStudent :exec
-- 她自己在抽屉里改的那一条。how='student' 之后，重算路由不会再动它。
INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale)
VALUES ($1, $2, 1.0, 'student', $3)
ON CONFLICT (keyword_id, discipline_id) DO UPDATE SET
  confidence = 1.0, how = 'student', rationale = EXCLUDED.rationale;

-- name: ListUnharvestedFinishedAtoms :many
-- 她已经完成、但采集器还没跑过的 atom。
--
-- 「完成」在三种 atom 上是三件事：阅读与写作是 status='finished'，项目是走到了
-- 复盘（'review' 及其之后）。没完成的东西不该长词 —— 一篇读了三段就关掉的文章
-- 说不出她关心什么。
SELECT a.id, a.kind
FROM atom a
LEFT JOIN reading     r ON r.atom_id = a.id
LEFT JOIN writing     w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
WHERE a.user_id = $1
  AND a.interest_harvested_at IS NULL
  AND (
    (a.kind = 'reading' AND r.status = 'finished')
    OR (a.kind = 'writing' AND w.status = 'finished')
    OR (a.kind = 'project' AND p.status IN ('review', 'keeping', 'archived'))
  )
ORDER BY a.last_activity_at DESC
LIMIT $2;

-- name: MarkAtomInterestHarvested :exec
-- 「尝试过一次，无论结果如何」。零个词也要盖章，否则一篇薄阅读会被反复重采。
UPDATE atom SET interest_harvested_at = now()
WHERE id = $1 AND interest_harvested_at IS NULL;

-- name: GetAtomInterestHarvestedAt :one
SELECT interest_harvested_at FROM atom WHERE id = $1;

/* ── 觉醒协议 · 兴趣测试 ─────────────────────────────────────────────────── */

-- 开一次作答。**先落库再做题**：这样一次中途退出的作答也留下一行（finished_at
-- 为空），而「她走到哪一屏就走开了」本身就是数据（铁律④）。
-- name: StartInterestQuiz :one
INSERT INTO interest_quiz (user_id) VALUES ($1)
RETURNING *;

-- 收一次作答。用 user_id 一起匹配，所以一个人改不了别人的那一行。
-- name: FinishInterestQuiz :one
UPDATE interest_quiz SET
  navigator          = $3,
  anchor_work        = $4,
  anchor_reason      = $5,
  hook               = $6,
  challenge_choice   = $7,
  challenge_attempts = $8,
  finished_at        = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- 她做完过几次。**只数做完的**：一次中途退出的作答不算「做过了」，所以那条
-- 邀请还会再出现 —— 一个在第三屏关掉页面的学生，不该从此再也见不到入口。
-- name: CountFinishedInterestQuizzes :one
SELECT count(*) FROM interest_quiz
WHERE user_id = $1 AND finished_at IS NOT NULL;

-- 她最近做完的那一次。结果页重新打开时读它。
-- name: LatestFinishedInterestQuiz :one
SELECT * FROM interest_quiz
WHERE user_id = $1 AND finished_at IS NOT NULL
ORDER BY finished_at DESC
LIMIT 1;

/* ── 继续深挖 ───────────────────────────────────────────────────────────── */

-- name: GetInterestKeywordForUser :one
SELECT * FROM interest_keyword WHERE id = $1 AND user_id = $2;

-- 这个词上她留下的每一句原话。**这是深挖调用唯一真正重要的输入** —— 没有它们，
-- 模型只能围着一个词泛泛地想，而那正是原型那四个空动词的来源。
-- name: ListKeywordEvidence :many
SELECT evidence FROM keyword_source
WHERE keyword_id = $1 AND evidence <> ''
ORDER BY happened_at DESC
LIMIT 8;

-- name: ListKeywordDig :many
SELECT * FROM keyword_dig WHERE keyword_id = $1 ORDER BY
  CASE kind WHEN 'think' THEN 1 WHEN 'read' THEN 2 WHEN 'write' THEN 3 ELSE 4 END;

-- name: UpsertKeywordDig :exec
INSERT INTO keyword_dig (keyword_id, kind, text, why) VALUES ($1,$2,$3,$4)
ON CONFLICT (keyword_id, kind) DO UPDATE SET text = EXCLUDED.text, why = EXCLUDED.why;

-- 盖章在生成之前。见迁移 0121。
-- name: MarkKeywordDigged :exec
UPDATE interest_keyword SET dig_at = now() WHERE id = $1;

-- name: GetKeywordDigAt :one
SELECT dig_at FROM interest_keyword WHERE id = $1;
