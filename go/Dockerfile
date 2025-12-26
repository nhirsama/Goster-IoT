# Build Stage
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app
# CGO_ENABLED=0 ensures a statically linked binary suitable for Alpine
RUN CGO_ENABLED=0 GOOS=linux go build -o goster-iot .

# Final Stage
FROM alpine:latest

WORKDIR /app

# Install common dependencies (certificates, timezone)
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/goster-iot .

# Copy the html directory for web templates/static files
COPY --from=builder /app/html ./html

# Create a directory for persistent data
RUN mkdir /data

# Set environment variable for DB path
ENV DB_PATH=/data/data.db

# Expose port 8080
EXPOSE 8080

# Declare volume for persistence
VOLUME ["/data"]

# Run the binary
CMD ["./goster-iot"]
