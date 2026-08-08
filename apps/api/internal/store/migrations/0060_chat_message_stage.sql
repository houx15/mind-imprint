-- +goose Up
-- chat_message gains a `stage` column: the studio_state.stage in effect when
-- the turn was persisted (topic_discussion/proposal_forming/plan_generation/
-- proposal_writing/proposal_review/body_writing/retrospective). Nullable, no
-- default — historical rows predate this concept and stay NULL. 过程即数据:
-- `surface` says WHICH room/producer wrote the turn; `stage` says WHERE in the
-- project lifecycle it happened, so the evaluation layer can read the
-- per-turn arc of thinking across the whole project.
ALTER TABLE chat_message ADD COLUMN stage text;

-- +goose Down
ALTER TABLE chat_message DROP COLUMN stage;
