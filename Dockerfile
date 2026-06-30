# Multi-stage build for any Voyager service
# Usage: docker build --build-arg SERVICE=api-gateway -t voyager-api-gateway .

ARG SERVICE=api-gateway

# Stage 1: Build
FROM golang:1.22-alpine AS builder

ARG SERVICE

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the specific service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/service ./cmd/${SERVICE}

# Stage 2: Runtime
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 appuser
USER appuser

WORKDIR /app

COPY --from=builder /app/service .
COPY --from=builder /app/configs ./configs

EXPOSE 8080 9090 50051 50052

ENTRYPOINT ["./service"]
