package apiclient

import "fmt"

// RouteConfig defines how to map a tool to an API endpoint
type RouteConfig struct {
	Method   string
	PathFunc func(input map[string]interface{}) string
	BodyFunc func(input map[string]interface{}) map[string]interface{}
}

// initializeRoutes sets up all tool-to-endpoint mappings
func (c *Client) initializeRoutes() {
	c.routes = make(map[string]RouteConfig)

	// ==================== AUTHENTICATION ====================
	c.routes["login"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/auth/login"
		},
		BodyFunc: extractFields("email", "password"),
	}

	// ==================== USER MANAGEMENT ====================
	// User routes
	c.routes["list_users"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/users", input, "searchFirstName", "searchLastName", "searchEmail", "searchPhone", "sortBy", "sortOrder", "limit", "offset")
		},
	}
	c.routes["get_user"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/users/%d", id)
		},
	}
	c.routes["create_user"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/users"
		},
		BodyFunc: extractFields("email", "username", "password", "firstName", "lastName", "phone", "avatarImage", "userStatusId", "roleId"),
	}
	c.routes["update_user"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/users/%d", id)
		},
		BodyFunc: extractFields("name", "email", "roleId", "isActive"),
	}

	// Role routes
	c.routes["list_roles"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/roles", input, "limit", "offset")
		},
	}
	c.routes["get_role"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/roles/%d", id)
		},
	}
	c.routes["create_role"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/roles"
		},
		BodyFunc: extractFields("name", "description", "level", "isActive"),
	}

	// Client routes
	c.routes["list_client_types"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/client-types", input, "limit", "offset")
		},
	}
	c.routes["list_client_statuses"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/clients/statuses", input, "limit", "offset")
		},
	}
	c.routes["list_client_categories"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/clients/categories", input, "limit", "offset", "sortBy")
		},
	}
	c.routes["list_clients_summary"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			mapStatusToID(input)
			return buildQueryPath("/v1/clients/summary", input, "searchName", "searchSimilarName", "similarNameThreshold", "searchFinancialAdvisor", "searchClientAssociate", "filterStatusId", "filterCategoryIds", "filterTypeIds", "filterFinancialAdvisorIds", "filterClientAssociateIds", "sortBy", "sortOrder", "limit", "offset")
		},
	}
	c.routes["list_clients"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			mapStatusToID(input)
			return buildQueryPath("/v1/clients", input, "searchName", "searchSimilarName", "similarNameThreshold", "searchFinancialAdvisor", "searchClientAssociate", "filterStatusId", "filterCategoryIds", "filterTypeIds", "filterFinancialAdvisorIds", "filterClientAssociateIds", "sortBy", "sortOrder", "limit", "offset")
		},
	}
	c.routes["get_client"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d", id)
		},
	}
	c.routes["create_client"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/clients"
		},
		BodyFunc: func(input map[string]interface{}) map[string]interface{} {
			payload := make(map[string]interface{})

			// Map LLM fields to API fields
			if name, ok := input["name"]; ok {
				payload["name"] = name
			}
			if description, ok := input["description"]; ok {
				payload["description"] = description
			}

			// Map typeIds -> clientTypeIds (API expects this field name)
			if typeIds, ok := input["typeIds"]; ok {
				payload["clientTypeIds"] = typeIds
			}

			// Map status string to clientStatusId
			if statusID, ok := input["clientStatusId"]; ok {
				payload["clientStatusId"] = statusID
			} else if status, ok := input["status"]; ok {
				// Map status name to ID
				statusStr, isStr := status.(string)
				if isStr {
					switch statusStr {
					case "prospect":
						payload["clientStatusId"] = 2 // Prospect status
					case "active":
						payload["clientStatusId"] = 1 // Active status
					case "pending":
						payload["clientStatusId"] = 3 // Pending status
					case "inactive":
						payload["clientStatusId"] = 4 // Inactive status
					}
				}
			}

			// Optional fields
			if isActive, ok := input["isActive"]; ok {
				payload["isActive"] = isActive
			} else {
				payload["isActive"] = true // Default to active
			}

			// Required: clientCategoryIds (empty array is acceptable)
			if categoryIds, ok := input["clientCategoryIds"]; ok {
				payload["clientCategoryIds"] = categoryIds
			} else {
				payload["clientCategoryIds"] = []interface{}{}
			}

			return payload
		},
	}
	c.routes["update_client"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d", id)
		},
		BodyFunc: func(input map[string]interface{}) map[string]interface{} {
			payload := make(map[string]interface{})

			// Simple string fields
			if name, ok := input["name"]; ok {
				payload["name"] = name
			}
			if description, ok := input["description"]; ok {
				payload["description"] = description
			}

			// Team and user assignments
			if assignedTeamIds, ok := input["assignedTeamIds"]; ok {
				payload["assignedTeamIds"] = assignedTeamIds
			}
			if financialAdvisorIds, ok := input["financialAdvisorIds"]; ok {
				payload["financialAdvisorIds"] = financialAdvisorIds
			}
			if clientAssociateIds, ok := input["clientAssociateIds"]; ok {
				payload["clientAssociateIds"] = clientAssociateIds
			}

			// Type and category arrays
			if clientTypeIds, ok := input["clientTypeIds"]; ok {
				payload["clientTypeIds"] = clientTypeIds
			}
			if clientCategoryIds, ok := input["clientCategoryIds"]; ok {
				payload["clientCategoryIds"] = clientCategoryIds
			}

			// Status - support both direct ID and string name
			if statusID, ok := input["clientStatusId"]; ok {
				payload["clientStatusId"] = statusID
			} else if status, ok := input["status"]; ok {
				// Map status string to ID
				statusStr, isStr := status.(string)
				if isStr {
					switch statusStr {
					case "prospect":
						payload["clientStatusId"] = 2
					case "active":
						payload["clientStatusId"] = 1
					case "pending":
						payload["clientStatusId"] = 3
					case "inactive":
						payload["clientStatusId"] = 4
					}
				}
			}

			// Active status
			if isActive, ok := input["isActive"]; ok {
				payload["isActive"] = isActive
			}

			return payload
		},
	}

	// Contact routes
	c.routes["list_contacts"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/clients/contacts", input, "limit", "offset")
		},
	}
	c.routes["get_client_contacts"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			return fmt.Sprintf("/v1/clients/%d/contacts", clientId)
		},
	}
	c.routes["create_contact"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			return fmt.Sprintf("/v1/clients/%d/contacts", clientId)
		},
		BodyFunc: extractFields("name", "email", "phone", "roleId"),
	}

	// Interaction routes
	c.routes["list_interactions"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/interactions", input,
				"searchSummary", "searchSemantic", "semanticThreshold", "semanticLimit",
				"filterClientIds", "filterContactIds", "filterUserIds",
				"filterInteractionType", "dateFrom", "dateTo", "followUpDateFrom",
				"followUpDateTo", "hasFollowUp", "excludeFields", "sortBy", "sortOrder", "limit", "offset")
		},
	}
	c.routes["get_interaction"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/interactions/%d", id)
		},
	}
	c.routes["create_interaction"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/interactions"
		},
		BodyFunc: extractFields("date", "interactionTypeId", "summary", "details", "followUpNotes", "followUpDate", "clientIds", "contactIds", "userIds"),
	}
	c.routes["update_interaction"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/interactions/%d", id)
		},
		BodyFunc: extractFields("date", "interactionTypeId", "summary", "details", "followUpNotes", "followUpDate", "clientIds", "contactIds", "userIds", "isActive"),
	}

	// Interaction Type routes
	c.routes["create_interaction_type"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/interactions/types"
		},
		BodyFunc: extractFields("name", "description"),
	}
	c.routes["get_interaction_type"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/interactions/types/%d", id)
		},
	}
	c.routes["list_interaction_types"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/interactions/types", input, "limit", "offset")
		},
	}
	c.routes["update_interaction_type"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/interactions/types/%d", id)
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["delete_interaction_type"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/interactions/types/%d", id)
		},
	}

	// Team routes
	c.routes["list_teams"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/teams", input, "limit", "offset")
		},
	}
	c.routes["get_team"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/teams/%d", id)
		},
	}
	c.routes["create_team"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/teams"
		},
		BodyFunc: extractFields("name", "description"),
	}

	// Notification routes
	c.routes["list_notifications"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			if unreadOnly, ok := input["unreadOnly"].(bool); ok && unreadOnly {
				return buildQueryPath("/v1/notifications/unread", input, "limit", "offset")
			}
			return buildQueryPath("/v1/notifications", input, "limit", "offset")
		},
	}
	c.routes["mark_notifications_read"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			if markAll, ok := input["markAllRead"].(bool); ok && markAll {
				return "/v1/notifications/mark-all-read"
			}
			return "/v1/notifications/mark-read"
		},
		BodyFunc: func(input map[string]interface{}) map[string]interface{} {
			body := make(map[string]interface{})
			if ids, ok := input["notificationIds"]; ok {
				body["notificationIds"] = ids
			}
			return body
		},
	}

	// Account routes
	c.routes["list_accounts"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/accounts", input, "limit", "offset")
		},
	}
	c.routes["create_account"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/accounts"
		},
		BodyFunc: extractFields("clientId", "platformId", "accountTypeId", "accountNumber", "description", "isActive"),
	}
	c.routes["get_account"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/%d", id)
		},
	}
	c.routes["update_account"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/%d", id)
		},
		BodyFunc: extractFields("platformId", "accountTypeId", "accountNumber", "description", "isActive"),
	}
	c.routes["delete_account"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/%d", id)
		},
	}
	c.routes["get_client_accounts"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			return fmt.Sprintf("/v1/clients/%d/accounts", clientId)
		},
	}

	// Account Platform routes
	c.routes["list_account_platforms"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/accounts/platforms", input, "limit", "offset")
		},
	}
	c.routes["create_account_platform"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/accounts/platforms"
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["get_account_platform"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/platforms/%d", id)
		},
	}
	c.routes["update_account_platform"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/platforms/%d", id)
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["delete_account_platform"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/platforms/%d", id)
		},
	}

	// Account Type routes
	c.routes["list_account_types"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/accounts/types", input, "limit", "offset")
		},
	}
	c.routes["create_account_type"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/accounts/types"
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["get_account_type"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/types/%d", id)
		},
	}
	c.routes["update_account_type"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/types/%d", id)
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["delete_account_type"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/accounts/types/%d", id)
		},
	}

	// Contact Role routes
	c.routes["list_contact_roles"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/clients/contacts/roles", input, "limit", "offset")
		},
	}
	c.routes["create_contact_role"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/clients/contacts/roles"
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["get_contact_role"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/contacts/roles/%d", id)
		},
	}
	c.routes["update_contact_role"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/contacts/roles/%d", id)
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["delete_contact_role"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/contacts/roles/%d", id)
		},
	}

	// Client Interaction Frequency routes
	c.routes["create_interaction_frequency"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			return fmt.Sprintf("/v1/clients/%d/interactions/frequencies", clientId)
		},
		BodyFunc: extractFields("interactionTypeId", "frequencyDays", "isActive"),
	}
	c.routes["get_interaction_frequencies"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			return fmt.Sprintf("/v1/clients/%d/interactions/frequencies", clientId)
		},
	}
	c.routes["update_interaction_frequency"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d/interactions/frequencies/%d", clientId, id)
		},
		BodyFunc: extractFields("interactionTypeId", "frequencyDays", "isActive"),
	}
	c.routes["delete_interaction_frequency"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d/interactions/frequencies/%d", clientId, id)
		},
	}

	// Export URL generation routes
	c.routes["generate_accounts_export_url"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildExportPath("/v1/accounts/export", input)
		},
	}
	c.routes["generate_clients_export_url"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildExportPath("/v1/clients/export", input)
		},
	}
	c.routes["generate_interactions_export_url"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildExportPath("/v1/interactions/export", input)
		},
	}
	c.routes["generate_contacts_export_url"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildExportPath("/v1/clients/contacts/export", input)
		},
	}

	// ==================== USER MANAGEMENT - NEW ROUTES ====================
	// User invite and activation
	c.routes["invite_user"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/users/invite"
		},
		BodyFunc: extractFields("invitedUserEmail", "invitedUserRoleId"),
	}
	c.routes["activate_user"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/users/activate"
		},
		BodyFunc: extractFields("email", "username", "password", "firstName", "lastName", "phone", "avatarImage"),
	}
	c.routes["list_users_summary"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/users/summary", input, "searchFirstName", "searchLastName", "searchEmail", "searchPhone", "sortBy", "sortOrder", "limit", "offset")
		},
	}
	c.routes["delete_user"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/users/%d", id)
		},
	}
	c.routes["get_client_visibility_rules"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			viewerUserId, _ := getIntFromInput(input, "viewerUserId")
			return fmt.Sprintf("/v1/users/%d/visibility", viewerUserId)
		},
	}
	c.routes["set_client_visibility_rules"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			viewerUserId, _ := getIntFromInput(input, "viewerUserId")
			return fmt.Sprintf("/v1/users/%d/visibility", viewerUserId)
		},
		BodyFunc: extractFields("rules"),
	}

	// Role management extensions
	c.routes["list_roles_summary"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/roles/summary", input, "limit", "offset")
		},
	}
	c.routes["update_role"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/roles/%d", id)
		},
		BodyFunc: extractFields("name", "description", "level", "isActive"),
	}
	c.routes["delete_role"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/roles/%d", id)
		},
	}
	c.routes["assign_permissions_to_role"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			roleId, _ := getIntFromInput(input, "roleId")
			return fmt.Sprintf("/v1/roles/%d/permissions", roleId)
		},
		BodyFunc: extractFields("permissionIds"),
	}
	c.routes["remove_permission_from_role"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			roleId, _ := getIntFromInput(input, "roleId")
			permissionId, _ := getIntFromInput(input, "permissionId")
			return fmt.Sprintf("/v1/roles/%d/permissions/%d", roleId, permissionId)
		},
	}

	// Permission CRUD
	c.routes["list_permissions"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return buildQueryPath("/v1/permissions", input, "limit", "offset")
		},
	}
	c.routes["create_permission"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/permissions"
		},
		BodyFunc: extractFields("name", "description", "resource", "action", "isActive"),
	}
	c.routes["get_permission"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/permissions/%d", id)
		},
	}
	c.routes["update_permission"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/permissions/%d", id)
		},
		BodyFunc: extractFields("name", "description", "resource", "action", "isActive"),
	}
	c.routes["delete_permission"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/permissions/%d", id)
		},
	}

	// ==================== TEAM MANAGEMENT - NEW ROUTES ====================
	c.routes["update_team"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/teams/%d", id)
		},
		BodyFunc: extractFields("name", "description", "isActive"),
	}
	c.routes["delete_team"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/teams/%d", id)
		},
	}
	c.routes["get_team_clients"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return buildQueryPath(fmt.Sprintf("/v1/teams/%d/clients", id), input, "limit", "offset")
		},
	}
	c.routes["assign_client_to_team"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/teams/%d/clients", id)
		},
		BodyFunc: extractFields("clientId"),
	}
	c.routes["remove_client_from_team"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			clientId, _ := getIntFromInput(input, "clientId")
			return fmt.Sprintf("/v1/teams/%d/clients/%d", id, clientId)
		},
	}

	// ==================== CLIENT TYPE MANAGEMENT - NEW ROUTES ====================
	c.routes["create_client_type"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/client-types"
		},
		BodyFunc: extractFields("name", "description", "displayOrder", "isActive"),
	}
	c.routes["get_client_type"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/client-types/%d", id)
		},
	}
	c.routes["update_client_type"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/client-types/%d", id)
		},
		BodyFunc: extractFields("name", "description", "displayOrder", "isActive"),
	}
	c.routes["update_client_type_order"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/client-types/order"
		},
		BodyFunc: extractFields("typeOrders"),
	}
	c.routes["delete_client_type"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/client-types/%d", id)
		},
	}

	// ==================== CLIENT CATEGORY & ADVANCED MANAGEMENT - NEW ROUTES ====================
	c.routes["create_client_category"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/clients/categories"
		},
		BodyFunc: extractFields("name", "description", "displayOrder", "isActive"),
	}
	c.routes["update_client_category"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/categories/%d", id)
		},
		BodyFunc: extractFields("name", "description", "displayOrder", "isActive"),
	}
	c.routes["update_client_category_order"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/clients/categories/order"
		},
		BodyFunc: extractFields("categoryOrders"),
	}
	c.routes["delete_client"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d", id)
		},
	}
	c.routes["get_client_categories"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d/categories", id)
		},
	}
	c.routes["assign_category_to_client"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d/categories", id)
		},
		BodyFunc: extractFields("categoryId"),
	}
	c.routes["remove_category_from_client"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			categoryId, _ := getIntFromInput(input, "categoryId")
			return fmt.Sprintf("/v1/clients/%d/categories/%d", id, categoryId)
		},
	}
	c.routes["get_client_teams"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d/teams", id)
		},
	}
	c.routes["assign_team_to_client"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/clients/%d/teams", id)
		},
		BodyFunc: extractFields("teamId"),
	}
	c.routes["remove_team_from_client"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			teamId, _ := getIntFromInput(input, "teamId")
			return fmt.Sprintf("/v1/clients/%d/teams/%d", id, teamId)
		},
	}
	c.routes["update_contact"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			contactId, _ := getIntFromInput(input, "contactId")
			return fmt.Sprintf("/v1/clients/%d/contacts/%d", clientId, contactId)
		},
		BodyFunc: extractFields("name", "email", "phone", "roleId"),
	}
	c.routes["remove_contact_from_client"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			contactId, _ := getIntFromInput(input, "contactId")
			return fmt.Sprintf("/v1/clients/%d/contacts/%d", clientId, contactId)
		},
	}
	c.routes["get_clients_with_due_interactions"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/clients/interactions/frequencies/due"
		},
	}

	// ==================== METADATA ORDERING - NEW ROUTES ====================
	c.routes["update_contact_role_order"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/clients/contacts/roles/order"
		},
		BodyFunc: extractFields("roleOrders"),
	}
	c.routes["update_interaction_type_order"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/interactions/types/order"
		},
		BodyFunc: extractFields("typeOrders"),
	}
	c.routes["update_account_platform_order"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/accounts/platforms/order"
		},
		BodyFunc: extractFields("platformOrders"),
	}
	c.routes["update_account_type_order"] = RouteConfig{
		Method: "PUT",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/accounts/types/order"
		},
		BodyFunc: extractFields("typeOrders"),
	}

	// ==================== INTERACTIONS & NOTIFICATIONS - NEW ROUTES ====================
	c.routes["delete_interaction"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/interactions/%d", id)
		},
	}
	c.routes["get_interactions_by_client"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			clientId, _ := getIntFromInput(input, "clientId")
			return buildQueryPath(fmt.Sprintf("/v1/clients/%d/interactions", clientId), input,
				"searchSummary", "filterInteractionType", "dateFrom", "dateTo",
				"followUpDateFrom", "followUpDateTo", "hasFollowUp", "sortBy", "sortOrder", "limit", "offset")
		},
	}
	c.routes["get_notification"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/notifications/%d", id)
		},
	}
	c.routes["delete_notification"] = RouteConfig{
		Method: "DELETE",
		PathFunc: func(input map[string]interface{}) string {
			id, _ := getIntFromInput(input, "id")
			return fmt.Sprintf("/v1/notifications/%d", id)
		},
	}
	c.routes["get_unread_notifications_count"] = RouteConfig{
		Method: "GET",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/notifications/unread/count"
		},
	}

	// ==================== AI/ORCHESTRATOR - NEW ROUTES ====================
	c.routes["orchestrator_chat"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/orchestrator/chat"
		},
		BodyFunc: extractFields("message", "conversationId", "context"),
	}
	c.routes["orchestrator_transcribe_audio"] = RouteConfig{
		Method: "POST",
		PathFunc: func(input map[string]interface{}) string {
			return "/v1/orchestrator/audio"
		},
		BodyFunc: extractFields("audioData", "mimeType", "language"),
	}
}
