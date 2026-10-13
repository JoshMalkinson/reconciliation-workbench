package domain

import (
	"os"
	"strings"
	"testing"
)

func TestDownloadableSamplePack(t *testing.T) {
	load := func(name string, admin bool) []Holding {
		t.Helper()
		b, err := os.ReadFile("../../web/public/test-files/" + name + ".csv")
		if err != nil {
			t.Fatal(err)
		}
		rows, err := Parse(b, admin)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	internal := load("01-matching-internal", false)
	items, err := Compare(internal, load("01-matching-administrator", true), nil)
	if err != nil || len(items) != 3 {
		t.Fatalf("matching: %v %v", items, err)
	}
	for _, item := range items {
		if item.Kind != "matched" {
			t.Fatalf("unexpected mismatch: %+v", item)
		}
	}
	internal = load("02-differences-internal", false)
	admin := load("02-differences-administrator", true)
	items, err = Compare(internal, admin, nil)
	if err != nil || len(items) != 5 {
		t.Fatalf("before mapping: %v %v", items, err)
	}
	items, err = Compare(internal, admin, map[string]string{"TEST-MAP-1": "TEST-EQUITY"})
	if err != nil || len(items) != 4 {
		t.Fatalf("after mapping: %v %v", items, err)
	}
	kinds := map[string]int{}
	for _, item := range items {
		kinds[item.Kind]++
		if item.Internal != nil && item.Internal.ID == "TEST-EQUITY" && (item.UnitsDelta != -20_000000 || item.ValueDelta != -240000) {
			t.Fatalf("incorrect equity deltas: %+v", item)
		}
	}
	for _, kind := range []string{"units_difference", "value_difference", "both_difference", "administrator_only"} {
		if kinds[kind] != 1 {
			t.Fatalf("classifications: %v", kinds)
		}
	}
	load("03-invalid-internal", false)
	invalid, err := os.ReadFile("../../web/public/test-files/03-invalid-administrator.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Parse(invalid, true)
	if err == nil || !strings.Contains(err.Error(), "row 2, quantity") {
		t.Fatalf("expected row 2 quantity error, got %v", err)
	}
}
