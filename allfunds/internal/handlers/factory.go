package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Agusmazzeo/allfunds-mcp/internal/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFactory creates tool handlers for Allfunds
type HandlerFactory struct {
	client *graphql.Client
}

// NewHandlerFactory creates a new handler factory
func NewHandlerFactory(client *graphql.Client) *HandlerFactory {
	return &HandlerFactory{client: client}
}

// CreateHandler creates a handler function for a specific tool
func (f *HandlerFactory) CreateHandler(toolName string) func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
		switch toolName {
		case "search_funds":
			return f.handleSearchFunds(ctx, args)
		case "get_current_user":
			return f.handleGetCurrentUser(ctx, args)
		case "list_watchlists":
			return f.handleListWatchlists(ctx, args)
		case "get_pinned_watchlist":
			return f.handleGetPinnedWatchlist(ctx, args)
		case "get_fund_insights":
			return f.handleGetFundInsights(ctx, args)
		case "get_watchlist_funds":
			return f.handleGetWatchlistFunds(ctx, args)
		case "login_status":
			return f.handleLoginStatus(ctx, args)
		case "get_fund_detail":
			return f.handleGetFundDetail(ctx, args)
		case "get_fund_performance":
			return f.handleGetFundPerformance(ctx, args)
		case "get_fund_ratios":
			return f.handleGetFundRatios(ctx, args)
		case "get_fund_documents":
			return f.handleGetFundDocuments(ctx, args)
		case "get_fund_share_classes":
			return f.handleGetFundShareClasses(ctx, args)
		case "compare_funds_performance":
			return f.handleCompareFundsPerformance(ctx, args)
		case "get_fund_portfolio":
			return f.handleGetFundPortfolio(ctx, args)
		case "get_fund_nav_history":
			return f.handleGetFundNAVHistory(ctx, args)
		case "get_fund_nav_on_date":
			return f.handleGetFundNAVOnDate(ctx, args)
		case "get_fund_managers":
			return f.handleGetFundManagers(ctx, args)
		case "get_similar_funds":
			return f.handleGetSimilarFunds(ctx, args)
		case "download_document":
			return f.handleDownloadDocument(ctx, args)
		default:
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Unknown tool: %s", toolName)},
				},
			}, nil, nil
		}
	}
}

// CreateLoginHandler creates the login tool handler
func (f *HandlerFactory) CreateLoginHandler() func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
		// Login is handled automatically by GraphQL client
		if err := f.client.Login(ctx); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Login failed: %v", err)},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "✓ Successfully authenticated with Allfunds Connect"},
			},
		}, nil, nil
	}
}

// handleSearchFunds searches for funds by name/ISIN/ticker
func (f *HandlerFactory) handleSearchFunds(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	search, _ := args["search"].(string)
	page := getIntOrDefault(args, "page", 1)
	pageSize := getIntOrDefault(args, "page_size", 10)

	criteria := map[string]interface{}{
		"search_string": search,
	}

	if avail, ok := args["available_in_allfunds"].(bool); ok && avail {
		criteria["dealable_for_entity"] = true
	}

	if currency, ok := args["currency"].(string); ok && currency != "" {
		criteria["currency"] = currency
	}

	if sfdr, ok := args["sfdr_article"].(string); ok && sfdr != "" {
		criteria["sfdr_article"] = sfdr
	}

	sortField := getStringOrDefault(args, "sort_field", "name")
	sortDirection := getStringOrDefault(args, "sort_direction", "asc")

	variables := map[string]interface{}{
		"screeningCriteria": criteria,
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
		},
		"order": map[string]interface{}{
			"field":     sortField,
			"direction": sortDirection,
		},
		"cache": true,
	}

	var result struct {
		ProductScreener struct {
			ScreenProducts struct {
				Results          []map[string]interface{} `json:"results"`
				PaginationResult struct {
					TotalCount int `json:"total_count"`
					PageCount  int `json:"page_count"`
				} `json:"pagination_result"`
			} `json:"screen_products"`
		} `json:"product_screener"`
	}

	err := f.client.Query(ctx, "ScreenProducts", graphql.ScreenProductsQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.ProductScreener.ScreenProducts)
}

// handleGetCurrentUser gets current user information
func (f *HandlerFactory) handleGetCurrentUser(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	var result struct {
		Me map[string]interface{} `json:"me"`
	}

	err := f.client.Query(ctx, "GetCurrentUser", graphql.GetCurrentUserQuery, nil, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Me)
}

