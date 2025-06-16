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
COPY services/device/ ./services/device/

# Set working directory to the service
WORKDIR /app/services/device

# Default command for development
CMD ["air", "-c", ".air.toml"]

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.work for workspace support
COPY go.work* ./

# Copy modules
COPY pkg/ ./pkg/
COPY services/device/ ./services/device/

# Build the service
WORKDIR /app/services/device
RUN go build -o device-service .

FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/services/device/device-service .

# Copy config directory
COPY --from=builder /app/services/device/config /app/config

# Expose service port
EXPOSE 8091

# Default command (matches available command in cmd package)
ENTRYPOINT ["./device-service"]
