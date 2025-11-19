package postgres

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LockManager provides distributed locks using PostgreSQL advisory locks
type LockManager struct {
	pool *pgxpool.Pool
}

func NewLockManager(pool *pgxpool.Pool) *LockManager { return &LockManager{pool: pool} }

// hashKey converts a string key to a uint32 for advisory locks
func hashKey(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// Acquire obtains an exclusive advisory lock. Blocks until acquired.
func (l *LockManager) Acquire(ctx context.Context, key string) (func(context.Context) error, error) {
	k := hashKey(key)
	if _, err := l.pool.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(k)); err != nil {
		return nil, fmt.Errorf("failed to acquire lock %s: %w", key, err)
	}
	return func(c context.Context) error {
		if _, err := l.pool.Exec(c, "SELECT pg_advisory_unlock($1)", int64(k)); err != nil {
			return fmt.Errorf("failed to release lock %s: %w", key, err)
		}
		return nil
	}, nil
}

// TryAcquire tries to obtain lock without blocking.
func (l *LockManager) TryAcquire(ctx context.Context, key string) (bool, func(context.Context) error, error) {
	k := hashKey(key)
	var ok bool
	if err := l.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", int64(k)).Scan(&ok); err != nil {
		return false, nil, fmt.Errorf("failed to try lock %s: %w", key, err)
	}
	if !ok {
		return false, nil, nil
	}
	return true, func(c context.Context) error {
		if _, err := l.pool.Exec(c, "SELECT pg_advisory_unlock($1)", int64(k)); err != nil {
			return fmt.Errorf("failed to release lock %s: %w", key, err)
		}
		return nil
	}, nil
}
