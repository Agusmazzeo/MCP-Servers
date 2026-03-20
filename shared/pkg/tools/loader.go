package tools

import (
	"encoding/json"
	"fmt"
	"os"
)

// ToolDefinition represents a tool definition from tools.json
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// LoadTools loads tool definitions from a JSON file
func LoadTools(filepath string) ([]*ToolDefinition, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools file: %w", err)
	}

	var tools []*ToolDefinition
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("failed to parse tools JSON: %w", err)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found in file")
	}

	return tools, nil
}
