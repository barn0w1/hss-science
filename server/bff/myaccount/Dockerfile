FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY server/go.mod server/go.sum ./server/

WORKDIR /app/server
RUN go mod download

WORKDIR /app
COPY server ./server

WORKDIR /app/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/myaccount-bff ./bff/myaccount/cmd/server

# Runtime Stage
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

COPY --from=builder /app/bin/myaccount-bff .

CMD ["./myaccount-bff"]
