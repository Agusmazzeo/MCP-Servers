# Dockerfile for OMS MCP Server
# Build context: oms/
# This Dockerfile is designed for DigitalOcean App Platform deployment

# Build stage
FROM golang:1.25.8-alpine AS builder

# Install git (required for go mod download from GitHub)
RUN apk add --no-cache git

# Allow Go to download newer toolchain versions automatically
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /oms-mcp ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /oms-mcp .

# Copy tools directory
COPY --from=builder /app/tools ./tools

# Set default tools path (can be overridden via environment variable)
ENV TOOLS_PATH=/app/tools/tools.json

# Expose HTTP port
EXPOSE 8083

# Run the server in HTTP mode
CMD ["/app/oms-mcp", "-mode=http", "-port=8083"]
