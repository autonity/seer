FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies (git + gcc for CGO)
RUN apk add --no-cache git gcc musl-dev linux-headers

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with CGO enabled (required for blst library)
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.buildTime=$(date -u '+%Y-%m-%dT%H:%M:%S')" \
    -o seer ./cmd

# Final minimal image
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata libgcc libstdc++

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/seer .

# Copy ABIs
COPY --from=builder /app/abis ./abis

# Create non-root user
RUN addgroup -g 1000 seer && \
    adduser -D -u 1000 -G seer seer && \
    chown -R seer:seer /app

USER seer

ENTRYPOINT ["./seer"]
CMD ["start", "--config", "/config/config.yaml"]

