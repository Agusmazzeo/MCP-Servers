# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY shared/go.mod shared/go.sum ./shared/
COPY services/zencrm/go.mod services/zencrm/go.sum ./services/zencrm/

# Download dependencies
RUN cd shared && go mod download
RUN cd services/zencrm && go mod download

# Copy source code
COPY shared/ ./shared/
COPY services/zencrm/ ./services/zencrm/

# Build the binary
RUN cd services/zencrm && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /zencrm-mcp ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /zencrm-mcp .

# Expose HTTP port
EXPOSE 8080

# Run the server in HTTP mode
CMD ["/app/zencrm-mcp", "-mode=http", "-port=8080"]
