-- +goose Up
-- 写作房间的「脚手架」转向（2026-08-27 产品裁定）。
--
-- 旧模型：AI 根据学生构思阶段说过的话，直接生成一份提纲候选。新裁定推翻它——
-- **AI 永远不直接生成提纲**。AI 只做两件事：从一个固定的、通用的结构库里
-- 推荐一副骨架（骨架里只有「你的立场」「最强理由」这类通用块名，没有一个字
-- 是她的内容），以及在每一块上给出引导性的问题。每一块填什么，全部由学生
-- 用自己的想法和经历来写。
--
-- 这条迁移只加列、不改也不删任何既有列，因此对 pro 端零影响（writing 系列
-- 表是 lite 独有的 atom 形态表，0099）。

-- structure_key：她选中的骨架在结构库里的 id（''=还没选）。既是渲染依据，
-- 也是过程信号——「她用了哪副骨架、后来换没换」本身就是 铁律④ 要的证据。
ALTER TABLE writing ADD COLUMN structure_key text NOT NULL DEFAULT '';

-- setup_at：进入房间时那个「设定」弹窗完成的时刻。NULL = 还没设定过，前端
-- 据此决定是否弹窗。用时间戳而不是布尔值，因为「什么时候定下语言和篇幅」
-- 同样是过程记录的一部分。
ALTER TABLE writing ADD COLUMN setup_at timestamptz;

-- role：这一块在骨架里的**通用**块名（「反方最强的说法」…），来自结构库，
-- 不是学生写的。与 text 严格分开：text 永远是她自己的一句话。两者混在一列
-- 里就再也分不清哪句是模板、哪句是她的思考——而报告要的正是这个区分。
ALTER TABLE writing_outline ADD COLUMN role text NOT NULL DEFAULT '';

-- 四阶段收拢为三阶段（结构/段落/成稿）：'ideate' 的内容已经移进设定弹窗和
-- 开场对话里，不再有对应页面。既有行平移到 'outline'，避免它们停在一个前端
-- 已经不再渲染的阶段上。CHECK 约束**不动**——收紧它要重建约束、且会让任何
-- 尚未更新的客户端硬失败；三阶段词表由 Go 层把关（writing_stage.go）。
UPDATE writing SET stage = 'outline' WHERE stage = 'ideate';

-- 新建的写作也不该再落在 'ideate' 上。默认值改成 'outline'（结构）——设定弹窗
-- 关掉之后，她要做的第一件事就是挑一副骨架，这就是第一步。
ALTER TABLE writing ALTER COLUMN stage SET DEFAULT 'outline';

-- 既有写作一律视为「已设定」：它们是在旧流程下建的，不该在她回来继续写时
-- 突然被一个设定弹窗拦住。只有此后新建的写作才会带 NULL 走进弹窗。
UPDATE writing SET setup_at = now() WHERE setup_at IS NULL;

-- +goose Down
ALTER TABLE writing ALTER COLUMN stage SET DEFAULT 'ideate';
ALTER TABLE writing_outline DROP COLUMN role;
ALTER TABLE writing DROP COLUMN setup_at;
ALTER TABLE writing DROP COLUMN structure_key;
