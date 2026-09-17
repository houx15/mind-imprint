-- +goose Up
-- 已经存下来的正文里，把「You Might Also Like」那一行和它之后的内容切掉。
--
-- 产品负责人 2026-09-17：「有时候我们抓取的文章最后会有：You Might Also Like。这个及
-- 之后的内容其实可以不要，不然会干扰正常阅读。」新抓的已经在抽取时切掉了
-- （materialize.CutRelatedTrailer）；这里补的是已经存进来的。
--
-- 🚨 只动**还没开始读**的那些：没有读法清单、没有对话、没有划线。
-- 读过的那一篇，清单、导读、报告都是按现在这份正文的段号排的 —— 从它们底下切掉
-- 段落，会让某一步、某个部分指着一段不存在的话。那几篇留着原样（线上 2026-09-17
-- 时是一篇，已完成）。
UPDATE reading_source rs
SET body = rtrim(regexp_replace(
      rs.body,
      E'\\n\\n[ \\t]*(you might also like|you may also like)[ \\t]*[:：]?[ \\t]*(\\n[\\s\\S]*)?$',
      '',
      'i'))
WHERE rs.body ~* E'\\n\\n[ \\t]*(you might also like|you may also like)[ \\t]*[:：]?[ \\t]*(\\n|$)'
  AND position(lower('also like') in lower(rs.body)) > 200
  AND NOT EXISTS (SELECT 1 FROM reading_task t WHERE t.atom_id = rs.atom_id)
  AND NOT EXISTS (SELECT 1 FROM atom_message m WHERE m.atom_id = rs.atom_id)
  AND NOT EXISTS (SELECT 1 FROM atom_annotation a WHERE a.atom_id = rs.atom_id);

-- +goose Down
-- 切掉的是网站的推荐栏，不回滚。
SELECT 1;
