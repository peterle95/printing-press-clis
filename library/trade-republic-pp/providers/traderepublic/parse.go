package traderepublic

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/transactions"
)

const (
	maxPortfolioBytes       int64 = 32 << 20
	maxTransactionBytes     int64 = 128 << 20
	maxTransactionLineBytes       = 1 << 20
	maxPortfolioRows              = 100_000
	maxTransactionRows            = 1_000_000
)

func parsePortfolioCSV(path, currency string, asOf time.Time) ([]instruments.Instrument, []portfolio.Position, error) {
	file, err := openBoundedRegularFile(path, maxPortfolioBytes)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(io.LimitReader(file, maxPortfolioBytes+1))
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	indexes, err := portfolioHeaderIndexes(header)
	if err != nil {
		return nil, nil, err
	}
	reader.FieldsPerRecord = len(header)

	seen := make(map[string]struct{})
	var normalizedInstruments []instruments.Instrument
	var positions []portfolio.Position
	for rowNumber := 2; ; rowNumber++ {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, nil, fmt.Errorf("row %d: %w", rowNumber, readErr)
		}
		if rowNumber-1 > maxPortfolioRows {
			return nil, nil, fmt.Errorf("portfolio exceeds %d rows", maxPortfolioRows)
		}
		if recordBlank(row) {
			continue
		}
		isin := instruments.NormalizeISIN(row[indexes["isin"]])
		if err := instruments.ValidateISIN(isin); err != nil {
			return nil, nil, fmt.Errorf("row %d: %w", rowNumber, err)
		}
		if _, duplicate := seen[isin]; duplicate {
			return nil, nil, fmt.Errorf("row %d: duplicate ISIN %s", rowNumber, isin)
		}
		seen[isin] = struct{}{}
		name := strings.TrimSpace(row[indexes["name"]])
		if name == "" {
			name = isin
		}
		quantity, err := parseCSVDecimal(row[indexes["quantity"]], "quantity", rowNumber)
		if err != nil {
			return nil, nil, err
		}
		price, err := parseCSVDecimal(row[indexes["price"]], "price", rowNumber)
		if err != nil {
			return nil, nil, err
		}
		averageCost, err := parseCSVDecimal(row[indexes["avgcost"]], "avgCost", rowNumber)
		if err != nil {
			return nil, nil, err
		}
		marketValue, err := parseCSVDecimal(row[indexes["netvalue"]], "netValue", rowNumber)
		if err != nil {
			return nil, nil, err
		}
		normalizedInstruments = append(normalizedInstruments, instruments.Instrument{
			ISIN:            isin,
			Name:            name,
			BaseCurrency:    currency,
			TradingCurrency: currency,
			UpdatedAt:       asOf,
		})
		positions = append(positions, portfolio.Position{
			ISIN:        isin,
			Name:        name,
			Quantity:    quantity,
			AverageCost: averageCost,
			Price:       price,
			MarketValue: marketValue,
			Currency:    currency,
			AsOf:        asOf,
			Source:      "pytr:portfolio_csv",
		})
	}
	sort.Slice(normalizedInstruments, func(i, j int) bool { return normalizedInstruments[i].ISIN < normalizedInstruments[j].ISIN })
	sort.Slice(positions, func(i, j int) bool { return positions[i].ISIN < positions[j].ISIN })
	return normalizedInstruments, positions, nil
}

func portfolioHeaderIndexes(header []string) (map[string]int, error) {
	indexes := make(map[string]int, len(header))
	for index, value := range header {
		value = strings.TrimPrefix(value, "\ufeff")
		key := normalizeFieldName(value)
		if _, duplicate := indexes[key]; duplicate {
			return nil, fmt.Errorf("duplicate portfolio column %q", value)
		}
		indexes[key] = index
	}
	for _, required := range []string{"name", "isin", "quantity", "price", "avgcost", "netvalue"} {
		if _, ok := indexes[required]; !ok {
			return nil, fmt.Errorf("missing portfolio column %q", required)
		}
	}
	return indexes, nil
}

