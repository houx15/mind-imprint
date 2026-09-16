-- 0156 —— 这一篇里放的是正文，还是只有摘要。
--
-- # 它修的是什么
--
-- 从探索地图点开一条新闻，原来会**同时**做两件事：另开一页原文，再进阅读室。
-- 产品负责人 2026-09-16 的裁定是只做后一件：
--
--   > now in exploration if we click one paper, we automatically open the
--   > original link. a new recommendation is: we jump to reading page. if we
--   > extracted the texts successfully, then begin reading directly. or if we
--   > only have abstract, we go to reading with abstract, with below a button
--   > 「我们无法直接获取正文，如果想要阅读全文，请跳转原网站」
--
-- 「抓到了正文」和「只有一段摘要」因此第一次成了两种要**说出来**的状态。在这一
-- 列出现之前，阅读室手上只有一段文字，没有办法分辨它是一整篇还是一段导语 ——
-- 一段两句话的摘要摆在那儿，长得和一篇很短的文章一模一样，而她会以为这就是全文。
--
-- # 为什么是一列布尔值，不是按长度猜
--
-- 按长度猜会把一篇真的很短的文章误判成摘要，也会把一段很长的导语放过去。
-- 「这份文本是怎么来的」只有写它的那一刻知道（feed 带的正文 / 抓回来的正文 /
-- feed 的导语），所以由那一刻记下来。
--
-- 默认 false：她自己粘进来的、从分级阅读库开出来的，都是正文。

-- +goose Up
ALTER TABLE reading_source ADD COLUMN excerpt_only boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN reading_source.excerpt_only IS
  'true = 这里放的只是摘要/导语，不是正文。阅读室据此摆出「跳转原网站」那一条。';

-- +goose Down
ALTER TABLE reading_source DROP COLUMN excerpt_only;
