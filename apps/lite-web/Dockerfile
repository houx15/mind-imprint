# syntax=docker/dockerfile:1
#
# Lite web SPA image. Build context MUST be the repo root: lite imports room
# components straight from apps/web/src (the "@/…" alias resolves there, see
# apps/lite-web/vite.config.ts and tsconfig.json) and depends on
# @mind-imprint/contracts, so a context scoped to apps/lite-web can't see
# either. Mirrors apps/web/Dockerfile; the only differences are the workspace
# filter (@mind-imprint/lite-web) and the extra `apps/web` source copy the
# alias needs.
#
#   docker build -f apps/lite-web/Dockerfile \
#     --build-arg VITE_API_BASE_URL=https://mind-api.uni-robot.cn -t mindimprint-lite-web .

# ---- build: install the `lite-web` dependency closure and build the bundle ----
FROM node:22-bookworm-slim AS build
WORKDIR /repo
# Pin the exact pnpm that generated pnpm-lock.yaml (same pin as apps/web).
RUN corepack enable && corepack prepare pnpm@10.29.3 --activate

# Workspace manifests first for layer caching. Every workspace package.json must
# be present so pnpm can validate the frozen lockfile, even though
# --filter @mind-imprint/lite-web... only installs lite-web's dependency
# closure — which pulls in @mind-imprint/web (and, transitively, the
# course-* packages web depends on) because lite-web declares
# "@mind-imprint/web": "workspace:*".
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY packages/contracts/package.json packages/contracts/package.json
COPY packages/course-contract/package.json packages/course-contract/package.json
COPY packages/course-runtime/package.json packages/course-runtime/package.json
COPY packages/course-renderer/package.json packages/course-renderer/package.json
COPY apps/web/package.json apps/web/package.json
COPY apps/site/package.json apps/site/package.json
COPY apps/lite-web/package.json apps/lite-web/package.json
RUN pnpm install --frozen-lockfile --filter @mind-imprint/lite-web...

# Sources needed to build the SPA. apps/web is required in full: lite's Vite
# alias, tsconfig paths AND tailwind.config.ts content glob all reach into
# apps/web/src (room components + design tokens), not just its package.json.
COPY tsconfig.base.json ./
COPY packages/contracts packages/contracts
COPY packages/course-contract packages/course-contract
COPY packages/course-runtime packages/course-runtime
COPY packages/course-renderer packages/course-renderer
COPY apps/web apps/web
COPY apps/lite-web apps/lite-web

# The public origin of the Go API, baked into the bundle at build time. The API
# client prefixes /api/v1 (see apps/lite-web/src/api/client.ts); an empty value
# would mean same-origin.
ARG VITE_API_BASE_URL=https://mind-api.uni-robot.cn
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

# The mirror of apps/web/Dockerfile's VITE_LITE_APP_URL: where to send a
# student whose school is on the FULL edition but who signed in here.
ARG VITE_PRO_APP_URL=https://mind-web.uni-robot.cn
ENV VITE_PRO_APP_URL=$VITE_PRO_APP_URL
RUN pnpm --filter @mind-imprint/lite-web build

# ---- runtime: static nginx serving the built bundle ----
FROM nginx:1.27-alpine AS runtime
COPY apps/lite-web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/apps/lite-web/dist /usr/share/nginx/html
EXPOSE 80
