# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY shared/go.mod shared/go.sum ./shared/
COPY services/allfunds/go.mod services/allfunds/go.sum ./services/allfunds/

# Download dependencies
RUN cd shared && go mod download
RUN cd services/allfunds && go mod download

# Copy source code
COPY shared/ ./shared/
COPY services/allfunds/ ./services/allfunds/

# Build the binary
RUN cd services/allfunds && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /allfunds-mcp ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /allfunds-mcp .

# Expose HTTP port
EXPOSE 8081

# Run the server in HTTP mode
CMD ["/app/allfunds-mcp", "-mode=http", "-port=8081"]
