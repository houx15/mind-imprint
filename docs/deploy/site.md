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

## 构建期变量

静态站，两个都在构建时打进 HTML：

| 变量 | 默认 | 作用 |
|---|---|---|
| `PUBLIC_APP_URL` | `https://mind-web.uni-robot.cn` | 「体验 Demo」「登录」指向的学生平台 |
| `SITE_URL` | `https://mind.uni-robot.cn` | canonical 与 hreflang 的绝对地址 |

改了域名就在 `deploy/docker-compose.site.yml` 的 `args` 里改，或用环境变量覆盖。

## 回滚

```bash
./.deploy-local/deploy-site.sh <上一个好的 commit sha>
```

营销站无状态、无数据库，回滚就是用旧 ref 再构建一次，没有迁移要撤。
