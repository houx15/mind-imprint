A general note: we now have too many round-corner rectangle boxes, making the whole website more like a proto and less like a technical product.

can we try to avoid so many round-cornered cards, buttons? either use full-round-corner, or use shades or color change to show accent (e.g. the nav bar). use flat rectangle shaded cards?

use pure black background color.

rename 产品 solution - 解决方案

rename 产品总览 - 完整案例

---

the current 产品总览 page:

title: 一个真实的学术写作案例 - 「中国有没有让地球变得更可持续？」

delete the "产品" icon.

"课程把方法练成习惯，项目让她做自己的真实课题，评估写下她是怎么想的，教师端让这一切看得见。" -> "学生学习、探索、思考，AI辅助、记录、搭建脚手架。"



delete the four products card, directly begin the journey.

we have two methods to show the journey:

1) like apple, left side have several cards and with scrolling transit from one to the next
2) a line 不规则的线条, 串联起来不同的节点，make the scroll continuous.

steps: (each step, left side is the step name, the whole right side is the asset. make the asset fill the whole right side)

(assets would be mp4 or web, for mp4, directly play them, like a gif, no need to replay)

(left texts can gradually show like 打字机, with scrolling)

- 课程-情境化的知识输入
    - 通过案例学习系统了解顶尖名校学术写作的标准
    - Assets: 01-course.webp
- 立题-与AI一同讨论研究议题
    - AI引导学生将宽泛想法详细定义为研究主题
    - assets: 02-proposal.mp4
- 研究-系统化的溯源检索
    - 引导学生追溯信息来源，严谨研究
    - assets: 03-research.mp4
- 研究-兔子洞收集岔路的探索
    - 兔子洞图谱记录学生的探索轨迹
    - assets: 04-warrenmap.webp
- 阅读-深入考察数据、信息、来源
    - AI使用学科透镜和工具包陪学生一同深度阅读
    - assets: 05-guided-reading.webp
- 写作-AI只提供引导，每个词都自己写
    - AI在需要时为学生提供引导性问题和写作反馈
    - assets: 06-writing.mp4
- 评估-详细的过程评估报告，记录每次尝试
    - 成稿之后，自己先写下复盘。系统随后生成九个板块的评估报告
    - assets: 07-evaluation.mp4
- assets are at @apps/site/assets/user-journey-0825





---

courses page:

general logic: first introduce course, with three examples. assets are at @apps/site/assets/courses-0825

你身体里的小队：IFS 内在部分与注意力审计

SIFT 信息横向调查：从一条推文到一篇有立场的议论文

信息过滤神器：从 CRAAP 到 CRRAAB



three courses, about inner feeling, mental ability; and two about cognitive, critical thinking. 

still, left is the category and right is the full size assets. scroll to change from one to the next, until the third, then go to the next.



then course list, with real materials (the same as online platform, we have course cards, with the names, and the descriptions) - but currently the courses covers are in students' private oss. I haven't considered out how to solve this maybe I should move it to the public site?



by default we only show two line of courses (with able to categorization)

but we can also show all by clcking a expand button.



then introduce cards, where we let students to collect, to 沉淀方法论. do we need to show all our cards here? or some examples?

with also the students' end card content examples.



