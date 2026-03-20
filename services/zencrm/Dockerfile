# Dockerfile for ZenCRM MCP Server
# Build context: services/zencrm/
# This Dockerfile is designed for DigitalOcean App Platform deployment

# Build stage
FROM golang:1.23-alpine AS builder

# Install git (required for go mod download from GitHub)
RUN apk add --no-cache git

# Allow Go to download newer toolchain versions automatically
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies (shared package will be fetched from GitHub)
RUN go mod download

# Copy source code
COPY . ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /zencrm-mcp ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /zencrm-mcp .

# Expose HTTP port
EXPOSE 8080

# Run the server in HTTP mode
CMD ["/app/zencrm-mcp", "-mode=http", "-port=8080"]
