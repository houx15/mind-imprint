-- 0157 —— 段落工具的产物可以是一份结构，讲解可以只针对一句话。
--
-- 两件事都来自产品负责人 2026-09-16 的走查，它们要动的是同一张表。
--
-- # data：关键单词不再是一段散文
--
--   > in the reading room, we have the 关键单词 … please, let's make the words
--   > like a set of cards. for each word, we have 词性, meaning. explanation,
--   > example sentences.
--
-- 一段散文没有办法变成一组卡片，也没有办法回到正文里把那个词标出来 —— 要标，
-- 就得知道**哪几个字**是那个词。所以关键单词的产物从 markdown 变成一个数组，
-- 每个词带 词性 / 在这句里的意思 / 讲解 / 例句，而 term 会被服务端拿回段落里
-- 逐字核对（核不上的丢掉）。那次核对正是荧光笔能落下去的全部依据。
--
-- body 保留：它仍然是这份产物的纯文字形态。一份只有 JSON 的记录，在任何一个
-- 不带解析器的地方（日后的报告、教师端）就是一段乱码。
--
-- # subject：语法讲的是一个句子，不是一整段
--
--   > 语法 - we should teach grammar in a sentence level. namely, we ask
--   > student to select a sentence, and we teach that. currently we only have
--   > paragraph level which is strange.
--
-- 于是同一段里可以有好几份语法讲解，一句一份。缓存的键因此要把「讲的是哪一句」
-- 算进去。subject 为空串 = 讲的是整段（其余工具全都如此），所以老数据不动、
-- 老的重放照旧命中。
--
-- 🚨 唯一索引要**先删后建**：把 subject 加进键之前，老索引仍然只认三列，
-- 同一段的第二句语法会撞上第一句。

-- +goose Up
ALTER TABLE reading_block_note ADD COLUMN data jsonb;
ALTER TABLE reading_block_note ADD COLUMN subject text NOT NULL DEFAULT '';

COMMENT ON COLUMN reading_block_note.data IS
  '结构化产物（关键单词的词卡数组等）。NULL = 这个工具只产出 body 那段文字。';
COMMENT ON COLUMN reading_block_note.subject IS
  '这份讲解针对的是哪一句（逐字抄自段落）。空串 = 整段。';

DROP INDEX reading_block_note_key_idx;
CREATE UNIQUE INDEX reading_block_note_key_idx
  ON reading_block_note (atom_id, block_id, tool, subject);

-- +goose Down
DROP INDEX reading_block_note_key_idx;
CREATE UNIQUE INDEX reading_block_note_key_idx ON reading_block_note (atom_id, block_id, tool);
ALTER TABLE reading_block_note DROP COLUMN subject;
ALTER TABLE reading_block_note DROP COLUMN data;
