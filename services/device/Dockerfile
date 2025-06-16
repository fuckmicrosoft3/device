# ---- Base Stage ----
# Use a slim, official Go image.
FROM golang:1.24-alpine as base

# Set the working directory inside the container
WORKDIR /app

# Copy Go modules and download dependencies.
# This is done in a separate layer to leverage Docker's caching.
COPY go.mod go.sum ./

# CORRECTED: Use `go mod tidy` to ensure all transitive dependencies are added to go.sum
# before the build stage. This resolves the "missing go.sum entry" error.
RUN go mod tidy

# ---- Dev Stage ----
# This stage is for development and includes Air for live reloading.
FROM base as dev

# Install Air for live-reloading
RUN go install github.com/air-verse/air

# Copy the rest of the application source code
COPY . .

# Expose the port the app runs on
EXPOSE 8080

# The command to start the app with live-reloading
CMD ["air"]


# ---- Build Stage ----
# This stage compiles the application into a single binary.
FROM base as builder

# Copy the entire source code
COPY . .

# Build the application, creating a static binary.
# CGO_ENABLED=0 is important for creating a truly static binary.
# -ldflags="-w -s" strips debug information to reduce the binary size.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/device-service ./cmd

# ---- Production Stage ----
# Use a minimal image for the final production container.
# "scratch" is an empty image, providing the smallest possible attack surface.
FROM scratch

# Set the working directory
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/device-service /app/device-service

# Copy the configuration file
COPY --from=builder /app/config/config.yaml /app/config/config.yaml

# This port must match the one your application listens on
EXPOSE 8080

# The command to run the application in production
ENTRYPOINT ["/app/device-service", "serve", "-c", "/app/config/config.yaml"]


