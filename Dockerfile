FROM golang:1.23-alpine AS builder
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /protoc-gen-grpc-mcp-gateway \
    ./cmd/protoc-gen-mcp-gateway

FROM scratch
COPY --from=builder /protoc-gen-grpc-mcp-gateway /protoc-gen-grpc-mcp-gateway
ENTRYPOINT ["/protoc-gen-grpc-mcp-gateway"]
