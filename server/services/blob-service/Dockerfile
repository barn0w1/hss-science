FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /blob-service ./services/blob-service/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /blob-service /blob-service

USER 10001:10001

ENTRYPOINT ["/blob-service"]
