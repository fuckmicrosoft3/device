# Development stage with air for live reloading
FROM golang:1.24-alpine AS dev

# Install air for live reloading and required tools
RUN go install github.com/air-verse/air@latest && \
    apk add --no-cache git

WORKDIR /app

# Copy go.work files for workspace support
COPY go.work* ./

# Copy the entire workspace structure needed for this service
COPY pkg/ ./pkg/
COPY services/sales/ ./services/sales/

# Set working directory to the service
WORKDIR /app/services/sales

# Default command for development
CMD ["air", "-c", ".air.toml"]

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.work for workspace support
COPY go.work* ./

# Copy modules
COPY pkg/ ./pkg/
COPY services/sales/ ./services/sales/

# Build the application
WORKDIR /app/services/sales
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sales-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/services/sales/sales-service .

# Copy config directory instead of env file
COPY --from=builder /app/services/sales/config /app/config

# Create a non-root user and set permissions
RUN adduser -D -g '' appuser && \
    chown -R appuser:appuser /app
USER appuser

# Expose the application port
EXPOSE 8080

# Set entry point
ENTRYPOINT ["./sales-service"]

# Default command based on cmd package
CMD ["api"]