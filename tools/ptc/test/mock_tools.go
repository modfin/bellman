package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/modfin/bellman/tools"
)

// GetMockBellmanTools returns ready-to-use dummy Bellman tools
func GetMockBellmanTools(enablePTC bool) []tools.Tool {
	var mockTools []tools.Tool

	// 1. Magic 8-Ball Tool
	type FutureArgs struct {
		Question string `json:"question"`
	}
	predictTool := tools.NewTool("predict_future",
		tools.WithDescription("Returns a mystical answer to a yes/no question."),
		tools.WithArgSchema(FutureArgs{}),
		tools.WithPTC(enablePTC),
		tools.WithResponseType[string](),
		tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
			var arg FutureArgs
			if err := json.Unmarshal(call.Argument, &arg); err != nil {
				return "", err
			}

			// Logic
			answers := []string{
				"It is certain.", "Reply hazy, try again.", "Don't count on it.",
				"The stars say yes.", "My sources say no.",
			}
			rand.Seed(time.Now().UnixNano())
			ans := answers[rand.Intn(len(answers))]
			return ans, nil
		}),
	)
	mockTools = append(mockTools, predictTool)

	// 2. Currency Converter Tool
	type CurrencyArgs struct {
		Amount float64 `json:"amount"`
		From   string  `json:"from"`
		To     string  `json:"to"`
	}
	convertTool := tools.NewTool("convert_currency",
		tools.WithDescription("Converts currency amounts (USD, EUR, SEK, GBP, JPY)."),
		tools.WithArgSchema(CurrencyArgs{}),
		tools.WithPTC(enablePTC),
		tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
			var arg CurrencyArgs
			if err := json.Unmarshal(call.Argument, &arg); err != nil {
				return "", err
			}

			// Logic
			rates := map[string]float64{"USD": 1.0, "EUR": 0.92, "SEK": 10.50, "GBP": 0.79, "JPY": 148.0}
			rateFrom, ok1 := rates[strings.ToUpper(arg.From)]
			rateTo, ok2 := rates[strings.ToUpper(arg.To)]

			if !ok1 || !ok2 {
				return fmt.Sprintf("Error: Unknown currency pair %s -> %s", arg.From, arg.To), nil
			}
			result := (arg.Amount / rateFrom) * rateTo
			return fmt.Sprintf("%.2f", result), nil
		}),
	)
	mockTools = append(mockTools, convertTool)

	// 3. Password Generator Tool
	type PasswordArgs struct {
		Length  int  `json:"length"`
		Special bool `json:"special"`
	}
	passTool := tools.NewTool("generate_password",
		tools.WithDescription("Generates a random string. 'special' adds symbols."),
		tools.WithArgSchema(PasswordArgs{}),
		tools.WithPTC(enablePTC),
		tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
			var arg PasswordArgs
			if err := json.Unmarshal(call.Argument, &arg); err != nil {
				return "", err
			}

			// Logic
			if arg.Length > 50 {
				return "Error: Password too long!", nil
			}
			chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			if arg.Special {
				chars += "!@#$%^&*()_+"
			}
			var result strings.Builder
			for i := 0; i < arg.Length; i++ {
				idx := (i * 7) % len(chars) // Mock deterministic random
				result.WriteByte(chars[idx])
			}
			return result.String(), nil
		}),
	)
	mockTools = append(mockTools, passTool)

	// 4. get company id
	type CompanyArgs struct {
		Name string `json:"name"`
	}
	// Define the return structure
	type CompanyData struct {
		ID          string `json:"company_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Domain      string `json:"domain"`
		Valuation   string `json:"valuation"`
	}

	companyTool := tools.NewTool("get_company",
		tools.WithDescription("Gets company object by name. Returns id, description, domain, and valuation."),
		tools.WithArgSchema(CompanyArgs{}),
		tools.WithPTC(enablePTC),
		tools.WithResponseType[CompanyData](),
		tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
			var arg CompanyArgs
			if err := json.Unmarshal(call.Argument, &arg); err != nil {
				return "", err
			}

			// Normalize input for easier matching
			query := strings.ToLower(strings.TrimSpace(arg.Name))

			var data CompanyData

			switch {
			case strings.Contains(query, "saab"):
				data = CompanyData{
					ID:          "comp_saab_001",
					Name:        "Saab AB",
					Description: "Swedish aerospace and defense company.",
					Domain:      "saab.com",
					Valuation:   "100B SEK",
				}
			case strings.Contains(query, "ericsson"):
				data = CompanyData{
					ID:          "comp_eric_002",
					Name:        "Telefonaktiebolaget LM Ericsson",
					Description: "Multinational networking and telecommunications company.",
					Domain:      "ericsson.com",
					Valuation:   "180B SEK",
				}
			case strings.Contains(query, "sas"):
				data = CompanyData{
					ID:          "comp_sas_003",
					Name:        "Scandinavian Airlines (SAS)",
					Description: "Flagship carrier of Denmark, Norway, and Sweden.",
					Domain:      "sas.se",
					Valuation:   "5B SEK",
				}
			default:
				return fmt.Sprintf(`{"error": "Company '%s' not found. Available companies: Saab, Ericsson, SAS"}`, arg.Name), nil
			}

			// Marshal result to JSON
			b, err := json.Marshal(data)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}),
	)
	mockTools = append(mockTools, companyTool)

	// 5. get stock by id
	type StockArgs struct {
		CompanyId string `json:"company_id"`
	}
	// Define the return structure for Stock
	type StockData struct {
		Symbol   string  `json:"symbol"`
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
		Change   string  `json:"day_change"`
		High     float64 `json:"day_high"`
		Low      float64 `json:"day_low"`
	}
	stockTool := tools.NewTool("get_stock",
		tools.WithDescription("Gets stock price and details by company id. Returns symbol, price, currency."),
		// FIX: Use StockArgs, not CompanyArgs
		tools.WithArgSchema(StockArgs{}),
		tools.WithPTC(enablePTC),
		tools.WithResponseType[StockData](),
		tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
			var arg StockArgs
			if err := json.Unmarshal(call.Argument, &arg); err != nil {
				return "", err
			}

			var data StockData

			// Match strictly by the IDs defined in the previous company tool
			switch arg.CompanyId {
			case "comp_saab_001":
				data = StockData{
					Symbol:   "SAAB B",
					Price:    785.50,
					Currency: "SEK",
					Change:   "+1.25%",
					High:     792.00,
					Low:      778.50,
				}
			case "comp_eric_002":
				data = StockData{
					Symbol:   "ERIC B",
					Price:    64.80,
					Currency: "SEK",
					Change:   "-0.45%",
					High:     65.20,
					Low:      64.10,
				}
			case "comp_sas_003":
				data = StockData{
					Symbol:   "SAS",
					Price:    0.04, // Realistic penny stock value for SAS
					Currency: "SEK",
					Change:   "0.00%",
					High:     0.05,
					Low:      0.03,
				}
			default:
				return fmt.Sprintf(`{"error": "Stock for company_id '%s' not found."}`, arg.CompanyId), nil
			}

			// Marshal result to JSON
			b, err := json.Marshal(data)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}),
	)
	mockTools = append(mockTools, stockTool)

	return mockTools
}
