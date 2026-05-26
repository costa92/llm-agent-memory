// Package memory — sqlite_store.go is the Phase C-2 implementation
// of coremem.SnapshotStore backed by SQLite via the pure-Go
// modernc.org/sqlite driver. No CGO required.
//
// Schema: see SchemaVersion + the migrator below. Two tables:
//
//	memory_store_schema (version, applied_at)
//	memory_snapshots    (key, kind, snapshot_json, updated_at)
//
// with a single index on memory_snapshots(key).
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	coremem "github.com/costa92/llm-agent/memory"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// SchemaVersion is the SQLiteStore schema version this binary
// implements. NewSQLiteStore migrates up to this version on open;
// a database recording a HIGHER version is refused with
// ErrSchemaVersionAhead.
const SchemaVersion = 1

// ErrSchemaVersionAhead is returned by NewSQLiteStore when the
// database's recorded schema version exceeds SchemaVersion. This
// protects against an older binary silently downgrading state
// written by a newer one.
var ErrSchemaVersionAhead = errors.New("memory: sqlite store schema ahead of code")

// ErrSQLiteDSNRequired is returned by NewSQLiteStore when dsn is
// empty.
var ErrSQLiteDSNRequired = errors.New("memory: sqlite store requires a non-empty DSN")

// SQLiteStore implements coremem.SnapshotStore + the optional
// LoadKind(ctx, key, kind) method consumed by
// coremem.Manager.ImportAll (manager.go:369-371). Goroutine-safe via
// the underlying *sql.DB.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens dsn (any modernc.org/sqlite DSN — file path,
// file:URI, or in-memory `file::memory:?cache=shared`), runs the
// in-code migrator up to SchemaVersion, and returns the store. Pool
// is left at driver defaults except for `:memory:` DSNs, where
// SetMaxOpenConns(1) is required for shared-in-memory tests.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	if dsn == "" {
		return nil, ErrSQLiteDSNRequired
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("memory: sqlite store open: %w", err)
	}
	if strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "mode=memory") {
		db.SetMaxOpenConns(1)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	current, err := s.currentVersion(context.Background())
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if current > SchemaVersion {
		_ = db.Close()
		return nil, fmt.Errorf("%w: db=%d, code=%d", ErrSchemaVersionAhead, current, SchemaVersion)
	}
	return s, nil
}

