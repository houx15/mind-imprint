-- +goose Up
-- 课程有了受众；项目可以把她送去上一课。
--
-- 两件事放在一次迁移里，因为第二件依赖第一件：课程先能分给不同的学生，项目里
-- 「递哪一课」才有得挑。
--
-- ── 一 · course.audience ───────────────────────────────────────────────────
--
-- 产品负责人 2026-09-04：「the current course, we need to have a level or label
-- so that we can present different courses to different kind of students
-- (currently pro or lite)」。
--
-- 所以这是一个**受众标签**，不是难度分级：它回答的是「这门课摆给谁看」。今天
-- 这个维度只有两个值（lite / pro），以后会有别的，所以存成 text[] 而不是一个
-- 枚举列——加一种受众不该需要一次迁移。
--
-- 🚨 词表和 category 一样**在应用层校验，不写 DB CHECK**（见 0075 的注释）：
-- 词表的唯一真相源是 TS 契约，DB 再抄一份就一定会漂移。
--
-- 默认 {lite,pro} 是产品负责人指定的过渡态——「currently all courses available
-- in lite version and pro version, and later I will add course or do some
-- changes」。空数组约定为「不限受众」，两边都看得见；这样一行既没被显式标注、
-- 又不小心被清空的课程不会从所有人的目录里消失。
ALTER TABLE course
  ADD COLUMN audience text[] NOT NULL DEFAULT ARRAY['lite','pro']::text[];

-- ── 二 · pbl_course_assignment ─────────────────────────────────────────────
--
-- 印记在项目里把一门课递给她：「你要做的这件事需要 X，我们有一课讲这个」。
--
-- 为什么单独一张表，而不是塞进 pbl_tool_instance.result：
--
--   · 一件工具的一生是「递出 → 打开 → 了结」；一次上课的一生是「被指派 →
--     去上 → 上完 → 回来说这一课对我这个项目有什么用」。后者跨越工具实例：
--     她可能把工具卡关了、过两天再回来上完。
--   · 回灌（pbl_refeed.go）要读的是「她上完哪一课、回来写下什么」。埋在一团
--     自由 JSON 里，那段代码就得先猜结构。
--
-- course_slug 是弱引用（text，没有 FK）：课程按 slug 寻址，而一门课下线之后
-- 这条指派仍然是她这个项目里真实发生过的一件事，不该跟着消失（铁律④）。
CREATE TABLE pbl_course_assignment (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id     uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  session_id  uuid REFERENCES pbl_session(id) ON DELETE SET NULL,
  course_slug text NOT NULL,
  -- 印记为什么现在递这一课。没有理由的课和没有理由的工具一样是伏击。
  why         text NOT NULL DEFAULT '',
  -- 她上完回来自己写下的那句话：这一课对我这个项目有什么用。
  -- 🚨 这一句就是闭环的内容本身——回灌读的是它，不是「她上过某门课」这种元信息。
  takeaway    text NOT NULL DEFAULT '',
  finished_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_course_assignment_atom_idx
  ON pbl_course_assignment (atom_id, created_at);
-- 同一个项目里同一门课只指派一次：印记重复递同一课，她看到的是两张一样的卡。
CREATE UNIQUE INDEX pbl_course_assignment_unique
  ON pbl_course_assignment (atom_id, course_slug);

-- +goose Down
DROP TABLE pbl_course_assignment;
ALTER TABLE course DROP COLUMN IF EXISTS audience;
