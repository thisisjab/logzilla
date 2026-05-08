# Stage 1: Build stage
FROM golang:1.25-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o logzilla ./cmd/engine

# Stage 2: Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
# ca-certificates: for HTTPS requests
# tzdata: for timezone support
# lua5.4: for Lua processor support (Lua 5.4 is current stable)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    lua5.4 \
    && rm -rf /var/lib/apt/lists/*

# Create logzilla user and group (non-root)
RUN addgroup --gid 1000 logzilla && \
    adduser --uid 1000 --gid 1000 --disabled-password --gecos "" logzilla

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/logzilla .

# Copy configuration example (user should override with volume)
COPY config-example.yaml .

# Change ownership to logzilla user
RUN chown -R logzilla:logzilla /app

# Switch to non-root user
USER logzilla

# Expose default ports (adjust based on your server configuration)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["./logzilla"]

# Default command (can be overridden)
CMD ["-config", "/app/config.yaml"]
