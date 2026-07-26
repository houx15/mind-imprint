# 部署分册 · HTTPS（宿主机 nginx + certbot）

宿主机 nginx 负责 TLS 终止与反代，与服务器上**已有站点共存**。本步骤只新增
`mind-api` / `mind-web` 两个 server block，不改动既有配置。

## 前置

- `mind-api.uni-robot.cn`、`mind-web.uni-robot.cn` 均已解析到本机（`getent hosts <域名>`）
- api 容器在 `127.0.0.1:8090`、web 容器在 `127.0.0.1:8091` 已就绪
- 宿主机已装 nginx + certbot（含 `python3-certbot-nginx` 插件）

## 1. 安装两个 server block

```bash
cd ~/mind-imprint
sudo cp deploy/nginx/mind-api.uni-robot.cn.conf /etc/nginx/sites-available/
sudo cp deploy/nginx/mind-web.uni-robot.cn.conf /etc/nginx/sites-available/
sudo ln -sf /etc/nginx/sites-available/mind-api.uni-robot.cn.conf /etc/nginx/sites-enabled/
sudo ln -sf /etc/nginx/sites-available/mind-web.uni-robot.cn.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

HTTP 校验（TLS 之前）：

```bash
curl -sS -H 'Host: mind-api.uni-robot.cn' http://127.0.0.1/healthz     # 200
curl -sS -I -H 'Host: mind-web.uni-robot.cn' http://127.0.0.1/          # 200 text/html
```

## 2. 签发证书（certbot nginx 插件）

```bash
sudo certbot --nginx -d mind-api.uni-robot.cn -d mind-web.uni-robot.cn \
  --redirect --agree-tos -m <你的邮箱> --no-eff-email
```

certbot 会自动为两个 server block 追加 `:443` 配置并加装 HTTP→HTTPS 跳转。续期由
`certbot.timer` 自动完成（`systemctl list-timers | grep certbot` 可查）。

`mind-api.conf` 顶部的 `map $http_upgrade $mind_api_conn_upgrade` 让同一路由同时支持 SSE
（陪练/工作室流式）与 WebSocket（语音）；`proxy_read_timeout 300s` 覆盖内联旗舰评估的 ~90s–2min。

## 3. 端到端校验

```bash
curl -sS https://mind-api.uni-robot.cn/healthz        # 200
curl -sS https://mind-api.uni-robot.cn/readyz         # 200
curl -sS -I https://mind-web.uni-robot.cn/            # 200 text/html
# 跨子域凭证登录（应看到 Set-Cookie: mk_session=...; Secure; HttpOnly; SameSite=Lax）
curl -sS -i -X POST https://mind-api.uni-robot.cn/api/v1/auth/signin \
  -H 'Content-Type: application/json' \
  -H 'Origin: https://mind-web.uni-robot.cn' \
  -d '{"email":"phoebe@demo.mindimprint.local","password":"phoebe-dev-pass"}'
```

浏览器里打开 `https://mind-web.uni-robot.cn`：

- 学生：`phoebe@demo.mindimprint.local` / `phoebe-dev-pass`
- 教师：`wu.teacher@demo.mindimprint.local` / `phoebe-dev-pass`
- 或营销站演示入口 `https://mind-web.uni-robot.cn/?trial=1` 自动以示例学生登录

## 排错

- **登录 200 但前端仍未登录**：多为跨域 Cookie。确认 `CORS_ORIGINS` 精确等于
  `https://mind-web.uni-robot.cn`（无尾斜杠），且两域都是 HTTPS（`COOKIE_SECURE=true` 下 Cookie 仅走 https）。
- **finish/评估 502/504**：宿主机 nginx `proxy_read_timeout` 需 ≥180s（本配置 300s）。
- **语音 503**：`VOICE_*` 未配置，属预期可选降级。
