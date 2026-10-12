package domain

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Validation struct {
	File    string `json:"file"`
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v *Validation) Error() string {
	return fmt.Sprintf("%s row %d, %s: %s", v.File, v.Row, v.Field, v.Message)
}

type Holding struct {
	ID       string   `json:"id"`
	SharedID string   `json:"sharedId"`
	Name     string   `json:"name"`
	Date     string   `json:"date"`
	Currency string   `json:"currency"`
	Row      int      `json:"row"`
	Units    int64    `json:"units"`
	Value    int64    `json:"value"`
	Original []string `json:"original"`
}
type Item struct {
	ID            string   `json:"id"`
	Internal      *Holding `json:"internal,omitempty"`
	Administrator *Holding `json:"administrator,omitempty"`
	Kind          string   `json:"kind"`
	Basis         string   `json:"basis"`
	UnitsDelta    int64    `json:"unitsDelta"`
	ValueDelta    int64    `json:"valueDelta"`
}

var internalHeaders = []string{"instrument_id", "instrument_name", "as_of", "currency", "units", "market_value"}
var adminHeaders = []string{"provider_code", "security_id", "description", "valuation_date", "ccy", "quantity", "value"}
var decimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

// Decimal accepts plain positive decimals, preserving precision without float64.
func Decimal(s string, places int) (int64, error) {
	if !decimal.MatchString(s) {
		return 0, fmt.Errorf("use a positive plain decimal")
	}
	parts := strings.Split(s, ".")
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > places {
		return 0, fmt.Errorf("at most %d decimal places", places)
	}
	scale := int64(1)
	for range places {
		scale *= 10
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 1_000_000_000 {
		return 0, fmt.Errorf("maximum is 1000000000")
	}
	f, _ := strconv.ParseInt(frac+strings.Repeat("0", places-len(frac)), 10, 64)
	n := whole*scale + f
	if n <= 0 || n > 1_000_000_000*scale {
		return 0, fmt.Errorf("must be above zero and at most 1000000000")
	}
	return n, nil
}
func Format(n int64, places int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	scale := int64(1)
	for range places {
		scale *= 10
	}
	return fmt.Sprintf("%s%d.%0*d", sign, n/scale, places, n%scale)
}
func Parse(data []byte, admin bool) ([]Holding, error) {
	file := "internal"
	headers := internalHeaders
	if admin {
		file = "administrator"
		headers = adminHeaders
	}
	bad := func(row int, field, msg string) error { return &Validation{file, row, field, msg} }
	if len(data) > 1<<20 {
		return nil, bad(0, "file", "maximum file size is 1 MiB")
	}
	if !utf8.Valid(data) {
		return nil, bad(0, "file", "must be UTF-8")
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{239, 187, 191})))
	reader.FieldsPerRecord = -1
	head, err := reader.Read()
	if err != nil || !slices.Equal(head, headers) {
		return nil, bad(1, "headers", "expected "+strings.Join(headers, ","))
	}
	var result []Holding
	seen := map[string]bool{}
	shared := map[string]bool{}
	date := ""
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			row := 0
			if e, ok := err.(*csv.ParseError); ok {
				row = e.Line
			}
			return nil, bad(row, "record", "invalid CSV: "+err.Error())
		}
		row, _ := reader.FieldPos(0)
		if len(result) >= 1000 {
			return nil, bad(row, "file", "maximum 1000 holdings")
		}
		if len(record) != len(headers) {
			return nil, bad(row, "record", "wrong number of columns")
		}
		original := slices.Clone(record)
		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}
		h := Holding{ID: strings.ToUpper(record[0]), Row: row, Original: original}
		offset := 0
		if admin {
			h.SharedID = strings.ToUpper(record[1])
			offset = 1
		}
		h.Name = record[1+offset]
		h.Date = record[2+offset]
		h.Currency = record[3+offset]
		if h.ID == "" || seen[h.ID] {
			return nil, bad(row, headers[0], "identifier is empty or duplicated")
		}
		seen[h.ID] = true
		if h.SharedID != "" {
			if shared[h.SharedID] {
				return nil, bad(row, "security_id", "duplicate shared identifier")
			}
			shared[h.SharedID] = true
		}
		if h.Name == "" {
			return nil, bad(row, headers[1+offset], "name is required")
		}
		parsed, e := time.Parse("2006-01-02", h.Date)
		if e != nil || parsed.Format("2006-01-02") != h.Date {
			return nil, bad(row, headers[2+offset], "use a valid YYYY-MM-DD date")
		}
		if date != "" && date != h.Date {
			return nil, bad(row, headers[2+offset], "all holdings must have the same date")
		}
		date = h.Date
		if h.Currency != "ZAR" {
			return nil, bad(row, headers[3+offset], "only ZAR is supported")
		}
		h.Units, err = Decimal(record[4+offset], 6)
		if err != nil {
			return nil, bad(row, headers[4+offset], err.Error())
		}
		h.Value, err = Decimal(record[5+offset], 2)
		if err != nil {
			return nil, bad(row, headers[5+offset], err.Error())
		}
		result = append(result, h)
	}
	if len(result) == 0 {
		return nil, bad(0, "file", "at least one holding is required")
	}
	return result, nil
}
func Compare(a, b []Holding, mappings map[string]string) ([]Item, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, fmt.Errorf("both sources are required")
	}
	if a[0].Date != b[0].Date {
		return nil, &Validation{"administrator", b[0].Row, "valuation_date", "must match internal date " + a[0].Date}
	}
	targets := map[string]int{}
	for i := range a {
		targets[a[i].ID] = i
	}
	paired := map[int]int{}
	used := map[int]bool{}
	basis := map[int]string{}
	for j := range b {
		target := b[j].SharedID
		why := "shared_id"
		if target == "" {
			target = mappings[b[j].ID]
			why = "approved_mapping"
		}
		i, exists := targets[target]
		if !exists {
			continue
		}
		if _, exists = paired[i]; exists {
			return nil, &Validation{"administrator", b[j].Row, "provider_code", "mapping conflict: multiple records target " + target}
		}
		paired[i] = j
		used[j] = true
		basis[i] = why
	}
	out := []Item{}
	for i := range a {
		item := Item{ID: fmt.Sprintf("I%d", a[i].Row), Internal: &a[i], Kind: "internal_only", Basis: "unmatched"}
		if j, ok := paired[i]; ok {
			item.Administrator = &b[j]
			item.Basis = basis[i]
			item.UnitsDelta = b[j].Units - a[i].Units
			item.ValueDelta = b[j].Value - a[i].Value
			switch {
			case item.UnitsDelta != 0 && item.ValueDelta != 0:
				item.Kind = "both_difference"
			case item.UnitsDelta != 0:
				item.Kind = "units_difference"
			case item.ValueDelta != 0:
				item.Kind = "value_difference"
			default:
				item.Kind = "matched"
			}
		}
		out = append(out, item)
	}
	for j := range b {
		if !used[j] {
			out = append(out, Item{ID: fmt.Sprintf("A%d", b[j].Row), Administrator: &b[j], Kind: "administrator_only", Basis: "unmatched"})
		}
	}
	return out, nil
}
