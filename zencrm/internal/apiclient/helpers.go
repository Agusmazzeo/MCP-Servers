package apiclient

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// getIntFromInput extracts an integer value from input map
func getIntFromInput(input map[string]interface{}, key string) (int, error) {
	value, ok := input[key]
	if !ok {
		return 0, fmt.Errorf("missing required field: %s", key)
	}
	return getIntFromValue(value)
}

// getIntFromValue converts various types to int
func getIntFromValue(value interface{}) (int, error) {
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("invalid type for integer: %T", value)
	}
}

// extractFields returns a function that extracts specified fields from input
func extractFields(fields ...string) func(map[string]interface{}) map[string]interface{} {
	return func(input map[string]interface{}) map[string]interface{} {
		payload := make(map[string]interface{})
		for _, field := range fields {
			if value, ok := input[field]; ok {
				payload[field] = value
			}
		}
		return payload
	}
}

// mapStatusToID maps user-friendly status strings to status IDs
// Status IDs: 1=Active, 2=Prospect, 3=Pending, 4=Inactive
func mapStatusToID(input map[string]interface{}) {
	if status, ok := input["status"]; ok {
		if statusStr, isStr := status.(string); isStr {
			switch strings.ToLower(statusStr) {
			case "active":
				input["filterStatusId"] = 1
			case "prospect":
				input["filterStatusId"] = 2
			case "pending":
				input["filterStatusId"] = 3
			case "inactive":
				input["filterStatusId"] = 4
			}
			// Remove the status field since API doesn't accept it
			delete(input, "status")
		}
	}
}

// buildQueryPath builds a URL path with query parameters
func buildQueryPath(basePath string, input map[string]interface{}, queryParams ...string) string {
	var params []string
	for _, param := range queryParams {
		if value, ok := input[param]; ok {
			// Handle array parameters (convert to comma-separated values)
			if arr, isArray := value.([]interface{}); isArray {
				if len(arr) > 0 {
					arrStrs := make([]string, len(arr))
					for i, item := range arr {
						arrStrs[i] = fmt.Sprintf("%v", item)
					}
					encodedValue := url.QueryEscape(strings.Join(arrStrs, ","))
					params = append(params, fmt.Sprintf("%s=%s", param, encodedValue))
				}
			} else {
				// URL-encode both the parameter name and value to handle special characters
				encodedValue := url.QueryEscape(fmt.Sprintf("%v", value))
				params = append(params, fmt.Sprintf("%s=%s", param, encodedValue))
			}
		}
	}

	if len(params) > 0 {
		return basePath + "?" + strings.Join(params, "&")
	}
	return basePath
}

// buildExportPath builds an export URL path with fields and filter parameters
func buildExportPath(basePath string, input map[string]interface{}) string {
	var params []string

	// Handle fields parameter (array of strings)
	if fields, ok := input["fields"].([]interface{}); ok && len(fields) > 0 {
		fieldStrs := make([]string, len(fields))
		for i, field := range fields {
			fieldStrs[i] = fmt.Sprintf("%v", field)
		}
		params = append(params, fmt.Sprintf("fields=%s", url.QueryEscape(strings.Join(fieldStrs, ","))))
	}

	// Handle all other parameters dynamically
	for key, value := range input {
		if key == "fields" {
			continue // Already handled above
		}

		// Convert value to string and add to params
		encodedValue := url.QueryEscape(fmt.Sprintf("%v", value))
		params = append(params, fmt.Sprintf("%s=%s", key, encodedValue))
	}

	if len(params) > 0 {
		return basePath + "?" + strings.Join(params, "&")
	}
	return basePath
}
