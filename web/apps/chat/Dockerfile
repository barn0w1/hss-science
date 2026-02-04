FROM node:24-alpine AS builder

WORKDIR /app

RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
COPY web/apps/chat/package.json ./web/apps/chat/
COPY web/packages/api/package.json ./web/packages/api/

WORKDIR /app/web
RUN pnpm install --frozen-lockfile

WORKDIR /app
COPY web ./web

WORKDIR /app/web/apps/chat

RUN pnpm build

# Run Stage
FROM nginx:alpine

COPY web/apps/chat/nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /app/web/apps/chat/dist /usr/share/nginx/html

CMD ["nginx", "-g", "daemon off;"]