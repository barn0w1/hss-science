FROM node:24-alpine AS builder

WORKDIR /app

RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
COPY web/apps/accounts/package.json ./web/apps/accounts/
COPY web/packages/api/package.json ./web/packages/api/

WORKDIR /app/web
RUN pnpm install --frozen-lockfile

WORKDIR /app
COPY web ./web

WORKDIR /app/web/apps/accounts

RUN pnpm build

# Run Stage
FROM nginx:alpine

COPY web/apps/accounts/nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /app/web/apps/accounts/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]