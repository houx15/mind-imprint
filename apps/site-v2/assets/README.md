# V2 品牌插画

使用文件：`v2/learning-together-v3.webp`（1536 × 1024）。本地预览副本在 `../public/media/v2/`，目标 OSS key 为 `v2/learning-together-v3.webp`，尚未上传。

2026-09-11 使用内置 image generation 工具生成，工具未暴露可选择/可验证的模型编号，因此不标注为某个特定模型。使用 MindMarket 页面局部截图作为造型风格参考，人物、动作与书本构图为新生成。没有将第三方网站原图直接用于官网。

原始输出位于本机 Codex generated_images：`exec-1f6d5391-976d-4e6f-937f-cad9d363b602.png`。用 cwebp quality 90 转为项目 WebP；没有后期合成人物。经用户明确要求，随后用代码去除了暖白背景，保留了白色服装和书页。早期拼贴方案已撤下，未进入当前构建。

## 最终生成提示词

> Use the supplied screenshot ONLY as a style reference for its bold flat vector characters, cheerful exaggerated rounded proportions and ultra simple line work. Create an ORIGINAL illustration of two teenage learners sharing a thought over an open book. Illustration only, no website, no text, no logos. A clean solid warm ivory #f5f2e6 rectangular background throughout. Full figures, one sitting on the left in a bright sky-blue oversized sweater holding a yellow book, the other leaning in from the right with an expressive oversized coral sleeve and a hand gently pointing at the book. Tiny heads, large flowing sculptural arms and trousers, stylized simple faces and amusing confident gestures like the reference. More graphic abstraction than anatomical realism. Colors strictly warm ivory, sky blue, yellow, coral and dark charcoal with natural skin accents. The two figures make one strong diagonal composition, generous negative space. Large solid unshaded color shapes, only a few thin dark contour strokes for facial features and fingers. No gradients, no blurred edges, no halos, no transparency, no glow, no shadow, no texture, no grain, no realistic anatomy, no realistic hair, no backdrop, no decorative objects. Absolutely NO realistic children or anime style. Wide 3:2 image with visibly flat solid ivory background. The look should be an accomplished contemporary creative-agency vector character illustration, not children's school-book illustration.

最终画面含少量淡紫裤装和轻微色彩变化。已在实际桌面/手机首页检查，不将提示词中的每条约束描述为已精确实现。

## 补充插画与透明背景

| 当前文件                               | 场景             | 原始生成文件                                    |
| -------------------------------------- | ---------------- | ----------------------------------------------- |
| `v2/curious-learner-v2.png` / `.webp`  | 观察与兴趣探索   | `exec-21064c5b-58a4-4966-8f5e-68c1f93fbfa4.png` |
| `v2/project-makers-v2.png` / `.webp`   | 学生共同制作项目 | `exec-26a529e8-ec6a-4f5a-afdf-3737f689697b.png` |
| `v2/teacher-dialogue-v2.png` / `.webp` | 教师倾听学生     | `exec-49ec6d97-5696-43af-a9af-fabf9c8248ab.png` |

补充插画均以已获用户确认的首页图为风格参考，保持大色块、夸张身体比例和少量轮廓线。场景分别限定为一个学生、放大镜和叶片；两个学生、桥梁模型和设计稿；师生两人、凳子与笔记本。提示词要求无文字、无背景装饰、暖白底。

用户随后明确要求使用代码去背景。`scripts/remove-illustration-background.py` 使用 Pillow / NumPy 估计画布边缘的背景色，保留与暖白背景不同的白色服装，并根据邻近前景计算边缘 alpha、去除残余底色。它针对这套均匀底色插画，不是通用抠图模型。脚本同时保存透明 PNG 与透明 WebP；项目页面使用 WebP。

`verification/transparent-contact-sheet.png` 是四张透明图叠在深色底上的检查图；`verification/transparency.json` 记录背景色、透明/半透明/不透明像素数量。原始生成 PNG 保留在 Codex 生成目录，未覆盖。
