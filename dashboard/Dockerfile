# ── Stage 1: Build the Go binary ──
FROM golang:alpine AS builder
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o binarypanel .

# ── Stage 2: Minimal runtime ──
FROM alpine:latest
RUN apk --no-cache add ca-certificates docker-cli docker-cli-compose git

COPY --from=builder /app/binarypanel /usr/local/bin/binarypanel
COPY --from=builder /app/static /static

EXPOSE 9000
ENTRYPOINT ["binarypanel"]