// Close closes the underlying database handle. Calling Close
// concurrently or multiple times is safe — *sql.DB.Close is
// itself idempotent and goroutine-safe.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// migrate runs every pending migration step in a single transaction.
// Idempotent: re-running against an already-current database is a
// no-op. Migrations are numbered 1..SchemaVersion and applied in
// order; a future SchemaVersion>1 extends the migrations slice
// below.
func (s *SQLiteStore) migrate(ctx context.Context) error {
	migrations := []struct {
		version int
		stmts   []string
	}{
		{
			version: 1,
			stmts: []string{
				`CREATE TABLE IF NOT EXISTS memory_store_schema (
					version    INTEGER PRIMARY KEY,
					applied_at TEXT NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS memory_snapshots (
					key           TEXT NOT NULL,
					kind          TEXT NOT NULL,
					snapshot_json BLOB NOT NULL,
					updated_at    TEXT NOT NULL,
					PRIMARY KEY (key, kind)
				)`,
				`CREATE INDEX IF NOT EXISTS idx_memory_snapshots_key ON memory_snapshots(key)`,
			},
		},
	}

	// Find which versions are not yet applied.
	applied, err := s.appliedVersions(ctx)
	if err != nil && !isNoSchemaTable(err) {
		return fmt.Errorf("memory: sqlite store migrate: read applied versions: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("memory: sqlite store migrate: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		for _, stmt := range m.stmts {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("memory: sqlite store migrate v%d: %w", m.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO memory_store_schema (version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("memory: sqlite store migrate v%d: record: %w", m.version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("memory: sqlite store migrate: commit: %w", err)
	}
	return nil
}

// appliedVersions reads the memory_store_schema table and returns a
// set of applied version numbers. Returns an empty set + nil error
// if the table does not exist yet (first migration run).
func (s *SQLiteStore) appliedVersions(ctx context.Context) (map[int]bool, error) {
	out := map[int]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM memory_store_schema`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return out, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func (s *SQLiteStore) currentVersion(ctx context.Context) (int, error) {
	var v sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(version) FROM memory_store_schema`).Scan(&v); err != nil {
		return 0, fmt.Errorf("memory: sqlite store: read current version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

// isNoSchemaTable returns true if the error indicates the
// memory_store_schema table does not exist yet. modernc.org/sqlite
// surfaces this via a SQLITE_ERROR with the message containing
// "no such table".
func isNoSchemaTable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such table: memory_store_schema")
}

// sanitizeSQLiteKey replaces every character outside [a-zA-Z0-9_-]
// with '_'. Empty input becomes "_". Keep in sync with the
// sanitizer in github.com/costa92/llm-agent/memory/persistence.go
// (persistence.go:206-219) so a caller key normalizes identically
// across SnapshotStore impls.
func sanitizeSQLiteKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// Save UPSERTs (key, snap.Kind) → snap. snap.Kind must be non-empty.
func (s *SQLiteStore) Save(ctx context.Context, key string, snap coremem.Snapshot) error {
	if snap.Kind == "" {
		return errors.New("memory: sqlite store save: snapshot kind is required")
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("memory: sqlite store save: encode: %w", err)
	}
	sk := sanitizeSQLiteKey(key)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO memory_snapshots (key, kind, snapshot_json, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(key, kind) DO UPDATE SET snapshot_json = excluded.snapshot_json, updated_at = excluded.updated_at`,
		sk, string(snap.Kind), payload, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("memory: sqlite store save: %w", err)
	}
	return nil
}

// Load returns the first snapshot found for key across the three
// kinds (working → episodic → semantic). Returns an error wrapping
// os.ErrNotExist when no row exists for any kind. Mirrors
// (*FilesystemStore).Load semantics from persistence.go:254-265.
func (s *SQLiteStore) Load(ctx context.Context, key string) (coremem.Snapshot, error) {
	for _, kind := range []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic, coremem.KindSemantic} {
		snap, err := s.LoadKind(ctx, key, kind)
		if err == nil {
			return snap, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return coremem.Snapshot{}, err
		}
	}
	return coremem.Snapshot{}, fmt.Errorf("memory: sqlite store: no snapshot for key %q: %w", key, os.ErrNotExist)
}

// LoadKind returns the snapshot for the exact (key, kind) tuple.
// Returns an error wrapping os.ErrNotExist when no row exists. The
// method exists explicitly so coremem.Manager.ImportAll's optional
// kindLoader type-assertion (manager.go:369-371) finds it and uses
// the per-kind path instead of falling back to Load.
func (s *SQLiteStore) LoadKind(ctx context.Context, key string, kind coremem.Kind) (coremem.Snapshot, error) {
	sk := sanitizeSQLiteKey(key)
	var payload []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT snapshot_json FROM memory_snapshots WHERE key = ? AND kind = ?`,
		sk, string(kind),
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return coremem.Snapshot{}, fmt.Errorf("memory: sqlite store: no snapshot for key %q kind %q: %w", key, kind, os.ErrNotExist)
	}
	if err != nil {
		return coremem.Snapshot{}, fmt.Errorf("memory: sqlite store load: %w", err)
	}
	var snap coremem.Snapshot
	if err := json.Unmarshal(payload, &snap); err != nil {
		return coremem.Snapshot{}, fmt.Errorf("memory: sqlite store load: decode: %w", err)
	}
	return snap, nil
}

// Delete removes all rows at key (across kinds). Missing rows are
// not an error. Mirrors (*FilesystemStore).Delete semantics from
// persistence.go:289-300.
func (s *SQLiteStore) Delete(ctx context.Context, key string) error {
	sk := sanitizeSQLiteKey(key)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_snapshots WHERE key = ?`, sk); err != nil {
		return fmt.Errorf("memory: sqlite store delete: %w", err)
	}
	return nil
}

// List returns the sorted set of unique keys present in the store.
// Mirrors (*FilesystemStore).List semantics from persistence.go:305-333.
func (s *SQLiteStore) List(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT key FROM memory_snapshots ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("memory: sqlite store list: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("memory: sqlite store list scan: %w", err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: sqlite store list iter: %w", err)
	}
	return out, nil
}

// Compile-time check: SQLiteStore satisfies coremem.SnapshotStore. If
// upstream renames or restructures SnapshotStore, this line will fail
// to compile — a deliberate early-warning signal that complements the
// runtime assertion in sqlite_store_test.go.
var _ coremem.SnapshotStore = (*SQLiteStore)(nil)
