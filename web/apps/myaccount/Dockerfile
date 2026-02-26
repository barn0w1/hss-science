FROM node:24-alpine AS builder

WORKDIR /app

RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
COPY web/apps/myaccount/package.json ./web/apps/myaccount/

WORKDIR /app/web
RUN pnpm install --frozen-lockfile

WORKDIR /app
COPY web ./web

WORKDIR /app/web/apps/myaccount

RUN pnpm build

# Run Stage
FROM nginx:alpine

COPY web/apps/myaccount/nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /app/web/apps/myaccount/dist /usr/share/nginx/html

CMD ["nginx", "-g", "daemon off;"]
