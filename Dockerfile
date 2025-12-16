# Build stage
FROM golang:1.24-alpine AS builder

# Install git and curl (needed for go mod download and health check)
RUN apk add --no-cache git curl

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/api/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and curl for HTTPS requests and health checks
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy web assets
COPY --from=builder /app/web ./web

# Copy .env file (optional, can be overridden by environment variables)
COPY .env.example .env

# Change ownership to non-root user
RUN chown -R appuser:appgroup /root/

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8081

# Health check 
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:8081/api/v1/health || exit 1

# Run the application
CMD ["./main"]