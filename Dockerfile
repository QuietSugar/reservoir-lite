# Build stage
FROM golang:1.26-alpine3.24 AS builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

# Pre-fetch dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build statically linked binary
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /build/reservoir main.go

# Runtime stage
FROM alpine:3.20

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories 2>/dev/null || true && \
    apk add --no-cache ca-certificates openssl tzdata curl

WORKDIR /app

COPY --from=builder /build/reservoir /app/reservoir
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh && \
    mkdir -p /app/ssl /app/var/cache

# Port 9999: Proxy service & Web management dashboard
EXPOSE 9999

VOLUME ["/app/ssl", "/app/var"]

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["--listen", "0.0.0.0:9999", "--no-api", "--log-to-stdout"]
