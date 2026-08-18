# 营销站部署 · mind.uni-robot.cn

> `apps/site`（Astro 静态站）。**与学生端完全分开部署**：独立 checkout、独立
> compose project、独立容器、独立端口、独立 vhost。改一行文案不会碰到 API、
> 数据库或 SPA；反过来，学生端发版也不会重启营销站。

## 为什么要分开

营销站和学生平台是两种东西：一个是纯静态、无密钥、随时可改的展示页；一个是持有
密钥、连数据库、发版要先备份再迁移的产品。把它们放进同一个 compose project 意味着
每次改文案都要走产品的部署脚本，而那条脚本会 `git reset` 产品的工作树、跑迁移、
重启 api。风险与收益完全不成比例。

所以两套栈在服务器上是这样的：

| | 学生平台 | 营销站 |
|---|---|---|
| 服务器 checkout | `~/mind-imprint` | `~/mind-imprint-site` |
| compose project | `mindimprint` | `mindimprint-site` |
| compose 文件 | `deploy/docker-compose.prod.yml` | `deploy/docker-compose.site.yml` |
| 容器 | `api` · `web` · `db` | `site` |
| 端口 | 8090 · 8091 · （db 不发布） | 8092 |
| 域名 | `mind-api` · `mind-web` | `mind` |
| 密钥 | `deploy/.env.prod` | **无，不需要** |
| 部署脚本 | `deploy/remote-deploy.sh` | `deploy/remote-deploy-site.sh` |
| 本地包装 | `.deploy-local/deploy.sh` | `.deploy-local/deploy-site.sh` |

**两份 checkout 是关键。** 如果共用一个工作树，用某个分支部署营销站会把产品的工作树
也停在那个分支上，下一次产品构建就会悄悄带上它。脚本里有一条断言专门防这件事。

## 拓扑

```
                Internet (HTTPS)
                      │
             ┌────────┴─────────┐
             │   host nginx     │   ← TLS 终止 (certbot)
             │  (47.93.151.131) │
             └──┬───────┬───────┬┘
   mind-api ────┘       │       └──── mind  ← 本文档
   127.0.0.1:8090       │             127.0.0.1:8092
        │          mind-web                │
        │          127.0.0.1:8091     ┌────┴─────┐
  ┌─────┴─────┐          │            │  site    │
  │ api (Go)  │    ┌─────┴──────┐     │ (nginx   │
  └─────┬─────┘    │ web (nginx │     │  static) │
        │          │  static)   │     └──────────┘
  ┌─────┴─────┐    └────────────┘
  │ db (pg16) │        ← 学生平台 (mindimprint)
  └───────────┘
```

## 文件清单（均已提交，不含密钥）

- `apps/site/Dockerfile` — Node 构建 → nginx 静态托管
- `apps/site/nginx.conf` — 容器内 nginx（**不是 SPA fallback**，见下）
- `deploy/docker-compose.site.yml` — 单服务 `site`，project 名 `mindimprint-site`
- `deploy/nginx/mind.uni-robot.cn.conf` — 宿主机反代站点
- `deploy/remote-deploy-site.sh` — 服务器上执行的标准部署脚本

## 日常部署（一条命令）

```bash
git commit && git push            # 脚本会把服务器 checkout reset 到该 ref
./.deploy-local/deploy-site.sh              # 部署 origin/main
./.deploy-local/deploy-site.sh origin/some-branch   # 部署某个分支
```

脚本自动校验，失败即 `SITE DEPLOY FAILED`：

- 首次运行会自动 `git clone` 出 `~/mind-imprint-site`（一次性）
- 断言 site checkout ≠ 产品 checkout
- 在**容器内**（而非走网络，避免重启瞬间截断）grep 构建产物，确认 CTA 指向
  `mind-web.uni-robot.cn`——`PUBLIC_APP_URL` 错了的话每个按钮都会指向 localhost
- 逐条请求 8 个路由（中英各 4 个）必须 200
- `/evaluation` 必须仍然重定向（旧链接不能死）
- 一个不存在的路径必须 **404**，不能回落到首页

## 首次搭建（只做一次）

DNS `mind.uni-robot.cn → 47.93.151.131` 需先生效。

```bash
# 1) 部署容器（会自动 clone + 起 8092）
./.deploy-local/deploy-site.sh origin/main

# 2) 宿主机 nginx 反代（在服务器上）
sudo cp ~/mind-imprint-site/deploy/nginx/mind.uni-robot.cn.conf /etc/nginx/sites-available/
sudo ln -sf /etc/nginx/sites-available/mind.uni-robot.cn.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

# 3) 签发证书（certbot 会自动加 :443 与 http→https 跳转）
sudo certbot --nginx -d mind.uni-robot.cn
```

`certbot --nginx` 只会改它自己那个 server block，不影响服务器上已有的其它站点。

## 两个容易踩的坑

**① 容器内 nginx 不能用 SPA fallback。** 学生端是 SPA，`try_files ... /index.html`
是对的；营销站是静态多页站，Astro 为每个路由生成真实目录（`/product/index.html`）。
如果照抄 SPA 的 fallback，任何拼错的路径都会渲染出首页——同一份内容出现在无数
URL 下，搜索引擎会当成重复内容。所以这里是 `try_files $uri $uri/index.html $uri.html =404`，
并配了 `error_page 404 /404.html`。

