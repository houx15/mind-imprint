-- +goose Up
-- Only pristine, unapproved initial proposals. Keep the old version and all work.
LOCK TABLE pbl_plan_version, pbl_plan_step IN SHARE ROW EXCLUSIVE MODE;
WITH candidates AS (
 SELECT v.atom_id FROM pbl_plan_version v
 JOIN pbl_site site ON site.atom_id = v.atom_id
 WHERE v.version = 1 AND v.approved_at IS NULL AND v.decided_by = 'ai'
 AND v.summary = '五步：想清楚给谁看 → 去看真的个人网站 → 给网站定调子 → 生成与分工 → 审改上线。' AND v.reason = '这是做个人网站通常的顺序。请审核计划并确认，或提出修改意见。'
 AND NOT EXISTS (SELECT 1 FROM pbl_plan_version other WHERE other.atom_id=v.atom_id AND other.id<>v.id)
 AND NOT EXISTS (SELECT 1 FROM pbl_pending_change c WHERE c.atom_id=v.atom_id)
 AND (SELECT jsonb_agg(to_jsonb(s) - 'id' - 'version_id' - 'created_at' ORDER BY s.ordinal)
      FROM pbl_plan_step s WHERE s.version_id=v.id) = '[{"title": "想清楚给谁看", "blurb": "一个网站该是什么样，由读它的人决定，所以先把这个人想清楚。", "goal": "定下一个真实的受众，和三到六个关键词。", "you_bring": "你对这几个人的判断。", "i_bring": "两三个可能的受众，每个带一张生成的画像和一组关键词。", "decide": "哪一个受众是真的，哪些关键词留下。", "then_bring": "留下的受众和关键词。", "ordinal": 1, "status": "tentative"}, {"title": "去看真的个人网站", "blurb": "个人网站是有作者的。先看几个真站是怎么做的。", "goal": "看懂三个真站的结构，再搭出你自己的结构。", "you_bring": "三个你自己找到、真的喜欢的网站。", "i_bring": "六个真站做起点，以及你贴进来的每一站的结构分析。", "decide": "哪些结构值得学，你的结构包含哪几块。", "then_bring": "一张你自己的结构导图。", "ordinal": 2, "status": "tentative"}, {"title": "给网站定调子", "blurb": "配色、风格、头图，决定别人第一眼看到什么。", "goal": "定下视觉基调，并补齐我手上还缺的材料。", "you_bring": "我还没有的那些——你讲给我听。", "i_bring": "我从你读过、写过、做过的东西里已经找到的材料，三组配色，几张头图草稿。", "decide": "配色、风格、要不要头图。", "then_bring": "定下的配色、风格和头图。", "ordinal": 3, "status": "tentative"}, {"title": "我来生成，你来分工", "blurb": "这一页由我生成，页面上说你的话的每一句归你。这份分工会写清楚。", "goal": "拿到第一版页面。", "you_bring": "你对这份分工的判断。", "i_bring": "结构、排版、配色、头图，和一份分工。", "decide": "哪些活归我，哪些归你。", "then_bring": "第一版页面。", "ordinal": 4, "status": "tentative"}, {"title": "逐处审改，然后上线", "blurb": "AI 可能出错，需要对它产出的内容做一次深度审核。", "goal": "审完、改完、拿到你的链接。", "you_bring": "你的意见。", "i_bring": "这一页，以及几处我自己也不确定的地方。", "decide": "这一页哪里还不对。", "then_bring": "上线的链接。", "ordinal": 5, "status": "tentative"}]'::jsonb
), proposed AS (
 INSERT INTO pbl_plan_version(atom_id,version,summary,reason,decided_by)
 SELECT atom_id,2,'五步：确定读者 → 构思与试用第一幕 → 自我介绍与作品展示 → 制作与分工 → 审改上线。','主页创作流程已更新：从自由风格与意象出发，试用第一幕，再介绍自己与展示作品。旧计划与已有材料保留，请核对新版安排。','ai'
 FROM candidates RETURNING id
)
INSERT INTO pbl_plan_step(version_id,ordinal,title,blurb,goal,you_bring,i_bring,decide,then_bring,status)
SELECT p.id,s.ordinal,s.title,s.blurb,s.goal,s.you_bring,s.i_bring,s.decide,s.then_bring,s.status
FROM proposed p CROSS JOIN jsonb_to_recordset('[{"title": "想清楚给谁看", "blurb": "不同读者关注的内容不同，请结合具体的人完善人物板。", "goal": "确定主要读者，完善各自的人物板与内容关键词。", "you_bring": "你对具体读者的了解和希望展示的内容。", "i_bring": "角色插画、可编辑人物卡，以及依据卡片内容生成的关键词总结。", "decide": "选择哪些读者，每个人关注什么，你准备展示什么。", "then_bring": "留下的受众和关键词。", "ordinal": 1, "status": "tentative"}, {"title": "构思与试用第一幕", "blurb": "从喜欢的感觉出发，选择意象，制作并试用第一幕。", "goal": "把自己的风格与意象变成能体验的第一幕。", "you_bring": "喜欢的感觉、画面构思和试用后的修改意见。", "i_bring": "意象联想、提示词整理、代码草稿与版本对照。", "decide": "访客第一眼看见什么，可以做什么，哪些效果需要修改。", "then_bring": "第一幕构思与试用记录。", "ordinal": 2, "status": "tentative"}, {"title": "介绍自己与展示作品", "blurb": "请先构思自我介绍，再安排作品的展示方式。", "goal": "让读者了解你是什么样的人，以及你做过什么。", "you_bring": "自己的介绍、真实作品与制作过程。", "i_bring": "可编辑的模块结构与逐项核对。", "decide": "展示哪些内容，用什么顺序和方式呈现。", "then_bring": "已确认的主页结构与自己的内容。", "ordinal": 3, "status": "tentative"}, {"title": "我来生成，你来分工", "blurb": "这一页由我生成，页面上说你的话的每一句归你。这份分工会写清楚。", "goal": "拿到第一版页面。", "you_bring": "你对这份分工的判断。", "i_bring": "结构、排版、配色、头图，和一份分工。", "decide": "哪些活归我，哪些归你。", "then_bring": "第一版页面。", "ordinal": 4, "status": "tentative"}, {"title": "逐处审改，然后上线", "blurb": "AI 可能出错，需要对它产出的内容做一次深度审核。", "goal": "审完、改完、拿到你的链接。", "you_bring": "你的意见。", "i_bring": "这一页，以及几处我自己也不确定的地方。", "decide": "这一页哪里还不对。", "then_bring": "上线的链接。", "ordinal": 5, "status": "tentative"}]'::jsonb)
AS s(ordinal integer,title text,blurb text,goal text,you_bring text,i_bring text,decide text,then_bring text,status text);

-- +goose Down
-- Historical proposals may have since been approved or edited; retain them.
SELECT 1;
