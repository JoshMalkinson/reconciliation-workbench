package store

import (
	"path/filepath"
	"testing"
)

var internalCSV = []byte("instrument_id,instrument_name,as_of,currency,units,market_value\nA,Equity,2026-10-01,ZAR,1000,120000\nB,Bond,2026-10-01,ZAR,10,50\n")
var adminCSV = []byte("provider_code,security_id,description,valuation_date,ccy,quantity,value\nEXT,,Equity A,2026-10-01,ZAR,980,117600\nBEXT,B,Bond,2026-10-01,ZAR,10,51\n")

func TestWorkflowPersistenceAndRetries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	req := CreateRequest{OperationID: "import-1", Internal: internalCSV, Administrator: adminCSV, InternalName: "internal.csv", AdministratorName: "admin.csv"}
	run, err := s.Create(req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Create(req)
	if err != nil || again.ID != run.ID {
		t.Fatalf("retry %v", err)
	}
	req.InternalName = "different"
	if _, err = s.Create(req); err == nil {
		t.Fatal("operation collision")
	}
	mapping := MappingRequest{OperationID: "map-1", RunID: run.ID, ProviderCode: "EXT", InternalID: "A", Reviewer: "Josh", Note: "Confirmed provider identifiers"}
	if _, err = s.Map(mapping); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Map(mapping); err != nil {
		t.Fatal("mapping retry", err)
	}
	newer, err := s.Create(CreateRequest{OperationID: "rerun-1", ParentID: run.ID})
	if err != nil {
		t.Fatal(err)
	}
	if newer.Items[0].Kind != "both_difference" || newer.ParentID != run.ID {
		t.Fatal("mapping not applied")
	}
	old, _ := s.Get(run.ID)
	if old.Items[0].Kind != "internal_only" {
		t.Fatal("old run changed")
	}
	review := ReviewRequest{OperationID: "review-1", RunID: newer.ID, ItemID: newer.Items[0].ID, Reviewer: "Josh", Reason: "timing_difference", Note: "Awaiting settled trade", Reviewed: true}
	if _, err = s.Review(review); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Review(review); err != nil {
		t.Fatal("review retry", err)
	}
	review.OperationID = "stale"
	if _, err = s.Review(review); err == nil {
		t.Fatal("stale review accepted")
	}
	review.OperationID = "reopen"
	review.ExpectedRevision = 1
	review.Reviewed = false
	review.Note = "Needs another look"
	if _, err = s.Review(review); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(newer.ID)
	if err != nil || len(got.Items[0].Events) != 2 || got.Items[0].Reviewed {
		t.Fatal("restart persistence", err)
	}
	all, _ := s.List()
	if len(all) != 2 {
		t.Fatal("duplicate/partial run")
	}
	if _, err = s.Create(CreateRequest{OperationID: "bad", Internal: []byte("bad"), Administrator: adminCSV}); err == nil {
		t.Fatal("invalid import")
	}
	all, _ = s.List()
	if len(all) != 2 {
		t.Fatal("partial invalid run")
	}
}
