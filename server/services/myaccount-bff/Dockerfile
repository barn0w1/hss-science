FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /myaccount-bff ./services/myaccount-bff/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /myaccount-bff /myaccount-bff

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/myaccount-bff"]
