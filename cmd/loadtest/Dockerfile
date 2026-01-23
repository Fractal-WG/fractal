# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the loadtest binary
RUN CGO_ENABLED=1 GOOS=linux go build -o loadtest ./cmd/loadtest

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates postgresql-client jq

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/loadtest .

# Copy scripts if needed
COPY scripts/run-loadtest.sh scripts/analyze-loadtest.sh /app/scripts/

# Make scripts executable
RUN chmod +x /app/scripts/*.sh

# Default command (can be overridden)
CMD ["./loadtest"]
