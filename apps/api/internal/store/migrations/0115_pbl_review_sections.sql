-- +goose Up
-- 复盘按产品负责人 2026-09-02 给的结构分段：
--
--   what    做了什么、经历了什么
--   how     感受如何——过程，以及结果
--   moment  最印象深刻的一件事
--   praise  对自己的肯定
--   improve 哪里可以做得更好，怎么做
--   with_ai 从与 AI 的协作里学到什么
--
-- 她原话：「but we can ask ai to generate concrete questions according to this
-- structure」——所以段是固定的，段里的问题由印记按这个项目真实发生过的事现写。
--
-- 我原来那一版没有段，只有一串按事件生成的问题。少了段，复盘就没有骨架：
-- 她答完一串具体的问题，仍然没被带着从"做了什么"走到"我学到了什么"。
ALTER TABLE pbl_review ADD COLUMN section text NOT NULL DEFAULT 'what'
  CHECK (section IN ('what', 'how', 'moment', 'praise', 'improve', 'with_ai'));

-- +goose Down
ALTER TABLE pbl_review DROP COLUMN section;
