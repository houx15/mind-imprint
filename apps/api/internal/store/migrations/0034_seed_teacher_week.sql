-- +goose Up
-- D2: 0029's events all sit within hours of now(), so every student reads 1
-- active day, every delta is +N off a zero baseline, and no rule except
-- never_used can fire. Spread them across real days, add a previous-week arm,
-- and give one student a second (older) report so 深度升档 has a baseline.
--
-- Anchored on date_trunc('week', now()) — Postgres's ISO week starts Monday,
-- the same Monday teacher.WeekWindow computes.

-- 1. Spread the existing studio events across distinct days inside this week.
--    ctid ordering is arbitrary but stable within one statement; all we need is
--    that a student's two events land on two different days. Both UPDATEs are
--    scoped by disjoint user_id sets and `created_at > week_start` only — NOT
--    an upper bound tied to `now()` — because the upper bound in an earlier
--    draft of this migration silently no-ops whenever the migration runs on a
--    Wednesday or later in the ISO week (0029's events sit at `now() - a few
--    hours`, so `created_at < week_start + 2 days` only matches on Mon/Tue).
UPDATE event SET created_at = date_trunc('week', now()) + interval '1 day 9 hours'
 WHERE type = 'prompt_sent' AND created_at > date_trunc('week', now())
   AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000912',
                   '00000000-0000-0000-0000-000000000915','00000000-0000-0000-0000-000000000918');
UPDATE event SET created_at = date_trunc('week', now()) + interval '3 days 14 hours'
 WHERE type = 'prompt_sent' AND created_at > date_trunc('week', now())
   AND user_id IN ('00000000-0000-0000-0000-000000000913','00000000-0000-0000-0000-000000000916',
                   '00000000-0000-0000-0000-000000000917');

-- 2. Extra distinct-day turns so 林知远/苏晚 read as genuinely active.
INSERT INTO event (user_id, project_id, surface, type, created_at) VALUES
  ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000951','studio','prompt_sent', date_trunc('week', now()) + interval '2 days 10 hours'),
  ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000951','studio','prompt_sent', date_trunc('week', now()) + interval '4 days 11 hours'),
  ('00000000-0000-0000-0000-000000000918','00000000-0000-0000-0000-000000000957','studio','prompt_sent', date_trunc('week', now()) + interval '2 days 16 hours'),
  ('00000000-0000-0000-0000-000000000918','00000000-0000-0000-0000-000000000957','studio','prompt_sent', date_trunc('week', now()) + interval '3 days 9 hours');

-- 3. 陈屿 (914) has no project in 0029, so scope these on the seeded course —
--    the course_id scope 0031 just added, and the one course 0012 seeds
--    (verified present at this point in the migration chain). Five distinct
--    days LAST week, one this week: 本周掉线 fires with a real 5 天 → 1 天
--    evidence line — but only once enough of THIS week has elapsed for
--    teacher.PrevWindow's comparison window to actually reach that far.
--
--    PrevWindow deliberately does NOT return a full previous week: it returns
--    [prevStart, prevStart + elapsed) where elapsed = now − thisWeekStart, so
--    week-to-date is compared against week-to-the-same-moment last week. That
--    means dropped_off (ActiveDays <= PrevActiveDays - 2) is structurally
--    unreachable early in the week — there is no seed that can force it, because
--    the comparison window itself is short. The best a seed can do is make the
--    rule reachable as early as honestly possible: five distinct UTC dates at
--    00:05 (Mon–Fri), rather than spread through the day, means the window only
--    has to elapse a few minutes past a day boundary to pick up one more day,
--    not until 10:00. With events pinned to the very start of each day,
--    PrevActiveDays reaches 3 (the 3rd of 5 distinct dates) as soon as elapsed
--    passes 2 days + 5 minutes — i.e. from Wednesday 00:05 UTC of the current
--    week onward. Before that (Monday 00:00 through Wednesday 00:05), the rule
--    cannot fire no matter what is seeded, and that is correct behaviour, not a
--    seeding gap.
INSERT INTO event (user_id, course_id, surface, type, created_at)
SELECT '00000000-0000-0000-0000-000000000914', c.id, 'course', 'course_message',
       date_trunc('week', now()) - interval '7 days' + (n || ' days 5 minutes')::interval
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c, generate_series(0,4) AS n;
INSERT INTO event (user_id, course_id, surface, type, created_at)
SELECT '00000000-0000-0000-0000-000000000914', c.id, 'course', 'course_message',
       date_trunc('week', now()) + interval '1 day 10 hours'
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c;

-- 4. step_viewed on both weeks so 完成课程节 has a real delta (more this week).
INSERT INTO event (user_id, course_id, surface, type, payload, created_at)
SELECT u.uid, c.id, 'course', 'step_viewed', jsonb_build_object('ordinal', n),
       date_trunc('week', now()) + interval '2 days 13 hours'
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c,
     generate_series(1,3) AS n,
     (VALUES ('00000000-0000-0000-0000-000000000911'::uuid),
             ('00000000-0000-0000-0000-000000000915'::uuid),
             ('00000000-0000-0000-0000-000000000918'::uuid)) AS u(uid);
INSERT INTO event (user_id, course_id, surface, type, payload, created_at)
SELECT u.uid, c.id, 'course', 'step_viewed', jsonb_build_object('ordinal', n),
       date_trunc('week', now()) - interval '5 days'
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c,
     generate_series(1,2) AS n,
     (VALUES ('00000000-0000-0000-0000-000000000911'::uuid),
             ('00000000-0000-0000-0000-000000000915'::uuid)) AS u(uid);

-- 5. 吴桐 (915) gets two project reports, one well before this week and one
--    inside it: max depth L2 → L3, so 深度升档 fires and one bucket change
--    appears. Minimal but complete axis coverage, same convention as 0029
--    (seeds bypass the Go normalizers). created_at is set explicitly (not the
--    column default) — student_evaluation orders by evaluations.created_at,
--    and an "older" report that defaulted to now() would tie with (or beat)
--    the newer one.
--    Anchored on date_trunc('week', now()), like every other timestamp in this
--    migration — NOT bare `now() - interval`. reports_this_week filters on
--    `created_at >= week_start AND < week_end` (this week's Mon–Mon window), so
--    an unanchored "1 day ago" lands in last week's ISO window whenever the
--    migration is applied on a Monday or Tuesday, silently moving 深度升档's
--    newer report out of the current week. Anchoring both to week_start keeps
--    the newer row inside the current week and the older row well before it
--    on every day of the week the migration might run.
INSERT INTO evaluations (id, project_id, scores, narrative, model, tier, prompt_tokens, completion_tokens, cost_estimate, status, completed_at, created_at) VALUES
  ('00000000-0000-0000-0000-000000000981','00000000-0000-0000-0000-000000000954',
   $j${"depthAxis":[{"code":"D1","level":"L2","evidence":"问题还停在「食物浪费严不严重」这一层。"},{"code":"D2","level":"L2","evidence":"引用了两条校园公众号推文，没有查证原始数据。"},{"code":"D3","level":"L2","evidence":"结论与证据之间缺一步推理。"},{"code":"D4","level":"L1","evidence":"只写了自己这一方的说法。"},{"code":"D5","level":"L2","evidence":"改了措辞，没有改判断。"},{"code":"D6","level":"L2","evidence":"复盘只写了「下次要更认真」。"}],"autonomyAxis":[{"code":"A1","level":2,"opportunity":"given_taken","evidence":"方向由老师给定，自己接了下来。"},{"code":"A2","level":2,"opportunity":"given_taken","evidence":"多数提问在被提示之后。"},{"code":"A3","level":1,"opportunity":"given_not_taken","evidence":"没有对 AI 设过边界。"},{"code":"A4","level":1,"opportunity":"given_not_taken","evidence":"没有请 AI 挑过毛病。"},{"code":"A5","level":2,"opportunity":"given_taken","evidence":"结论署了自己的名，但理由是 AI 给的。"},{"code":"A6","level":2,"opportunity":"given_taken","evidence":"更在意写完，不太追问真假。"}],"narrative":"起步阶段：任务在推进，判断还多在外部。"}$j$,
   '起步阶段：任务在推进，判断还多在外部。','deepseek-v4-pro','flagship',0,0,0,'done',
   date_trunc('week', now()) - interval '10 days', date_trunc('week', now()) - interval '10 days'),
  ('00000000-0000-0000-0000-000000000982','00000000-0000-0000-0000-000000000954',
   $j${"depthAxis":[{"code":"D1","level":"L3","evidence":"问题收窄到「本校午餐时段的浪费成因」。"},{"code":"D2","level":"L3","evidence":"改用了食堂三周的称重记录作为主证据。"},{"code":"D3","level":"L2","evidence":"推理链补上了一步，但仍有跳跃。"},{"code":"D4","level":"L2","evidence":"补了食堂工作人员这一方的说法。"},{"code":"D5","level":"L3","evidence":"根据 AI 指出的漏洞改了样本说明。"},{"code":"D6","level":"L2","evidence":"复盘写清了自己改了什么，还没写为什么。"}],"autonomyAxis":[{"code":"A1","level":3,"opportunity":"given_taken","evidence":"自己把范围从全校收到了午餐时段。"},{"code":"A2","level":3,"opportunity":"given_taken","evidence":"三次主动开口提问。"},{"code":"A3","level":2,"opportunity":"given_taken","evidence":"说过一次「不要帮我写」。"},{"code":"A4","level":3,"opportunity":"given_taken","evidence":"三次要求 AI「挑我方法里最致命的问题」，并据此改了样本说明。"},{"code":"A5","level":3,"opportunity":"given_taken","evidence":"结论是自己下的，理由也自己写。"},{"code":"A6","level":3,"opportunity":"given_taken","evidence":"发现数据对不上时自己回去核了一遍。"}],"narrative":"这一轮开始主动请 AI 当反方，判断收回了自己手里。"}$j$,
   '这一轮开始主动请 AI 当反方，判断收回了自己手里。','deepseek-v4-pro','flagship',0,0,0,'done',
   date_trunc('week', now()) + interval '2 days', date_trunc('week', now()) + interval '2 days');

-- +goose Down
DELETE FROM evaluations WHERE id IN ('00000000-0000-0000-0000-000000000981','00000000-0000-0000-0000-000000000982');
DELETE FROM event WHERE type = 'step_viewed'
   AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000915','00000000-0000-0000-0000-000000000918');
DELETE FROM event WHERE user_id = '00000000-0000-0000-0000-000000000914' AND type = 'course_message';
DELETE FROM event WHERE type = 'prompt_sent'
   AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000918')
   AND created_at > date_trunc('week', now()) + interval '2 days';
