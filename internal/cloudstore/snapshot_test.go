package cloudstore

import (
	"bytes"
	"context"
	"errors"
	"reconciliation-workbench/internal/store"
	"sync"
	"testing"
)

type memory struct {
	mu          sync.Mutex
	values      map[string][]byte
	generations map[string]int64
	fail        bool
}

func (m *memory) Load(_ context.Context, key string) ([]byte, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return bytes.Clone(m.values[key]), m.generations[key], nil
}
func (m *memory) Save(_ context.Context, key string, b []byte, generation int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("storage unavailable")
	}
	if m.generations[key] != generation {
		return ErrConflict
	}
	m.values[key] = bytes.Clone(b)
	m.generations[key]++
	return nil
}
func fixture() store.CreateRequest {
	return store.CreateRequest{OperationID: "one", InternalName: "internal.csv", AdministratorName: "admin.csv", Internal: []byte("instrument_id,instrument_name,as_of,currency,units,market_value\nA,Equity,2026-10-01,ZAR,1000,120000"), Administrator: []byte("provider_code,security_id,description,valuation_date,ccy,quantity,value\nEXT,,Equity,2026-10-01,ZAR,980,117600")}
}
func TestDurableWorkspaceIsolationAndReview(t *testing.T) {
	b := &memory{values: map[string][]byte{}, generations: map[string]int64{}}
	r := New(context.Background(), b, "alice")
	run, err := r.Create(fixture())
	if err != nil {
		t.Fatal(err)
	}
	r = New(context.Background(), b, "alice")
	got, err := r.Get(run.ID)
	if err != nil || got.ID != run.ID {
		t.Fatal("restoration", err)
	}
	repeated, err := r.Create(fixture())
	if err != nil || repeated.ID != run.ID {
		t.Fatal("retry", err)
	}
	bob := New(context.Background(), b, "bob")
	if _, err = bob.Get(run.ID); err == nil {
		t.Fatal("workspace data leaked")
	}
	if _, err = r.Map(store.MappingRequest{OperationID: "map", RunID: run.ID, ProviderCode: "EXT", InternalID: "A", Reviewer: "Josh", Note: "Confirmed identity"}); err != nil {
		t.Fatal(err)
	}
	rerun, err := New(context.Background(), b, "alice").Create(store.CreateRequest{OperationID: "rerun", ParentID: run.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Review(store.ReviewRequest{OperationID: "review", RunID: rerun.ID, ItemID: rerun.Items[0].ID, Reviewed: true, Reviewer: "Josh", Reason: "other", Note: "Investigated"}); err != nil {
		t.Fatal(err)
	}
	got, err = New(context.Background(), b, "alice").Get(rerun.ID)
	if err != nil || !got.Items[0].Reviewed || len(got.Items[0].Events) != 1 {
		t.Fatal("review persistence", err)
	}
	original, _ := r.Get(run.ID)
	if original.Items[0].Kind != "internal_only" {
		t.Fatal("old run changed")
	}
	b.fail = true
	if _, err = r.Create(store.CreateRequest{OperationID: "failed", ParentID: run.ID}); err == nil {
		t.Fatal("failed upload reported success")
	}
	b.fail = false
	all, _ := r.List()
	if len(all) != 2 {
		t.Fatal("failed write leaked")
	}
}

type competing struct {
	*memory
	loaded  chan struct{}
	release chan struct{}
}

func (b *competing) Load(ctx context.Context, key string) ([]byte, int64, error) {
	data, g, e := b.memory.Load(ctx, key)
	b.loaded <- struct{}{}
	<-b.release
	return data, g, e
}
func TestConcurrentSnapshotsCannotOverwrite(t *testing.T) {
	b := &competing{memory: &memory{values: map[string][]byte{}, generations: map[string]int64{}}, loaded: make(chan struct{}, 2), release: make(chan struct{})}
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := New(context.Background(), b, "same").Create(fixture()); results <- err }()
	}
	<-b.loaded
	<-b.loaded
	close(b.release)
	one, two := <-results, <-results
	if (one == nil) == (two == nil) {
		t.Fatalf("expected one successful publication: %v %v", one, two)
	}
	r := New(context.Background(), b.memory, "same")
	all, err := r.List()
	if err != nil || len(all) != 1 {
		t.Fatal("concurrent state", err)
	}
	retry, err := r.Create(fixture())
	if err != nil || retry.ID != all[0].ID {
		t.Fatal("retry after conflict", err)
	}
}
