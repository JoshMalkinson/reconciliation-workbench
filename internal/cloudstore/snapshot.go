// Package cloudstore persists bounded demo workspaces as conditionally-written
// SQLite snapshots. Temporary files are never the durable source of truth.
package cloudstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reconciliation-workbench/internal/store"
)

const MaxSnapshot = 32 << 20

var ErrConflict = errors.New("snapshot generation conflict")

type Objects interface {
	Load(context.Context, string) ([]byte, int64, error)
	Save(context.Context, string, []byte, int64) error
}
type Repository struct {
	ctx     context.Context
	objects Objects
	key     string
}

func New(ctx context.Context, objects Objects, key string) *Repository {
	return &Repository{ctx, objects, key}
}
func execute[T any](r *Repository, write bool, fn func(*store.Store) (T, error)) (T, error) {
	var zero T
	data, generation, err := r.objects.Load(r.ctx, r.key)
	if err != nil {
		return zero, err
	}
	if len(data) > MaxSnapshot {
		return zero, errors.New("workspace exceeds snapshot limit")
	}
	dir, err := os.MkdirTemp("", "reconcile-workspace-")
	if err != nil {
		return zero, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "workspace.db")
	if len(data) > 0 {
		if err = os.WriteFile(path, data, 0600); err != nil {
			return zero, err
		}
	}
	db, err := store.Open(path)
	if err != nil {
		return zero, err
	}
	defer db.Close()
	result, err := fn(db)
	if err != nil {
		return zero, err
	}
	if !write {
		return result, nil
	}
	runs, err := db.List()
	if err != nil {
		return zero, err
	}
	if len(runs) > 20 {
		return zero, &store.Fault{Message: "This demo workspace allows 20 saved runs. Export your results and use a fresh browser workspace to continue."}
	}
	// Close the final connection before reading the main file: SQLite checkpoints
	// and removes WAL here. Restoring tests verify review/mapping data is included.
	if err = db.Close(); err != nil {
		return zero, err
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	if len(data) > MaxSnapshot {
		return zero, &store.Fault{Message: "This demo workspace has reached its 32 MiB limit. Export your results before starting a fresh workspace."}
	}
	if err = r.objects.Save(r.ctx, r.key, data, generation); err != nil {
		if errors.Is(err, ErrConflict) {
			return zero, &store.Fault{Message: "This workspace changed in another request. Reload the run and retry.", Conflict: true}
		}
		return zero, err
	}
	return result, nil
}
func (r *Repository) Create(req store.CreateRequest) (*store.Run, error) {
	return execute(r, true, func(s *store.Store) (*store.Run, error) { return s.Create(req) })
}
func (r *Repository) Get(id string) (*store.Run, error) {
	return execute(r, false, func(s *store.Store) (*store.Run, error) { return s.Get(id) })
}
func (r *Repository) List() ([]store.Run, error) {
	return execute(r, false, func(s *store.Store) ([]store.Run, error) { return s.List() })
}
func (r *Repository) Mappings() ([]store.Mapping, error) {
	return execute(r, false, func(s *store.Store) ([]store.Mapping, error) { return s.Mappings() })
}
func (r *Repository) Map(req store.MappingRequest) (*store.Mapping, error) {
	return execute(r, true, func(s *store.Store) (*store.Mapping, error) { return s.Map(req) })
}
func (r *Repository) Review(req store.ReviewRequest) (*store.Item, error) {
	return execute(r, true, func(s *store.Store) (*store.Item, error) { return s.Review(req) })
}