**② `.dockerignore` 里的 `*.png`。** 仓库根的 `.dockerignore` 为了瘦身排除了所有 png，
这会连带把营销站 `public/` 里的截图一起排除掉，线上全部 404。已在文件末尾加
`!apps/site/public/**` 重新放行（后写的规则优先）。加图片时如果线上不显示，先查这里。

## 素材（对象存储）

营销站的图片/视频放在**自己的**桶里，与学生端完全分开：

| | 学生端 | 营销站 |
|---|---|---|
| bucket | `mind-imprint` | **`mind-open`** |
| 对外域名 | `mind-oss.uni-robot.cn` | **`mind-assets.uni-robot.cn`** |
| 密钥位置 | `deploy/.env.prod`（服务端，Go API 用） | `.deploy-local/site-oss.env`（**只在本机**） |
| 上传方式 | API 的 `/oss/*` 管理端点 | `deploy/upload-site-assets.sh` |

**营销站容器不持有任何密钥，也不需要。** 桶是私有的，但由 CDN 授权回源读取，
访问者拿到的就是一个普通 URL。签名密钥只有「上传」这一步需要，而上传发生在开发机上——
静态站本来也守不住秘密。

### 放一张图上线

```bash
# 1) 放进 apps/site/assets/（此目录不进 git，OSS 才是存放地）
#    目录结构 1:1 映射成 key：apps/site/assets/home/x.png -> home/x.png
# 2) 上传
deploy/upload-site-assets.sh
# 3) 页面里引用
#    <Figure asset="home/x.png" name="…" caption="…" />
# 4) 正常发版
./.deploy-local/deploy-site.sh
```

首次使用先建凭据文件（git 忽略）：

```bash
cp deploy/site-oss.env.example .deploy-local/site-oss.env   # 填入真实 AK/SK
deploy/upload-site-assets.sh --check                        # 端到端自检
```

`--check` 会上传一个探针对象再经 CDN 读回来，能一次性证明「桶可写 + CDN 可读私有桶」。

### ⚠️ 当前状态：CDN 还没配好

`mind-assets.uni-robot.cn` **目前直接指向 OSS 自定义域名，不是 CDN**，实测：

```
dig  mind-assets.uni-robot.cn  →  mind-open.cn-beijing.taihangpkx.cn  (OSS 自定义域名 CNAME)
TLS  证书 CN = cn-beijing.oss.aliyuncs.com   → 与该域名不匹配，HTTPS 直接失败
GET  http://mind-assets.uni-robot.cn/<key>   → 403 AccessDenied (bucket acl)
GET  https://mind-open.oss-cn-beijing.aliyuncs.com/<key> 未签名 → 同样 403
```

写入是通的（上传脚本已验证成功），**读不通**：私有桶前面还没有被授权的 CDN。

按本项目的一贯做法——**素材一律走 CDN，从不直连 OSS**——需要在控制台把这个域名
从「OSS 自定义域名」改成「CDN 加速域名」：

1. 阿里云 CDN 新建加速域名 `mind-assets.uni-robot.cn`，源站类型选 **OSS 域名**，
   填 `mind-open.oss-cn-beijing.aliyuncs.com`
2. 打开**私有 Bucket 回源**授权（让 CDN 拿到读这个私有桶的权限）
3. 在 CDN 里为该域名签发/上传 HTTPS 证书（当前证书是 OSS 自己的，域名对不上）
4. **URL 鉴权保持关闭**。学生端的课程素材用 URL 鉴权是对的（那些内容要控访问），
   但营销站是公开的、且是静态页，浏览器侧无法在请求时签名——这里开了就全挂。
5. DNS 的 CNAME 从当前的 `mind-open.cn-beijing.taihangpkx.cn`（OSS 自定义域名）
   改成 CDN 分配的那个

配好之后跑 `deploy/upload-site-assets.sh --check`，通过即可开始挂图。
在此之前，页面里的 `<Figure>` 全是占位符、没有任何一处引用 CDN，所以线上不受影响。

## 构建期变量

静态站，两个都在构建时打进 HTML：

| 变量 | 默认 | 作用 |
|---|---|---|
| `PUBLIC_APP_URL` | `https://mind-web.uni-robot.cn` | 「体验 Demo」「登录」指向的学生平台 |
| `SITE_URL` | `https://mind.uni-robot.cn` | canonical 与 hreflang 的绝对地址 |
| `PUBLIC_ASSET_BASE_URL` | `https://mind-assets.uni-robot.cn` | 素材 CDN；留空则回落到本地 `public/media/` |

三个都是**公开值**，不是密钥。AK/SK 只存在于 `.deploy-local/site-oss.env`，
既不进镜像也不进服务器。

改了域名就在 `deploy/docker-compose.site.yml` 的 `args` 里改，或用环境变量覆盖。

## 回滚

```bash
./.deploy-local/deploy-site.sh <上一个好的 commit sha>
```

营销站无状态、无数据库，回滚就是用旧 ref 再构建一次，没有迁移要撤。
