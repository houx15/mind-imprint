-- +goose Up
-- 兴趣线索库 —— 一条线索就是一趟 awakening_run。
--
-- 0181 立了一条规矩：**同一个人最多只有一趟没走完**（awakening_run_open_idx）。
-- 那条规矩当时是对的：它防的是「她在第三屏关掉页面、第二天又开一趟，把上次
-- 的能量卡牌丢掉」。
--
-- 2026-09-21 的反馈把它推翻了：
--
--   「如果学生只是暂时对上次的线索没有进一步的想法，想先放一放，清空了就
--     没有记录了。所以我想能不能有一个线索库，保存学生曾提出的所有线索。」
--
-- 一个人同时有好几条想追的线索，这是正常的。原来只能有一条，所以「换一条」
-- 只能实现成**清空**，而清空就是把她写过的字删掉。
--
-- 于是：一条线索 = 一趟 run，她可以同时停着好几条。每条线索自己带着助手、
-- 自己的那几轮回答、自己的报告 —— 陪练的上文和选词的语料都按 run_id 取，
-- 所以这一改同时把上下文按话题分开了：从前她中途换话题，陪练会把上一个
-- 话题带进来，报告也会把两件事混在一起。

-- 一条线索最多只有一个人在做 —— 但一个人可以停着很多条。
DROP INDEX IF EXISTS awakening_run_open_idx;

-- 取而代之：按「最近动过」排的普通索引，线索库和「她上次在哪条」都读它。
CREATE INDEX awakening_run_open_idx
  ON awakening_run (user_id, updated_at DESC) WHERE finished_at IS NULL;

-- 线索的名字。
--
-- 空串 = 还没起名（她刚开始答，或者那次起名没成）。界面这时退回用她第一句
-- 回答的开头，所以**没有名字也永远有东西可显示**。
--
-- 名字由模型给几个候选、她自己挑一个（产品负责人 2026-09-21：「model suggest
-- and user selects」）。🚨 候选里必须始终有一个是从她原话裁出来的 —— 模型那
-- 一次没回上来时用它，绝不编一个名字塞进去。
ALTER TABLE awakening_run ADD COLUMN title text NOT NULL DEFAULT '';

-- 一条线索可以总结不止一次。
--
-- 她总结了一条线索，过两天又想到新的东西，点进去接着答第 4、5 问，再总结一次
-- —— 那是第二份报告，不是把第一份改掉。所以 run_id 上那个 UNIQUE 要去掉。
--
-- 重复点「现在总结」不会白花两次调用：应用层的判据是**上一份报告之后有没有
-- 新的回答**，没有就把已有那份拿回去（见 api/awakening.go 的 summarize）。
ALTER TABLE awakening_report DROP CONSTRAINT awakening_report_run_id_key;
CREATE INDEX awakening_report_run_idx ON awakening_report (run_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS awakening_report_run_idx;
-- 回滚前要先删掉重复的报告行，否则 UNIQUE 建不起来。
DELETE FROM awakening_report a USING awakening_report b
  WHERE a.run_id = b.run_id AND a.created_at < b.created_at;
ALTER TABLE awakening_report ADD CONSTRAINT awakening_report_run_id_key UNIQUE (run_id);
ALTER TABLE awakening_run DROP COLUMN title;
DROP INDEX IF EXISTS awakening_run_open_idx;
-- 同样：回滚前每个人只能留一趟没走完的。
UPDATE awakening_run SET finished_at = now()
  WHERE finished_at IS NULL AND id NOT IN (
    SELECT DISTINCT ON (user_id) id FROM awakening_run
    WHERE finished_at IS NULL ORDER BY user_id, updated_at DESC
  );
CREATE UNIQUE INDEX awakening_run_open_idx
  ON awakening_run (user_id) WHERE finished_at IS NULL;
