package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// MCPServeMux is a stateless request multiplexer for MCP JSON-RPC requests.
// It routes MCP tool calls to registered gRPC handlers.
type MCPServeMux struct {
	mu            sync.RWMutex
	tools         map[string]*ToolHandler
	metadata      ServerMetadata
	requestLogger RequestLogger
}

// ToolHandler handles an MCP tool call by invoking a gRPC method
type ToolHandler struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any
	ReadOnly    bool
	Idempotent  bool
	Destructive bool
	Handler     func(ctx context.Context, args map[string]any) (any, error)
}

// ServerMetadata contains server information
type ServerMetadata struct {
	Name    string
	Version string
}

// RequestLogger handles MCP request logging.
type RequestLogger func(ctx context.Context, req *MCPRequest)

// Option configures the MCPServeMux.
type Option func(*MCPServeMux)

// WithRequestLogger sets the request logger for MCP requests.
func WithRequestLogger(logger RequestLogger) Option {
	return func(mux *MCPServeMux) {
		if logger != nil {
			mux.requestLogger = logger
		}
	}
}

// NewMCPServeMux creates a new stateless MCP request multiplexer
func NewMCPServeMux(metadata ServerMetadata, opts ...Option) *MCPServeMux {
	mux := &MCPServeMux{
		tools:         make(map[string]*ToolHandler),
		metadata:      metadata,
		requestLogger: func(context.Context, *MCPRequest) {},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(mux)
		}
	}
	return mux
}

// RegisterTool registers a new tool handler
func (mux *MCPServeMux) RegisterTool(tool *ToolHandler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()
	mux.tools[tool.Name] = tool
}

// ServeHTTP implements http.Handler for stateless MCP JSON-RPC requests
func (mux *MCPServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		sendError(w, nil, -32600, "Invalid request method")
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, nil, -32700, fmt.Sprintf("Parse error: %v", err))
		return
	}

	ctx := r.Context()
	mux.requestLogger(ctx, &req)

	switch req.Method {
	case "initialize":
		mux.handleInitialize(w, ctx, req.ID)
	case "notifications/initialized":
		// Client notification that initialization is complete.
		// Per JSON-RPC 2.0 spec, notifications (ID == nil) don't expect a response.
		// Just acknowledge it silently by sending empty 204 response.
		w.WriteHeader(http.StatusNoContent)
	case "tools/list":
		mux.handleListTools(w, ctx, req.ID)
	case "tools/call":
		mux.handleCallTool(w, ctx, req.ID, req.Params)
	default:
		// Per JSON-RPC 2.0 spec, notifications (requests with ID == nil) don't get error responses.
		// Only respond with error if this was an actual request (has an ID).
		if req.ID != nil {
			sendError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		} else {
			// Unknown notification - silently ignore
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP JSON-RPC error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (mux *MCPServeMux) handleInitialize(w http.ResponseWriter, ctx context.Context, id interface{}) {
	result := map[string]interface{}{
		"protocolVersion": "2025-11-25",
		"serverInfo": map[string]interface{}{
			"name":    mux.metadata.Name,
			"version": mux.metadata.Version,
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}

	sendSuccess(w, id, result)
}

func (mux *MCPServeMux) handleListTools(w http.ResponseWriter, ctx context.Context, id interface{}) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(mux.tools))
	for _, tool := range mux.tools {
		t := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if tool.Title != "" {
			t["title"] = tool.Title
		}
		if tool.InputSchema != nil {
			t["inputSchema"] = tool.InputSchema
		} else {
			t["inputSchema"] = DefaultInputSchema()
		}

		annotations := make(map[string]interface{})
		if tool.ReadOnly {
			annotations["readOnlyHint"] = true
		}
		if tool.Idempotent {
			annotations["idempotentHint"] = true
		}
		if tool.Destructive {
			annotations["destructiveHint"] = true
		}
		if len(annotations) > 0 {
			t["annotations"] = annotations
		}

		tools = append(tools, t)
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	sendSuccess(w, id, result)
}

// DefaultInputSchema provides a permissive object schema for tool inputs.
func DefaultInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": true,
	}
}

func (mux *MCPServeMux) handleCallTool(w http.ResponseWriter, ctx context.Context, id interface{}, params map[string]interface{}) {
	toolName, ok := params["name"].(string)
	if !ok {
		sendError(w, id, -32602, "Missing tool name")
		return
	}

	arguments, _ := params["arguments"].(map[string]interface{})

	mux.mu.RLock()
	tool, exists := mux.tools[toolName]
	mux.mu.RUnlock()

	if !exists {
		sendError(w, id, -32601, fmt.Sprintf("Tool not found: %s", toolName))
		return
	}

	// Call the tool handler
	output, err := tool.Handler(ctx, arguments)
	if err != nil {
		sendError(w, id, -32000, err.Error())
		return
	}

	// Format response
	response := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("%v", output),
			},
		},
		"structuredContent": output,
	}

	sendSuccess(w, id, response)
}

func sendSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if code == -32600 || code == -32601 {
		status = http.StatusBadRequest
	} else if code == -32700 {
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}