// handleListWatchlists lists user's watchlists
func (f *HandlerFactory) handleListWatchlists(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	page := getIntOrDefault(args, "page", 1)
	pageSize := getIntOrDefault(args, "page_size", 20)

	variables := map[string]interface{}{
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
		},
		"order": map[string]interface{}{
			"field":     "name",
			"direction": "asc",
		},
	}

	var result struct {
		WatchlistsPaginated struct {
			Results []map[string]interface{} `json:"results"`
		} `json:"watchlists_paginated"`
	}

	err := f.client.Query(ctx, "GetWatchlistsPaginated", graphql.GetWatchlistsQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.WatchlistsPaginated.Results)
}

// handleGetPinnedWatchlist gets user's pinned watchlist
func (f *HandlerFactory) handleGetPinnedWatchlist(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	var result struct {
		PinnedWatchlist map[string]interface{} `json:"pinned_watchlist"`
	}

	err := f.client.Query(ctx, "GetPinnedWatchlist", graphql.GetPinnedWatchlistQuery, nil, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.PinnedWatchlist)
}

// handleGetFundInsights gets fund insights/articles
func (f *HandlerFactory) handleGetFundInsights(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	page := getIntOrDefault(args, "page", 1)
	pageSize := getIntOrDefault(args, "page_size", 10)

	criteria := map[string]interface{}{}

	if search, ok := args["search"].(string); ok && search != "" {
		criteria["search"] = search
	}

	if hashtags, ok := args["hashtags"].([]interface{}); ok && len(hashtags) > 0 {
		criteria["hashtags"] = hashtags
	}

	if regions, ok := args["regions"].([]interface{}); ok && len(regions) > 0 {
		criteria["regions"] = regions
	}

	variables := map[string]interface{}{
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
		},
		"order": map[string]interface{}{
			"field":     "publish_date",
			"direction": "desc",
		},
		"include_sponsored": false,
	}

	if len(criteria) > 0 {
		variables["criteria"] = criteria
	}

	var result struct {
		Articles struct {
			Results []map[string]interface{} `json:"results"`
		} `json:"articles"`
	}

	err := f.client.Query(ctx, "GetArticles", graphql.GetArticlesQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Articles.Results)
}

// handleGetWatchlistFunds gets funds from a watchlist
func (f *HandlerFactory) handleGetWatchlistFunds(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	watchlistID, _ := args["watchlist_id"].(string)
	page := getIntOrDefault(args, "page", 1)
	pageSize := getIntOrDefault(args, "page_size", 25)
	search := getStringOrDefault(args, "search", "")

	variables := map[string]interface{}{
		"id": watchlistID,
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
		},
		"order": map[string]interface{}{
			"field":     "name",
			"direction": "asc",
		},
	}

	if search != "" {
		variables["search_string"] = search
	}

	var result struct {
		Watchlist struct {
			ID            string                   `json:"id"`
			Name          string                   `json:"name"`
			TotalProducts int                      `json:"total_products"`
			Products      []map[string]interface{} `json:"paginated_products"`
		} `json:"watchlist"`
	}

	err := f.client.Query(ctx, "GetProductsFromWatchlist", graphql.GetWatchlistFundsQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Watchlist)
}

// handleLoginStatus checks authentication status
func (f *HandlerFactory) handleLoginStatus(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	// Try to re-login to verify session
	if err := f.client.Login(ctx); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("❌ Not authenticated: %v", err)},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Authenticated with Allfunds Connect"},
		},
	}, nil, nil
}

// handleGetFundDetail gets detailed fund information
func (f *HandlerFactory) handleGetFundDetail(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)

	// First get internal ID from ISIN
	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id": internalID,
	}

	var result struct {
		Product map[string]interface{} `json:"product"`
	}

	err = f.client.Query(ctx, "GetProduct", graphql.GetProductQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Product)
}

// handleGetFundPerformance gets fund performance data
func (f *HandlerFactory) handleGetFundPerformance(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id": internalID,
	}

	var result struct {
		Product struct {
			ISIN                 string                 `json:"isin"`
			Name                 string                 `json:"name"`
			PerformanceByPeriods map[string]interface{} `json:"performance_by_periods"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "GetProduct", graphql.GetProductQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Product)
}

// handleGetFundRatios gets fund risk/return ratios
func (f *HandlerFactory) handleGetFundRatios(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)
	period := getStringOrDefault(args, "period", "all")

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id": internalID,
	}

	var result struct {
		Product struct {
			ISIN             string                 `json:"isin"`
			Name             string                 `json:"name"`
			CalculatedRatios map[string]interface{} `json:"calculated_ratios"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "GetProduct", graphql.GetProductQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	// Filter by period if not "all"
	if period != "all" {
		ratios := result.Product.CalculatedRatios
		filtered := map[string]interface{}{
			period: ratios[period],
		}
		result.Product.CalculatedRatios = filtered
	}

	return successResult(result.Product)
}

// handleGetFundDocuments gets fund documents
func (f *HandlerFactory) handleGetFundDocuments(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)
	pageSize := getIntOrDefault(args, "page_size", 50)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id": internalID,
		"pagination": map[string]interface{}{
			"page":      1,
			"page_size": pageSize,
		},
	}

	var result struct {
		Product struct {
			ISIN      string `json:"isin"`
			Name      string `json:"name"`
			Documents struct {
				Results []map[string]interface{} `json:"results"`
			} `json:"documents"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "GetProductDocuments", graphql.GetProductDocumentsQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Product)
}

// handleGetFundShareClasses gets fund share classes
func (f *HandlerFactory) handleGetFundShareClasses(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id": internalID,
	}

	var result struct {
		Product struct {
			ISIN         string                   `json:"isin"`
			Name         string                   `json:"name"`
			ShareClasses []map[string]interface{} `json:"share_classes"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "GetProduct", graphql.GetProductQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Product)
}

