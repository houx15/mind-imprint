-- +goose Up
-- 她给这次阅读体验打的星（1–5），挂在报告底部。
--
-- 这不是对她的评分（铁律②禁的是那个），方向是反的：她评的是我们。所以它落在
-- atom 上而不是 reading 上——写作报告将来要问同一个问题，问的也是同一件事。
--
-- 可空是默认状态：没打星和打了一星必须分得开，否则「她没说」会被读成「她给了
-- 最低分」。CHECK 里显式放行 NULL，同一个理由。
ALTER TABLE atom ADD COLUMN experience_rating smallint
  CHECK (experience_rating IS NULL OR (experience_rating BETWEEN 1 AND 5));

-- +goose Down
ALTER TABLE atom DROP COLUMN experience_rating;
