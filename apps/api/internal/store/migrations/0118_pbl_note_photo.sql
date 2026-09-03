-- +goose Up
-- 便签可以带一张照片。
--
-- 🚨 docs/2026-09-01-pbl-detail.md 对「观察日记」的原话：
--
--	“Students return with information, maybe a photo, voice note, drawing,
--	 or short paragraph.”
--
-- 一直只做了 short paragraph。产品负责人 2026-09-03 指出 OSS 早就上线了
-- （2026-07-27，presigned PUT/GET + CDN），缺的只是这一列和前端那点接线。
--
-- 存的是 OSS 的 key，不是 URL：URL 是签出来的、会过期，存进库里第二天就是一条
-- 死链。要看的时候拿 key 去换一个签好的 GET（POST /oss/resolve-url）。
--
-- 空字符串 = 这条便签没有照片。用 '' 而不是 NULL，和这张表里别的可选文本
-- （cluster）保持一致，读的那头就不用到处判空。
ALTER TABLE pbl_note ADD COLUMN image_key text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pbl_note DROP COLUMN image_key;
