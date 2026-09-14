# Lite 场景插画 · 2026-09-14

使用内置 imagegen 生成，参考 `apps/lite-web/src/home/assets/curious-learner-v2.webp` 的圆润人物、大色块与简洁五官。最终素材保存在 `apps/lite-web/src/learning/assets/`，均为 1536 × 1024、具有真实 alpha 通道的无损 WebP；原始 PNG 保留在生图工具的生成目录中。

| 素材 | 使用位置 | 最终生成文件 ID |
| --- | --- | --- |
| reading-quest.webp | 阅读任务预览、专注空状态 | exec-3fa471b9-ae46-4818-9578-7d265bb9a054 |
| discovery-observatory.webp | 探索科学类选题、提问空状态 | exec-c44d9089-3a03-464d-9c08-a76ab1a0cf81 |
| ideas-workbench.webp | 便签与想法画布空状态、社会类探索选题 | exec-5dd5ae46-af86-45f1-93d9-712d2f02c54c |
| reading-keepsake.webp | 阅读报告及导出、完成状态 | exec-e636f983-572f-4767-868d-0ef876c0bec3 |

## 最终提示词

前三张使用相同的提示词模板，仅替换 Scene：

> Create a companion illustration to this exact reference for the same website. Match its very simple flat graphic style, dot eyes, tiny simple faces, oversized rounded clothes, cheerful expression and flat blue/lavender/teal/yellow colors. New scene: {Scene} Entire composition compact with generous transparent margin. No typography whatsoever. No detailed scene, no realistic facial detail, no watercolor. Transparent PNG background. No glow or backdrop. Same simplicity as reference.

- 阅读任务：A teenage student stepping up three open books, holding a yellow bookmark flag.
- 探索发现：A teenage student looking through a lavender telescope on a small teal tripod, with one floating yellow planet. Only these objects.
- 便签协作：Two teenage students arranging three big colorful sticky notes together, one curved hand drawn arrow connecting two notes. Only these objects.

阅读手记：

> Create a matching companion illustration to this exact reference for the same website. Same very simple flat graphic style, simple dot eyes, oversized rounded clothes, no realistic facial details. New scene: teenage girl sitting with a large open journal held proudly, a teal bookmark and one yellow pencil. All shapes have perfectly crisp graphic boundaries. Transparent background like reference. No glow, no shading gradients, no shadow, no lettering, no background decoration, no checkerboard. Keep generous padding all around subject. Match reference simplicity.

## 检查

- 首轮较细腻的画法与背景处理版本未采用；最终统一为简洁人物风格。
- 四张最终文件的 alpha 通道均已检查；格式转换使用无损 WebP，不修改生成内容。
- 浏览器实看阅读报告、阅读任务预览、探索明暗主题，未出现白底矩形。
- 阅读报告完成真实 PNG 导出并检查；未改变后端 API 或任务完成规则。