func parseCSVDecimal(value, field string, row int) (money.Decimal, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, ",") {
		return 0, fmt.Errorf("row %d field %s is localized; expected a nonlocalized decimal", row, field)
	}
	parsed, err := money.Parse(value)
	if err != nil {
		return 0, fmt.Errorf("row %d field %s: %w", row, field, err)
	}
	return parsed, nil
}

func parseTransactionJSONL(path, currency string, location *time.Location, since *time.Time) ([]transactions.Transaction, error) {
	file, err := openBoundedRegularFile(path, maxTransactionBytes)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(io.LimitReader(file, maxTransactionBytes+1))
	scanner.Buffer(make([]byte, 64<<10), maxTransactionLineBytes)
	seen := make(map[string]struct{})
	result := make([]transactions.Transaction, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber > maxTransactionRows {
			return nil, fmt.Errorf("transaction export exceeds %d rows", maxTransactionRows)
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON object: %w", lineNumber, err)
		}
		row := make(map[string]json.RawMessage, len(raw))
		for key, value := range raw {
			row[normalizeFieldName(key)] = value
		}
		transaction, err := transactionFromJSONRow(row, line, currency, location)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if since != nil && transaction.OccurredAt.Before(*since) {
			continue
		}
		transactions.EnsureIdentity(&transaction)
		if _, duplicate := seen[transaction.Fingerprint]; duplicate {
			continue
		}
		seen[transaction.Fingerprint] = struct{}{}
		result = append(result, transaction)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan JSONL: %w", err)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].OccurredAt.Equal(result[j].OccurredAt) {
			return result[i].Fingerprint < result[j].Fingerprint
		}
		return result[i].OccurredAt.Before(result[j].OccurredAt)
	})
	return result, nil
}

