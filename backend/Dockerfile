# ===================================================
# Stage 1: Build Stage
# ===================================================
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install ca-certificates and git for dependency download
RUN apk add --no-cache ca-certificates git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY . .

# Build static binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/server ./cmd/server

# ===================================================
# Stage 2: Minimal Production Runtime Stage
# ===================================================
FROM alpine:3.21

WORKDIR /app

# Security: run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Install trusted CA root certificates
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /app/server

USER appuser

EXPOSE 8080

ENV PORT=8080
ENV ENVIRONMENT=production

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:${PORT}/health || exit 1

ENTRYPOINT ["/app/server"]
