-- +goose Up
-- 第三关：给网站定调子。
--
-- 产品负责人 2026-09-03：「besides, what is color palette and your choice?
-- style? hero image, do you need? can generate it here - your website is
-- becoming real.」
--
-- 配色从第一关留下的关键词派生，所以这一关不是「挑一个你喜欢的颜色」——它是
-- 「哪一组颜色配得上你说的那个读者」。她挑，理由已经在关键词里了。
ALTER TABLE pbl_site ADD COLUMN palette jsonb NOT NULL DEFAULT '{}'::jsonb;

-- 头图在我们自己 OSS 里的 key。空 = 她没要头图（这是一个合法的选择：
-- 三个版式里有两个本来就没有头图的位置）。
--
-- 🚨 存 key 不存上游那个地址：上游地址带签名会过期，存它等于几天后她的主页
-- 顶上是一张碎图。见 internal/gateway/images.go 的实测记录。
ALTER TABLE pbl_site ADD COLUMN hero_key text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pbl_site DROP COLUMN palette;
ALTER TABLE pbl_site DROP COLUMN hero_key;
