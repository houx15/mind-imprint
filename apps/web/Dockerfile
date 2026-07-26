# syntax=docker/dockerfile:1
#
# Web SPA image. Build context MUST be the repo root (the SPA depends on the
# @mind-imprint/contracts workspace package). Two stages: a Node builder that
# produces the static bundle, and a tiny nginx image that serves it.
#
#   docker build -f apps/web/Dockerfile \
#     --build-arg VITE_API_BASE_URL=https://mind-api.uni-robot.cn -t mindimprint-web .

# ---- build: install the `web` dependency closure and build the bundle ----
FROM node:22-bookworm-slim AS build
WORKDIR /repo
# Pin the exact pnpm that generated pnpm-lock.yaml. The committed root
# package.json has no `packageManager` field, so a bare `corepack enable` would
# fall back to corepack's default pnpm — a different version that reads
# onlyBuiltDependencies from package.json instead of pnpm-workspace.yaml, and
# then errors on esbuild's build script. Pinning keeps lockfile + config behavior
# consistent.
RUN corepack enable && corepack prepare pnpm@10.29.3 --activate

# Workspace manifests first for layer caching. Every workspace package.json must
# be present so pnpm can validate the frozen lockfile, even though --filter web...
# only installs the web project's dependency closure (skips site/peraspera).
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY packages/contracts/package.json packages/contracts/package.json
COPY apps/web/package.json apps/web/package.json
COPY apps/site/package.json apps/site/package.json
COPY apps/peraspera/package.json apps/peraspera/package.json
RUN pnpm install --frozen-lockfile --filter web...

# Sources needed to build the SPA.
COPY tsconfig.base.json ./
COPY packages/contracts packages/contracts
COPY apps/web apps/web

# The public origin of the Go API, baked into the bundle at build time. The API
# client prefixes /api/v1; an empty value would mean same-origin.
ARG VITE_API_BASE_URL=https://mind-api.uni-robot.cn
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN pnpm --filter web build

# ---- runtime: static nginx serving the built bundle ----
FROM nginx:1.27-alpine AS runtime
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/apps/web/dist /usr/share/nginx/html
EXPOSE 80
