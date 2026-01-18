FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY server/go.mod server/go.sum ./server/

WORKDIR /app/server
RUN go mod download

WORKDIR /app
COPY server ./server

WORKDIR /app/server/services/accounts
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/accounts ./cmd/server/main.go

# Runtime Stage
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

COPY --from=builder /app/bin/accounts .

EXPOSE 3000 50051

CMD ["./accounts"]