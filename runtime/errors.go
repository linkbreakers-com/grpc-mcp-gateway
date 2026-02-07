package runtime

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolError returns a CallToolResult that marks the tool invocation as failed.
func ToolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
	}
}
