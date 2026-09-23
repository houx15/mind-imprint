-- 她自己说这一篇是什么文体。
--
-- 产品负责人 2026-09-23：
--
--   「for writing, maybe we need to let the students select/talk with ai about
--     what genre they are going to write. sometimes they are writing a 记叙文,
--     sometimes 散文, sometimes 议论文, sometimes 书信.」
--
-- # 🚨 这推翻了 writing_setup.go 里那条「没有文体单选」
--
-- 那个文件头写着：
--
--   「设定弹窗只有三样东西……**没有文体单选**——这是产品的明确要求：
--     学生未必知道「文体」是什么意思，与其让她在一个她读不懂的词上做选择，
--     不如让她用自己的话再说两句，由模型去判断这是议论还是记叙。」
--
-- 同一个人后来要的是另一件事。两者可以并存，而且这一列正是并存的办法：
-- **推断照旧跑，她只是可以纠正它。** 进门那一步仍然不问她文体（那条理由
-- 今天照样成立），但房间里印记会说出它按什么在教，她一眼看得见、点一下能改。
--
-- # 为什么是一列，不是一张表
--
-- 它是这一篇的一个属性，一篇只有一个，没有历史要留。
-- 空串 = 她没说过，走推断（writingGenreOf）。
--
-- # 为什么不写 CHECK
--
-- 和 course.audience、writing_outline.kind 同一个理由：闭表在
-- writing_genre.go 里，DB 的 CHECK 会变成第二份真相，而两份迟早分岔。
-- 服务端收的时候校验（validateWritingGenre），收不下的一律当成没说。

-- +goose Up
ALTER TABLE writing ADD COLUMN genre text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE writing DROP COLUMN genre;
