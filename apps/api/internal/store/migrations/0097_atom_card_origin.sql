-- +goose Up
-- 「这副透镜是谁提出来的」——路由器提的，还是学生自己从透镜库里挑的。
--
-- 在这一列之前，AI 路由提出的卡和学生自己召唤的卡写出来的 atom_card 行
-- 逐字节相同：同样的 card_id、同样的 status='proposed'、同样的 anchors。
-- 而这两件事在教学上是完全不同的两回事——「AI 提醒她该查来源了」和「她自己
-- 想到该查来源了」正是 P2 报告里 autonomy（自主性）那一轴要测的信号本身。
--
-- 铁律④「过程即数据」：记录是证据，报告是从证据里长出来的。这条信息**事后
-- 无法重建**——两条路径写下的行没有任何可区分的痕迹。所以它必须在写入的那
-- 一刻就落库，而不是等到需要报告时再去猜。
--
-- 默认 'router'：这条迁移之前的每一行都无从分辨，而 router 是当时唯一由
-- 系统自动发起的路径，把旧行标成 'student' 会凭空发明一次她没做过的自主
-- 选择。旧行因此一律记为 router——保守、且与「不替学生编造」一致。
ALTER TABLE atom_card
  ADD COLUMN origin text NOT NULL DEFAULT 'router'
  CHECK (origin IN ('router', 'student'));

-- +goose Down
ALTER TABLE atom_card DROP COLUMN origin;
