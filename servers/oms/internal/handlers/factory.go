package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/Agusmazzeo/oms-mcp/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFactory creates tool handlers for OMS
type HandlerFactory struct {
	client *client.Client
}

// NewHandlerFactory creates a new handler factory
func NewHandlerFactory(client *client.Client) *HandlerFactory {
	return &HandlerFactory{client: client}
}

// CreateHandler creates a handler function for a specific tool
func (f *HandlerFactory) CreateHandler(toolName string) func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
		switch toolName {
		case "create_equity_order":
			return f.handleCreateEquityOrder(ctx, args)
		case "create_mutual_fund_order":
			return f.handleCreateMutualFundOrder(ctx, args)
		case "create_bond_order":
			return f.handleCreateBondOrder(ctx, args)
		case "get_brokers":
			return f.handleGetBrokers(ctx, args)
		case "get_advisors":
			return f.handleGetAdvisors(ctx, args)
		case "get_orders":
			return f.handleGetOrders(ctx, args)
		case "oms_login":
			return f.handleOMSLogin(ctx, args)
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

// handleCreateEquityOrder creates an equity (stock) order
func (f *HandlerFactory) handleCreateEquityOrder(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	direction, _ := args["direction"].(string)
	broker, _ := args["broker"].(string)
	customerAccount, _ := args["customer_account"].(string)
	symbol, _ := args["symbol"].(string)
	quantity := getFloatOrDefault(args, "quantity", 0)
	orderType := getStringOrDefault(args, "order_type", "market")
	limitPrice := getFloatOrDefault(args, "limit_price", 0)
	timeFrame := getStringOrDefault(args, "time_frame", "day")
	commissionType := getStringOrDefault(args, "commission_type", "cash")
	commissionValue := getFloatOrDefault(args, "commission_value", 0)
	notionalUSD := getFloatOrDefault(args, "notional_usd", 0)
	skipMinimum := getBoolOrDefault(args, "skip_minimum", false)
	notes := getStringOrDefault(args, "notes", "")

	// Determine notional for minimum fee check
	if orderType == "limit" && limitPrice > 0 && notionalUSD == 0 {
		notionalUSD = quantity * limitPrice
	}

	// If commission is in % and we don't know the notional, we can't validate the USD 100 minimum
	finalCommType := commissionType
	finalCommValue := commissionValue
	if commissionType == "percentage" && notionalUSD == 0 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "⚠️ Para validar el mínimo de USD 100, necesito saber el monto aproximado total a operar. Por favor indicá el valor estimado de la operación en USD (parámetro notional_usd)."},
			},
		}, nil, nil
	}

	// Enforce USD 100 minimum (unless skip_minimum=true)
	if !skipMinimum {
		if commissionType == "percentage" && notionalUSD > 0 {
			feeUSD := (commissionValue / 100) * notionalUSD
			if feeUSD < 100 {
				finalCommType = "cash"
				finalCommValue = 100
			}
		} else if commissionType == "cash" {
			if commissionValue < 100 {
				finalCommValue = 100
			}
		}
	}

	order := map[string]interface{}{
		"direction":        direction,
		"broker":           broker,
		"customer_account": customerAccount,
		"symbol":           symbol,
		"quantity":         quantity,
		"order_type":       orderType,
		"limit_price":      limitPrice,
		"time_frame":       timeFrame,
		"commission_type":  finalCommType,
		"commission_value": finalCommValue,
		"notes":            notes,
	}

	result, err := f.client.Post(ctx, "/api/crearEditarOrden", order)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result)
}

