-- +goose Up
-- 从探索地图开出来、而正文其实只是一段摘要的那些阅读，补上 excerpt_only。
--
-- 产品负责人 2026-09-17 截图：「栉水母不仅美丽。它们是生物学奇迹。」一共两段 ——
-- 一段导语、一行「Source」。阅读室一个字都没提醒她这只是摘要，她就在两段话上
-- 开始了「通读」。
--
-- 原因：mintReadingForPlanet 的第一条路（feed 自带的 body）只要非空就当正文用，
-- 而有的源在 body 里放的就是摘要（那一篇 403 个字符）。第二条路（现抓原页面）
-- 早就有「不到 600 个字符不算正文」这道线，第一条路没有。代码已经补上；这里补
-- 已经建好的那些行。
--
-- 只动**从地图来的**（source_url 对得上 news_planet 的某一条）、而且正文不到
-- 600 个字符的那些。她自己粘进来的短文、分级阅读库的文章都不碰 —— 前者她粘的
-- 就是全部，后者有自己的长度。
UPDATE reading_source rs
SET excerpt_only = true
FROM news_planet np
WHERE rs.source_url = np.url
  AND char_length(rs.body) < 600
  AND NOT rs.excerpt_only;

-- +goose Down
-- 不回滚：分不出哪些是这一条改的、哪些是后来本来就标上的。标成「只有摘要」
-- 的代价只是多一条提醒，而把真正的摘要标回正文会让她在两段话上开始通读。
SELECT 1;
