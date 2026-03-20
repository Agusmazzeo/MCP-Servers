package apiclient

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// enhanceErrorMessage provides LLM-friendly error messages with actionable context
func enhanceErrorMessage(statusCode int, errorDetails, method, path string, body map[string]interface{}) string {
	// Extract helpful hints based on common error patterns
	var hint string

	// Check for specific error patterns
	switch {
	case strings.Contains(errorDetails, "At least one client type ID is required"):
		hint = "MISSING_CLIENT_TYPE_IDS: You must include 'typeIds' array in create_client. Use cached list_client_types data to get valid IDs."

	case strings.Contains(errorDetails, "interactionTypeId is required"):
		hint = "MISSING_INTERACTION_TYPE: You must include 'interactionTypeId' in create_interaction. Use cached list_interaction_types data."

	case strings.Contains(errorDetails, "foreign key constraint") && strings.Contains(errorDetails, "client_id"):
		hint = "CLIENT_NOT_FOUND: The client ID doesn't exist. Create the client first using create_client, then use the returned ID."

	case statusCode == 400 && strings.Contains(path, "searchName="):
		// Extract the search parameter to show what failed
		hint = "SEARCH_PARAMETER_ERROR: The searchName parameter may not support spaces. Try searching with first word only (e.g., 'Mercado' instead of 'Mercado Libre')."

	case statusCode == 400 && method == "POST":
		hint = "VALIDATION_ERROR: Check required fields. Missing or invalid parameters in request body."

	case statusCode == 404:
		hint = "NOT_FOUND: The resource doesn't exist. Verify the ID or create it first."

	case statusCode == 429:
		hint = "HTTP_429: Rate limit exceeded. Too many requests to the API. The system will stop to prevent further rate limiting."

	case statusCode == 500:
		hint = "SERVER_ERROR: Internal server error. Report this to user and don't retry."

	case statusCode >= 500:
		hint = fmt.Sprintf("HTTP_%d: Server error. The API service is experiencing issues.", statusCode)

	default:
		hint = fmt.Sprintf("HTTP_%d: %s", statusCode, errorDetails)
	}

	// Return structured error that LLM can parse and handle
	if errorDetails != "" {
		return fmt.Sprintf("%s | Details: %s", hint, errorDetails)
	}
	return hint
}

// isFatalError determines if an error should immediately stop execution
func isFatalError(errorMessage string) bool {
	// Fatal errors that should stop immediately:
	// 1. Rate limit errors (429)
	// 2. Server errors (500, 502, 503, 504)
	// 3. Authentication errors (401, 403)
	// 4. Critical validation errors (duplicate key violations)

	fatalPatterns := []string{
		"HTTP_429",           // Rate limit exceeded
		"HTTP_500",           // Internal server error
		"HTTP_502",           // Bad gateway
		"HTTP_503",           // Service unavailable
		"HTTP_504",           // Gateway timeout
		"HTTP_401",           // Unauthorized
		"HTTP_403",           // Forbidden
		"SERVER_ERROR",       // Internal server error hint
		"duplicate key",      // Database constraint violation
		"connection refused", // Service is down
		"context deadline exceeded", // Timeout
	}

	for _, pattern := range fatalPatterns {
		if strings.Contains(errorMessage, pattern) {
			return true
		}
	}

	return false
}

// isCountOnlyRequest determines if a GET request is asking for a count only
func isCountOnlyRequest(path string) bool {
	// Parse the URL to extract query parameters
	parsedURL, err := url.Parse(path)
	if err != nil {
		return false
	}

	query := parsedURL.Query()

	// Check if limit=0 (only case where we truly only want the count)
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit == 0 {
			// limit=0 means we only care about the count
			return true
		}
	}

	return false
}
