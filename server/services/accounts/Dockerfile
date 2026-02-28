FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /accounts ./services/accounts/

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=builder /accounts /accounts
ENTRYPOINT ["/accounts"]
