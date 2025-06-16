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
COPY services/truck/ ./services/truck/

# Set working directory to the service
WORKDIR /app/services/truck

# Default command for development
CMD ["air", "-c", ".air.toml"]

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go.work for workspace support
COPY go.work* ./

# Copy modules
COPY pkg/ ./pkg/
COPY services/truck/ ./services/truck/

# Build the application
WORKDIR /app/services/truck
RUN CGO_ENABLED=0 GOOS=linux go build -o operations-service .

# Final stage
FROM alpine:3.18

WORKDIR /app

# Install CA certificates
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/services/truck/operations-service /app/operations-service

# Copy config directory
COPY --from=builder /app/services/truck/config /app/config

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Expose port
EXPOSE 8000

# Set entry point
ENTRYPOINT ["/app/operations-service"]
CMD ["serve"]