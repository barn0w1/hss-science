FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /identity-service ./services/identity-service/

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=builder /identity-service /identity-service

ENTRYPOINT ["/identity-service"]
CMD ["server"]