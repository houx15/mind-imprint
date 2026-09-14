# Lite 学生电脑版 · 完成审查

范围为当前 `routing.ts` 的学生端路由、各学习工作区、十五类项目工具、公开页面及账号状态。以现有功能和接口为边界。桌面为主要验收对象。

| 要求 | 当前实现与证据 |
| --- | --- |
| 官网 v2 / A 学习空间风格 | 已确认的首页、书签 AI、共享 Lite 色阶与纸张背景保留；首页与入口截图见 review.html、student-verification.json |
| 使用并迭代 design tokens | lite.css / lite.ts 管理共享主题；student-surfaces.css 管理布局；主题选择仅在设置中，默认松石青 |
| 阅读图片与文字选择 | 真实文章图片已渲染；段号与正文分离；选区可引用，点击段落仍有整段工具；reading-text-selection.png、reading-quest-preview.png |
| 阅读任务与闯关反馈 | 当前阶段、任务路线、预览及原文定位；完成/跳过分别反馈；desktop-final-verification.json 中 quest-transitions 确认状态不混淆 |
| 报告更适合阅读和分享 | 阅读采用连续手记版式，加入收获与来源归属；真实 PNG 导出非空白；writing-finished.png、reading-finished.png、report-export.png |
| 插画与空状态更丰富 | 四张新增透明插画，分配到任务、探索、工具与报告；写作段落、材料区、共用空状态有场景插画；illustration-generation.md |
| 项目材料区清晰 | 只列已有材料，空状态集中展示；project-room.png |
| 工具可操作、有反馈 | 便签关系选择、想法整合预览、结构材料目标；拖拽原位轮廓/目标提示/Esc 取消/保存结果高亮；canvas-feedback-verification.json、tool-interaction-verification.json |
| 探索有吸引力，明暗主题可用 | 每日选题索引、核心问题、切换预览、兴趣关联、原有详情入口；explore-light.png、explore-dark.png 及 discovery 浏览器检查 |
| 其余学生页面统一 | 路由逐项对应：首页、三个入口、分级库、课程目录/详情/播放/结束、写作各阶段、兴趣树/引导、个人主页/访客、设置、账号；student-verification.json 与桌面复核记录 |
| 后端接口保持 | 本分支相对 main 的 apps/api 和 packages/contracts 无差异；浏览器夹具核对任务预览不产生保存、整合确认仍调用原接口 |
| 分支隔离与 main 同步 | codex/lite-visual-refresh；本地 main 无未合入提交。未合回 main、未部署 |

## 验证结论

- Lite 测试 66 文件 / 644 项通过，类型检查通过，生产构建通过。
- 当前桌面复核覆盖 13 个页面/阶段，在 1512×1040 与 1366×900 下无页面错误、无页面横向溢出；查看了阅读、写作、项目、探索与导出的代表截图。
- 两类画布 Esc 取消均恢复原位且无保存请求；想法松开仅预览，确认后才保存并突出结果。
- 阅读完成、跳过反馈与服务端任务状态保持一致。
- 既有全页面记录继续覆盖账号、课程完整路径、十五类工具、报告分享及兴趣引导；本轮未改变这些功能的 API 行为。
- 临时浏览器挂载入口已移除。截图与数据均为本地夹具，结论不包括生产端生成质量、真实账号鉴权或发布流程。

现阶段的桌面界面迭代已完成。教师布置作业的首页优先级属于用户明确留待后续的功能；Pro 与教师端不在本轮范围内。
