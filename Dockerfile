# Multi-stage build Dockerfile using wolfi as final image
# Build stage
FROM cgr.dev/chainguard/go:latest AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -o debrid-downloader ./cmd/debrid-downloader

# Final stage using wolfi
FROM cgr.dev/chainguard/wolfi-base:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Create directories
RUN mkdir -p /app /downloads && \
    chown -R appuser:appuser /app /downloads

# Copy binary from builder stage
COPY --from=builder /app/debrid-downloader /app/debrid-downloader
RUN chmod +x /app/debrid-downloader

# Switch to non-root user
USER appuser

# Set working directory
WORKDIR /app

# Expose port
EXPOSE 3000

# Set default environment variables
ENV SERVER_PORT=3000
ENV DATABASE_PATH=/app/debrid.db
ENV BASE_DOWNLOADS_PATH=/downloads
ENV LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

# Command to run the application
CMD ["/app/debrid-downloader"]