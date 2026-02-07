package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// MCPServeMux is a stateless request multiplexer for MCP JSON-RPC requests.
// It routes MCP tool calls to registered gRPC handlers.
type MCPServeMux struct {
	mu       sync.RWMutex
	tools    map[string]*ToolHandler
	metadata ServerMetadata
}

// ToolHandler handles an MCP tool call by invoking a gRPC method
type ToolHandler struct {
	Name        string
	Title       string
	Description string
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

// NewMCPServeMux creates a new stateless MCP request multiplexer
func NewMCPServeMux(metadata ServerMetadata) *MCPServeMux {
	return &MCPServeMux{
		tools:    make(map[string]*ToolHandler),
		metadata: metadata,
	}
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
	logMCPRequest(&req)

	switch req.Method {
	case "initialize":
		mux.handleInitialize(w, ctx, req.ID)
	case "tools/list":
		mux.handleListTools(w, ctx, req.ID)
	case "tools/call":
		mux.handleCallTool(w, ctx, req.ID, req.Params)
	default:
		sendError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func logMCPRequest(req *MCPRequest) {
	if req == nil {
		return
	}
	if req.Method == "tools/call" {
		if name, ok := req.Params["name"].(string); ok && name != "" {
			log.Printf("MCP tools/call %s", name)
			return
		}
		log.Printf("MCP tools/call")
		return
	}
	log.Printf("MCP %s", req.Method)
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
		"protocolVersion": "2024-11-05",
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
