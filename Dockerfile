# =========================
# Build Stage
# =========================
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the CLI binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/terraform-cost \
    ./cmd/cli

# Build the server binary (if exists)
# RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
#     -ldflags='-w -s -extldflags "-static"' \
#     -o /build/terraform-cost-server \
#     ./cmd/server

# =========================
# Production Stage
# =========================
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Import user from builder
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Create directories
RUN mkdir -p /app/data /app/cache /app/config && \
    chown -R appuser:appuser /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/terraform-cost /app/terraform-cost

# Copy examples for testing
COPY --from=builder /build/examples /app/examples

# Use non-root user
USER appuser

# Environment variables
ENV HOME=/app \
    TERRAFORM_COST_CACHE_DIR=/app/cache \
    TERRAFORM_COST_DATA_DIR=/app/data

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/terraform-cost", "version"]

# Default entrypoint
ENTRYPOINT ["/app/terraform-cost"]

# Default command (show help)
CMD ["--help"]
