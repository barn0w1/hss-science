FROM node:24-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PNPM_STORE_PATH="/pnpm/store"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

# ========================
# fetch
# ========================
FROM base AS fetch
WORKDIR /app

COPY web/pnpm-lock.yaml ./

# デバッグ（後で削除OK）
RUN pnpm store path

RUN pnpm fetch

# ========================
# build
# ========================
FROM base AS build
WORKDIR /app

# storeコピー
COPY --from=fetch /pnpm/store /pnpm/store

RUN pnpm store path

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./

COPY web/apps ./apps
COPY web/packages ./packages

RUN pnpm install --offline --frozen-lockfile

COPY web/ ./

WORKDIR /app/apps/accounts
RUN pnpm build

# ========================
# runner
# ========================
FROM base AS runner
WORKDIR /app

COPY --from=fetch /pnpm/store /pnpm/store

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
COPY web/apps ./apps
COPY web/packages ./packages

RUN pnpm install --offline --frozen-lockfile --prod

COPY --from=build /app/apps/accounts/build ./apps/accounts/build

WORKDIR /app/apps/accounts
ENV NODE_ENV=production

EXPOSE 3000
CMD ["pnpm", "start"]