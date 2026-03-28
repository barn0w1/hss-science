FROM node:24-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

# ========================
# fetch
# ========================
FROM base AS fetch
WORKDIR /app

COPY web/pnpm-lock.yaml ./
RUN pnpm fetch

# ========================
# build
# ========================
FROM base AS build
WORKDIR /app

COPY --from=fetch /root/.pnpm-store /root/.pnpm-store

COPY web/package.json web/pnpm-workspace.yaml ./

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

COPY --from=fetch /root/.pnpm-store /root/.pnpm-store

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
COPY web/apps ./apps
COPY web/packages ./packages

RUN pnpm install --offline --frozen-lockfile --prod

COPY --from=build /app/apps/accounts/build ./apps/accounts/build

WORKDIR /app/apps/accounts
ENV NODE_ENV=production

EXPOSE 3000
CMD ["pnpm", "start"]