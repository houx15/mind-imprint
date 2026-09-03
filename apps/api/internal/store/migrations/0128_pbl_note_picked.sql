-- +goose Up
-- 她挑出来先试的那条办法，和为什么先试它。
--
-- 🚨 线上走查抓到的：她在「解决方案」里挑了第三条，界面把 picked 塞进 onFinish
-- 的 payload 里就完事了——而**印记从来看不到 onFinish 的 payload**，回灌是回头
-- 读表的。表里没有这一列，于是印记只好照着列表里的第一条说「你那条点子说……」，
-- 说的是她没挑的那一条。
--
-- 有终点信号、但做出来的东西没回到印记那儿，就不算闭环。
ALTER TABLE pbl_note ADD COLUMN picked_at timestamptz;
ALTER TABLE pbl_note ADD COLUMN pick_why text NOT NULL DEFAULT '';

-- 这张纸是她自己拖过的吗。
--
-- 列名不叫 placed：0125 已经有一个「放进结构里」的 placed 语义，
-- 两个 placed 会互相冒充。
--
-- 🚨 同一次走查抓到的第二件：坐标视图打开时，回灌给**每一条**便签都缀上一句
-- 「（她摆在「很要紧，而且我确定」那一角）」——包括她根本没碰过的。
-- 观察日记带回来的便签位置是 boardSpot() 按座位号算的，点子默认 (0,0)，
-- 而 (0,0) 在坐标视图里恰好是「我确定 + 很要紧」那一角。
--
-- 于是印记会当着她的面，把代码排的座位说成是她的判断。这比不说更糟：
-- 她要么以为自己做过这个判断，要么发现印记在编。只有她亲手拖过的才算数。
ALTER TABLE pbl_note ADD COLUMN dragged boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE pbl_note DROP COLUMN picked_at;
ALTER TABLE pbl_note DROP COLUMN pick_why;
ALTER TABLE pbl_note DROP COLUMN dragged;
