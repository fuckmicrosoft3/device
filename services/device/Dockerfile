# services/device/Dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o device-service ./main.go

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Create data directories
RUN mkdir -p /data/firmware /data/wal /data/dead_letter /data/backup && \
    chown -R appuser:appgroup /data

# Copy binary from builder
COPY --from=builder /app/device-service .

# Copy config file
COPY config/config.yaml ./config/

# Change to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["./device-service"]
CMD ["serve"]


