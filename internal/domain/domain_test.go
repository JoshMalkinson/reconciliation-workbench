package domain

import (
	"strings"
	"testing"
)

const ih = "instrument_id,instrument_name,as_of,currency,units,market_value\n"
const ah = "provider_code,security_id,description,valuation_date,ccy,quantity,value\n"

func TestParseAndCompare(t *testing.T) {
	a, err := Parse([]byte(ih+"fund-a,Equity,2026-10-01,ZAR,1000,120000.00\nB,Bond,2026-10-01,ZAR,2.000001,3.01"), false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse([]byte(ah+"EXT-001,,Equity A,2026-10-01,ZAR,980,117600.00\nEXT-2,B,Bond,2026-10-01,ZAR,2.000001,3.01"), true)
	if err != nil {
		t.Fatal(err)
	}
	items, err := Compare(a, b, map[string]string{})
	if err != nil || len(items) != 3 {
		t.Fatalf("%v %v", items, err)
	}
	items, err = Compare(a, b, map[string]string{"EXT-001": "FUND-A"})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Kind != "both_difference" || items[0].UnitsDelta != -20_000000 || items[0].ValueDelta != -240000 || items[0].Basis != "approved_mapping" {
		t.Fatalf("%+v", items[0])
	}
	if items[1].Kind != "matched" {
		t.Fatal(items[1])
	}
}

func TestValidation(t *testing.T) {
	for _, tail := range []string{"A,Name,2026-10-01,ZAR,1e3,2", "A,Name,2026-02-30,ZAR,1,2", "A,Name,2026-10-01,USD,1,2", "A,Name,2026-10-01,ZAR,0,2", "A,Name,2026-10-01,ZAR,1,2.001", "A,Name,2026-10-01,ZAR,1000000000.000001,2", "A,,2026-10-01,ZAR,1,2", "A,Name,2026-10-01,ZAR,1,2\na,Other,2026-10-01,ZAR,1,2"} {
		if _, err := Parse([]byte(ih+tail), false); err == nil {
			t.Errorf("accepted %s", tail)
		}
	}
	if _, err := Parse([]byte(ih), false); err == nil {
		t.Fatal("empty file accepted")
	}
	if _, err := Parse([]byte(strings.Repeat("a", (1<<20)+1)), false); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := Parse(append([]byte(ih), 0xff), false); err == nil {
		t.Fatal("invalid UTF8")
	}
	a, err := Parse([]byte("\ufeff"+ih+"\nA,\"Name\ncontinued\",2026-10-01,ZAR,1000000000,1000000000.00\n"), false)
	if err != nil || a[0].Row != 3 || a[0].Units != 1_000_000_000_000000 {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestMatchingConflictsAndDates(t *testing.T) {
	a, _ := Parse([]byte(ih+"A,Name,2026-10-01,ZAR,1,2"), false)
	b, _ := Parse([]byte(ah+"X,A,Name,2026-10-01,ZAR,1,2\nY,,Other,2026-10-01,ZAR,1,2"), true)
	if _, err := Compare(a, b, map[string]string{"Y": "A"}); err == nil {
		t.Fatal("ambiguous mapping")
	}
	b = b[:1]
	b[0].Date = "2026-10-02"
	if _, err := Compare(a, b, nil); err == nil {
		t.Fatal("date mismatch")
	}
	b[0].Date = a[0].Date
	b[0].SharedID = "Z"
	items, err := Compare(a, b, map[string]string{"X": "A"})
	if err != nil || len(items) != 2 {
		t.Fatal("shared ID overridden")
	}
}
