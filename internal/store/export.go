package store

import (
	"bytes"
	"encoding/csv"
	"reconciliation-workbench/internal/domain"
	"strconv"
	"strings"
)

func safeText(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) || strings.ContainsAny(s, "\t\r\n") {
		return "'" + s
	}
	return s
}
func ExportCSV(run *Run) ([]byte, error) {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	headers := []string{"run_id", "parent_run_id", "exported_at", "valuation_date", "internal_sha256", "administrator_sha256", "item_id", "classification", "matching_basis", "review_state", "internal_id", "internal_name", "internal_row", "internal_units", "internal_value", "administrator_code", "administrator_security_id", "administrator_name", "administrator_row", "administrator_units", "administrator_value", "units_delta", "value_delta", "reviewer", "review_reason", "review_note", "internal_original_record", "administrator_original_record"}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	exported := now()
	for _, item := range run.Items {
		state := "Open"
		if item.Kind == "matched" {
			state = "Not required"
		} else if item.Reviewed {
			state = "Reviewed"
		}
		r := []string{run.ID, run.ParentID, exported, run.Date, run.InternalHash, run.AdministratorHash, item.ID, item.Kind, item.Basis, state}
		if h := item.Internal; h != nil {
			r = append(r, safeText(h.ID), safeText(h.Name), strconv.Itoa(h.Row), domain.Format(h.Units, 6), domain.Format(h.Value, 2))
		} else {
			r = append(r, "", "", "", "", "")
		}
		if h := item.Administrator; h != nil {
			r = append(r, safeText(h.ID), safeText(h.SharedID), safeText(h.Name), strconv.Itoa(h.Row), domain.Format(h.Units, 6), domain.Format(h.Value, 2))
		} else {
			r = append(r, "", "", "", "", "", "")
		}
		if item.Internal != nil && item.Administrator != nil {
			r = append(r, domain.Format(item.UnitsDelta, 6), domain.Format(item.ValueDelta, 2))
		} else {
			r = append(r, "", "")
		}
		if len(item.Events) > 0 {
			e := item.Events[len(item.Events)-1]
			r = append(r, safeText(e.Reviewer), e.Reason, safeText(e.Note))
		} else {
			r = append(r, "", "", "")
		}
		for _, h := range []*domain.Holding{item.Internal, item.Administrator} {
			if h == nil {
				r = append(r, "")
			} else {
				r = append(r, safeText(string(encode(h.Original))))
			}
		}
		if err := w.Write(r); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return out.Bytes(), w.Error()
}
