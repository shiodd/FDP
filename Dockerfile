FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /file-download-proxy .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /file-download-proxy /file-download-proxy

EXPOSE 9517

CMD ["/file-download-proxy"]