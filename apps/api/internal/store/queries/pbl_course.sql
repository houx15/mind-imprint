-- 项目里的课程指派（migration 0133）。
--
-- 印记在项目里递出一门课，她去上，上完回来写一句「这一课对我这个项目有什么
-- 用」。那一句是闭环的内容本身，回灌读的就是它。

-- name: AssignPblCourse :one
-- 同一个项目里同一门课只指派一次（唯一索引）。重复递就更新理由——印记第二次
-- 递它，说明这一刻的理由和上一次不一样，而她屏幕上仍然只该有一张卡。
INSERT INTO pbl_course_assignment (atom_id, session_id, course_slug, why)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id, course_slug) DO UPDATE SET why = EXCLUDED.why
RETURNING *;

-- name: ListPblCourseAssignments :many
SELECT * FROM pbl_course_assignment WHERE atom_id = $1 ORDER BY created_at;

-- name: GetPblCourseAssignment :one
-- 带上 user_id，调用方一次查询就能判归属，不用先取 atom 再比一次。
SELECT c.*, a.user_id
FROM pbl_course_assignment c JOIN atom a ON a.id = c.atom_id
WHERE c.id = $1;

-- name: FinishPblCourseAssignment :one
-- 她上完回来了。takeaway 由 Go 校验（trim 之后非空）——一次没留下任何一句话
-- 的上课，对这个项目来说等于没发生，而闭环要回灌的正是那句话。
-- finished_at 用 COALESCE 保持首次完成的时间：她后来改了那句话，"什么时候上完
-- 的"不该跟着变。
UPDATE pbl_course_assignment
SET takeaway = $2, finished_at = COALESCE(finished_at, now())
WHERE id = $1
RETURNING *;
