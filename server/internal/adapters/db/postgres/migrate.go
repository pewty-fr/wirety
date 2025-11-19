package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RunMigrations applies *.sql files in dir in numeric order (prefix before first underscore) using schema_migrations table.
func RunMigrations(ctx context.Context, db *sql.DB, dir string) error {
	// advisory lock to avoid concurrent migration (lock key 42)
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock(42)`); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer db.ExecContext(ctx, `SELECT pg_advisory_unlock(42)`)

	applied := map[int]bool{}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err == nil { // if table doesn't exist that's okay (first migration creates it)
		defer rows.Close()
		for rows.Next() {
			var v int
			if err = rows.Scan(&v); err != nil {
				return err
			}
			applied[v] = true
		}
	}

	// gather files
	entries := []string{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".sql") {
			entries = append(entries, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i] < entries[j] })

	for _, file := range entries {
		base := filepath.Base(file)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) == 0 {
			continue
		}
		version, err := strconv.Atoi(strings.TrimLeft(parts[0], "0"))
		if err != nil {
			continue
		}
		if applied[version] {
			continue
		}
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", base, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		if _, err = tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec migration %s: %w", base, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", base, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", base, err)
		}
	}
	return nil
}
