package tools

import (
	"encoding/json"
	"fmt"
	"os"
)

// ToolDefinition represents a single tool definition from tools.json
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// LoadTools loads tool definitions from a JSON file
func LoadTools(path string) ([]ToolDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools file: %w", err)
	}

	var tools []ToolDefinition
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("failed to parse tools JSON: %w", err)
	}

	return tools, nil
}