// handleCreateMutualFundOrder creates a mutual fund order
func (f *HandlerFactory) handleCreateMutualFundOrder(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	direction, _ := args["direction"].(string)
	broker, _ := args["broker"].(string)
	customerAccount, _ := args["customer_account"].(string)
	cusip, _ := args["cusip"].(string)
	quantityType, _ := args["quantity_type"].(string)
	quantity := getFloatOrDefault(args, "quantity", 0)
	lastPrice := getFloatOrDefault(args, "last_price", 0)
	fullLiquidation := getBoolOrDefault(args, "full_liquidation", false)
	feePct := getFloatOrDefault(args, "fee_pct", 0)
	commissionType := getStringOrDefault(args, "commission_type", "percentage")
	feeInclusion := getStringOrDefault(args, "fee_inclusion", "added")
	amountUSDForFee := getFloatOrDefault(args, "amount_usd_for_fee", 0)
	skipMinimum := getBoolOrDefault(args, "skip_minimum", false)
	notes := getStringOrDefault(args, "notes", "")
	dividendType := getStringOrDefault(args, "dividend_type", "cash")
	capitalType := getStringOrDefault(args, "capital_type", "cash")

	// Parse numeric params
	qty := quantity
	lastPx := lastPrice

	finalQuantity := qty
	finalQuantityType := quantityType

	// For sells, convert to shares if needed
	if direction == "sell" {
		finalQuantityType = "shares"
		if quantityType == "dollars" {
			if lastPx == 0 {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{
						&mcp.TextContent{Text: "❌ For selling by USD amount, last_price (latest NAV) is required to convert to shares."},
					},
				}, nil, nil
			}
			finalQuantity = ceilTwoDecimals(qty / lastPx)
		}
	}

	// Determine notional USD for fee calculation and minimum enforcement
	var notionalUSD float64
	if quantityType == "dollars" {
		notionalUSD = qty
	} else if amountUSDForFee > 0 {
		notionalUSD = amountUSDForFee
	}

	// Resolve commission value
	var finalCommValue float64

	if commissionType == "percentage" {
		if notionalUSD == 0 {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "❌ amount_usd_for_fee es requerido cuando quantity_type=shares y commission_type=percentage, para poder validar el mínimo de USD 100."},
				},
			}, nil, nil
		}
		feeUSD := (feePct / 100) * notionalUSD
		if direction == "sell" {
			// MF SELLS: ALWAYS store as USD (never as %)
			if !skipMinimum && feeUSD < 100 {
				finalCommValue = 100
			} else {
				finalCommValue = feeUSD
			}
		} else {
			// MF BUYS: store raw % as-is if fee >= 100, otherwise USD 100 minimum
			if !skipMinimum && feeUSD < 100 {
				finalCommValue = 100
			} else {
				finalCommValue = feePct
			}
		}
	} else {
		// cash: store USD amount with minimum
		if !skipMinimum && feePct < 100 {
			finalCommValue = 100
		} else {
			finalCommValue = feePct
		}
	}

	order := map[string]interface{}{
		"direction":         direction,
		"broker":            broker,
		"customer_account":  customerAccount,
		"cusip":             cusip,
		"quantity_type":     finalQuantityType,
		"quantity":          finalQuantity,
		"full_liquidation":  fullLiquidation,
		"fee_inclusion":     feeInclusion,
		"commission_value":  finalCommValue,
		"notes":             notes,
		"dividend_type":     dividendType,
		"capital_type":      capitalType,
	}

	result, err := f.client.Post(ctx, "/api/crearEditarOrden", order)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result)
}

// handleCreateBondOrder creates a bond order
func (f *HandlerFactory) handleCreateBondOrder(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	direction, _ := args["direction"].(string)
	broker, _ := args["broker"].(string)
	customerAccount, _ := args["customer_account"].(string)
	isin, _ := args["isin"].(string)
	quantity := getFloatOrDefault(args, "quantity", 0)
	price := getFloatOrDefault(args, "price", 0)
	markupPct := getFloatOrDefault(args, "markup_pct", 0)
	skipMinimum := getBoolOrDefault(args, "skip_minimum", false)
	notes := getStringOrDefault(args, "notes", "")

	// Notional = face value × (price / 100) — bonds quoted as % of par
	notionalUSD := quantity * (price / 100)
	feeUSD := (markupPct / 100) * notionalUSD

	// Enforce USD 100 minimum:
	// If markup% yields < USD 100, zero out markup and use flat commission = 100
	finalMarkupPct := markupPct
	flatCommission := 0.0
	minimumApplied := !skipMinimum && feeUSD < 100
	if minimumApplied {
		finalMarkupPct = 0
		flatCommission = 100
	}

	order := map[string]interface{}{
		"direction":        direction,
		"broker":           broker,
		"customer_account": customerAccount,
		"isin":             isin,
		"quantity":         quantity,
		"price":            price,
		"markup_pct":       finalMarkupPct,
		"flat_commission":  flatCommission,
		"notes":            notes,
	}

	result, err := f.client.Post(ctx, "/api/crearEditarOrden", order)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result)
}

// handleGetBrokers gets list of available brokers
func (f *HandlerFactory) handleGetBrokers(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	result, err := f.client.Get(ctx, "/api/getBrokers")
	if err != nil {
		return errorResult(err)
	}

	return successResult(result)
}

// handleGetAdvisors gets list of advisors
func (f *HandlerFactory) handleGetAdvisors(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	result, err := f.client.Get(ctx, "/api/getAsesores")
	if err != nil {
		return errorResult(err)
	}

	return successResult(result)
}

// handleGetOrders gets list of orders
func (f *HandlerFactory) handleGetOrders(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	advisorID := int(getFloatOrDefault(args, "advisor_id", 136))

	path := fmt.Sprintf("/api/getAdvisorOrders?advisorId=%d", advisorID)

	result, err := f.client.Get(ctx, path)
	if err != nil {
		return errorResult(err)
	}

	return successResult(result)
}

// handleOMSLogin checks OMS login status
func (f *HandlerFactory) handleOMSLogin(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
	if err := f.client.Login(ctx); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("❌ Not authenticated: %v", err)},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "✅ Authenticated with OMS"},
		},
	}, nil, nil
}

// Helper functions
func getFloatOrDefault(args map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := args[key]; ok {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
		if intVal, ok := val.(int); ok {
			return float64(intVal)
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

func getBoolOrDefault(args map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

func ceilTwoDecimals(n float64) float64 {
	return math.Ceil(n*100) / 100
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
