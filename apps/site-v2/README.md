> **Production:** 已于 2026-09-11 发布到 https://mind.uni-robot.cn/ 。新插画已上传 OSS；当前发布步骤见 [部署记录](../../docs/deploy/site-v2.md)。下文“未发布 / 未上传”等描述属于此前开发阶段记录。

# Mind Imprint 官网 V2

面向学校与教育工作者的独立双语营销站。Astro 静态输出，无后端接口、模型调用、密钥或数据库依赖。旧版 `apps/site` 保持原样。

## 查看与运行

在仓库根目录运行：

```sh
pnpm install --frozen-lockfile --filter site-v2...
pnpm --filter site-v2 dev
```

英文主版本：http://localhost:4322/ ，中文版：http://localhost:4322/zh/ 。原 `/en/` 路径兼容跳转到英文根目录。

```sh
pnpm --filter site-v2 check
pnpm --filter site-v2 build
pnpm --filter site-v2 preview
```

本次开发复用了本机已有的 Astro 依赖（未提交的 `node_modules` 链接）。独立安装由锁文件中的 `apps/site-v2` importer 支持，已通过 frozen/offline 锁文件校验。构建不引用旧站组件。

## 页面与内容

12 个页面主题，每个提供中英文，共 24 页，另有 404 页面、sitemap 和 robots：

- 首页：随滚动绘制的学习旅程曲线、学习场景、三种可切换的示例、教师视角、版本与资源。
- 产品全览与六个功能页：探索、阅读、写作、项目、课程、过程评估。
- 学校与教师：教师界面、课堂使用思路、六组常见问题。
- Lite 与 Pro：以学习流程区分版本，提供真实产品入口。
- 教学资源：六种代表性思维方法与可展开步骤。
- 关于思维印记：学习理念与学生自主性。

课程目录来自现有官网的 33 项实际课程内容，仅复用内容数据；新建搜索、五类方向筛选、课程展开和空结果状态。课程在产品中的可用范围仍由版本和受众设置决定。

`src/data/features.ts` 为六类功能文案；`src/data/courses.json` 为课程目录；`src/config.ts` 为语言、产品入口与素材路径；`src/pages/[...page].astro` 生成双语路由。中英文切换保留当前页面主题。英文在根目录，中文在 `/zh/`；`/en/` 的 12 个旧入口提供兼容跳转，Nginx 下返回 301，纯静态托管使用生成的跳转 HTML。

## 视觉依据

完整调研见 [研究报告](../../docs/research/2026-09-11-education-websites/report.md) 与 [视觉参考板](../../docs/research/2026-09-11-education-websites/index.html)。

