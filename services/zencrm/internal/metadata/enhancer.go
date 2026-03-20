package metadata

import (
	"fmt"
)

// GenerateMetadata generates metadata including frontend URLs for tool results
func GenerateMetadata(toolName string, input map[string]interface{}, response interface{}, frontendBaseURL string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Only generate URLs if frontend base URL is configured
	if frontendBaseURL == "" {
		return metadata
	}

	// Helper to extract ID from response
	getIDFromResponse := func(resp interface{}) (int, bool) {
		if respMap, ok := resp.(map[string]interface{}); ok {
			if id, ok := respMap["id"]; ok {
				switch v := id.(type) {
				case float64:
					return int(v), true
				case int:
					return v, true
				}
			}
		}
		return 0, false
	}

	// Helper to extract ID from input
	getIDFromInput := func(key string) (int, bool) {
		if id, ok := input[key]; ok {
			switch v := id.(type) {
			case float64:
				return int(v), true
			case int:
				return v, true
			}
		}
		return 0, false
	}

	// Generate frontend URL based on tool name
	switch toolName {
	case "create_interaction":
		if id, ok := getIDFromResponse(response); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/interactions/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this interaction in the CRM"
		}

	case "list_interactions":
		metadata["frontend_url"] = fmt.Sprintf("%s/interactions", frontendBaseURL)
		metadata["url_description"] = "View these interactions in the CRM"

	case "get_interaction":
		if id, ok := getIDFromInput("id"); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/interactions/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this interaction in the CRM"
		}

	case "create_client":
		if id, ok := getIDFromResponse(response); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/clients/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this client in the CRM"
		}

	case "list_clients", "list_clients_summary":
		metadata["frontend_url"] = fmt.Sprintf("%s/clients", frontendBaseURL)
		metadata["url_description"] = "View these clients in the CRM"

	case "get_client":
		if id, ok := getIDFromInput("id"); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/clients/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this client in the CRM"
		}

	case "list_users":
		metadata["frontend_url"] = fmt.Sprintf("%s/users", frontendBaseURL)
		metadata["url_description"] = "View these users in the CRM"

	case "get_user":
		if id, ok := getIDFromInput("id"); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/users/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this user in the CRM"
		}

	case "list_teams":
		metadata["frontend_url"] = fmt.Sprintf("%s/teams", frontendBaseURL)
		metadata["url_description"] = "View these teams in the CRM"

	case "get_team":
		if id, ok := getIDFromInput("id"); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/teams/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this team in the CRM"
		}

	case "list_accounts":
		metadata["frontend_url"] = fmt.Sprintf("%s/accounts", frontendBaseURL)
		metadata["url_description"] = "View these accounts in the CRM"

	case "get_account":
		if id, ok := getIDFromInput("id"); ok {
			metadata["frontend_url"] = fmt.Sprintf("%s/accounts/%d", frontendBaseURL, id)
			metadata["url_description"] = "View this account in the CRM"
		}
	}

	return metadata
}
