# use web/ as build context
FROM node:24-alpine AS builder

# Enable pnpm
RUN corepack enable

WORKDIR /app

# Copy worksapce config
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./

# Copy all source code (Respect .dockerignore to skip node_modules)
COPY . .

# Install dependencies
RUN pnpm install --frozen-lockfile

# Build variables (Passed via --build-arg in GH Actions)
ARG VITE_API_BASE_URL
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

# Build the specific app
# Assuming the package name in web/apps/accounts/package.json is "accounts"
RUN pnpm --filter accounts build

# Runtime Stage
FROM nginx:alpine

# Copy nginx config
COPY apps/accounts/nginx.conf /etc/nginx/conf.d/default.conf

# Copy built assets
COPY --from=builder /app/apps/accounts/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
