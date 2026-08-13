## Key principles

- on platform (both teacher and student end)
    - like any other data visualization platform, our report should not narrow to the center part of the page, but fills all parts except the left bars.
    - more visualization, instead of many texts.
    - we would have onplatform version and pdf export function. pdf export would be another format, which is not basically a print of the current webpage.



## Content Structure

- basic data:
    - Title, type, start date, end date
    - key action timeline (start, finished framework, finished proposal, finished paper writing, finished the whole project)
    - ai chat turns (including agent and subagent and ai comments. all ai calls); materials read count; total written words; ai comment counts, edit counts.
- abstract
    - general overview (text field, including keywords highlighting. a paragraph that reviews students' key journey)
        - e.g.  {student_name}在该写作项目中与AI深度配合，同时保留了自己完整的深度思考。{student_name} 看到题目联想到了最近读过的一篇公众号文章，且没有直接将公众号作为可靠信源，而是持续追溯到了NASA Ames、Nature Sustainability论文、ESSD/GCB与OWID排放数据、The Economist视频和World Bank背景材料。在研究中还额外关注了反方论证的视频，在写作中对于论证逻辑、数据信源、反方论点及反驳都进行了清晰陈述，在复盘中对于AI使用边界进行了明确表达。
    - Material one sentence (text field, a sentence that summarizes students materials using)
        - e.g. {student_name}一共找到了xx条来源，并准确识别了各条来源之间的论证关系，厘清了每条论证能说明什么，不能说明什么。
    - Writing one sentence (text field, a sentence that summarizes students writing process)
        - e.g. 学生一共进行了x次文章打磨，结论逐步从xxx收束到xxx。
    - AI one sentence (text field, a sentence that summarizes students AI usage)
        - e.g. 学生在过程中明确了AI使用边界，保留了写作和判断由自己进行，AI只进行辅助和建议的边界。
    - Suggestion paragraph (a paragraph with highlighting, about the suggestion towards next practice)
        - e.g. 建议下一次可以主动识别materials里面哪些与当前论证的相关性较弱，可以尝试收入兔子洞。这样在论证写作的时候将会更加容易。
    - suggestion sentences: list, 3-4 sentences
        - e.g. 如果继续扩展这篇文章：可以单独开一段讨论per-capita / historical emissions，但不要在800词正文里随手加。
        - 如果要重新加入MEE：必须补齐真实URL、打开理由、摘录、来源功能，并说明它是policy context，不是独立结果证明。
        - 如果要更严谨使用视频：保留观看时间点、人物、场景、视频字幕或自己的观看笔记，避免出现AI生成的视频细节。
    - recommend courses: 0-3 ids of recommended courses and why.
- event list.
    - a detailed event list, about student's time line.
    - merge the chat, student's action (including. reading room, adding graph node, writexxx, review, etc. briefly summarize each part's action (less than 20 words) and ai turn - the count of all ai calls.)
- Material list
    - a detailed material list, added time, source, link , function (where does it be used), comment
- D-axis result
    - (what is D-axis? Cognitive Depth - 认知深度。Delphi技能谱系；想得有多深)
    - D1_task_understanding  (任务理解与问题表述 - 能否将问题理解清楚透彻，能否结合自身经历逐步限定问题范围，能否将问题进行拆解找到可回答的子问题)
        - (format same for the following dimensions):
        - Level (1-4)
        - Summary: a sentence about students' status on this dimension
        - evidence: a list, of what students' events/chats that support the evaluation of this evidence
        - suggestion: a sentence of suggestion to students
    - D2_materials (证据与信源 - 信源意识，追溯一手来源，区分转述与原文，核查数据)
    - D3_claim_structure （论证结构 - 从主张到证据到推理的完整性，构建完整的论证链条，构造让步段）
    - D4_perspective（视角与偏见 - 多视角比较、识别来源的立场与利益，合理处理饭放证据）
    - D5_iteration （反馈处理与修订 - 理解批评/建议，持续修订迭代，修改措辞也修改论证结构）
    - D6_meta_cognition（反思与元认知 - 从复盘事实，到审视自己的方法与框架，到总结出可迁移的方法论）
- A-axis result
    - (what is A-axis? Intellectual Autonomy - 智识自主。Delphi 倾向谱系；是否自己驱动认知；)
    - A1_direction（方向自主 - 是否主动制定目标和路线；是否自己拟定或修改计划；是否主动增加探索深度）
    - A2_autonomy（发起自主 - 在无人要求的情况下进行信源及论证的提问、核查、主动修订）
    - A3_borderline（边界主权 - 给AI设边界，拒绝建议并给出理由，守住自己的想法和原则）
    - A4_critical（对抗与检验 - 主动寻找反方，带证据主动质疑AI，抵抗AI迎合陷阱）
    - A5_responsibility（判断与署名 - 自己为判断负责，自己为结论给理由，自己进行反思）
    - A6_truth （求真优先 - 在证据和自己的观点冲突时接受反证，修正立场）
- prompt lens (a list of 3-10 prompts of students prompts that are great or need to be enhanced, both in subagent and in chat can be candidates)
    - summary (a paragraph summarize students' prompt usage)
    - prompt list - each prompt:
        - phase/stage
        - student_prompt
        - comment
        - suggestion
- tool card & subagent usage (a list)
    - tool card/subagent name
    - stage
    - Purpose
    - summary
- risky summary (a list that concludes students' risky behaviour that may violate some principles.)
    - each element:
        - type (AI代劳/缺少信源/论证逻辑/数据口径/兔子洞跑题)
        - behaviour
        - Suggestion

## Design-Web

1. change left sidebar 图鉴 to 评估, and it has two tabs, 成长报告，图鉴
2. in 成长报告，top「你的思维印记」
    1. under that is a timeline, like those blog's vertical timeline. one report, one timepoint, with date, paper name.
    2. click this report, go to a report page
3. report page. left is a 目录, a ruler-like 目录 where it would scroll and change according to my hover.
    1. 9 parts as above-stated
    2. can also scroll the report to view the relevant parts.
4. try to visualize the parts, event list can be a timeline? 
    1. material list as a bullet point list or do you have other ideas?
    2. D/A axis?  can we have better visualization, to avoid the report looks like all texts and don't look like cool? please help me about this.
    3. we don't visualize D/A axis levels, because it is not accurate now, we use color to indicate the level.

## Design-PDF  

on each report page, we can export pdf.

pdf is different from web, we need a document-like one. each part can be some introduction paragraph (what is this part, how does this mean), and a table of the detailed data.

we may need to first compose a template for this? and so that we can insert data into it and can generate the report?