# Complete MCP Server Example

This example demonstrates a **production-ready** MCP server implementation using `grpc-mcp-gateway`. It showcases all the essential patterns for building a secure, observable MCP server with proper authentication, logging, and error handling.

## Features Demonstrated

### 1. **Request Logging**
- Comprehensive logging of all MCP protocol messages
- Special handling for different request types (`initialize`, `tools/list`, `tools/call`)
- Request/response correlation for debugging

### 2. **Bearer Token Authentication**
- JWT/Token validation middleware
- Proper HTTP 401/403 error handling
- JSON-RPC 2.0 compliant error responses

### 3. **JSON-RPC Error Handling**
- Converts HTTP authentication errors to JSON-RPC format
- Preserves request context for debugging failed auth attempts
- Logs both HTTP and JSON-RPC method information

### 4. **Health Checks**
- Standard `/healthz` endpoint for Kubernetes/Docker
- Separate from MCP protocol endpoint

### 5. **CORS Support**
- Configurable CORS headers
- Essential for web-based MCP clients

### 6. **Dual Server Architecture**
- gRPC server for business logic
- HTTP server for MCP protocol
- Local gRPC client connection for MCP-to-gRPC bridging

## Architecture

```
┌─────────────┐
│ MCP Client  │
│ (Claude)    │
└──────┬──────┘
       │ HTTP + JSON-RPC
       │ Bearer Token
       ▼
┌─────────────────────────────┐
│  HTTP Server (:8080)        │
│  ├─ CORS Middleware         │
│  ├─ Auth Middleware         │
│  └─ MCP Multiplexer         │
└──────┬──────────────────────┘
       │ Local gRPC
       ▼
┌─────────────────────────────┐
│  gRPC Server (:50051)       │
│  └─ TasksService            │
└─────────────────────────────┘
```

## Running the Example

### Prerequisites

1. Install Go 1.21+
2. Install protobuf compiler: `brew install protobuf`
3. Install Go protobuf plugins:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/linkbreakers-com/protoc-gen-go-mcp@latest
```

### Generate Code

```bash
# Generate gRPC and MCP handler code
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       --go-mcp_out=. --go-mcp_opt=paths=source_relative \
       tasks.proto
```

### Run the Server

```bash
# Default ports (HTTP: 8080, gRPC: 50051)
go run main.go

# Custom ports
HTTP_PORT=3000 GRPC_PORT=50052 go run main.go
```

## Testing the Server

### 1. Health Check

```bash
curl http://localhost:8080/healthz
# Expected: ok
```

### 2. MCP Initialize (without auth - should fail)

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test", "version": "1.0.0"}
    }
  }'
# Expected: 401 Unauthorized (Missing Authorization header)
```

### 3. MCP Initialize (with auth)

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo-token-12345" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test", "version": "1.0.0"}
    }
  }'
# Expected: JSON-RPC success response with server capabilities
```

### 4. Send Notification (notifications/initialized)

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo-token-12345" \
  -d '{
    "jsonrpc": "2.0",
    "method": "notifications/initialized",
    "params": {}
  }'
# Expected: 204 No Content (notifications don't get responses)
```

### 5. List Tools

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo-token-12345" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
# Expected: JSON-RPC response with list of available tools
```

## Key Implementation Patterns

### Pattern 1: Request Logger

The request logger helps you understand what's happening in your MCP server:

```go
runtime.WithRequestLogger(func(ctx context.Context, req *runtime.MCPRequest) {
    switch req.Method {
    case "tools/call":
        if name, ok := req.Params["name"].(string); ok {
            log.Printf("MCP tools/call: %s", name)
        }
    case "tools/list":
        log.Printf("MCP tools/list")
    case "initialize":
        log.Printf("MCP initialize")
    case "notifications/initialized":
        log.Printf("MCP notifications/initialized")
    }
})
```

### Pattern 2: Authentication Middleware Chain

Layer your middleware properly:

```go
// 1. Parse and validate Bearer token
authHandler := withBearerAuth(mcpMux)

// 2. Convert auth errors to JSON-RPC format
authHandler = withAuthJSONRPC(authHandler)

// 3. Apply CORS
corsHandler := cors.New(cors.Options{...})
http.ListenAndServe(addr, corsHandler.Handler(authHandler))
```

### Pattern 3: JSON-RPC Error Responses

Always return proper JSON-RPC errors for protocol compliance:

```go
func writeJSONRPCError(w http.ResponseWriter, status int, message string) {
    resp := map[string]any{
        "jsonrpc": "2.0",
        "id":      nil,
        "error": map[string]any{
            "code":    -32000,
            "message": message,
        },
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(resp)
}
```

### Pattern 4: Response Recording for Auth

Capture the response to detect auth failures and convert them:

```go
func withAuthJSONRPC(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rec := newResponseRecorder()
        next.ServeHTTP(rec, r)

        if rec.status == http.StatusUnauthorized {
            writeJSONRPCError(w, rec.status, "unauthorized")
            return
        }

        // Pass through normal responses
        w.WriteHeader(rec.status)
        w.Write(rec.body.Bytes())
    })
}
```

## Production Considerations

### 1. Authentication

Replace the simple token validation with:
- JWT validation (using `github.com/golang-jwt/jwt`)
- Database lookups for API keys
- OAuth2/OIDC integration
- Rate limiting per token

### 2. Observability

Add proper observability:
- Structured logging (using `zerolog`, `zap`, or `slog`)
- Metrics (Prometheus/OpenTelemetry)
- Distributed tracing
- Error tracking (Sentry, Rollbar)

### 3. Security

Enhance security:
- Use TLS/HTTPS in production
- Implement rate limiting (per IP, per token)
- Add request size limits
- Validate all inputs
- Use secure token storage

### 4. Kubernetes Deployment

Example Kubernetes manifests:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-server
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mcp-server
  template:
    metadata:
      labels:
        app: mcp-server
    spec:
      containers:
      - name: mcp-server
        image: your-registry/mcp-server:latest
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: HTTP_PORT
          value: "8080"
        - name: GRPC_PORT
          value: "50051"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
```

## Common Issues

### Issue 1: Client Disconnects After Initialize

**Symptom**: Logs show "initialize" but client immediately disconnects

**Cause**: Server returning errors for `notifications/initialized`

**Solution**: Ensure you're using `grpc-mcp-gateway` v0.4.1+ which properly handles notifications

### Issue 2: CORS Errors in Browser

**Symptom**: Browser console shows CORS errors

**Solution**: Configure CORS properly:
```go
cors.New(cors.Options{
    AllowedOrigins: []string{"https://yourdomain.com"},
    AllowedHeaders: []string{"Authorization", "Content-Type"},
    AllowedMethods: []string{"POST", "OPTIONS"},
})
```

### Issue 3: Authentication Loops

**Symptom**: Client keeps retrying authentication

**Solution**: Check that your auth middleware returns proper JSON-RPC errors, not plain HTTP errors

## Resources

- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [gRPC-MCP Gateway Documentation](https://github.com/linkbreakers-com/grpc-mcp-gateway)

## License

This example is provided as-is for educational purposes.
