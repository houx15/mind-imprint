# Build from the repository root. This image contains only the V2 static site.
FROM node:22-bookworm-slim AS build
WORKDIR /repo
RUN corepack enable && corepack prepare pnpm@10.29.3 --activate
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY packages/contracts/package.json packages/contracts/package.json
COPY packages/course-contract/package.json packages/course-contract/package.json
COPY packages/course-runtime/package.json packages/course-runtime/package.json
COPY packages/course-renderer/package.json packages/course-renderer/package.json
COPY apps/web/package.json apps/web/package.json
COPY apps/lite-web/package.json apps/lite-web/package.json
COPY apps/site/package.json apps/site/package.json
COPY apps/site-v2/package.json apps/site-v2/package.json
COPY apps/peraspera/package.json apps/peraspera/package.json
RUN pnpm install --frozen-lockfile --filter site-v2...
COPY apps/site-v2/astro.config.mjs apps/site-v2/tsconfig.json apps/site-v2/
COPY apps/site-v2/src apps/site-v2/src
COPY apps/site-v2/public apps/site-v2/public
ARG SITE_URL=https://mind.uni-robot.cn
ARG PUBLIC_PRO_URL=https://mind-web.uni-robot.cn
ARG PUBLIC_LITE_URL=https://mind-lite.uni-robot.cn
ARG PUBLIC_ASSET_BASE_URL=https://mind-assets.uni-robot.cn
ARG PUBLIC_V2_ASSET_BASE_URL=
ENV SITE_URL=$SITE_URL PUBLIC_PRO_URL=$PUBLIC_PRO_URL PUBLIC_LITE_URL=$PUBLIC_LITE_URL PUBLIC_ASSET_BASE_URL=$PUBLIC_ASSET_BASE_URL PUBLIC_V2_ASSET_BASE_URL=$PUBLIC_V2_ASSET_BASE_URL
RUN pnpm --filter site-v2 build
FROM nginx:1.27-alpine AS runtime
COPY apps/site-v2/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/apps/site-v2/dist /usr/share/nginx/html
EXPOSE 80
