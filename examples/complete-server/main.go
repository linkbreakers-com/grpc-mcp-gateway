package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/linkbreakers-com/grpc-mcp-gateway/runtime"
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Configuration
	httpPort := getEnv("HTTP_PORT", "8080")
	grpcPort := getEnv("GRPC_PORT", "50051")

	log.Printf("Starting complete MCP server example")
	log.Printf("HTTP Port: %s", httpPort)
	log.Printf("gRPC Port: %s", grpcPort)

	// Start gRPC server in background
	grpcServer, grpcConn := startGrpcServer(grpcPort)
	defer grpcServer.Stop()
	defer grpcConn.Close()

	// Create MCP multiplexer with request logging
	mcpMux := runtime.NewMCPServeMux(
		runtime.ServerMetadata{
			Name:    "tasks-mcp-server",
			Version: "1.0.0",
		},
		runtime.WithRequestLogger(func(ctx context.Context, req *runtime.MCPRequest) {
			if req == nil {
				return
			}
			// Log all request types with details
			switch req.Method {
			case "tools/call":
				if name, ok := req.Params["name"].(string); ok && name != "" {
					log.Printf("MCP tools/call: %s", name)
				} else {
					log.Printf("MCP tools/call")
				}
			case "tools/list":
				log.Printf("MCP tools/list - client discovering available tools")
			case "initialize":
				log.Printf("MCP initialize - client connecting")
			case "notifications/initialized":
				log.Printf("MCP notifications/initialized - handshake complete")
			default:
				log.Printf("MCP %s", req.Method)
			}
		}),
	)

	// Register MCP handlers from gRPC services
	// In a real implementation, this would be generated code
	log.Printf("Registering MCP service handlers...")
	// RegisterTasksServiceMCPHandler(mcpMux, NewTasksServiceClient(grpcConn))
	log.Printf("All MCP service handlers registered successfully")

	// Setup authentication and HTTP middleware
	authHandler := withBearerAuth(mcpMux)
	authHandler = withAuthJSONRPC(authHandler)

	// HTTP mux with routes
	mux := http.NewServeMux()
	mux.Handle("/", authHandler)
	mux.HandleFunc("/healthz", healthHandler)

	// CORS configuration
	corsHandler := cors.New(cors.Options{
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedOrigins:     []string{"*"},
		AllowCredentials:   false,
		AllowedHeaders:     []string{"Authorization", "Content-Type"},
		OptionsPassthrough: false,
	})

	// Start HTTP server
	addr := ":" + httpPort
	log.Printf("MCP server listening on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  - / (MCP protocol)")
	log.Printf("  - /healthz (health check)")

	if err := http.ListenAndServe(addr, corsHandler.Handler(mux)); err != nil {
		log.Fatalf("Failed to start MCP HTTP server: %v", err)
	}
}

// startGrpcServer creates and starts a gRPC server
func startGrpcServer(grpcPort string) (*grpc.Server, *grpc.ClientConn) {
	listener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	// Register your gRPC services here
	// pb.RegisterTasksServiceServer(grpcServer, &tasksServer{})

	// Start serving in background
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Create client connection for MCP-to-gRPC calls
	grpcConn, err := grpc.NewClient(
		"localhost:"+grpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}

	return grpcServer, grpcConn
}

// withBearerAuth validates Bearer tokens
func withBearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if !validateToken(token) {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// withAuthJSONRPC wraps authentication errors in JSON-RPC format
func withAuthJSONRPC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and buffer the request body for logging on auth failure
		var requestBody []byte
		var jsonrpcMethod string
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				requestBody = bodyBytes
				// Parse JSON-RPC request to extract method
				var req struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(bodyBytes, &req) == nil {
					jsonrpcMethod = req.Method
				}
				// Restore body for next handler
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Capture response for error handling
		rec := newResponseRecorder()
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		// Convert auth errors to JSON-RPC errors
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			logMsg := fmt.Sprintf("MCP authentication failed - status: %d, method: %s, jsonrpc_method: %s",
				status, r.Method, jsonrpcMethod)
			if len(requestBody) > 0 && len(requestBody) < 500 {
				logMsg += fmt.Sprintf(", request: %s", string(requestBody))
			}
			log.Println(logMsg)

			// Copy headers
			for k, v := range rec.header {
				for _, vv := range v {
					w.Header().Add(k, vv)
				}
			}
			writeJSONRPCError(w, status, strings.TrimSpace(rec.body.String()))
			return
		}

		// Pass through successful responses
		for k, v := range rec.header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(status)
		w.Write(rec.body.Bytes())
	})
}

// responseRecorder captures HTTP responses
type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header: make(http.Header),
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

// writeJSONRPCError writes a JSON-RPC 2.0 error response
func writeJSONRPCError(w http.ResponseWriter, status int, message string) {
	if message == "" {
		message = "unauthorized"
	}
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

// validateToken validates bearer tokens (implement your own logic)
func validateToken(token string) bool {
	// Example: Accept a hardcoded token for demo purposes
	// In production, validate JWT, check database, etc.
	return token == "demo-token-12345" || len(token) > 10
}

// healthHandler provides a health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// loggingInterceptor logs gRPC requests
func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)

	status := "OK"
	if err != nil {
		status = "ERROR"
	}

	log.Printf("gRPC %s %s duration=%v", status, info.FullMethod, duration)
	return resp, err
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