func transactionFromJSONRow(row map[string]json.RawMessage, rawLine, currency string, location *time.Location) (transactions.Transaction, error) {
	timestamp, err := scalarString(row["date"])
	if err != nil || timestamp == "" {
		if err == nil {
			err = fmt.Errorf("date is required")
		}
		return transactions.Transaction{}, err
	}
	occurredAt, assumption, err := parseTransactionTime(timestamp, location)
	if err != nil {
		return transactions.Transaction{}, err
	}
	typeValue, err := scalarString(row["type"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("type: %w", err)
	}
	amount, err := decimalFromJSON(row["value"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("value: %w", err)
	}
	quantity, err := decimalFromJSON(row["shares"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("shares: %w", err)
	}
	fees, err := decimalFromJSON(row["fees"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("fees: %w", err)
	}
	taxes, err := decimalFromJSON(row["taxes"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("taxes: %w", err)
	}
	note, err := scalarString(row["note"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("note: %w", err)
	}
	isin, err := normalizedOptionalISIN(row["isin"])
	if err != nil {
		return transactions.Transaction{}, err
	}
	secondaryISIN, err := normalizedOptionalISIN(row["isin2"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("secondary %w", err)
	}
	secondaryQuantity, err := decimalFromJSON(row["shares2"])
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("shares2: %w", err)
	}
	if isin == "" && secondaryISIN != "" {
		isin = secondaryISIN
		quantity = secondaryQuantity
	} else if secondaryISIN != "" && secondaryISIN != isin {
		suffix := "secondary ISIN " + secondaryISIN
		if !secondaryQuantity.IsZero() {
			suffix += " shares " + secondaryQuantity.String()
		}
		if strings.TrimSpace(note) == "" {
			note = suffix
		} else {
			note = strings.TrimSpace(note) + "; " + suffix
		}
	}
	return transactions.Transaction{
		OccurredAt:         occurredAt,
		OriginalTimestamp:  timestamp,
		TimezoneAssumption: assumption,
		Kind:               normalizePytrKind(typeValue),
		ISIN:               isin,
		Quantity:           quantity,
		Amount:             amount,
		Fees:               fees.Abs(),
		Taxes:              taxes.Abs(),
		Currency:           currency,
		Note:               strings.TrimSpace(note),
		Source:             "pytr:transaction_jsonl",
		RawJSON:            rawLine,
	}, nil
}

func normalizePytrKind(value string) transactions.Kind {
	key := strings.ToUpper(strings.TrimSpace(value))
	key = strings.NewReplacer("-", "_", " ", "_").Replace(key)
	switch key {
	case "BUY", "PURCHASE":
		return transactions.Buy
	case "SELL", "SALE":
		return transactions.Sell
	case "DIVIDEND", "DISTRIBUTION":
		return transactions.Dividend
	case "DEPOSIT":
		return transactions.Deposit
	case "REMOVAL", "WITHDRAWAL":
		return transactions.Withdrawal
	case "INTEREST", "INTEREST_CHARGE":
		return transactions.Interest
	case "FEE", "FEES", "FEES_REFUND":
		return transactions.Fee
	case "TAX", "TAXES", "TAX_REFUND":
		return transactions.Tax
	case "TRANSFER_IN":
		return transactions.TransferIn
	case "TRANSFER_OUT":
		return transactions.TransferOut
	default:
		return transactions.Unknown
	}
}

func parseTransactionTime(value string, location *time.Location) (time.Time, string, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z0700"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, "", nil
		}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed, location.String(), nil
		}
	}
	return time.Time{}, "", fmt.Errorf("invalid transaction date %q", value)
}

func normalizedOptionalISIN(raw json.RawMessage) (string, error) {
	value, err := scalarString(raw)
	if err != nil {
		return "", fmt.Errorf("ISIN: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	value = instruments.NormalizeISIN(value)
	if err := instruments.ValidateISIN(value); err != nil {
		return "", err
	}
	return value, nil
}

func decimalFromJSON(raw json.RawMessage) (money.Decimal, error) {
	value, err := scalarString(raw)
	if err != nil || value == "" {
		return 0, err
	}
	if strings.Contains(value, ",") {
		return 0, fmt.Errorf("localized decimal %q is not allowed", value)
	}
	value, err = expandDecimalExponent(value)
	if err != nil {
		return 0, err
	}
	return money.Parse(value)
}

func scalarString(raw json.RawMessage) (string, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return strings.TrimSpace(value), nil
	}
	if raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9') {
		return string(raw), nil
	}
	return "", fmt.Errorf("expected a string, number, or null")
}

// expandDecimalExponent converts a JSON decimal exponent lexeme without
// passing through float64, preserving the exact decimal digits supplied by
// pytr before money.Parse applies the application's fixed eight-place scale.
func expandDecimalExponent(value string) (string, error) {
	value = strings.TrimSpace(value)
	index := strings.IndexAny(value, "eE")
	if index < 0 {
		return value, nil
	}
	mantissa, exponentText := value[:index], value[index+1:]
	exponent, err := strconv.Atoi(exponentText)
	if err != nil || exponent < -1000 || exponent > 1000 {
		return "", fmt.Errorf("invalid decimal exponent %q", value)
	}
	sign := ""
	if strings.HasPrefix(mantissa, "+") || strings.HasPrefix(mantissa, "-") {
		sign, mantissa = mantissa[:1], mantissa[1:]
	}
	if mantissa == "" || strings.Count(mantissa, ".") > 1 {
		return "", fmt.Errorf("invalid decimal %q", value)
	}
	decimalPoint := strings.IndexByte(mantissa, '.')
	if decimalPoint < 0 {
		decimalPoint = len(mantissa)
	}
	digits := strings.ReplaceAll(mantissa, ".", "")
	if digits == "" {
		return "", fmt.Errorf("invalid decimal %q", value)
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return "", fmt.Errorf("invalid decimal %q", value)
		}
	}
	newPoint := decimalPoint + exponent
	switch {
	case newPoint <= 0:
		return sign + "0." + strings.Repeat("0", -newPoint) + digits, nil
	case newPoint >= len(digits):
		return sign + digits + strings.Repeat("0", newPoint-len(digits)), nil
	default:
		return sign + digits[:newPoint] + "." + digits[newPoint:], nil
	}
}

func normalizeFieldName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
}

func recordBlank(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func openBoundedRegularFile(path string, maximum int64) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular file %s", filepathBase(path))
	}
	if info.Size() > maximum {
		return nil, fmt.Errorf("file %s exceeds %d bytes", filepathBase(path), maximum)
	}
	return os.Open(path)
}

func filepathBase(path string) string {
	if index := strings.LastIndexAny(path, `/\\`); index >= 0 {
		return path[index+1:]
	}
	return path
}
