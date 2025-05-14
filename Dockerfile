# ---- Build Stage ----
# Use an official Go image as the builder.
# Choose a specific Go version matching your project.
FROM golang:1.23-alpine AS builder
# Using alpine for a smaller builder image, but bookworm (Debian) is also fine if you have CGO dependencies.

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy the source code into the container
COPY . .

# Build the Go application
# -o /app/main: output the binary to /app/main
# -ldflags="-w -s": strip debug symbols and symbol table to reduce binary size (optional, good for production)
# CGO_ENABLED=0: disables CGO, needed for truly static binaries if using Alpine as the final base
# GOOS=linux GOARCH=amd64: explicitly set target OS/architecture (good practice)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/main .
# If your main.go is not in the root, adjust the last "." e.g., ./cmd/app/ if main.go is in cmd/app

# ---- Release Stage ----
# Use a minimal base image for the final container.
# Alpine is very small. For Debian-based, debian:stable-slim or distroless are options.
FROM alpine:latest
# FROM debian:stable-slim # If you prefer Debian and don't need Alpine's extreme smallness

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/main /app/main

# Copy frontend assets (if your Go binary serves them from a relative path)
COPY ./frontend ./frontend
# Copy certs (if needed inside the container and not mounted as a volume)
# COPY ./certs ./certs

# Expose the port your Go application listens on (e.g., 8443 or 5895)
# Replace 5895 with the actual port from your SERVER_ADDR config
EXPOSE 5895

# Command to run the executable
# The binary is now at /app/main
CMD ["/app/main"]