# syntax=docker/dockerfile:1
#
# Marketing site image (思维印记 · mind.uni-robot.cn).
#
# Deliberately SEPARATE from the student platform: its own image, its own
# compose project, its own container and vhost. Nothing here touches the API,
# the database, or the SPA — deploying the site can never break the product.
#
# Build context MUST be the repo root: the site is a pnpm workspace member, so
# the lockfile and every workspace manifest have to be present for a frozen
# install.
#
#   docker build -f apps/site/Dockerfile \
#     --build-arg PUBLIC_APP_URL=https://mind-web.uni-robot.cn \
#     --build-arg SITE_URL=https://mind.uni-robot.cn \
#     -t mindimprint-site .

# ---- build: install the `site` dependency closure and render the static site --
FROM node:22-bookworm-slim AS build
WORKDIR /repo
# Pin the exact pnpm that generated pnpm-lock.yaml (same pin as apps/web).
RUN corepack enable && corepack prepare pnpm@10.29.3 --activate

# Workspace manifests first for layer caching. Every workspace package.json must
# be present so pnpm can validate the frozen lockfile, even though
# `--filter site...` only installs the site's dependency closure.
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY packages/contracts/package.json packages/contracts/package.json
COPY packages/course-contract/package.json packages/course-contract/package.json
COPY packages/course-runtime/package.json packages/course-runtime/package.json
COPY packages/course-renderer/package.json packages/course-renderer/package.json
COPY apps/web/package.json apps/web/package.json
COPY apps/site/package.json apps/site/package.json
COPY apps/peraspera/package.json apps/peraspera/package.json
RUN pnpm install --frozen-lockfile --filter site...

# The site is self-contained (no workspace deps) — only its own sources.
COPY tsconfig.base.json ./
COPY apps/site apps/site

# Where the site's CTAs point (the student platform), and the site's own public
# origin — used for canonical + hreflang alternates. Both are build-time: this
# is a static site, so they bake into the HTML.
ARG PUBLIC_APP_URL=https://mind-web.uni-robot.cn
ARG SITE_URL=https://mind.uni-robot.cn
ENV PUBLIC_APP_URL=$PUBLIC_APP_URL
ENV SITE_URL=$SITE_URL
RUN pnpm --filter site build

# ---- runtime: static nginx serving the rendered site ----
FROM nginx:1.27-alpine AS runtime
COPY apps/site/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/apps/site/dist /usr/share/nginx/html
EXPOSE 80
