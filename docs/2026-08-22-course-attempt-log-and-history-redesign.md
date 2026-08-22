# 课程学习记录重构：可重学的「尝试日志」+ 更好的记录页

**日期：** 2026-08-22 · **状态：** 已定稿，实施中

## 目标

1. **课程列表页更宽 + 一行三门**（不再居中窄栏、硬 2 列）。—— 纯前端，已完成。
2. **学习记录页重排**：按时间列出；已学完点击 → 看**当时那一次**的报告；学习中点击 → 继续。
3. **尝试日志（attempt log）数据模型**：
   - 学习中：一条活动记录，日期＝最后一次学习时间，随进度更新。
   - 学完：该记录**冻结**成一条已完成记录，带一个**固定**的完成日期。
   - **重学**：另起一条新的学习中记录；旧的已完成记录（含它那一次的报告）保留。同一门课因此可以有多条已完成记录，各自对应「当时那一次」。

## 关键发现（决定了做法）

- 2.0 课程报告是**每次从 `course_session` blob 现算**的（无报告表、无 LLM、无缓存）。
- `POST /restart` 会**删除** session → 报告数据随之消失。
- `course_session` 只有 `status` + `updated_at`，**没有完成时间**；且 `UNIQUE(user_id, course_id)` → 每门课每人**只有一行**。

因此「看当时那一次的报告」+「重学＝新记录」必须引入**多行（每次一行）**的模型。好处：报告本就由某一次的 blob 现算，**保留每一次的 session 即等于免费拥有每一次的报告**——无需新建报告存储。

## 数据模型（migration 0077）

`course_session`：
- **DROP** `UNIQUE(user_id, course_id)` → 每门课每人可多行（多次尝试）。
- **ADD** `completed_at timestamptz`（可空）：首次 `status='completed'` 时写入一次并冻结。
- 回填：现有 `status='completed'` 的行 `completed_at := updated_at`。
- 新索引 `(user_id, course_id, created_at DESC)`：「当前尝试」＝按 `created_at` 最新的一行。

**「当前尝试」= 最新一行。** 所有原来按 `(user, course)` 取一行的读，改成取最新一行。

## 受影响的查询（5 处读 `course_session`）

| 查询 | 变更 |
|---|---|
| `GetCourseSessionBySlug`（resume/存档 get-or-create） | `ORDER BY created_at DESC LIMIT 1` |
| `SaveCourseSession`（存档写） | 只更新最新一行；`status='completed'` 且 `completed_at IS NULL` 时写入 `completed_at=now()` |
| `GetCourseSessionProgressBySlug`（目录进度环·单课） | `ORDER BY created_at DESC LIMIT 1` |
| `ListCourseProgressForUser`（目录进度环·批量） | session 支腿改 `DISTINCT ON (slug) … ORDER BY slug, created_at DESC`（每课仅最新一次） |
| `ListCourseHistory`（学习记录） | **保持每次一行**；新增 `attempt_id`、`completed_at`；`ORDER BY COALESCE(completed_at, updated_at) DESC` |

新增查询：
- `GetCourseSessionForReport`（按 attempt id，owner+课程 双重限定）——看某一次的报告。
- `DeleteIncompleteLatestCourseSession`——重学前若最新一次未完成则丢弃它（避免残缺记录堆积）。

## 行为

- **重学（2.0）** = `DeleteIncompleteLatestCourseSession` +（用定义）铸一条全新 `created` 尝试；旧的已完成尝试原样保留。重学（legacy）= 沿用删 `course_progress`。
- **报告**：`GET /courses/{slug}/report?attempt={id}` → 用那一次的 blob 现算；无 `attempt` 参数＝最新一次（沿用现状）。给了 attempt 但不存在/不属于本人 → 404。
- **学习记录**：已完成行 → `report(slug, attemptId)`；学习中行 → `player(slug)`（resume）。日期：已完成用 `completed_at`，学习中用 `updated_at`。

## 前端

- `CourseHistoryItem` +`attemptId?` +`completedAt?`；`getCourseReport(slug, attemptId?)`。
- `CoursesContainer` 深链改为 `initialOpen: { slug, mode: 'detail'|'player'|'report', attemptId? }`；report view 带 `attemptId`。
- `CourseReport` 接受可选 `attemptId`。
- `LearningHistory` 按时间分组重排（今天/昨天/本周/更早），行内明确动作；行 key 用 `attemptId`（多次尝试会有多行同 slug）。
- `CoursesView`：容器 1320px + `repeat(auto-fill, minmax(340px,1fr))`（桌面三列，自适应回落）。—— 已完成。

## 不变式 / 风险

- 边界校验仍归后端、深结构归契约；铁律不触及（记录只读、AI 不代写）。
- 改的是 `course_session` 存量表（试用班在用）：ADD COLUMN + DROP CONSTRAINT + 建索引对现有行安全；回填只补 `completed_at`。**不改 definition，不动 hash → 不会重置进行中的会话。**
- 上线需用户确认（PROD 迁移 + 部署）。
