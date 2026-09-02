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
