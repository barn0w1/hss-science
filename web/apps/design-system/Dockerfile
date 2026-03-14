FROM node:24-alpine AS builder

WORKDIR /app

RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
COPY web/apps/design-system/package.json ./web/apps/design-system/
COPY web/packages/tokens/package.json ./web/packages/tokens/

WORKDIR /app/web
RUN pnpm install --frozen-lockfile

WORKDIR /app
COPY web ./web

WORKDIR /app/web/apps/design-system

RUN pnpm build

# Run Stage
FROM nginx:alpine

COPY web/apps/design-system/nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /app/web/apps/design-system/dist /usr/share/nginx/html

CMD ["nginx", "-g", "daemon off;"]
