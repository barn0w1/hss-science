FROM node:24-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/accounts/package.json ./apps/accounts/
RUN pnpm install --frozen-lockfile

FROM base AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/apps/accounts/node_modules ./apps/accounts/node_modules
COPY . .
WORKDIR /app/apps/accounts
RUN pnpm build

FROM base AS runner
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/accounts/package.json ./apps/accounts/
RUN pnpm install --frozen-lockfile --prod
COPY --from=build /app/apps/accounts/build ./apps/accounts/build
WORKDIR /app/apps/accounts
ENV NODE_ENV=production
EXPOSE 3000
CMD ["pnpm", "start"]