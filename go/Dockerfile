# syntax=docker/dockerfile:1.7

# Build Stage
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the source code
COPY . .

# Build the Go app
# CGO_ENABLED=0 ensures a statically linked binary suitable for Alpine
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o goster-iot .

# Final Stage
FROM alpine:latest

WORKDIR /app

# Install common dependencies (certificates, timezone)
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/goster-iot .

# Copy database migrations used by `goster-iot db migrate`
COPY --from=builder /app/db ./db

# Create a directory for persistent data
RUN mkdir /data

# Set environment variable for DB path
ENV DB_PATH=/data/data.db

# Expose web and device TCP API ports
EXPOSE 8080
EXPOSE 8081

# Declare volume for persistence
VOLUME ["/data"]

# Run the binary
CMD ["./goster-iot"]
