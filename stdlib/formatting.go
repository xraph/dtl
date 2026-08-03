package stdlib

import (
	"fmt"
	"math"
	"strings"

	"github.com/xraph/dtl/executor"
)

func registerFormatting(m map[string]*executor.BuiltinFunc) {
	register(m, "format_number", 1, 3, fnFormatNumber,
		"format_number(n, decimals?, separator?) -> string -- Formats a number with a thousands separator (default ',')")
	register(m, "format_currency", 1, 3, fnFormatCurrency,
		"format_currency(n, currency?, locale?) -> string -- Formats a number as currency (USD, EUR, GBP, ...)")
	register(m, "format_percent", 1, 2, fnFormatPercent,
		"format_percent(n, decimals?) -> string -- Formats a number as a percentage")

	// Legacy namespace spellings, aliased so they cannot drift from the bare names.
	alias(m, "system::formatting::format_number", "format_number")
	alias(m, "system::formatting::format_currency", "format_currency")
	alias(m, "system::formatting::format_percent", "format_percent")
}

func fnFormatNumber(args []any) (any, error) {
	n := executor.ToFloat(args[0])
	decimals := 2
	if len(args) > 1 {
		decimals = int(executor.ToInt(args[1]))
	}
	separator := ","
	if len(args) > 2 {
		separator = executor.ToString(args[2])
	}

	// Determine decimal separator
	decimalSep := "."
	if separator == "." {
		decimalSep = ","
	}

	// Format the number
	negative := n < 0
	n = math.Abs(n)

	// Round to decimals
	factor := math.Pow10(decimals)
	n = math.Round(n*factor) / factor

	// Split integer and decimal parts
	intPart := int64(n)
	var decPart string
	if decimals > 0 {
		frac := n - float64(intPart)
		decPart = fmt.Sprintf("%.*f", decimals, frac)
		decPart = decPart[1:] // remove leading "0"
	}

	// Format integer part with thousands separator
	intStr := fmt.Sprintf("%d", intPart)
	if separator != "" && len(intStr) > 3 {
		var sb strings.Builder
		start := len(intStr) % 3
		if start > 0 {
			sb.WriteString(intStr[:start])
		}
		for i := start; i < len(intStr); i += 3 {
			if sb.Len() > 0 {
				sb.WriteString(separator)
			}
			sb.WriteString(intStr[i : i+3])
		}
		intStr = sb.String()
	}

	result := intStr
	if decimals > 0 {
		// Replace "." with the actual decimal separator
		decPart = strings.Replace(decPart, ".", decimalSep, 1)
		result += decPart
	}

	if negative {
		result = "-" + result
	}
	return result, nil
}

func fnFormatCurrency(args []any) (any, error) {
	n := executor.ToFloat(args[0])
	currency := "USD"
	if len(args) > 1 {
		currency = strings.ToUpper(executor.ToString(args[1]))
	}

	// Get currency symbol
	symbol := currency
	switch currency {
	case "USD":
		symbol = "$"
	case "EUR":
		symbol = "€"
	case "GBP":
		symbol = "£"
	case "JPY", "CNY":
		symbol = "¥"
	case "KRW":
		symbol = "₩"
	case "INR":
		symbol = "₹"
	case "BRL":
		symbol = "R$"
	}

	// Format number with 2 decimal places
	formatted, err := fnFormatNumber([]any{n, int64(2), ","})
	if err != nil {
		return nil, err
	}

	return symbol + executor.ToString(formatted), nil
}

func fnFormatPercent(args []any) (any, error) {
	n := executor.ToFloat(args[0])
	decimals := 1
	if len(args) > 1 {
		decimals = int(executor.ToInt(args[1]))
	}

	// If value looks like a ratio (between -1 and 1 exclusive, and not 0), multiply by 100
	if n != 0 && n > -1 && n < 1 {
		n *= 100
	}

	return fmt.Sprintf("%.*f%%", decimals, n), nil
}