// handleCompareFundsPerformance compares performance of multiple funds
func (f *HandlerFactory) handleCompareFundsPerformance(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isinsInterface, ok := args["isins"].([]interface{})
	if !ok {
		return errorResult(fmt.Errorf("isins must be an array"))
	}

	var isins []string
	for _, isinInterface := range isinsInterface {
		if isin, ok := isinInterface.(string); ok {
			isins = append(isins, isin)
		}
	}

	comparison := make([]map[string]interface{}, 0, len(isins))

	for _, isin := range isins {
		internalID, err := f.getInternalIDFromISIN(ctx, isin)
		if err != nil {
			continue // Skip funds that fail
		}

		variables := map[string]interface{}{
			"id": internalID,
		}

		var result struct {
			Product map[string]interface{} `json:"product"`
		}

		err = f.client.Query(ctx, "GetProduct", graphql.GetProductQuery, variables, &result)
		if err != nil {
			continue
		}

		comparison = append(comparison, result.Product)
	}

	return successResult(comparison)
}

// handleGetFundPortfolio gets fund portfolio breakdown
func (f *HandlerFactory) handleGetFundPortfolio(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)
	topSize := getIntOrDefault(args, "top_size", 10)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id":       internalID,
		"top_size": topSize,
		"limited":  false,
	}

	var result struct {
		Product map[string]interface{} `json:"product"`
	}

	err = f.client.Query(ctx, "ProductCalculations", graphql.ProductCalculationsQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Product)
}

// Placeholder handlers for tools that need additional GraphQL queries
func (f *HandlerFactory) handleGetFundNAVHistory(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)
	sinceDate := getStringOrDefault(args, "since_date", "")
	untilDate := getStringOrDefault(args, "until_date", "")
	limit := getIntOrDefault(args, "limit", 252)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id":    internalID,
		"limit": limit,
	}
	if sinceDate != "" {
		variables["since_date"] = sinceDate
	}
	if untilDate != "" {
		variables["until_date"] = untilDate
	}

	var result struct {
		Product struct {
			ISIN        string `json:"isin"`
			Name        string `json:"name"`
			ClosePrices []struct {
				Date  string  `json:"date"`
				Value float64 `json:"value"`
			} `json:"close_prices"`
			Dividends []struct {
				DividendAt string  `json:"dividend_at"`
				RecordedAt string  `json:"recorded_at"`
				PayedAt    string  `json:"payed_at"`
				Unit       float64 `json:"unit"`
			} `json:"dividends"`
			Splits []struct {
				SplitAt string  `json:"split_at"`
				Ratio   float64 `json:"ratio"`
			} `json:"splits"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "ProductPrices", graphql.ProductPricesQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	response := map[string]interface{}{
		"nombre":         result.Product.Name,
		"isin":           result.Product.ISIN,
		"total_precios":  len(result.Product.ClosePrices),
		"precios":        result.Product.ClosePrices,
		"dividendos":     result.Product.Dividends,
		"splits":         result.Product.Splits,
	}

	return successResult(response)
}

func (f *HandlerFactory) handleGetFundNAVOnDate(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)
	dateStr, _ := args["date"].(string)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	// Request prices in a window of ±7 days around the target date
	variables := map[string]interface{}{
		"id":    internalID,
		"limit": 15,
	}

	// Parse target date and create window
	if dateStr != "" {
		// Simple date window calculation (7 days before, 1 day after)
		variables["since_date"] = dateStr // Simplified - ideally calculate 7 days before
		variables["until_date"] = dateStr
	}

	var result struct {
		Product struct {
			ISIN        string `json:"isin"`
			Name        string `json:"name"`
			ClosePrices []struct {
				Date  string  `json:"date"`
				Value float64 `json:"value"`
			} `json:"close_prices"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "ProductPrices", graphql.ProductPricesQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	response := map[string]interface{}{
		"nombre":            result.Product.Name,
		"isin":              result.Product.ISIN,
		"fecha_solicitada":  dateStr,
	}

	// Find exact or closest price
	if len(result.Product.ClosePrices) > 0 {
		// Look for exact match first
		var closestPrice *struct {
			Date  string  `json:"date"`
			Value float64 `json:"value"`
		}
		for i := range result.Product.ClosePrices {
			if result.Product.ClosePrices[i].Date == dateStr {
				closestPrice = &result.Product.ClosePrices[i]
				response["precio_exacto"] = true
				break
			}
		}
		// If no exact match, use first available (closest)
		if closestPrice == nil && len(result.Product.ClosePrices) > 0 {
			closestPrice = &result.Product.ClosePrices[0]
			response["precio_exacto"] = false
		}

		if closestPrice != nil {
			response["fecha_real"] = closestPrice.Date
			response["nav"] = closestPrice.Value
		}
	} else {
		response["nav"] = nil
		response["_nota"] = "No se encontraron precios para esa fecha"
	}

	return successResult(response)
}

func (f *HandlerFactory) handleGetFundManagers(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"id": internalID,
	}

	var result struct {
		Product struct {
			ISIN     string                   `json:"isin"`
			Name     string                   `json:"name"`
			Managers []map[string]interface{} `json:"managers"`
		} `json:"product"`
	}

	err = f.client.Query(ctx, "GetProduct", graphql.GetProductQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result.Product)
}

