package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"reconciliation-workbench/internal/domain"
	"strings"
	"sync"
	"time"
)

// Fault is safe to present to a caller; database errors are never exposed by transport.
type Fault struct {
	Message  string
	Conflict bool
}

func (e *Fault) Error() string { return e.Message }
func invalid(s string) error   { return &Fault{Message: s} }
func conflict(s string) error  { return &Fault{Message: s, Conflict: true} }

type Event struct {
	At       string `json:"at"`
	Reviewer string `json:"reviewer"`
	Reason   string `json:"reason"`
	Note     string `json:"note"`
	Reviewed bool   `json:"reviewed"`
	Revision int32  `json:"revision"`
}
type Item struct {
	domain.Item
	Reviewed bool    `json:"reviewed"`
	Revision int32   `json:"revision"`
	Events   []Event `json:"events"`
}
type Mapping struct {
	ProviderCode string `json:"providerCode"`
	InternalID   string `json:"internalId"`
	Reviewer     string `json:"reviewer"`
	Note         string `json:"note"`
	At           string `json:"at"`
	RunID        string `json:"runId"`
}
type Run struct {
	ID                string    `json:"id"`
	ParentID          string    `json:"parentId"`
	At                string    `json:"at"`
	Date              string    `json:"date"`
	InternalName      string    `json:"internalName"`
	AdministratorName string    `json:"administratorName"`
	InternalHash      string    `json:"internalHash"`
	AdministratorHash string    `json:"administratorHash"`
	Items             []Item    `json:"items"`
	Mappings          []Mapping `json:"mappings"`
}
type CreateRequest struct {
	OperationID                     string
	ParentID                        string
	Internal, Administrator         []byte
	InternalName, AdministratorName string
}
type MappingRequest struct{ OperationID, RunID, ProviderCode, InternalID, Reviewer, Note string }
type ReviewRequest struct {
	OperationID, RunID, ItemID, Reviewer, Reason, Note string
	Reviewed                                           bool
	ExpectedRevision                                   int32
}
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS runs(id TEXT PRIMARY KEY, body BLOB NOT NULL, internal BLOB NOT NULL, administrator BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS mappings(code TEXT PRIMARY KEY, body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS reviews(run_id TEXT NOT NULL,item_id TEXT NOT NULL,body BLOB NOT NULL,PRIMARY KEY(run_id,item_id));
 CREATE TABLE IF NOT EXISTS operations(id TEXT PRIMARY KEY, hash TEXT NOT NULL, result BLOB NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func digest(b []byte) string  { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func now() string             { return time.Now().UTC().Format(time.RFC3339Nano) }
func id() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func encode(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type queryer interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}

// Every write and its operation result commit together. One connection serializes
// writers; SQLite's transaction also protects against a second app process.
func mutate[T any](s *Store, op, kind string, request any, work func(*sql.Tx) (T, error)) (T, error) {
	var zero T
	if len(op) < 1 || len(op) > 128 {
		return zero, invalid("operation ID is required (maximum 128 characters)")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	hash := digest(append([]byte(kind), encode(request)...))
	var oldHash string
	var saved []byte
	err = tx.QueryRow("SELECT hash,result FROM operations WHERE id=?", op).Scan(&oldHash, &saved)
	if err == nil {
		if hash != oldHash {
			return zero, conflict("operation ID was already used for a different request")
		}
		err = json.Unmarshal(saved, &zero)
		return zero, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return zero, err
	}
	result, err := work(tx)
	if err != nil {
		return zero, err
	}
	_, err = tx.Exec("INSERT INTO operations VALUES(?,?,?)", op, hash, encode(result))
	if err != nil {
		return zero, err
	}
	if err = tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}
func get(q queryer, id string) (*Run, error) {
	var body []byte
	err := q.QueryRow("SELECT body FROM runs WHERE id=?", id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("run not found")
	}
	if err != nil {
		return nil, err
	}
	var run Run
	if err = json.Unmarshal(body, &run); err != nil {
		return nil, err
	}
	rows, err := q.Query("SELECT item_id,body FROM reviews WHERE run_id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := map[string][]Event{}
	for rows.Next() {
		var itemID string
		var b []byte
		if err = rows.Scan(&itemID, &b); err != nil {
			return nil, err
		}
		var ev []Event
		if err = json.Unmarshal(b, &ev); err != nil {
			return nil, err
		}
		reviews[itemID] = ev
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range run.Items {
		if ev := reviews[run.Items[i].ID]; len(ev) > 0 {
			run.Items[i].Events = ev
			last := ev[len(ev)-1]
			run.Items[i].Reviewed = last.Reviewed
			run.Items[i].Revision = last.Revision
		}
	}
	return &run, nil
}
func (s *Store) Get(id string) (*Run, error) { return get(s.db, id) }
func mappings(q queryer) ([]Mapping, error) {
	out := []Mapping{}
	rows, err := q.Query("SELECT body FROM mappings ORDER BY code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		var m Mapping
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) Mappings() ([]Mapping, error) { return mappings(s.db) }
func (s *Store) List() ([]Run, error) {
	rows, err := s.db.Query("SELECT body FROM runs ORDER BY rowid DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var b []byte
		var r Run
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		r.Items = nil
		r.Mappings = nil
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) Create(req CreateRequest) (*Run, error) {
	return mutate(s, req.OperationID, "create", req, func(tx *sql.Tx) (*Run, error) {
		if req.ParentID != "" {
			if len(req.Internal) > 0 || len(req.Administrator) > 0 {
				return nil, invalid("rerun cannot also supply new files")
			}
			parent, err := get(tx, req.ParentID)
			if err != nil {
				return nil, err
			}
			if err = tx.QueryRow("SELECT internal,administrator FROM runs WHERE id=?", req.ParentID).Scan(&req.Internal, &req.Administrator); err != nil {
				return nil, err
			}
			req.InternalName = parent.InternalName
			req.AdministratorName = parent.AdministratorName
		}
		if len(req.InternalName) > 255 || len(req.AdministratorName) > 255 {
			return nil, invalid("file names must be at most 255 characters")
		}
		a, err := domain.Parse(req.Internal, false)
		if err != nil {
			return nil, err
		}
		b, err := domain.Parse(req.Administrator, true)
		if err != nil {
			return nil, err
		}
		snapshot, err := mappings(tx)
		if err != nil {
			return nil, err
		}
		lookup := map[string]string{}
		for _, m := range snapshot {
			lookup[m.ProviderCode] = m.InternalID
		}
		compared, err := domain.Compare(a, b, lookup)
		if err != nil {
			return nil, err
		}
		run := &Run{ID: id(), ParentID: req.ParentID, At: now(), Date: a[0].Date, InternalName: req.InternalName, AdministratorName: req.AdministratorName, InternalHash: digest(req.Internal), AdministratorHash: digest(req.Administrator), Mappings: snapshot, Items: []Item{}}
		for _, item := range compared {
			run.Items = append(run.Items, Item{Item: item, Events: []Event{}})
		}
		_, err = tx.Exec("INSERT INTO runs VALUES(?,?,?,?)", run.ID, encode(run), req.Internal, req.Administrator)
		return run, err
	})
}
func metadata(reviewer, note string) error {
	if strings.TrimSpace(reviewer) == "" || len(reviewer) > 100 {
		return invalid("reviewer name is required (maximum 100 characters)")
	}
	if strings.TrimSpace(note) == "" || len(note) > 2000 {
		return invalid("note is required (maximum 2000 characters)")
	}
	return nil
}
func (s *Store) Map(req MappingRequest) (*Mapping, error) {
	return mutate(s, req.OperationID, "map", req, func(tx *sql.Tx) (*Mapping, error) {
		if err := metadata(req.Reviewer, req.Note); err != nil {
			return nil, err
		}
		run, err := get(tx, req.RunID)
		if err != nil {
			return nil, err
		}
		internalOK, adminOK := false, false
		for _, item := range run.Items {
			if item.Kind == "internal_only" && item.Internal.ID == req.InternalID {
				internalOK = true
			}
			if item.Kind == "administrator_only" && item.Administrator.ID == req.ProviderCode && item.Administrator.SharedID == "" {
				adminOK = true
			}
		}
		if !internalOK || !adminOK {
			return nil, invalid("choose an unmatched internal record and an administrator record without a shared ID")
		}
		existing, err := mappings(tx)
		if err != nil {
			return nil, err
		}
		lookup := map[string]string{}
		for _, m := range existing {
			if m.ProviderCode == req.ProviderCode {
				return nil, conflict("this provider code already has an approved mapping")
			}
			lookup[m.ProviderCode] = m.InternalID
		}
		lookup[req.ProviderCode] = req.InternalID
		var a, b []domain.Holding
		for _, item := range run.Items {
			if item.Internal != nil {
				a = append(a, *item.Internal)
			}
			if item.Administrator != nil {
				b = append(b, *item.Administrator)
			}
		}
		if _, err = domain.Compare(a, b, lookup); err != nil {
			return nil, err
		}
		m := &Mapping{ProviderCode: req.ProviderCode, InternalID: req.InternalID, Reviewer: strings.TrimSpace(req.Reviewer), Note: strings.TrimSpace(req.Note), At: now(), RunID: req.RunID}
		_, err = tx.Exec("INSERT INTO mappings VALUES(?,?)", m.ProviderCode, encode(m))
		return m, err
	})
}
func (s *Store) Review(req ReviewRequest) (*Item, error) {
	return mutate(s, req.OperationID, "review", req, func(tx *sql.Tx) (*Item, error) {
		if err := metadata(req.Reviewer, req.Note); err != nil {
			return nil, err
		}
		if req.Reason != "timing_difference" && req.Reason != "source_correction_requested" && req.Reason != "other" {
			return nil, invalid("select a review reason")
		}
		run, err := get(tx, req.RunID)
		if err != nil {
			return nil, err
		}
		for _, item := range run.Items {
			if item.ID != req.ItemID {
				continue
			}
			if item.Kind == "matched" {
				return nil, invalid("matched items do not need review")
			}
			if item.Revision != req.ExpectedRevision {
				return nil, conflict("this item changed in another tab; reload the run before reviewing")
			}
			if item.Reviewed == req.Reviewed {
				return nil, conflict("review is already in this state")
			}
			item.Revision++
			item.Reviewed = req.Reviewed
			item.Events = append(item.Events, Event{At: now(), Reviewer: strings.TrimSpace(req.Reviewer), Reason: req.Reason, Note: strings.TrimSpace(req.Note), Reviewed: req.Reviewed, Revision: item.Revision})
			_, err = tx.Exec("INSERT INTO reviews VALUES(?,?,?) ON CONFLICT(run_id,item_id) DO UPDATE SET body=excluded.body", run.ID, item.ID, encode(item.Events))
			return &item, err
		}
		return nil, invalid(fmt.Sprintf("item %s not found", req.ItemID))
	})
}
