FROM node:24-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

# ========================
# deps
# ========================
FROM base AS deps
WORKDIR /app

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./

COPY web/apps ./apps
COPY web/packages ./packages

RUN pnpm install --frozen-lockfile

# ========================
# build
# ========================
FROM base AS build
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules

COPY web/ ./

WORKDIR /app/apps/accounts
RUN pnpm build

# ========================
# runner
# ========================
FROM base AS runner
WORKDIR /app

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
COPY web/apps ./apps
COPY web/packages ./packages

RUN pnpm install --frozen-lockfile --prod

COPY --from=build /app/apps/accounts/build ./apps/accounts/build

WORKDIR /app/apps/accounts
ENV NODE_ENV=production

EXPOSE 3000
CMD ["pnpm", "start"]