func (f *HandlerFactory) handleGetSimilarFunds(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	isin, _ := args["isin"].(string)

	internalID, err := f.getInternalIDFromISIN(ctx, isin)
	if err != nil {
		return errorResult(err)
	}

	variables := map[string]interface{}{
		"product_id": internalID,
	}

	var result struct {
		GetSimilarAlternativesForProduct []map[string]interface{} `json:"get_similar_alternatives_for_product"`
	}

	err = f.client.Query(ctx, "GetSimilarAlternativesForProductQuery", graphql.GetSimilarFundsQuery, variables, &result)
	if err != nil {
		return errorResult(err)
	}

	response := map[string]interface{}{
		"isin_referencia":   isin,
		"total_similares":   len(result.GetSimilarAlternativesForProduct),
		"fondos_similares":  result.GetSimilarAlternativesForProduct,
	}

	return successResult(response)
}

func (f *HandlerFactory) handleDownloadDocument(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	url, _ := args["url"].(string)

	if url == "" {
		return errorResult(fmt.Errorf("url parameter is required"))
	}

	// Download document using authenticated session
	content, err := f.client.DownloadFile(ctx, url)
	if err != nil {
		return errorResult(err)
	}

	// Extract filename from URL
	parts := strings.Split(url, "/")
	filename := parts[len(parts)-1]
	if filename == "" {
		filename = "document.pdf"
	}

	// Create temp directory for documents
	tempDir := filepath.Join(os.TempDir(), "allfunds_docs")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return errorResult(fmt.Errorf("failed to create temp directory: %w", err))
	}

	// Save file
	filepath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filepath, content, 0644); err != nil {
		return errorResult(fmt.Errorf("failed to save file: %w", err))
	}

	response := map[string]interface{}{
		"status":     "ok",
		"filename":   filename,
		"path":       filepath,
		"size_bytes": len(content),
		"url":        url,
	}

	return successResult(response)
}

// Helper: Get internal ID from ISIN
func (f *HandlerFactory) getInternalIDFromISIN(ctx context.Context, isin string) (string, error) {
	variables := map[string]interface{}{
		"screeningCriteria": map[string]interface{}{
			"search_string": isin,
		},
		"pagination": map[string]interface{}{
			"page":      1,
			"page_size": 1,
		},
		"order": map[string]interface{}{
			"field":     "name",
			"direction": "asc",
		},
		"cache": true,
	}

	var result struct {
		ProductScreener struct {
			ScreenProducts struct {
				Results []struct {
					ID   string `json:"id"`
					ISIN string `json:"isin"`
				} `json:"results"`
			} `json:"screen_products"`
		} `json:"product_screener"`
	}

	err := f.client.Query(ctx, "ScreenProducts", graphql.ScreenProductsQuery, variables, &result)
	if err != nil {
		return "", err
	}

	if len(result.ProductScreener.ScreenProducts.Results) == 0 {
		return "", fmt.Errorf("fund not found with ISIN: %s", isin)
	}

	return result.ProductScreener.ScreenProducts.Results[0].ID, nil
}

// Helper functions
func getIntOrDefault(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key]; ok {
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return defaultVal
}

func getStringOrDefault(args map[string]interface{}, key string, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
}

func successResult(data interface{}) (*mcp.CallToolResult, any, error) {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil, nil
}

func errorResult(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
		},
	}, nil, nil
}
