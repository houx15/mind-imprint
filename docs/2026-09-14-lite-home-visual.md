# Lite 学生首页 · A 版视觉实施

分支：`codex/lite-visual-refresh`。基于 `main` 的 `5f6a1965`，独立 worktree；尚未合并或部署。

## 已完成

- `/` 与 `/index.html` 显示新增学习首页；`/explore` 及阅读、写作、项目、课程、兴趣树、个人主页深链接保留。未知路径仍按原行为进入探索。
- 首页汇总现有阅读、写作、项目接口，按实际活动时间列出最多三条未完成/未归档记录。各来源独立失败时显示错误，不填充示例记录。不新增后端、数据库、模型请求或业务状态。
- 课程取自现有、已经按受众过滤的目录接口；展示前两门及其真实名称和时长，不声称算法推荐。
- 导航与首页采用 A 版布局、独立的八色视觉映射、浅色背景与 V2 插画。只对未保存配色的账号默认使用青绿；已有明确配色保留。背景选择与暗色模式均保留。
- 首页、导航使用新折纸书签角色；现有阅读/写作/项目内的 AI 角色尚未替换。
- 首页在宽屏展开左侧导航，工作区保留紧凑导航；小屏始终紧凑，避免触屏悬停遮挡。

本次有意只完成首页和导航。其他工作区的内部排版、嵌套、主题色阶、设置页色板名称仍保持原实现，后续分批迁移。Pro、教师端、共享 `apps/web` 文件均无改动。Lite 教师端并行实现若修改 `LiteApp.tsx`，合并时需保留其角色分流，把本次学生首页与导航挂在学生分支。

## 验证

- Lite TypeScript 检查通过。
- 路由、鉴权和跨类型学习记录投影：38 项测试通过。
- Vite 生产构建通过，仍有单包超过 500 kB 的提示；没有借此扩大到打包重构。
- Chromium 实际渲染：1512×1040、390×844；青绿/蓝色切换、无学习记录、接口失败与重试、长标题、暗色模式。移动端无页面横向溢出。
- 点击“继续项目”进入原有项目 URL；配色切换仍调用原有保存接口。最后一轮测试浏览器无未捕获运行时错误。
- 浏览器检查采用本地接口拦截的示例数据，未登录生产账号、未修改真实学生数据，也不代表生产端到端验收。

截图：`docs/design/lite-visual-refresh/desktop.png`、`mobile.png`；其余状态截图和浏览器结果保留在同目录。

## 素材来源

三张人物插画复制自 `apps/site-v2/assets/v2/` 的同名 WebP，保留原图不覆盖。新增书签由内置 image_gen 生成，参考本次用户选定 A 图中的右侧角色；源文件保留在 Codex generated_images，应用消费压缩后的 `apps/lite-web/src/home/assets/yinji-bookmark.webp`。

生成提示词：

> Create a single production illustration asset, transparent background PNG: the teal folded-paper bookmark AI character shown on the RIGHT of this reference homepage. Reference is identity/style reference. Isolate and faithfully redraw ONLY that same character: tall teal bookmark body with V cut at bottom, top right triangular folded paper corner, two charcoal oval eyes and tiny curved smile, thin charcoal bent arms resting on hips, two stick feet. Subtle flat editorial texture, minimal facets, NOT glossy 3D, no text, no UI, no logo, no decorations. Center character fully visible with generous transparent padding, square canvas 1024px, occupies 80% height. Calm friendly expression. Actual alpha transparency.
