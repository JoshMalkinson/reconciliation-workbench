package store

import (
	"encoding/csv"
	"reconciliation-workbench/internal/domain"
	"strings"
	"testing"
)

func TestExportPreservesMissingAndNeutralizesFormulas(t *testing.T) {
	run := &Run{ID: "run", Items: []Item{{Item: domain.Item{ID: "I2", Kind: "internal_only", Internal: &domain.Holding{ID: "A", Name: "=1+2", Units: 1_000001, Value: 123, Row: 2}}}}}
	data, err := ExportCSV(run)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for i, h := range rows[0] {
		fields[h] = rows[1][i]
	}
	if fields["internal_name"] != "'=1+2" || fields["administrator_units"] != "" || fields["units_delta"] != "" || fields["internal_units"] != "1.000001" {
		t.Fatal(fields)
	}
}
