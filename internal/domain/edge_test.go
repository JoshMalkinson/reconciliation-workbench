package domain

import (
	"fmt"
	"strings"
	"testing"
)

func TestFileContractEdges(t *testing.T) {
	cases := []struct {
		name, data string
		admin      bool
	}{
		{"extra header", strings.TrimSpace(ih) + ",extra\nA,Name,2026-10-01,ZAR,1,2,x", false},
		{"malformed quote", ih + "A,\"Unclosed,2026-10-01,ZAR,1,2", false},
		{"wrong columns", ih + "A,Name,2026-10-01,ZAR,1", false},
		{"mixed date", ih + "A,Name,2026-10-01,ZAR,1,2\nB,Other,2026-10-02,ZAR,1,2", false},
		{"duplicate shared ID", ah + "X,A,One,2026-10-01,ZAR,1,2\nY, a ,Two,2026-10-01,ZAR,1,2", true},
		{"duplicate provider ID", ah + "x,A,One,2026-10-01,ZAR,1,2\n X ,B,Two,2026-10-01,ZAR,1,2", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse([]byte(c.data), c.admin); err == nil {
				t.Fatal("invalid file accepted")
			}
		})
	}
	var data strings.Builder
	data.WriteString(ih)
	for i := range 1001 {
		fmt.Fprintf(&data, "A%d,Name,2026-10-01,ZAR,1,2\n", i)
	}
	if _, err := Parse([]byte(data.String()), false); err == nil {
		t.Fatal("row limit not enforced")
	}
}
func TestDifferenceClassifications(t *testing.T) {
	a := []Holding{{ID: "A", Date: "2026-10-01", Row: 2, Units: 1000000, Value: 100}}
	for _, c := range []struct {
		units, value int64
		kind         string
	}{{1000000, 100, "matched"}, {1000001, 100, "units_difference"}, {1000000, 99, "value_difference"}, {999999, 101, "both_difference"}} {
		b := []Holding{{ID: "EXT", SharedID: "A", Date: a[0].Date, Units: c.units, Value: c.value}}
		items, err := Compare(a, b, nil)
		if err != nil || items[0].Kind != c.kind || items[0].UnitsDelta != c.units-a[0].Units || items[0].ValueDelta != c.value-a[0].Value {
			t.Fatalf("%s: %+v %v", c.kind, items, err)
		}
	}
	b := []Holding{{ID: "EXT", Date: a[0].Date}}
	items, err := Compare(a, b, map[string]string{"EXT": "ABSENT"})
	if err != nil || len(items) != 2 {
		t.Fatal("absent mapping target", err)
	}
}