信息结构参考教育产品官网的受众入口、场景详情、产品演示与教师视角。用户随后指定 [MindMarket](https://mindmarket.com/) 的插画感觉：采用简洁人物、大色块、细线条和适度夸张的身体比例，撤下原先细节密集的拼贴图。

新首页使用原创人物插画 `learning-together-v3.webp`，暖白底、绿色品牌区块，蓝、黄、珊瑚色集中在插画中。功能说明使用 SVG / CSS 小型示意；学习示例明确标识为示例，不伪装成实时 AI。没有添加虚构客户、认证、学校成效数据或采购联系表单。

生成记录见 [assets/README.md](assets/README.md)。研究截图只用于内部设计研究，不进入官网构建产物。

## OSS 与环境变量

现有产品截图、工具卡、课程封面和报告继续使用公开 CDN `https://mind-assets.uni-robot.cn`。

四张插画均由代码去除暖白背景，保留白色服装与书页。提供透明 PNG 和透明 WebP；WebP 保留两份：

- `assets/v2/*.webp`：待上传的版本化素材。
- `public/media/v2/*.webp`：让本地及构建预览直接可用的副本。

此次未上传新素材、未替换线上站点。确认视觉后，可使用仓库原有单文件上传入口：

```sh
deploy/upload-site-assets.sh apps/site-v2/assets/v2/learning-together-v3.webp v2/learning-together-v3.webp
deploy/upload-site-assets.sh apps/site-v2/assets/v2/curious-learner-v2.webp v2/curious-learner-v2.webp
deploy/upload-site-assets.sh apps/site-v2/assets/v2/teacher-dialogue-v2.webp v2/teacher-dialogue-v2.webp
deploy/upload-site-assets.sh apps/site-v2/assets/v2/project-makers-v2.webp v2/project-makers-v2.webp
```

上传并检查该公开 URL 后，构建时设置 `PUBLIC_V2_ASSET_BASE_URL=https://mind-assets.uni-robot.cn`，新插画即可全部从 OSS/CDN 提供。使用新 key，不覆盖旧站图片。环境变量示例见 `.env.example`。

| 变量                       | 用途                                                    |
| -------------------------- | ------------------------------------------------------- |
| `SITE_URL`                 | 当前部署的公开域名，决定 canonical、hreflang 和 sitemap |
| `PUBLIC_PRO_URL`           | Pro 登录及演示入口                                      |
| `PUBLIC_LITE_URL`          | Lite 入口                                               |
| `PUBLIC_ASSET_BASE_URL`    | 原有公开素材 CDN                                        |
| `PUBLIC_V2_ASSET_BASE_URL` | V2 新素材 CDN；不设时使用本地 `/media`                  |

默认域名延续现有官网，预览部署如需被搜索引擎索引，应按实际域名设置 `SITE_URL`。页面不收集表单或运行分析脚本。英文字体使用 Google Fonts 的 DM Sans，并提供本地回退字体。

## 独立部署

可将 `dist/` 直接交给静态托管，也可使用独立容器：

```sh
docker build -f apps/site-v2/Dockerfile -t mindimprint-site-v2 .
docker run --rm -p 127.0.0.1:8093:80 mindimprint-site-v2
```

从仓库根目录构建。Docker 构建已提供但本次未实跑。Nginx 使用真实静态路由，未知地址返回 404，HTML 不缓存，带哈希资源长期缓存。此配置未改动旧站、API、Pro、Lite 的部署文件。

## 本次验证

- Astro check：0 errors、0 warnings、0 hints。
- Astro build：成功，24 个内容页、404、12 个兼容跳转页，另有 sitemap/robots。
- frozen/offline 锁文件校验通过。
- 内部链接、锚点与本地图片引用检查见 `verification/static-check.json`。
- 45 张现有 CDN 图片与报告 PDF 均返回 HTTP 200（共 46 项，只读 HEAD 检查）。
- 动效：SVG 曲线根据滚动位置计算已绘制长度与节点位置；内容渐入使用原生浏览器观察器；不劫持滚动。系统减少动态效果时完整呈现曲线和内容，关闭位移。
- 真浏览器：1280 像素桌面，390 与 320 像素手机；检查了中英文首页、手机菜单、语言切换、示例切换、课程搜索、空结果、分类、课程展开与 FAQ。
- 390 像素英文学校、课程、版本、资源、项目和评估页面未发现整页横向溢出。版本对比表在自身容器内横向滚动。
- 未接入或改动产品后台；未进行账户权限、付费或真实课堂成效验证。

检查记录位于 `verification/`。浏览器截图位于研究目录 `assets/v2-*`。

### Content expansion · 2026-09-11

English and Chinese pages now foreground the main learning platform: AI use and
ethics, information and data literacy, Chinese/English reading and writing, and
real-world projects including website creation. `LearningShowcase.astro` supplies
concrete activities to the home, product overview and relevant feature pages.
The four featured courses link to entries 30, 33, 20 and 16 in the existing
33-course catalog; deep links open their descriptions and clear active filters.
Bilingual prompts and the campus website are clearly labelled illustrations,
not student submissions. Website creation was checked against the current
`apps/lite-web/src/projects/tools/` implementation; course descriptions come
from the existing course catalog.

`/research-studio/` and `/zh/research-studio/` introduce the dedicated research
workflow. Research Studio is the current marketing name for the Pro workspace;
internal identifiers and application endpoints are unchanged. `/editions/`
remains available with the research content for old links. The sitemap points
to the new research route. No claim of curriculum accreditation is made.

Verification for this content update: Astro check reported no errors, warnings
or hints; static build succeeded. Browser checks covered the new research page,
course deep-link expansion, desktop bilingual layout and 390px home, research,
Chinese research, project and course pages with no horizontal overflow.

### Homepage length refinement

The home page now keeps the hero, scroll journey, six feature links, educator
preview, Research Studio introduction and closing CTA. Full literacy, bilingual
and project examples remain on the product overview and relevant feature pages.
The interactive walkthrough is at `/product/#learning-demo`; the home hero
links directly to it. Thinking-tool details remain on `/resources/`.
At a 1280×800 browser viewport the home page decreased from 10,374px to 6,022px
(about 42%). The hero-to-demo link and the 390px Chinese layout were checked
in the browser; the temporary QA tab was closed.

### About page

The five team biographies and four CDN portraits are copied from
`apps/site/src/components/pages/AboutPage.astro` into `src/data/team.ts`.
The bilingual founders' letter is newly written from that site's team and
product information, for review in the local preview. It is not a historical
quotation and uses no Per Aspera content. No publication date is invented.
The page provides direct links to `#team` and `#founders-letter`.

### Curriculum and restored recordings · 2026-09-11

Seven bilingual curriculum-area pages live at `/courses/<area>/`: AI ethics,
AI use, source evaluation, historical evidence, multimedia reading, data
literacy and self-exploration. The hub is `/product/courses/#course-areas`,
linked from the main Curriculum navigation. Courses may span several areas;
the new area descriptions are editorial groupings of the existing 33 courses,
not claims of additional course inventory.

All ten video keys found in `apps/site/src` have been restored: IFS, SIFT and
CRAAP in the corresponding area pages; four research-process clips in
Research Studio; three platform recordings on the product overview. The
assessment page also links its original recording. All use existing CDN media,
poster images, native controls and `preload=none`, with no autoplay. The
research walkthrough also restores the original course, map and reading stills.

Evidence: `verification/restored-media.json` records 20 successful CDN checks
(ten videos and ten posters). The IFS recording was played in-browser. All
seven curriculum area pages were checked at 390px without horizontal overflow.

### Scroll playback, course carousel and assessment detail

Product recordings now play muted and loop when at least 35% visible, pause
when outside the viewport or the browser tab is hidden, and retain native
controls. Reduced-motion preferences disable automatic playback. This
supersedes the earlier no-autoplay note.

The courses page presents IFS, SIFT and CRAAP in a swipeable, keyboard-operable
carousel. Research Studio has a sticky step rail, scroll-linked progress and
active-step transitions. Its visible recording plays while recordings in
other steps pause. Reduced motion removes transitions.

Assessment has a main navigation entry and an expanded independent feature
page with the existing D1–D6 and A1–A6 rubric, nine report sections, example
reasoning and the full PDF. Research Studio and Schools both include an
assessment introduction linking to it. Rubric content comes from the old
site's EvaluationPage; claims about removing psychological confounds or
predicting ability were not carried over.

Browser verification confirmed SIFT autoplay with the other two carousel
videos paused, research progression from framing to writing with playback
following the visible step, and 12 dimensions/9 report sections at 390px
without horizontal overflow.
